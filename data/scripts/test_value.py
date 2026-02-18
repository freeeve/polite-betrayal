#!/usr/bin/env python3
"""Tests for the value network model and training utilities.

Verifies model architecture, forward pass shapes, loss computation,
metrics, encoder weight transfer, and dataset loading with synthetic data.
"""

import sys
import tempfile
from pathlib import Path

import numpy as np
import torch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from gnn import DiplomacyPolicyNet
from value_net import AttentionPooling, DiplomacyValueNet

from train_value import (
    VALUE_DIM,
    ValueDataset,
    collate_fn,
    compute_value_loss,
    compute_value_metrics,
)

NUM_AREAS = 81
NUM_FEATURES = 36
NUM_POWERS = 7


def _make_dummy_adj() -> torch.Tensor:
    """Create a simple adjacency matrix for testing."""
    adj = torch.eye(NUM_AREAS)
    for i in range(NUM_AREAS - 1):
        adj[i, i + 1] = 1.0
        adj[i + 1, i] = 1.0
    return adj


def _make_dummy_batch(batch_size: int = 4) -> dict:
    """Create a dummy value training batch."""
    board = torch.randn(batch_size, NUM_AREAS, NUM_FEATURES)
    power_idx = torch.randint(0, NUM_POWERS, (batch_size,))
    value = torch.rand(batch_size, VALUE_DIM)
    # Make win/draw/survival binary
    value[:, 1] = (value[:, 1] > 0.5).float()
    value[:, 2] = (value[:, 2] > 0.5).float()
    value[:, 3] = (value[:, 3] > 0.5).float()
    return {
        "board": board,
        "power_idx": power_idx,
        "value": value,
    }


def _make_dummy_npz(tmpdir: str, n_samples: int = 20) -> Path:
    """Create a synthetic .npz file for value training."""
    boards = np.random.randn(n_samples, NUM_AREAS, NUM_FEATURES).astype(np.float32)
    power_indices = np.random.randint(0, NUM_POWERS, size=(n_samples,), dtype=np.int32)
    values = np.random.rand(n_samples, VALUE_DIM).astype(np.float32)
    # Make indicators binary
    values[:, 1] = (values[:, 1] > 0.5).astype(np.float32)
    values[:, 2] = (values[:, 2] > 0.5).astype(np.float32)
    values[:, 3] = (values[:, 3] > 0.5).astype(np.float32)

    # Also include order fields so we can reuse the same npz format
    max_orders = 5
    order_labels = np.zeros((n_samples, max_orders, 169), dtype=np.float32)
    order_masks = np.zeros((n_samples, max_orders), dtype=np.float32)
    years = np.full(n_samples, 1901, dtype=np.int32)

    path = Path(tmpdir) / "test.npz"
    np.savez_compressed(
        path,
        boards=boards,
        order_labels=order_labels,
        order_masks=order_masks,
        power_indices=power_indices,
        values=values,
        years=years,
    )
    return path


class TestAttentionPooling:
    """Test the attention pooling module."""

    def test_output_shape(self):
        pool = AttentionPooling(dim=64)
        x = torch.randn(4, NUM_AREAS, 64)
        out = pool(x)
        assert out.shape == (4, 64), f"Expected (4, 64), got {out.shape}"

    def test_single_sample(self):
        pool = AttentionPooling(dim=32)
        x = torch.randn(1, 10, 32)
        out = pool(x)
        assert out.shape == (1, 32)

    def test_gradient_flows(self):
        pool = AttentionPooling(dim=64)
        x = torch.randn(2, NUM_AREAS, 64, requires_grad=True)
        out = pool(x)
        loss = out.sum()
        loss.backward()
        assert x.grad is not None, "Gradient should flow through pooling"


