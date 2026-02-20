#!/usr/bin/env python3
"""REINFORCE policy training for the Diplomacy self-play RL loop.

Trains the GAT policy network using REINFORCE on self-play data with:
  - Advantage-weighted policy gradient (reward - baseline)
  - Entropy regularization to prevent policy collapse
  - Optional KL regularization against a frozen supervised checkpoint
  - Optional mixed training with supervised cross-entropy data

Usage:
    python3 train_policy_rl.py \
      --selfplay-data data/processed/train.npz \
      --val-data data/processed/val.npz \
      --supervised-data data/supervised/train.npz \
      --supervised-mix 0.3 \
      --supervised-checkpoint checkpoints/best_policy.pt \
      --kl-coeff 0.1 \
      --entropy-coeff 0.01 \
      --init-checkpoint checkpoints/best_policy.pt \
      --epochs 20 \
      --checkpoint-dir checkpoints/rl/

Supports MPS (Apple Silicon), CUDA, and CPU backends.
"""

import argparse
import json
import logging
import math
import sys
import time
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.data import DataLoader, Dataset

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from gnn import DiplomacyPolicyNet

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

CHECKPOINT_DIR = Path(__file__).resolve().parent.parent / "checkpoints" / "rl"

NUM_AREAS = 81
NUM_FEATURES = 47
NUM_POWERS = 7
ORDER_TYPES = 7
ORDER_VOCAB_SIZE = ORDER_TYPES + NUM_AREAS + NUM_AREAS  # 169


class SelfPlayDataset(Dataset):
    """Dataset for self-play NPZ files with reward labels.

    Each sample contains:
      - board: [81, 47] board state tensor
      - order_labels: [max_orders, 169] one-hot order vectors
      - order_masks: [max_orders] binary mask for valid orders
      - power_index: int, active power
      - reward: float, per-phase outcome reward
    """

    def __init__(self, npz_path: Path):
        log.info("Loading self-play dataset from %s", npz_path)
        data = np.load(npz_path)
        self.boards = data["boards"]
        self.order_labels = data["order_labels"]
        self.order_masks = data["order_masks"]
        self.power_indices = data["power_indices"]
        self.rewards = data["rewards"]
        self.n_samples = self.boards.shape[0]
        self.max_orders = self.order_labels.shape[1]
        log.info("  %d samples, max_orders=%d", self.n_samples, self.max_orders)

    def __len__(self) -> int:
        return self.n_samples

    def __getitem__(self, idx: int) -> dict:
        board = torch.from_numpy(self.boards[idx])
        order_labels = torch.from_numpy(self.order_labels[idx])
        order_mask = torch.from_numpy(self.order_masks[idx])
        power_idx = int(self.power_indices[idx])
        reward = float(self.rewards[idx])

        # Extract unit source province indices from order labels
        src_section = order_labels[:, ORDER_TYPES:ORDER_TYPES + NUM_AREAS]
        unit_indices = src_section.argmax(dim=-1)
        has_src = src_section.sum(dim=-1) > 0
        unit_indices[~has_src] = -1

        return {
            "board": board,
            "order_labels": order_labels,
            "order_mask": order_mask,
            "power_idx": power_idx,
            "unit_indices": unit_indices,
            "reward": reward,
        }


class SupervisedDataset(Dataset):
    """Dataset for supervised NPZ files (cross-entropy training)."""

    def __init__(self, npz_path: Path):
        log.info("Loading supervised dataset from %s", npz_path)
        data = np.load(npz_path)
        self.boards = data["boards"]
        self.order_labels = data["order_labels"]
        self.order_masks = data["order_masks"]
        self.power_indices = data["power_indices"]
        self.n_samples = self.boards.shape[0]
        self.max_orders = self.order_labels.shape[1]
        log.info("  %d samples, max_orders=%d", self.n_samples, self.max_orders)

    def __len__(self) -> int:
        return self.n_samples

    def __getitem__(self, idx: int) -> dict:
        board = torch.from_numpy(self.boards[idx])
        order_labels = torch.from_numpy(self.order_labels[idx])
        order_mask = torch.from_numpy(self.order_masks[idx])
        power_idx = int(self.power_indices[idx])

        src_section = order_labels[:, ORDER_TYPES:ORDER_TYPES + NUM_AREAS]
        unit_indices = src_section.argmax(dim=-1)
        has_src = src_section.sum(dim=-1) > 0
        unit_indices[~has_src] = -1

        return {
            "board": board,
            "order_labels": order_labels,
            "order_mask": order_mask,
            "power_idx": power_idx,
            "unit_indices": unit_indices,
        }


