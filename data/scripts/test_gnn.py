#!/usr/bin/env python3
"""Tests for the GNN policy network model and training utilities.

Verifies model architecture, forward pass shapes, loss computation,
accuracy metrics, and dataset loading with synthetic data.
"""

import sys
import tempfile
from pathlib import Path

import numpy as np
import torch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from gnn import DiplomacyPolicyNet, GATBlock, GATLayer

from train_policy import (
    ORDER_VOCAB_SIZE,
    DiplomacyDataset,
    collate_fn,
    compute_accuracy,
    compute_loss,
)

NUM_AREAS = 81
NUM_FEATURES = 36
NUM_POWERS = 7


def _make_dummy_adj() -> torch.Tensor:
    """Create a simple adjacency matrix for testing."""
    adj = torch.eye(NUM_AREAS)
    # Add some random edges
    for i in range(NUM_AREAS - 1):
        adj[i, i + 1] = 1.0
        adj[i + 1, i] = 1.0
    return adj


def _make_dummy_batch(batch_size: int = 4, max_orders: int = 5) -> dict:
    """Create a dummy training batch."""
    board = torch.randn(batch_size, NUM_AREAS, NUM_FEATURES)
    order_labels = torch.zeros(batch_size, max_orders, ORDER_VOCAB_SIZE)
    order_mask = torch.zeros(batch_size, max_orders)
    unit_indices = torch.full((batch_size, max_orders), -1, dtype=torch.long)
    power_idx = torch.randint(0, NUM_POWERS, (batch_size,))

    # Fill some valid orders
    for b in range(batch_size):
        n_orders = torch.randint(1, max_orders + 1, (1,)).item()
        for j in range(n_orders):
            # Random order type
            otype = torch.randint(0, 7, (1,)).item()
            order_labels[b, j, otype] = 1.0
            # Random source province
            src = torch.randint(0, NUM_AREAS, (1,)).item()
            order_labels[b, j, 7 + src] = 1.0
            # Random destination
            dst = torch.randint(0, NUM_AREAS, (1,)).item()
            order_labels[b, j, 7 + NUM_AREAS + dst] = 1.0
            order_mask[b, j] = 1.0
            unit_indices[b, j] = src

    return {
        "board": board,
        "order_labels": order_labels,
        "order_mask": order_mask,
        "power_idx": power_idx,
        "unit_indices": unit_indices,
    }


def _make_dummy_npz(tmpdir: str, n_samples: int = 20, max_orders: int = 5) -> Path:
    """Create a synthetic .npz file matching the feature extraction format."""
    boards = np.random.randn(n_samples, NUM_AREAS, NUM_FEATURES).astype(np.float32)
    order_labels = np.zeros((n_samples, max_orders, ORDER_VOCAB_SIZE), dtype=np.float32)
    order_masks = np.zeros((n_samples, max_orders), dtype=np.float32)
    power_indices = np.random.randint(0, NUM_POWERS, size=(n_samples,), dtype=np.int32)
    values = np.random.rand(n_samples, 4).astype(np.float32)
    years = np.full(n_samples, 1901, dtype=np.int32)

    for i in range(n_samples):
        n_orders = np.random.randint(1, max_orders + 1)
        for j in range(n_orders):
            otype = np.random.randint(0, 7)
            order_labels[i, j, otype] = 1.0
            src = np.random.randint(0, NUM_AREAS)
            order_labels[i, j, 7 + src] = 1.0
            dst = np.random.randint(0, NUM_AREAS)
            order_labels[i, j, 7 + NUM_AREAS + dst] = 1.0
            order_masks[i, j] = 1.0

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