class TestDiplomacyValueNet:
    """Test the full value network."""

    def test_forward_shape(self):
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        batch = _make_dummy_batch(batch_size=4)
        adj = _make_dummy_adj()
        out = model(batch["board"], adj, batch["power_idx"])
        assert out.shape == (4, 4), f"Expected (4, 4), got {out.shape}"

    def test_output_range(self):
        """All outputs should be in [0, 1] due to sigmoid."""
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        batch = _make_dummy_batch(batch_size=8)
        adj = _make_dummy_adj()
        out = model(batch["board"], adj, batch["power_idx"])
        assert (out >= 0).all() and (out <= 1).all(), (
            f"Output should be in [0, 1], got min={out.min():.4f} max={out.max():.4f}"
        )

    def test_encode_shape(self):
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        board = torch.randn(3, NUM_AREAS, NUM_FEATURES)
        adj = _make_dummy_adj()
        emb = model.encode(board, adj)
        assert emb.shape == (3, NUM_AREAS, 64)

    def test_parameter_count(self):
        model = DiplomacyValueNet(hidden_dim=256, num_gat_layers=3, num_heads=4)
        n = model.count_parameters()
        assert 1_000_000 < n < 20_000_000, f"Parameter count {n:,} outside expected range"

    def test_gradient_flows(self):
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        batch = _make_dummy_batch(batch_size=2)
        adj = _make_dummy_adj()
        out = model(batch["board"], adj, batch["power_idx"])
        loss = out.sum()
        loss.backward()
        has_grad = False
        for p in model.parameters():
            if p.grad is not None and p.grad.abs().sum() > 0:
                has_grad = True
                break
        assert has_grad, "No gradients found in model parameters"

    def test_different_batch_sizes(self):
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        adj = _make_dummy_adj()
        for bs in [1, 4, 16]:
            batch = _make_dummy_batch(batch_size=bs)
            out = model(batch["board"], adj, batch["power_idx"])
            assert out.shape == (bs, 4)

    def test_different_powers_produce_different_outputs(self):
        """Different power indices should produce different value estimates."""
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        adj = _make_dummy_adj()
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        # Same board, different powers
        boards = board.expand(NUM_POWERS, -1, -1)
        power_indices = torch.arange(NUM_POWERS)
        out = model(boards, adj, power_indices)
        # At least some outputs should differ between powers
        diffs = 0
        for i in range(NUM_POWERS):
            for j in range(i + 1, NUM_POWERS):
                if not torch.allclose(out[i], out[j], atol=1e-6):
                    diffs += 1
        assert diffs > 0, "Different powers should produce different value estimates"


class TestEncoderTransfer:
    """Test weight transfer from policy to value network."""

    def test_load_encoder_from_policy(self):
        policy = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        value = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)

        # Before transfer, weights should differ
        p_w = policy.input_proj[0].weight.data.clone()
        v_w = value.input_proj[0].weight.data.clone()
        assert not torch.allclose(p_w, v_w), "Weights should differ before transfer"

        # Transfer
        value.load_encoder_from_policy(policy)

        # After transfer, encoder weights should match
        assert torch.allclose(
            policy.input_proj[0].weight.data,
            value.input_proj[0].weight.data,
        ), "Input projection weights should match after transfer"

        assert torch.allclose(
            policy.gat_blocks[0].gat.W.weight.data,
            value.gat_blocks[0].gat.W.weight.data,
        ), "GAT weights should match after transfer"

        assert torch.allclose(
            policy.power_embed.weight.data,
            value.power_embed.weight.data,
        ), "Power embeddings should match after transfer"

    def test_encoder_produces_same_output_after_transfer(self):
        policy = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        value = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        value.load_encoder_from_policy(policy)

        board = torch.randn(2, NUM_AREAS, NUM_FEATURES)
        adj = _make_dummy_adj()

        policy.eval()
        value.eval()
        with torch.no_grad():
            p_emb = policy.encode(board, adj)
            v_emb = value.encode(board, adj)

        assert torch.allclose(p_emb, v_emb, atol=1e-6), (
            "Encoder outputs should match after weight transfer"
        )


class TestValueLoss:
    """Test value loss computation."""

    def test_loss_is_scalar(self):
        preds = torch.rand(4, 4)
        targets = torch.rand(4, 4)
        targets[:, 1:] = (targets[:, 1:] > 0.5).float()
        losses = compute_value_loss(preds, targets)
        assert losses["total"].dim() == 0, "Loss should be a scalar"
        assert losses["total"].item() >= 0, "Loss should be non-negative"

    def test_loss_components(self):
        preds = torch.rand(4, 4)
        targets = torch.rand(4, 4)
        targets[:, 1:] = (targets[:, 1:] > 0.5).float()
        losses = compute_value_loss(preds, targets)
        assert "sc_mse" in losses
        assert "bce" in losses
        assert losses["sc_mse"] >= 0
        assert losses["bce"] >= 0

    def test_perfect_prediction_low_loss(self):
        targets = torch.tensor([[0.5, 1.0, 0.0, 1.0]])
        preds = torch.tensor([[0.5, 0.99, 0.01, 0.99]])
        losses = compute_value_loss(preds, targets)
        assert losses["total"].item() < 0.1, (
            f"Loss should be low for near-perfect prediction, got {losses['total'].item()}"
        )