def collate_selfplay(batch: list[dict]) -> dict:
    """Collate self-play samples with reward field."""
    max_orders = max(b["order_labels"].shape[0] for b in batch)
    B = len(batch)

    boards = torch.stack([b["board"] for b in batch])
    power_indices = torch.tensor([b["power_idx"] for b in batch], dtype=torch.long)
    rewards = torch.tensor([b["reward"] for b in batch], dtype=torch.float32)

    order_labels = torch.zeros(B, max_orders, ORDER_VOCAB_SIZE)
    order_masks = torch.zeros(B, max_orders)
    unit_indices = torch.full((B, max_orders), -1, dtype=torch.long)

    for i, b in enumerate(batch):
        n = b["order_labels"].shape[0]
        order_labels[i, :n] = b["order_labels"]
        order_masks[i, :n] = b["order_mask"]
        unit_indices[i, :n] = b["unit_indices"]

    return {
        "board": boards,
        "order_labels": order_labels,
        "order_mask": order_masks,
        "power_idx": power_indices,
        "unit_indices": unit_indices,
        "reward": rewards,
    }


def collate_supervised(batch: list[dict]) -> dict:
    """Collate supervised samples (no reward field)."""
    max_orders = max(b["order_labels"].shape[0] for b in batch)
    B = len(batch)

    boards = torch.stack([b["board"] for b in batch])
    power_indices = torch.tensor([b["power_idx"] for b in batch], dtype=torch.long)

    order_labels = torch.zeros(B, max_orders, ORDER_VOCAB_SIZE)
    order_masks = torch.zeros(B, max_orders)
    unit_indices = torch.full((B, max_orders), -1, dtype=torch.long)

    for i, b in enumerate(batch):
        n = b["order_labels"].shape[0]
        order_labels[i, :n] = b["order_labels"]
        order_masks[i, :n] = b["order_mask"]
        unit_indices[i, :n] = b["unit_indices"]

    return {
        "board": boards,
        "order_labels": order_labels,
        "order_mask": order_masks,
        "power_idx": power_indices,
        "unit_indices": unit_indices,
    }


def compute_reinforce_loss(
    logits: torch.Tensor,
    order_labels: torch.Tensor,
    order_mask: torch.Tensor,
    rewards: torch.Tensor,
) -> tuple[torch.Tensor, torch.Tensor]:
    """Compute REINFORCE policy gradient loss with mean-reward baseline.

    Args:
        logits: [B, M, V] model predictions
        order_labels: [B, M, V] one-hot target vectors (actions taken)
        order_mask: [B, M] binary mask for valid orders
        rewards: [B] per-sample reward signal

    Returns:
        (policy_loss, mean_entropy) tuple
    """
    B, M, V = logits.shape

    log_probs = F.log_softmax(logits, dim=-1)  # [B, M, V]
    probs = F.softmax(logits, dim=-1)           # [B, M, V]

    # Compute log-prob of the taken action: sum over one-hot positions
    # order_labels is one-hot-like, so this extracts log pi(a|s)
    target_sum = order_labels.sum(dim=-1, keepdim=True).clamp(min=1e-8)
    target_probs = order_labels / target_sum
    action_log_probs = (log_probs * target_probs).sum(dim=-1)  # [B, M]

    # Mask invalid orders and average per sample
    masked_log_probs = action_log_probs * order_mask  # [B, M]
    orders_per_sample = order_mask.sum(dim=-1).clamp(min=1.0)  # [B]
    sample_log_probs = masked_log_probs.sum(dim=-1) / orders_per_sample  # [B]

    # Advantage: reward - batch mean baseline
    baseline = rewards.mean()
    advantage = rewards - baseline  # [B]

    # REINFORCE: loss = -log(pi(a|s)) * advantage
    policy_loss = -(sample_log_probs * advantage.detach()).mean()

    # Entropy: H(pi) = -sum(p * log p) averaged over valid orders
    entropy_per_pos = -(probs * log_probs).sum(dim=-1)  # [B, M]
    masked_entropy = (entropy_per_pos * order_mask).sum(dim=-1) / orders_per_sample  # [B]
    mean_entropy = masked_entropy.mean()

    return policy_loss, mean_entropy