class TestGATLayer:
    """Test the GAT attention layer."""

    def test_output_shape(self):
        layer = GATLayer(in_dim=36, out_dim=64, num_heads=4)
        x = torch.randn(2, NUM_AREAS, 36)
        adj = _make_dummy_adj()
        out = layer(x, adj)
        assert out.shape == (2, NUM_AREAS, 64), f"Expected (2, 81, 64), got {out.shape}"

    def test_different_head_counts(self):
        for heads in [1, 2, 4, 8]:
            layer = GATLayer(in_dim=64, out_dim=64, num_heads=heads)
            x = torch.randn(1, NUM_AREAS, 64)
            adj = _make_dummy_adj()
            out = layer(x, adj)
            assert out.shape == (1, NUM_AREAS, 64)

    def test_batched_adjacency(self):
        layer = GATLayer(in_dim=36, out_dim=64, num_heads=4)
        x = torch.randn(3, NUM_AREAS, 36)
        adj = _make_dummy_adj().unsqueeze(0).expand(3, -1, -1)
        out = layer(x, adj)
        assert out.shape == (3, NUM_AREAS, 64)

    def test_gradient_flows(self):
        layer = GATLayer(in_dim=36, out_dim=64, num_heads=4)
        x = torch.randn(1, NUM_AREAS, 36, requires_grad=True)
        adj = _make_dummy_adj()
        out = layer(x, adj)
        loss = out.sum()
        loss.backward()
        assert x.grad is not None, "Gradient should flow through GAT layer"


class TestGATBlock:
    """Test the GAT block with residuals."""

    def test_output_shape(self):
        block = GATBlock(dim=64, num_heads=4)
        x = torch.randn(2, NUM_AREAS, 64)
        adj = _make_dummy_adj()
        out = block(x, adj)
        assert out.shape == (2, NUM_AREAS, 64)

    def test_residual_connection(self):
        block = GATBlock(dim=64, num_heads=4)
        x = torch.randn(1, NUM_AREAS, 64)
        adj = _make_dummy_adj()
        out = block(x, adj)
        # Output should not be identical to input (layers transform it)
        assert not torch.allclose(x, out), "Block should transform input"


class TestDiplomacyPolicyNet:
    """Test the full policy network."""

    def test_forward_shape(self):
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        batch = _make_dummy_batch(batch_size=2, max_orders=4)
        adj = _make_dummy_adj()
        logits = model(batch["board"], adj, batch["unit_indices"], batch["power_idx"])
        assert logits.shape == (2, 4, ORDER_VOCAB_SIZE), f"Got {logits.shape}"

    def test_encode_shape(self):
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        board = torch.randn(3, NUM_AREAS, NUM_FEATURES)
        adj = _make_dummy_adj()
        emb = model.encode(board, adj)
        assert emb.shape == (3, NUM_AREAS, 64)

    def test_parameter_count(self):
        model = DiplomacyPolicyNet(hidden_dim=256, num_gat_layers=3, num_heads=4)
        n = model.count_parameters()
        # Target: 5-10M parameters
        assert 1_000_000 < n < 20_000_000, f"Parameter count {n:,} outside expected range"

    def test_gradient_flows(self):
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        batch = _make_dummy_batch(batch_size=2, max_orders=3)
        adj = _make_dummy_adj()
        logits = model(batch["board"], adj, batch["unit_indices"], batch["power_idx"])
        loss = logits.sum()
        loss.backward()
        # Check at least some gradients exist
        has_grad = False
        for p in model.parameters():
            if p.grad is not None and p.grad.abs().sum() > 0:
                has_grad = True
                break
        assert has_grad, "No gradients found in model parameters"

    def test_different_batch_sizes(self):
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        adj = _make_dummy_adj()
        for bs in [1, 4, 16]:
            batch = _make_dummy_batch(batch_size=bs, max_orders=5)
            logits = model(batch["board"], adj, batch["unit_indices"], batch["power_idx"])
            assert logits.shape[0] == bs