class TestValueMetrics:
    """Test value evaluation metrics."""

    def test_perfect_sc_correlation(self):
        preds = torch.tensor([[0.1, 0.5, 0.5, 0.5], [0.5, 0.5, 0.5, 0.5], [0.9, 0.5, 0.5, 0.5]])
        targets = torch.tensor([[0.1, 0.5, 0.5, 0.5], [0.5, 0.5, 0.5, 0.5], [0.9, 0.5, 0.5, 0.5]])
        metrics = compute_value_metrics(preds, targets)
        assert metrics["sc_corr"] > 0.99, f"Correlation should be ~1.0, got {metrics['sc_corr']}"

    def test_perfect_win_accuracy(self):
        preds = torch.tensor([[0.5, 0.9, 0.1, 0.9], [0.5, 0.1, 0.9, 0.1]])
        targets = torch.tensor([[0.5, 1.0, 0.0, 1.0], [0.5, 0.0, 1.0, 0.0]])
        metrics = compute_value_metrics(preds, targets)
        assert metrics["win_acc"] == 1.0, f"Win acc should be 1.0, got {metrics['win_acc']}"
        assert metrics["surv_acc"] == 1.0, f"Surv acc should be 1.0, got {metrics['surv_acc']}"

    def test_metrics_in_valid_range(self):
        preds = torch.rand(20, 4)
        targets = torch.rand(20, 4)
        targets[:, 1:] = (targets[:, 1:] > 0.5).float()
        metrics = compute_value_metrics(preds, targets)
        assert 0.0 <= metrics["win_acc"] <= 1.0
        assert 0.0 <= metrics["surv_acc"] <= 1.0
        assert -1.0 <= metrics["sc_corr"] <= 1.0
        assert metrics["sc_mse_34"] >= 0


class TestValueDataset:
    """Test dataset loading for value training."""

    def test_load_synthetic_npz(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=10)
            ds = ValueDataset(npz_path)
            assert len(ds) == 10
            sample = ds[0]
            assert sample["board"].shape == (NUM_AREAS, NUM_FEATURES)
            assert sample["value"].shape == (VALUE_DIM,)
            assert 0 <= sample["power_idx"] < NUM_POWERS

    def test_collate_fn(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=8)
            ds = ValueDataset(npz_path)
            batch_items = [ds[i] for i in range(4)]
            batch = collate_fn(batch_items)
            assert batch["board"].shape == (4, NUM_AREAS, NUM_FEATURES)
            assert batch["value"].shape == (4, VALUE_DIM)
            assert batch["power_idx"].shape == (4,)


class TestEndToEnd:
    """Test a full training step with synthetic data."""

    def test_single_training_step(self):
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4)

        model.train()
        predictions = model(batch["board"], adj, batch["power_idx"])
        losses = compute_value_loss(predictions, batch["value"])

        optimizer.zero_grad()
        losses["total"].backward()
        optimizer.step()

        assert torch.isfinite(losses["total"]), f"Loss is not finite: {losses['total'].item()}"

    def test_loss_decreases_over_steps(self):
        """Verify model can overfit a single batch (sanity check)."""
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4)

        initial_loss = None
        for step in range(50):
            model.train()
            predictions = model(batch["board"], adj, batch["power_idx"])
            losses = compute_value_loss(predictions, batch["value"])
            optimizer.zero_grad()
            losses["total"].backward()
            optimizer.step()
            if initial_loss is None:
                initial_loss = losses["total"].item()

        final_loss = losses["total"].item()
        assert final_loss < initial_loss, (
            f"Loss should decrease: {initial_loss:.4f} -> {final_loss:.4f}"
        )

    def test_checkpoint_save_load(self):
        """Verify model can be saved and loaded from a checkpoint."""
        model = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=2)

        model.eval()
        with torch.no_grad():
            out1 = model(batch["board"], adj, batch["power_idx"])

        with tempfile.TemporaryDirectory() as tmpdir:
            ckpt_path = Path(tmpdir) / "test.pt"
            torch.save({"model_state_dict": model.state_dict()}, ckpt_path)

            model2 = DiplomacyValueNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
            ckpt = torch.load(ckpt_path, weights_only=True)
            model2.load_state_dict(ckpt["model_state_dict"])
            model2.eval()
            with torch.no_grad():
                out2 = model2(batch["board"], adj, batch["power_idx"])

        assert torch.allclose(out1, out2, atol=1e-6), "Outputs should match after reload"


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        TestAttentionPooling,
        TestDiplomacyValueNet,
        TestEncoderTransfer,
        TestValueLoss,
        TestValueMetrics,
        TestValueDataset,
        TestEndToEnd,
    ]

    total = 0
    passed = 0
    failed = 0
    errors = []

    for cls in test_classes:
        instance = cls()
        methods = [m for m in dir(instance) if m.startswith("test_")]
        for method_name in sorted(methods):
            total += 1
            test_name = f"{cls.__name__}.{method_name}"
            try:
                getattr(instance, method_name)()
                passed += 1
                print(f"  PASS  {test_name}")
            except Exception as e:
                failed += 1
                errors.append((test_name, str(e)))
                print(f"  FAIL  {test_name}: {e}")

    print(f"\n{'=' * 50}")
    print(f"Results: {passed}/{total} passed, {failed} failed")
    if errors:
        print("\nFailures:")
        for name, err in errors:
            print(f"  {name}: {err}")
    print(f"{'=' * 50}")

    return failed == 0


if __name__ == "__main__":
    success = run_all_tests()
    sys.exit(0 if success else 1)