def compute_kl_divergence(
    current_logits: torch.Tensor,
    ref_logits: torch.Tensor,
    order_mask: torch.Tensor,
) -> torch.Tensor:
    """Compute KL(pi_ref || pi_current) averaged over valid order positions.

    Args:
        current_logits: [B, M, V] current policy logits
        ref_logits: [B, M, V] reference (frozen) policy logits
        order_mask: [B, M] binary mask for valid orders

    Returns:
        Scalar KL divergence
    """
    B, M, V = current_logits.shape

    ref_probs = F.softmax(ref_logits, dim=-1)
    current_log_probs = F.log_softmax(current_logits, dim=-1)
    ref_log_probs = F.log_softmax(ref_logits, dim=-1)

    # KL(ref || current) = sum ref * (log ref - log current)
    kl_per_pos = (ref_probs * (ref_log_probs - current_log_probs)).sum(dim=-1)  # [B, M]

    masked_kl = kl_per_pos * order_mask
    num_valid = order_mask.sum().clamp(min=1.0)
    return masked_kl.sum() / num_valid


def compute_supervised_loss(
    logits: torch.Tensor,
    order_labels: torch.Tensor,
    order_mask: torch.Tensor,
) -> torch.Tensor:
    """Standard cross-entropy loss for supervised examples (same as train_policy.py)."""
    B, M, V = logits.shape

    logits_flat = logits.reshape(B * M, V)
    targets_flat = order_labels.reshape(B * M, V)
    mask_flat = order_mask.reshape(B * M)

    log_probs = F.log_softmax(logits_flat, dim=-1)
    target_sum = targets_flat.sum(dim=-1, keepdim=True).clamp(min=1e-8)
    target_probs = targets_flat / target_sum

    kl = F.kl_div(log_probs, target_probs, reduction="none").sum(dim=-1)
    masked_kl = kl * mask_flat
    num_valid = mask_flat.sum().clamp(min=1.0)
    return masked_kl.sum() / num_valid


def get_device() -> torch.device:
    """Select the best available device (MPS > CUDA > CPU)."""
    if torch.backends.mps.is_available():
        return torch.device("mps")
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


def get_lr_scheduler(optimizer, warmup_steps: int, total_steps: int):
    """Cosine decay with linear warmup."""

    def lr_lambda(step):
        if step < warmup_steps:
            return step / max(warmup_steps, 1)
        progress = (step - warmup_steps) / max(total_steps - warmup_steps, 1)
        return 0.5 * (1.0 + math.cos(math.pi * progress))

    return torch.optim.lr_scheduler.LambdaLR(optimizer, lr_lambda)


def build_model(args, device: torch.device) -> DiplomacyPolicyNet:
    """Build the policy model, optionally loading from a checkpoint."""
    model = DiplomacyPolicyNet(
        num_areas=NUM_AREAS,
        num_features=NUM_FEATURES,
        hidden_dim=args.hidden_dim,
        num_gat_layers=args.num_layers,
        num_heads=args.num_heads,
        order_vocab_size=ORDER_VOCAB_SIZE,
        num_powers=NUM_POWERS,
        dropout=args.dropout,
    ).to(device)

    if args.init_checkpoint:
        log.info("Initializing model from checkpoint: %s", args.init_checkpoint)
        ckpt = torch.load(args.init_checkpoint, map_location=device, weights_only=True)
        model.load_state_dict(ckpt["model_state_dict"])
        log.info("Loaded model weights from checkpoint")

    return model


def build_reference_model(args, device: torch.device) -> DiplomacyPolicyNet | None:
    """Build a frozen reference model for KL regularization."""
    if not args.supervised_checkpoint:
        return None

    log.info("Loading reference model for KL from: %s", args.supervised_checkpoint)
    ref_model = DiplomacyPolicyNet(
        num_areas=NUM_AREAS,
        num_features=NUM_FEATURES,
        hidden_dim=args.hidden_dim,
        num_gat_layers=args.num_layers,
        num_heads=args.num_heads,
        order_vocab_size=ORDER_VOCAB_SIZE,
        num_powers=NUM_POWERS,
        dropout=args.dropout,
    ).to(device)

    ckpt = torch.load(args.supervised_checkpoint, map_location=device, weights_only=True)
    ref_model.load_state_dict(ckpt["model_state_dict"])

    # Freeze all parameters
    for param in ref_model.parameters():
        param.requires_grad = False
    ref_model.eval()

    log.info("Reference model loaded and frozen")
    return ref_model


