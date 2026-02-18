#!/usr/bin/env python3
"""Training script for the Diplomacy value network.

Loads extracted .npz features (with value labels), trains the GAT-based
value network, and saves checkpoints with training metrics.

Usage:
    python3 scripts/train_value.py [--data-dir processed/features] [--epochs 50]

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
from value_net import DiplomacyValueNet

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

PROCESSED_DIR = Path(__file__).resolve().parent.parent / "processed" / "features"
CHECKPOINT_DIR = Path(__file__).resolve().parent.parent / "checkpoints"

NUM_AREAS = 81
NUM_FEATURES = 36
NUM_POWERS = 7
VALUE_DIM = 4  # [sc_share, win, draw, survival]


class ValueDataset(Dataset):
    """PyTorch dataset for value network training.

    Each sample contains:
      - board: [81, 36] board state tensor
      - power_index: int, the power to evaluate
      - value: [4] target value label
    """

    def __init__(self, npz_path: Path):
        log.info("Loading dataset from %s", npz_path)
        data = np.load(npz_path)
        self.boards = data["boards"]              # [N, 81, 36]
        self.power_indices = data["power_indices"] # [N]
        self.values = data["values"]               # [N, 4]
        self.n_samples = self.boards.shape[0]
        log.info("  %d samples", self.n_samples)

    def __len__(self) -> int:
        return self.n_samples

    def __getitem__(self, idx: int) -> dict:
        return {
            "board": torch.from_numpy(self.boards[idx]),
            "power_idx": int(self.power_indices[idx]),
            "value": torch.from_numpy(self.values[idx]),
        }


def collate_fn(batch: list[dict]) -> dict:
    """Collate value samples into a batch."""
    return {
        "board": torch.stack([b["board"] for b in batch]),
        "power_idx": torch.tensor([b["power_idx"] for b in batch], dtype=torch.long),
        "value": torch.stack([b["value"] for b in batch]),
    }


def compute_value_loss(predictions: torch.Tensor, targets: torch.Tensor) -> dict:
    """Compute value loss with separate components.

    Args:
        predictions: [B, 4] sigmoid outputs from value net
        targets: [B, 4] ground truth [sc_share, win, draw, survival]

    Returns:
        Dict with total loss and per-component losses.
    """
    # MSE on SC share (index 0) - regression target
    sc_loss = F.mse_loss(predictions[:, 0], targets[:, 0])

    # Binary cross-entropy on win/draw/survival (indices 1, 2, 3)
    bce_loss = F.binary_cross_entropy(
        predictions[:, 1:],
        targets[:, 1:],
    )

    total = sc_loss + bce_loss
    return {
        "total": total,
        "sc_mse": sc_loss.item(),
        "bce": bce_loss.item(),
    }


def compute_value_metrics(predictions: torch.Tensor, targets: torch.Tensor) -> dict:
    """Compute evaluation metrics for value predictions.

    Args:
        predictions: [B, 4] sigmoid outputs
        targets: [B, 4] ground truth

    Returns:
        Dict with various accuracy/correlation metrics.
    """
    B = predictions.shape[0]

    # SC share MSE (denormalized to actual SC count out of 34)
    sc_pred = predictions[:, 0] * 34.0
    sc_true = targets[:, 0] * 34.0
    sc_mse = F.mse_loss(sc_pred, sc_true).item()

    # SC share correlation
    sc_corr = _pearson_correlation(predictions[:, 0], targets[:, 0])

    # Win prediction accuracy (threshold at 0.5)
    win_pred = (predictions[:, 1] > 0.5).float()
    win_true = targets[:, 1]
    win_acc = (win_pred == win_true).float().mean().item()

    # Survival prediction accuracy
    surv_pred = (predictions[:, 3] > 0.5).float()
    surv_true = targets[:, 3]
    surv_acc = (surv_pred == surv_true).float().mean().item()

    return {
        "sc_mse_34": sc_mse,
        "sc_corr": sc_corr,
        "win_acc": win_acc,
        "surv_acc": surv_acc,
    }


def _pearson_correlation(x: torch.Tensor, y: torch.Tensor) -> float:
    """Compute Pearson correlation between two 1D tensors."""
    if x.shape[0] < 2:
        return 0.0
    x_mean = x - x.mean()
    y_mean = y - y.mean()
    num = (x_mean * y_mean).sum()
    denom = (x_mean.pow(2).sum() * y_mean.pow(2).sum()).sqrt()
    if denom < 1e-8:
        return 0.0
    return (num / denom).item()


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


def evaluate(
    model: DiplomacyValueNet,
    dataloader: DataLoader,
    adj: torch.Tensor,
    device: torch.device,
) -> dict:
    """Run evaluation on a dataset split."""
    model.eval()
    all_preds = []
    all_targets = []
    total_loss = 0.0
    total_sc_mse = 0.0
    total_bce = 0.0
    num_batches = 0

    with torch.no_grad():
        for batch in dataloader:
            board = batch["board"].to(device)
            power_idx = batch["power_idx"].to(device)
            value = batch["value"].to(device)

            predictions = model(board, adj, power_idx)
            losses = compute_value_loss(predictions, value)

            total_loss += losses["total"].item()
            total_sc_mse += losses["sc_mse"]
            total_bce += losses["bce"]
            num_batches += 1

            all_preds.append(predictions.cpu())
            all_targets.append(value.cpu())

    n = max(num_batches, 1)
    preds_cat = torch.cat(all_preds, dim=0)
    targets_cat = torch.cat(all_targets, dim=0)
    metrics = compute_value_metrics(preds_cat, targets_cat)

    return {
        "loss": total_loss / n,
        "sc_mse": total_sc_mse / n,
        "bce": total_bce / n,
        **metrics,
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
    train_ds = ValueDataset(Path(args.data_dir) / "train.npz")
    val_ds = ValueDataset(Path(args.data_dir) / "val.npz")

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
    model = DiplomacyValueNet(
        num_areas=NUM_AREAS,
        num_features=NUM_FEATURES,
        hidden_dim=args.hidden_dim,
        num_gat_layers=args.num_layers,
        num_heads=args.num_heads,
        num_powers=NUM_POWERS,
        dropout=args.dropout,
    ).to(device)

    # Optionally load encoder from policy checkpoint
    if args.policy_checkpoint:
        log.info("Loading encoder from policy checkpoint: %s", args.policy_checkpoint)
        from gnn import DiplomacyPolicyNet
        ckpt = torch.load(args.policy_checkpoint, map_location=device, weights_only=True)
        policy_net = DiplomacyPolicyNet(
            num_areas=NUM_AREAS,
            num_features=NUM_FEATURES,
            hidden_dim=args.hidden_dim,
            num_gat_layers=args.num_layers,
            num_heads=args.num_heads,
            num_powers=NUM_POWERS,
            dropout=args.dropout,
        )
        policy_net.load_state_dict(ckpt["model_state_dict"])
        model.load_encoder_from_policy(policy_net)
        log.info("Encoder weights loaded from policy network")

    num_params = model.count_parameters()
    log.info("Model parameters: %s (%.2fM)", f"{num_params:,}", num_params / 1e6)

    # Optimizer
    optimizer = torch.optim.AdamW(
        model.parameters(),
        lr=args.lr,
        weight_decay=args.weight_decay,
    )

    # LR scheduler
    total_steps = len(train_loader) * args.epochs
    warmup_steps = min(len(train_loader) * 2, total_steps // 10)
    scheduler = get_lr_scheduler(optimizer, warmup_steps, total_steps)

    # Checkpoint directory
    ckpt_dir = Path(args.checkpoint_dir)
    ckpt_dir.mkdir(parents=True, exist_ok=True)

    # Training log
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
        epoch_sc_mse = 0.0
        epoch_bce = 0.0
        epoch_batches = 0
        epoch_start = time.time()

        for batch_idx, batch in enumerate(train_loader):
            board = batch["board"].to(device)
            power_idx = batch["power_idx"].to(device)
            value = batch["value"].to(device)

            optimizer.zero_grad()
            predictions = model(board, adj, power_idx)
            losses = compute_value_loss(predictions, value)
            losses["total"].backward()

            torch.nn.utils.clip_grad_norm_(model.parameters(), args.grad_clip)
            optimizer.step()
            scheduler.step()
            global_step += 1

            epoch_loss += losses["total"].item()
            epoch_sc_mse += losses["sc_mse"]
            epoch_bce += losses["bce"]
            epoch_batches += 1

            if (batch_idx + 1) % args.log_interval == 0:
                avg_loss = epoch_loss / epoch_batches
                lr = scheduler.get_last_lr()[0]
                log.info(
                    "  Epoch %d [%d/%d] loss=%.4f sc_mse=%.4f bce=%.4f lr=%.2e",
                    epoch, batch_idx + 1, len(train_loader),
                    avg_loss, epoch_sc_mse / epoch_batches,
                    epoch_bce / epoch_batches, lr,
                )

        epoch_time = time.time() - epoch_start
        n = max(epoch_batches, 1)
        train_loss = epoch_loss / n
        train_sc_mse = epoch_sc_mse / n
        train_bce = epoch_bce / n

        # Validation
        val_metrics = evaluate(model, val_loader, adj, device)

        log.info(
            "Epoch %d/%d (%.1fs): train_loss=%.4f (sc=%.4f bce=%.4f) | "
            "val_loss=%.4f sc_mse34=%.2f sc_corr=%.3f win_acc=%.3f surv_acc=%.3f",
            epoch, args.epochs, epoch_time,
            train_loss, train_sc_mse, train_bce,
            val_metrics["loss"], val_metrics["sc_mse_34"],
            val_metrics["sc_corr"], val_metrics["win_acc"], val_metrics["surv_acc"],
        )

        epoch_record = {
            "epoch": epoch,
            "train_loss": train_loss,
            "train_sc_mse": train_sc_mse,
            "train_bce": train_bce,
            "val_loss": val_metrics["loss"],
            "val_sc_mse": val_metrics["sc_mse"],
            "val_bce": val_metrics["bce"],
            "val_sc_mse_34": val_metrics["sc_mse_34"],
            "val_sc_corr": val_metrics["sc_corr"],
            "val_win_acc": val_metrics["win_acc"],
            "val_surv_acc": val_metrics["surv_acc"],
            "lr": scheduler.get_last_lr()[0],
            "time_s": epoch_time,
        }
        history.append(epoch_record)

        # Checkpoint on best validation loss
        if val_metrics["loss"] < best_val_loss:
            best_val_loss = val_metrics["loss"]
            best_epoch = epoch
            ckpt_path = ckpt_dir / "best_value.pt"
            torch.save({
                "epoch": epoch,
                "model_state_dict": model.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "val_loss": val_metrics["loss"],
                "val_sc_mse_34": val_metrics["sc_mse_34"],
                "val_sc_corr": val_metrics["sc_corr"],
                "val_win_acc": val_metrics["win_acc"],
                "val_surv_acc": val_metrics["surv_acc"],
                "args": vars(args),
            }, ckpt_path)
            log.info("  Saved best checkpoint (val_loss=%.4f) to %s", best_val_loss, ckpt_path)

        # Periodic checkpoint
        if epoch % args.save_every == 0:
            ckpt_path = ckpt_dir / f"value_epoch{epoch:03d}.pt"
            torch.save({
                "epoch": epoch,
                "model_state_dict": model.state_dict(),
                "val_loss": val_metrics["loss"],
            }, ckpt_path)

    # Save final model
    final_path = ckpt_dir / "final_value.pt"
    torch.save({
        "epoch": args.epochs,
        "model_state_dict": model.state_dict(),
        "val_loss": val_metrics["loss"],
        "val_sc_mse_34": val_metrics["sc_mse_34"],
        "val_sc_corr": val_metrics["sc_corr"],
        "val_win_acc": val_metrics["win_acc"],
        "val_surv_acc": val_metrics["surv_acc"],
        "args": vars(args),
    }, final_path)
    log.info("Saved final model to %s", final_path)

    # Save training history
    history_path = ckpt_dir / "value_training_history.json"
    with open(history_path, "w") as f:
        json.dump(history, f, indent=2)
    log.info("Saved training history to %s", history_path)

    # Summary
    best = history[best_epoch - 1]
    print(f"\n{'=' * 60}")
    print("Value Network Training Complete")
    print(f"{'=' * 60}")
    print(f"Best epoch:       {best_epoch}")
    print(f"Best val loss:    {best_val_loss:.4f}")
    print(f"SC MSE (of 34):   {best['val_sc_mse_34']:.2f}")
    print(f"SC correlation:   {best['val_sc_corr']:.3f}")
    print(f"Win accuracy:     {best['val_win_acc']:.3f}")
    print(f"Survival acc:     {best['val_surv_acc']:.3f}")
    print(f"Model params:     {num_params:,} ({num_params / 1e6:.2f}M)")
    print(f"Checkpoint dir:   {ckpt_dir}")
    print(f"{'=' * 60}")


def main():
    parser = argparse.ArgumentParser(
        description="Train value network for Diplomacy position evaluation"
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
        "--policy-checkpoint", type=str, default="",
        help="Path to policy network checkpoint for encoder weight transfer",
    )
    parser.add_argument("--epochs", type=int, default=50, help="Number of training epochs")
    parser.add_argument("--batch-size", type=int, default=64, help="Training batch size")
    parser.add_argument("--lr", type=float, default=3e-4, help="Peak learning rate")
    parser.add_argument("--weight-decay", type=float, default=0.01, help="AdamW weight decay")
    parser.add_argument("--grad-clip", type=float, default=1.0, help="Gradient clipping norm")
    parser.add_argument("--hidden-dim", type=int, default=256, help="GNN hidden dimension")
    parser.add_argument("--num-layers", type=int, default=3, help="Number of GAT layers")
    parser.add_argument("--num-heads", type=int, default=4, help="Number of attention heads")
    parser.add_argument("--dropout", type=float, default=0.1, help="Dropout rate")
    parser.add_argument("--log-interval", type=int, default=50, help="Log every N batches")
    parser.add_argument("--save-every", type=int, default=10, help="Save checkpoint every N epochs")

    args = parser.parse_args()
    train(args)


if __name__ == "__main__":
    main()
