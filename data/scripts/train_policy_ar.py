#!/usr/bin/env python3
"""Training script for the autoregressive Diplomacy policy network.

Uses teacher forcing during training: at each decoder step, the model
receives the ground truth previous order (not its own prediction).
Loss is the sum of per-unit cross-entropy over the sequence.

Usage:
    python3 scripts/train_policy_ar.py [--data-dir processed/features] [--epochs 50]

    # Warm-start encoder from pretrained independent model
    python3 scripts/train_policy_ar.py --pretrained-encoder checkpoints/best_policy.pt

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
from autoregressive_decoder import DiplomacyAutoRegressivePolicyNet
from gnn import DiplomacyPolicyNet

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

PROCESSED_DIR = Path(__file__).resolve().parent.parent / "processed" / "features"
CHECKPOINT_DIR = Path(__file__).resolve().parent.parent / "checkpoints"

NUM_AREAS = 81
NUM_FEATURES = 47
NUM_POWERS = 7
ORDER_TYPES = 7
ORDER_VOCAB_SIZE = ORDER_TYPES + NUM_AREAS + NUM_AREAS  # 169


class DiplomacyDataset(Dataset):
    """PyTorch dataset wrapping extracted .npz feature files.

    Same format as train_policy.py, but also provides the full order
    sequence for teacher forcing.
    """

    def __init__(self, npz_path: Path):
        log.info("Loading dataset from %s", npz_path)
        data = np.load(npz_path)
        self.boards = data["boards"]
        self.order_labels = data["order_labels"]
        self.order_masks = data["order_masks"]
        self.power_indices = data["power_indices"]
        self.values = data["values"]

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
        }


def collate_fn(batch: list[dict]) -> dict:
    """Custom collate with padding to max sequence length in batch."""
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


def compute_loss(
    logits: torch.Tensor,
    order_labels: torch.Tensor,
    order_mask: torch.Tensor,
) -> torch.Tensor:
    """Compute masked cross-entropy loss over order predictions.

    Uses KL divergence against soft targets (matching train_policy.py).
    """
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


def compute_accuracy(
    logits: torch.Tensor,
    order_labels: torch.Tensor,
    order_mask: torch.Tensor,
    top_k: int = 1,
) -> float:
    """Compute top-k accuracy of order predictions."""
    B, M, V = logits.shape
    targets = order_labels.argmax(dim=-1)

    if top_k == 1:
        preds = logits.argmax(dim=-1)
        correct = (preds == targets).float()
    else:
        _, top_indices = logits.topk(top_k, dim=-1)
        targets_exp = targets.unsqueeze(-1).expand_as(top_indices)
        correct = (top_indices == targets_exp).any(dim=-1).float()

    masked_correct = correct * order_mask
    num_valid = order_mask.sum().clamp(min=1.0)
    return (masked_correct.sum() / num_valid).item()


def get_device() -> torch.device:
    """Select the best available device."""
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


def evaluate(
    model: DiplomacyAutoRegressivePolicyNet,
    dataloader: DataLoader,
    adj: torch.Tensor,
    device: torch.device,
) -> dict:
    """Run evaluation on a dataset split."""
    model.eval()
    total_loss = 0.0
    total_acc1 = 0.0
    total_acc5 = 0.0
    num_batches = 0

    with torch.no_grad():
        for batch in dataloader:
            board = batch["board"].to(device)
            order_labels = batch["order_labels"].to(device)
            order_mask = batch["order_mask"].to(device)
            power_idx = batch["power_idx"].to(device)
            unit_indices = batch["unit_indices"].to(device)

            # Teacher forcing for evaluation (consistent with training)
            logits = model(board, adj, unit_indices, power_idx, target_orders=order_labels)
            loss = compute_loss(logits, order_labels, order_mask)
            acc1 = compute_accuracy(logits, order_labels, order_mask, top_k=1)
            acc5 = compute_accuracy(logits, order_labels, order_mask, top_k=5)

            total_loss += loss.item()
            total_acc1 += acc1
            total_acc5 += acc5
            num_batches += 1

    n = max(num_batches, 1)
    return {
        "loss": total_loss / n,
        "acc_top1": total_acc1 / n,
        "acc_top5": total_acc5 / n,
    }


def train(args):
    """Main training loop."""
    device = get_device()
    log.info("Using device: %s", device)

    # Load adjacency matrix
    adj_path = Path(args.data_dir) / "adjacency.npy"
    if not adj_path.exists():
        log.error("Adjacency matrix not found: %s. Run features.py first.", adj_path)
        sys.exit(1)
    adj_np = np.load(adj_path)
    adj = torch.from_numpy(adj_np).to(device)

    # Load datasets
    train_ds = DiplomacyDataset(Path(args.data_dir) / "train.npz")
    val_ds = DiplomacyDataset(Path(args.data_dir) / "val.npz")

    train_loader = DataLoader(
        train_ds,
        batch_size=args.batch_size,
        shuffle=True,
        num_workers=0,
        collate_fn=collate_fn,
        drop_last=True,
    )
    val_loader = DataLoader(
        val_ds,
        batch_size=args.batch_size,
        shuffle=False,
        num_workers=0,
        collate_fn=collate_fn,
    )

    # Build model
    model = DiplomacyAutoRegressivePolicyNet(
        num_areas=NUM_AREAS,
        num_features=NUM_FEATURES,
        hidden_dim=args.hidden_dim,
        num_gat_layers=args.num_layers,
        num_heads=args.num_heads,
        decoder_dim=args.decoder_dim,
        decoder_layers=args.decoder_layers,
        decoder_heads=args.decoder_heads,
        num_powers=NUM_POWERS,
        order_vocab_size=ORDER_VOCAB_SIZE,
        dropout=args.dropout,
    ).to(device)

    # Warm-start encoder from pretrained independent model
    if args.pretrained_encoder:
        log.info("Loading pretrained encoder from %s", args.pretrained_encoder)
        ckpt = torch.load(args.pretrained_encoder, weights_only=True, map_location="cpu")
        state = ckpt.get("model_state_dict", ckpt)
        pretrained = DiplomacyPolicyNet(
            hidden_dim=args.hidden_dim,
            num_gat_layers=args.num_layers,
            num_heads=args.num_heads,
        )
        pretrained.load_state_dict(state)
        model.load_encoder_from_policy(pretrained)
        log.info("Encoder weights loaded from pretrained model")
        del pretrained

    num_params = model.count_parameters()
    enc_params = model.count_encoder_parameters()
    dec_params = model.count_decoder_parameters()
    log.info(
        "Model parameters: %s (%.2fM) | encoder: %s | decoder: %s",
        f"{num_params:,}", num_params / 1e6,
        f"{enc_params:,}", f"{dec_params:,}",
    )

    # Optimizer with different LR for encoder vs decoder
    encoder_params = (
        list(model.input_proj.parameters())
        + list(model.gat_blocks.parameters())
        + list(model.power_embed.parameters())
    )
    decoder_params = list(model.decoder.parameters())

    if args.pretrained_encoder:
        # Lower LR for pretrained encoder, higher for new decoder
        optimizer = torch.optim.AdamW([
            {"params": encoder_params, "lr": args.lr * 0.1},
            {"params": decoder_params, "lr": args.lr},
        ], weight_decay=args.weight_decay)
    else:
        optimizer = torch.optim.AdamW(
            model.parameters(), lr=args.lr, weight_decay=args.weight_decay
        )

    # LR scheduler
    total_steps = len(train_loader) * args.epochs
    warmup_steps = min(len(train_loader) * 2, total_steps // 10)
    scheduler = get_lr_scheduler(optimizer, warmup_steps, total_steps)

    # Checkpoint directory
    ckpt_dir = Path(args.checkpoint_dir)
    ckpt_dir.mkdir(parents=True, exist_ok=True)

    history = []
    best_val_loss = float("inf")
    best_epoch = 0
    global_step = 0

    log.info(
        "Starting training: %d epochs, %d steps/epoch, %d total steps",
        args.epochs, len(train_loader), total_steps,
    )

    for epoch in range(1, args.epochs + 1):
        model.train()
        epoch_loss = 0.0
        epoch_acc1 = 0.0
        epoch_batches = 0
        epoch_start = time.time()

        for batch_idx, batch in enumerate(train_loader):
            board = batch["board"].to(device)
            order_labels = batch["order_labels"].to(device)
            order_mask = batch["order_mask"].to(device)
            power_idx = batch["power_idx"].to(device)
            unit_indices = batch["unit_indices"].to(device)

            optimizer.zero_grad()

            # Teacher forcing: pass target_orders so decoder sees ground truth
            logits = model(board, adj, unit_indices, power_idx, target_orders=order_labels)
            loss = compute_loss(logits, order_labels, order_mask)
            loss.backward()

            torch.nn.utils.clip_grad_norm_(model.parameters(), args.grad_clip)
            optimizer.step()
            scheduler.step()
            global_step += 1

            acc1 = compute_accuracy(logits, order_labels, order_mask, top_k=1)
            epoch_loss += loss.item()
            epoch_acc1 += acc1
            epoch_batches += 1

            if (batch_idx + 1) % args.log_interval == 0:
                avg_loss = epoch_loss / epoch_batches
                avg_acc = epoch_acc1 / epoch_batches
                lr = scheduler.get_last_lr()[0]
                log.info(
                    "  Epoch %d [%d/%d] loss=%.4f acc@1=%.3f lr=%.2e",
                    epoch, batch_idx + 1, len(train_loader), avg_loss, avg_acc, lr,
                )

        epoch_time = time.time() - epoch_start
        train_loss = epoch_loss / max(epoch_batches, 1)
        train_acc1 = epoch_acc1 / max(epoch_batches, 1)

        val_metrics = evaluate(model, val_loader, adj, device)

        log.info(
            "Epoch %d/%d (%.1fs): train_loss=%.4f train_acc@1=%.3f | "
            "val_loss=%.4f val_acc@1=%.3f val_acc@5=%.3f",
            epoch, args.epochs, epoch_time,
            train_loss, train_acc1,
            val_metrics["loss"], val_metrics["acc_top1"], val_metrics["acc_top5"],
        )

        epoch_record = {
            "epoch": epoch,
            "train_loss": train_loss,
            "train_acc1": train_acc1,
            "val_loss": val_metrics["loss"],
            "val_acc1": val_metrics["acc_top1"],
            "val_acc5": val_metrics["acc_top5"],
            "lr": scheduler.get_last_lr()[0],
            "time_s": epoch_time,
        }
        history.append(epoch_record)

        if val_metrics["loss"] < best_val_loss:
            best_val_loss = val_metrics["loss"]
            best_epoch = epoch
            ckpt_path = ckpt_dir / "best_policy_ar.pt"
            torch.save({
                "epoch": epoch,
                "model_state_dict": model.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "val_loss": val_metrics["loss"],
                "val_acc1": val_metrics["acc_top1"],
                "val_acc5": val_metrics["acc_top5"],
                "args": vars(args),
            }, ckpt_path)
            log.info("  Saved best checkpoint (val_loss=%.4f) to %s", best_val_loss, ckpt_path)

        if epoch % args.save_every == 0:
            ckpt_path = ckpt_dir / f"policy_ar_epoch{epoch:03d}.pt"
            torch.save({
                "epoch": epoch,
                "model_state_dict": model.state_dict(),
                "val_loss": val_metrics["loss"],
                "val_acc1": val_metrics["acc_top1"],
            }, ckpt_path)

    # Save final model
    final_path = ckpt_dir / "final_policy_ar.pt"
    torch.save({
        "epoch": args.epochs,
        "model_state_dict": model.state_dict(),
        "val_loss": val_metrics["loss"],
        "val_acc1": val_metrics["acc_top1"],
        "val_acc5": val_metrics["acc_top5"],
        "args": vars(args),
    }, final_path)
    log.info("Saved final model to %s", final_path)

    # Save training history
    history_path = ckpt_dir / "training_history_ar.json"
    with open(history_path, "w") as f:
        json.dump(history, f, indent=2)

    print(f"\n{'=' * 60}")
    print("Training Complete (Autoregressive)")
    print(f"{'=' * 60}")
    print(f"Best epoch:       {best_epoch}")
    print(f"Best val loss:    {best_val_loss:.4f}")
    print(f"Best val acc@1:   {history[best_epoch - 1]['val_acc1']:.3f}")
    print(f"Best val acc@5:   {history[best_epoch - 1]['val_acc5']:.3f}")
    print(f"Model params:     {num_params:,} ({num_params / 1e6:.2f}M)")
    print(f"  Encoder:        {enc_params:,}")
    print(f"  Decoder:        {dec_params:,}")
    print(f"Checkpoint dir:   {ckpt_dir}")
    print(f"{'=' * 60}")


def main():
    parser = argparse.ArgumentParser(
        description="Train autoregressive policy network for Diplomacy"
    )
    parser.add_argument(
        "--data-dir", type=str, default=str(PROCESSED_DIR),
        help="Directory containing train.npz, val.npz, adjacency.npy",
    )
    parser.add_argument(
        "--checkpoint-dir", type=str, default=str(CHECKPOINT_DIR),
        help="Directory for model checkpoints",
    )
    parser.add_argument(
        "--pretrained-encoder", type=str, default=None,
        help="Path to pretrained independent policy .pt checkpoint for encoder warm-start",
    )
    parser.add_argument("--epochs", type=int, default=50)
    parser.add_argument("--batch-size", type=int, default=64)
    parser.add_argument("--lr", type=float, default=1e-4)
    parser.add_argument("--weight-decay", type=float, default=0.01)
    parser.add_argument("--grad-clip", type=float, default=1.0)
    parser.add_argument("--hidden-dim", type=int, default=512, help="Encoder hidden dim")
    parser.add_argument("--num-layers", type=int, default=6, help="GAT encoder layers")
    parser.add_argument("--num-heads", type=int, default=8, help="GAT attention heads")
    parser.add_argument("--decoder-dim", type=int, default=256, help="Decoder hidden dim")
    parser.add_argument("--decoder-layers", type=int, default=2, help="Decoder layers")
    parser.add_argument("--decoder-heads", type=int, default=4, help="Decoder attention heads")
    parser.add_argument("--dropout", type=float, default=0.15)
    parser.add_argument("--log-interval", type=int, default=50)
    parser.add_argument("--save-every", type=int, default=10)

    args = parser.parse_args()
    train(args)


if __name__ == "__main__":
    main()