def evaluate(
    model: DiplomacyPolicyNet,
    dataloader: DataLoader,
    adj: torch.Tensor,
    device: torch.device,
) -> dict:
    """Run evaluation on a validation set."""
    model.eval()
    total_loss = 0.0
    total_entropy = 0.0
    num_batches = 0

    with torch.no_grad():
        for batch in dataloader:
            board = batch["board"].to(device)
            order_labels = batch["order_labels"].to(device)
            order_mask = batch["order_mask"].to(device)
            power_idx = batch["power_idx"].to(device)
            unit_indices = batch["unit_indices"].to(device)
            rewards = batch["reward"].to(device)

            logits = model(board, adj, unit_indices, power_idx)
            policy_loss, entropy = compute_reinforce_loss(
                logits, order_labels, order_mask, rewards,
            )
            total_loss += policy_loss.item()
            total_entropy += entropy.item()
            num_batches += 1

    n = max(num_batches, 1)
    return {
        "loss": total_loss / n,
        "entropy": total_entropy / n,
    }


def make_mixed_batch(
    selfplay_iter,
    supervised_iter,
    selfplay_loader: DataLoader,
    supervised_loader: DataLoader,
    supervised_mix: float,
    device: torch.device,
) -> tuple[dict | None, dict | None, object, object]:
    """Fetch one batch from each data source, cycling iterators as needed.

    Returns (selfplay_batch, supervised_batch, selfplay_iter, supervised_iter).
    supervised_batch may be None if no supervised data is configured.
    """
    # Get self-play batch
    try:
        sp_batch = next(selfplay_iter)
    except StopIteration:
        selfplay_iter = iter(selfplay_loader)
        sp_batch = next(selfplay_iter)

    # Get supervised batch if configured
    sup_batch = None
    if supervised_loader is not None and supervised_mix > 0:
        try:
            sup_batch = next(supervised_iter)
        except StopIteration:
            supervised_iter = iter(supervised_loader)
            sup_batch = next(supervised_iter)

    return sp_batch, sup_batch, selfplay_iter, supervised_iter