class TestLossAndAccuracy:
    """Test loss computation and accuracy metrics."""

    def test_loss_is_scalar(self):
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        logits = torch.randn(4, 3, ORDER_VOCAB_SIZE)
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
        assert loss.dim() == 0, "Loss should be a scalar"
        assert loss.item() >= 0, "KL divergence loss should be non-negative"

    def test_loss_is_zero_for_perfect_prediction(self):
        batch = _make_dummy_batch(batch_size=2, max_orders=3)
        # Make logits match targets exactly (large values at target positions)
        logits = batch["order_labels"] * 100.0 - 50.0  # Strong signal at target
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
        assert loss.item() < 0.1, f"Loss should be near-zero for perfect prediction, got {loss.item()}"

    def test_loss_masked_properly(self):
        batch = _make_dummy_batch(batch_size=2, max_orders=5)
        logits = torch.randn(2, 5, ORDER_VOCAB_SIZE)
        # Zero out all masks
        batch["order_mask"] = torch.zeros(2, 5)
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
        assert loss.item() == 0.0, "Loss should be zero when all orders are masked"

    def test_accuracy_top1(self):
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        # Perfect prediction
        logits = batch["order_labels"] * 100.0
        acc = compute_accuracy(logits, batch["order_labels"], batch["order_mask"], top_k=1)
        assert acc == 1.0, f"Top-1 accuracy should be 1.0 for perfect prediction, got {acc}"

    def test_accuracy_top5(self):
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        logits = torch.randn(4, 3, ORDER_VOCAB_SIZE)
        acc5 = compute_accuracy(logits, batch["order_labels"], batch["order_mask"], top_k=5)
        assert 0.0 <= acc5 <= 1.0, f"Top-5 accuracy should be in [0, 1], got {acc5}"

    def test_accuracy_top5_ge_top1(self):
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        logits = torch.randn(4, 3, ORDER_VOCAB_SIZE)
        acc1 = compute_accuracy(logits, batch["order_labels"], batch["order_mask"], top_k=1)
        acc5 = compute_accuracy(logits, batch["order_labels"], batch["order_mask"], top_k=5)
        assert acc5 >= acc1 - 1e-6, "Top-5 accuracy should be >= Top-1"


class TestDataset:
    """Test dataset loading."""

    def test_load_synthetic_npz(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=10, max_orders=4)
            ds = DiplomacyDataset(npz_path)
            assert len(ds) == 10
            sample = ds[0]
            assert sample["board"].shape == (NUM_AREAS, NUM_FEATURES)
            assert sample["order_labels"].shape[1] == ORDER_VOCAB_SIZE
            assert sample["order_mask"].shape[0] == sample["order_labels"].shape[0]

    def test_collate_fn(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=8, max_orders=4)
            ds = DiplomacyDataset(npz_path)
            batch_items = [ds[i] for i in range(4)]
            batch = collate_fn(batch_items)
            assert batch["board"].shape == (4, NUM_AREAS, NUM_FEATURES)
            assert batch["order_labels"].shape[0] == 4
            assert batch["order_mask"].shape[0] == 4
            assert batch["unit_indices"].shape[0] == 4

    def test_unit_indices_extraction(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=5, max_orders=3)
            ds = DiplomacyDataset(npz_path)
            sample = ds[0]
            # Valid orders should have non-negative unit indices
            mask = sample["order_mask"]
            indices = sample["unit_indices"]
            for j in range(mask.shape[0]):
                if mask[j] > 0:
                    assert indices[j] >= 0, f"Valid order {j} should have non-negative unit index"
                else:
                    assert indices[j] == -1, f"Padded order {j} should have index -1"


class TestEndToEnd:
    """Test a full training step with synthetic data."""

    def test_single_training_step(self):
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4, max_orders=3)

        model.train()
        logits = model(batch["board"], adj, batch["unit_indices"], batch["power_idx"])
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])

        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

        # Loss should be finite
        assert torch.isfinite(loss), f"Loss is not finite: {loss.item()}"

    def test_loss_decreases_over_steps(self):
        """Verify model can overfit a single batch (sanity check)."""
        model = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4, max_orders=3)

        initial_loss = None
        for step in range(30):
            model.train()
            logits = model(batch["board"], adj, batch["unit_indices"], batch["power_idx"])
            loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            if initial_loss is None:
                initial_loss = loss.item()

        final_loss = loss.item()
        assert final_loss < initial_loss, (
            f"Loss should decrease: {initial_loss:.4f} -> {final_loss:.4f}"
        )


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        TestGATLayer,
        TestGATBlock,
        TestDiplomacyPolicyNet,
        TestLossAndAccuracy,
        TestDataset,
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