def train(args):
    """Main RL training loop."""
    device = get_device()
    log.info("Using device: %s", device)

    # Load adjacency matrix
    adj_path = Path(args.adj_path)
    if not adj_path.exists():
        log.error("Adjacency matrix not found: %s", adj_path)
        sys.exit(1)
    adj_np = np.load(adj_path)
    adj = torch.from_numpy(adj_np).to(device)

    # Load self-play dataset
    sp_ds = SelfPlayDataset(Path(args.selfplay_data))
    sp_loader = DataLoader(
        sp_ds,
        batch_size=args.batch_size,
        shuffle=True,
        num_workers=0,
        collate_fn=collate_selfplay,
        drop_last=True,
    )

    # Load optional supervised dataset
    sup_loader = None
    if args.supervised_data:
        sup_ds = SupervisedDataset(Path(args.supervised_data))
        sup_batch_size = max(1, int(args.batch_size * args.supervised_mix / (1 - args.supervised_mix + 1e-8)))
        sup_loader = DataLoader(
            sup_ds,
            batch_size=sup_batch_size,
            shuffle=True,
            num_workers=0,
            collate_fn=collate_supervised,
            drop_last=True,
        )
        log.info("Supervised mix: %.0f%% (%d samples, batch_size=%d)",
                 args.supervised_mix * 100, len(sup_ds), sup_batch_size)

    # Load validation dataset
    val_loader = None
    if args.val_data:
        val_ds = SelfPlayDataset(Path(args.val_data))
        val_loader = DataLoader(
            val_ds,
            batch_size=args.batch_size,
            shuffle=False,
            num_workers=0,
            collate_fn=collate_selfplay,
        )

    # Build model
    model = build_model(args, device)
    num_params = model.count_parameters()
    log.info("Model parameters: %s (%.2fM)", f"{num_params:,}", num_params / 1e6)

    # Build frozen reference model for KL regularization
    ref_model = build_reference_model(args, device)

    # Optimizer
    optimizer = torch.optim.AdamW(
        model.parameters(),
        lr=args.lr,
        weight_decay=args.weight_decay,
    )

    # Resume from checkpoint
    start_epoch = 1
    global_step = 0
    if args.resume:
        resume_path = Path(args.checkpoint_dir) / "latest_rl.pt"
        if resume_path.exists():
            log.info("Resuming from checkpoint: %s", resume_path)
            ckpt = torch.load(resume_path, map_location=device, weights_only=True)
            model.load_state_dict(ckpt["model_state_dict"])
            optimizer.load_state_dict(ckpt["optimizer_state_dict"])
            start_epoch = ckpt.get("epoch", 0) + 1
            global_step = ckpt.get("global_step", 0)
            log.info("Resumed from epoch %d, step %d", start_epoch - 1, global_step)
        else:
            log.warning("No checkpoint found at %s, starting fresh", resume_path)

    # LR scheduler
    steps_per_epoch = len(sp_loader)
    total_steps = steps_per_epoch * args.epochs
    warmup_steps = min(steps_per_epoch * 2, total_steps // 10)
    scheduler = get_lr_scheduler(optimizer, warmup_steps, total_steps)
    # Advance scheduler to match resumed step
    for _ in range(global_step):
        scheduler.step()

    # Checkpoint directory
    ckpt_dir = Path(args.checkpoint_dir)
    ckpt_dir.mkdir(parents=True, exist_ok=True)

    # Training log
    history = []
    best_val_loss = float("inf")
    best_epoch = 0

    log.info(
        "Starting RL training: epochs=%d, steps/epoch=%d, total_steps=%d, "
        "entropy_coeff=%.4f, kl_coeff=%.4f",
        args.epochs, steps_per_epoch, total_steps,
        args.entropy_coeff, args.kl_coeff,
    )

    for epoch in range(start_epoch, args.epochs + 1):
        model.train()
        epoch_policy_loss = 0.0
        epoch_entropy = 0.0
        epoch_kl = 0.0
        epoch_sup_loss = 0.0
        epoch_total_loss = 0.0
        epoch_batches = 0
        epoch_start = time.time()

        sp_iter = iter(sp_loader)
        sup_iter = iter(sup_loader) if sup_loader is not None else None

        for batch_idx in range(steps_per_epoch):
            # Fetch batches
            sp_batch, sup_batch, sp_iter, sup_iter = make_mixed_batch(
                sp_iter, sup_iter, sp_loader, sup_loader,
                args.supervised_mix, device,
            )

            # Move self-play batch to device
            board = sp_batch["board"].to(device)
            order_labels = sp_batch["order_labels"].to(device)
            order_mask = sp_batch["order_mask"].to(device)
            power_idx = sp_batch["power_idx"].to(device)
            unit_indices = sp_batch["unit_indices"].to(device)
            rewards = sp_batch["reward"].to(device)

            optimizer.zero_grad()

            # Forward pass
            logits = model(board, adj, unit_indices, power_idx)

            # REINFORCE loss
            policy_loss, entropy = compute_reinforce_loss(
                logits, order_labels, order_mask, rewards,
            )

            total_loss = policy_loss - args.entropy_coeff * entropy

            # KL regularization against reference model
            kl_value = 0.0
            if ref_model is not None and args.kl_coeff > 0:
                with torch.no_grad():
                    ref_logits = ref_model(board, adj, unit_indices, power_idx)
                kl = compute_kl_divergence(logits, ref_logits, order_mask)
                total_loss = total_loss + args.kl_coeff * kl
                kl_value = kl.item()

            # Supervised loss on mixed data
            sup_loss_value = 0.0
            if sup_batch is not None:
                sup_board = sup_batch["board"].to(device)
                sup_labels = sup_batch["order_labels"].to(device)
                sup_mask = sup_batch["order_mask"].to(device)
                sup_power = sup_batch["power_idx"].to(device)
                sup_units = sup_batch["unit_indices"].to(device)

                sup_logits = model(sup_board, adj, sup_units, sup_power)
                sup_loss = compute_supervised_loss(sup_logits, sup_labels, sup_mask)
                total_loss = total_loss + args.supervised_mix * sup_loss
                sup_loss_value = sup_loss.item()

            total_loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), args.grad_clip)
            optimizer.step()
            scheduler.step()
            global_step += 1

            epoch_policy_loss += policy_loss.item()
            epoch_entropy += entropy.item()
            epoch_kl += kl_value
            epoch_sup_loss += sup_loss_value
            epoch_total_loss += total_loss.item()
            epoch_batches += 1

            if (batch_idx + 1) % args.log_interval == 0:
                n = epoch_batches
                lr = scheduler.get_last_lr()[0]
                log.info(
                    "  Epoch %d [%d/%d] total=%.4f policy=%.4f entropy=%.3f "
                    "kl=%.4f sup=%.4f lr=%.2e",
                    epoch, batch_idx + 1, steps_per_epoch,
                    epoch_total_loss / n, epoch_policy_loss / n,
                    epoch_entropy / n, epoch_kl / n,
                    epoch_sup_loss / n, lr,
                )

        # Epoch stats
        epoch_time = time.time() - epoch_start
        n = max(epoch_batches, 1)
        train_metrics = {
            "total_loss": epoch_total_loss / n,
            "policy_loss": epoch_policy_loss / n,
            "entropy": epoch_entropy / n,
            "kl": epoch_kl / n,
            "sup_loss": epoch_sup_loss / n,
        }

        # Validation
        val_metrics = {"loss": 0.0, "entropy": 0.0}
        if val_loader is not None:
            val_metrics = evaluate(model, val_loader, adj, device)

        log.info(
            "Epoch %d/%d (%.1fs): total=%.4f policy=%.4f entropy=%.3f kl=%.4f "
            "sup=%.4f | val_loss=%.4f val_entropy=%.3f",
            epoch, args.epochs, epoch_time,
            train_metrics["total_loss"], train_metrics["policy_loss"],
            train_metrics["entropy"], train_metrics["kl"],
            train_metrics["sup_loss"],
            val_metrics["loss"], val_metrics["entropy"],
        )

        epoch_record = {
            "epoch": epoch,
            "train_total_loss": train_metrics["total_loss"],
            "train_policy_loss": train_metrics["policy_loss"],
            "train_entropy": train_metrics["entropy"],
            "train_kl": train_metrics["kl"],
            "train_sup_loss": train_metrics["sup_loss"],
            "val_loss": val_metrics["loss"],
            "val_entropy": val_metrics["entropy"],
            "lr": scheduler.get_last_lr()[0],
            "time_s": epoch_time,
        }
        history.append(epoch_record)

        # Checkpoint on best validation loss (or train loss if no val data)
        check_loss = val_metrics["loss"] if val_loader is not None else train_metrics["total_loss"]
        if check_loss < best_val_loss:
            best_val_loss = check_loss
            best_epoch = epoch
            ckpt_path = ckpt_dir / "best_rl_policy.pt"
            torch.save({
                "epoch": epoch,
                "global_step": global_step,
                "model_state_dict": model.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "val_loss": val_metrics["loss"],
                "val_entropy": val_metrics["entropy"],
                "train_entropy": train_metrics["entropy"],
                "args": vars(args),
            }, ckpt_path)
            log.info("  Saved best checkpoint (loss=%.4f) to %s", best_val_loss, ckpt_path)

        # Save latest checkpoint for resume
        latest_path = ckpt_dir / "latest_rl.pt"
        torch.save({
            "epoch": epoch,
            "global_step": global_step,
            "model_state_dict": model.state_dict(),
            "optimizer_state_dict": optimizer.state_dict(),
            "val_loss": val_metrics["loss"],
            "args": vars(args),
        }, latest_path)

        # Periodic checkpoint
        if epoch % args.save_every == 0:
            ckpt_path = ckpt_dir / f"rl_policy_epoch{epoch:03d}.pt"
            torch.save({
                "epoch": epoch,
                "global_step": global_step,
                "model_state_dict": model.state_dict(),
                "val_loss": val_metrics["loss"],
            }, ckpt_path)

    # Save final model
    final_path = ckpt_dir / "final_rl_policy.pt"
    torch.save({
        "epoch": args.epochs,
        "global_step": global_step,
        "model_state_dict": model.state_dict(),
        "val_loss": val_metrics["loss"],
        "val_entropy": val_metrics["entropy"],
        "args": vars(args),
    }, final_path)
    log.info("Saved final model to %s", final_path)

    # Save training history
    history_path = ckpt_dir / "rl_training_history.json"
    with open(history_path, "w") as f:
        json.dump(history, f, indent=2)
    log.info("Saved training history to %s", history_path)

    # Summary
    best = history[best_epoch - start_epoch] if best_epoch >= start_epoch else history[-1]
    print(f"\n{'=' * 60}")
    print("RL Policy Training Complete")
    print(f"{'=' * 60}")
    print(f"Best epoch:       {best_epoch}")
    print(f"Best loss:        {best_val_loss:.4f}")
    print(f"Best entropy:     {best['train_entropy']:.3f}")
    print(f"Best KL:          {best['train_kl']:.4f}")
    print(f"Entropy coeff:    {args.entropy_coeff}")
    print(f"KL coeff:         {args.kl_coeff}")
    print(f"Supervised mix:   {args.supervised_mix}")
    print(f"Model params:     {num_params:,} ({num_params / 1e6:.2f}M)")
    print(f"Checkpoint dir:   {ckpt_dir}")
    print(f"{'=' * 60}")


def main():
    parser = argparse.ArgumentParser(
        description="REINFORCE policy training for Diplomacy self-play RL"
    )

    # Data paths
    parser.add_argument(
        "--selfplay-data", type=str, required=True,
        help="Path to self-play NPZ training data",
    )
    parser.add_argument(
        "--val-data", type=str, default="",
        help="Path to validation NPZ data",
    )
    parser.add_argument(
        "--adj-path", type=str,
        default=str(Path(__file__).resolve().parent.parent / "processed" / "features" / "adjacency.npy"),
        help="Path to adjacency.npy matrix",
    )

    # Supervised mixing
    parser.add_argument(
        "--supervised-data", type=str, default="",
        help="Path to supervised NPZ data for mixed training",
    )
    parser.add_argument(
        "--supervised-mix", type=float, default=0.3,
        help="Fraction of each batch devoted to supervised cross-entropy (default: 0.3)",
    )

    # Checkpoint paths
    parser.add_argument(
        "--init-checkpoint", type=str, default="",
        help="Initialize model weights from this checkpoint",
    )
    parser.add_argument(
        "--supervised-checkpoint", type=str, default="",
        help="Frozen supervised checkpoint for KL regularization",
    )
    parser.add_argument(
        "--checkpoint-dir", type=str, default=str(CHECKPOINT_DIR),
        help="Directory for model checkpoints",
    )
    parser.add_argument(
        "--resume", action="store_true",
        help="Resume training from latest checkpoint in checkpoint-dir",
    )

    # RL hyperparameters
    parser.add_argument("--entropy-coeff", type=float, default=0.01,
                        help="Entropy regularization coefficient (default: 0.01)")
    parser.add_argument("--kl-coeff", type=float, default=0.1,
                        help="KL regularization coefficient against reference (default: 0.1)")

    # Training hyperparameters
    parser.add_argument("--epochs", type=int, default=20, help="Number of training epochs")
    parser.add_argument("--batch-size", type=int, default=64, help="Training batch size")
    parser.add_argument("--lr", type=float, default=3e-5, help="Peak learning rate")
    parser.add_argument("--weight-decay", type=float, default=0.01, help="AdamW weight decay")
    parser.add_argument("--grad-clip", type=float, default=1.0, help="Gradient norm clipping")

    # Architecture (must match init/reference checkpoints)
    parser.add_argument("--hidden-dim", type=int, default=512, help="GNN hidden dimension")
    parser.add_argument("--num-layers", type=int, default=6, help="Number of GAT layers")
    parser.add_argument("--num-heads", type=int, default=8, help="Number of attention heads")
    parser.add_argument("--dropout", type=float, default=0.15, help="Dropout rate")

    # Logging
    parser.add_argument("--log-interval", type=int, default=50, help="Log every N batches")
    parser.add_argument("--save-every", type=int, default=5, help="Save periodic checkpoint every N epochs")

    args = parser.parse_args()
    train(args)


if __name__ == "__main__":
    main()
