#!/usr/bin/env python3
"""Tests for the autoregressive order decoder.

Verifies decoder architecture, forward pass shapes, teacher forcing,
autoregressive generation, causal masking, loss computation,
encoder weight transfer, and training convergence.
"""

import sys
import tempfile
from pathlib import Path

import numpy as np
import torch
import torch.nn.functional as F

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from autoregressive_decoder import (
    AutoregressiveDecoder,
    DiplomacyAutoRegressivePolicyNet,
    OrderEmbedding,
)
from gnn import DiplomacyPolicyNet

from train_policy_ar import (
    ORDER_VOCAB_SIZE,
    DiplomacyDataset,
    collate_fn,
    compute_accuracy,
    compute_loss,
)

NUM_AREAS = 81
NUM_FEATURES = 47
NUM_POWERS = 7
MAX_UNITS = 17


def _make_dummy_adj() -> torch.Tensor:
    adj = torch.eye(NUM_AREAS)
    for i in range(NUM_AREAS - 1):
        adj[i, i + 1] = 1.0
        adj[i + 1, i] = 1.0
    return adj


def _make_dummy_batch(batch_size: int = 4, max_orders: int = 5) -> dict:
    board = torch.randn(batch_size, NUM_AREAS, NUM_FEATURES)
    order_labels = torch.zeros(batch_size, max_orders, ORDER_VOCAB_SIZE)
    order_mask = torch.zeros(batch_size, max_orders)
    unit_indices = torch.full((batch_size, max_orders), -1, dtype=torch.long)
    power_idx = torch.randint(0, NUM_POWERS, (batch_size,))

    for b in range(batch_size):
        n_orders = torch.randint(1, max_orders + 1, (1,)).item()
        for j in range(n_orders):
            otype = torch.randint(0, 7, (1,)).item()
            order_labels[b, j, otype] = 1.0
            src = torch.randint(0, NUM_AREAS, (1,)).item()
            order_labels[b, j, 7 + src] = 1.0
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


class TestOrderEmbedding:
    """Test the order embedding module."""

    def test_output_shape_2d(self):
        embed = OrderEmbedding(hidden_dim=64)
        order_vec = torch.zeros(4, ORDER_VOCAB_SIZE)
        order_vec[:, 0] = 1.0  # hold
        order_vec[:, 7] = 1.0  # src=0
        out = embed(order_vec)
        assert out.shape == (4, 64), f"Expected (4, 64), got {out.shape}"

    def test_output_shape_3d(self):
        embed = OrderEmbedding(hidden_dim=64)
        order_vec = torch.zeros(2, 5, ORDER_VOCAB_SIZE)
        order_vec[:, :, 0] = 1.0
        order_vec[:, :, 7] = 1.0
        out = embed(order_vec)
        assert out.shape == (2, 5, 64), f"Expected (2, 5, 64), got {out.shape}"

    def test_null_order_uses_null_embed(self):
        embed = OrderEmbedding(hidden_dim=64)
        # All-zero order vector
        null_vec = torch.zeros(1, ORDER_VOCAB_SIZE)
        out = embed(null_vec)
        expected = embed.null_embed.unsqueeze(0)
        assert torch.allclose(out, expected, atol=1e-6), "Null order should use null_embed"

    def test_different_orders_different_embeddings(self):
        embed = OrderEmbedding(hidden_dim=64)
        # Hold at province 0
        order1 = torch.zeros(1, ORDER_VOCAB_SIZE)
        order1[0, 0] = 1.0
        order1[0, 7] = 1.0
        # Move from province 0 to province 1
        order2 = torch.zeros(1, ORDER_VOCAB_SIZE)
        order2[0, 1] = 1.0
        order2[0, 7] = 1.0
        order2[0, 7 + NUM_AREAS + 1] = 1.0

        emb1 = embed(order1)
        emb2 = embed(order2)
        assert not torch.allclose(emb1, emb2), "Different orders should produce different embeddings"


class TestAutoRegressiveDecoder:
    """Test the autoregressive decoder module."""

    def test_teacher_forcing_shape(self):
        decoder = AutoregressiveDecoder(encoder_dim=64, decoder_dim=32, num_layers=1, num_heads=2)
        board_emb = torch.randn(2, NUM_AREAS, 64)
        unit_indices = torch.randint(0, NUM_AREAS, (2, 5))
        power_indices = torch.randint(0, NUM_POWERS, (2,))
        target_orders = torch.zeros(2, 5, ORDER_VOCAB_SIZE)
        target_orders[:, :, 0] = 1.0  # all holds
        target_orders[:, :, 7] = 1.0

        logits = decoder.forward_teacher_forcing(
            board_emb, unit_indices, power_indices, target_orders
        )
        assert logits.shape == (2, 5, ORDER_VOCAB_SIZE), f"Got {logits.shape}"

    def test_autoregressive_shape(self):
        decoder = AutoregressiveDecoder(encoder_dim=64, decoder_dim=32, num_layers=1, num_heads=2, max_units=5)
        board_emb = torch.randn(2, NUM_AREAS, 64)
        unit_indices = torch.randint(0, NUM_AREAS, (2, 5))
        power_indices = torch.randint(0, NUM_POWERS, (2,))

        generated, logits = decoder.forward_autoregressive(
            board_emb, unit_indices, power_indices
        )
        assert generated.shape == (2, 5, ORDER_VOCAB_SIZE), f"Got {generated.shape}"
        assert logits.shape == (2, 5, ORDER_VOCAB_SIZE), f"Got {logits.shape}"

    def test_autoregressive_output_is_onehot(self):
        decoder = AutoregressiveDecoder(encoder_dim=64, decoder_dim=32, num_layers=1, num_heads=2, max_units=3)
        board_emb = torch.randn(1, NUM_AREAS, 64)
        unit_indices = torch.randint(0, NUM_AREAS, (1, 3))
        power_indices = torch.zeros(1, dtype=torch.long)

        generated, _ = decoder.forward_autoregressive(
            board_emb, unit_indices, power_indices
        )
        # Each generated order should be one-hot (exactly one 1.0)
        for step in range(3):
            order = generated[0, step]
            assert order.sum().item() == 1.0, f"Step {step}: expected one-hot, got sum={order.sum().item()}"

    def test_causal_mask_is_upper_triangular(self):
        decoder = AutoregressiveDecoder(encoder_dim=64, decoder_dim=32)
        mask = decoder._build_causal_mask(5, torch.device("cpu"))
        assert mask.shape == (5, 5)
        # Lower triangle (including diagonal) should be False
        for i in range(5):
            for j in range(i + 1):
                assert not mask[i, j].item(), f"mask[{i},{j}] should be False (not masked)"
            for j in range(i + 1, 5):
                assert mask[i, j].item(), f"mask[{i},{j}] should be True (masked)"

    def test_gradient_flows(self):
        decoder = AutoregressiveDecoder(encoder_dim=64, decoder_dim=32, num_layers=1, num_heads=2)
        board_emb = torch.randn(1, NUM_AREAS, 64, requires_grad=True)
        unit_indices = torch.randint(0, NUM_AREAS, (1, 3))
        power_indices = torch.zeros(1, dtype=torch.long)
        target_orders = torch.zeros(1, 3, ORDER_VOCAB_SIZE)
        target_orders[:, :, 0] = 1.0
        target_orders[:, :, 7] = 1.0

        logits = decoder.forward_teacher_forcing(
            board_emb, unit_indices, power_indices, target_orders
        )
        loss = logits.sum()
        loss.backward()
        assert board_emb.grad is not None, "Gradient should flow to board embeddings"

    def test_parameter_count(self):
        decoder = AutoregressiveDecoder(
            encoder_dim=512, decoder_dim=256, num_layers=2, num_heads=4
        )
        n = decoder.count_parameters()
        # 2 layers, 256-d: roughly 1-3M params
        assert 500_000 < n < 5_000_000, f"Decoder param count {n:,} outside expected range"


class TestFullModel:
    """Test the full autoregressive policy network."""

    def test_teacher_forcing_forward_shape(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        batch = _make_dummy_batch(batch_size=2, max_orders=4)
        adj = _make_dummy_adj()

        logits = model(
            batch["board"], adj, batch["unit_indices"], batch["power_idx"],
            target_orders=batch["order_labels"],
        )
        assert logits.shape == (2, 4, ORDER_VOCAB_SIZE), f"Got {logits.shape}"

    def test_inference_forward_shape(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        batch = _make_dummy_batch(batch_size=2, max_orders=4)
        adj = _make_dummy_adj()

        logits = model(
            batch["board"], adj, batch["unit_indices"], batch["power_idx"],
            target_orders=None,
        )
        assert logits.shape == (2, 4, ORDER_VOCAB_SIZE), f"Got {logits.shape}"

    def test_different_batch_sizes(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        adj = _make_dummy_adj()
        for bs in [1, 4, 8]:
            batch = _make_dummy_batch(batch_size=bs, max_orders=5)
            logits = model(
                batch["board"], adj, batch["unit_indices"], batch["power_idx"],
                target_orders=batch["order_labels"],
            )
            assert logits.shape[0] == bs

    def test_encoder_shape(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        board = torch.randn(3, NUM_AREAS, NUM_FEATURES)
        adj = _make_dummy_adj()
        power_idx = torch.randint(0, NUM_POWERS, (3,))
        emb = model.encode(board, adj, power_idx)
        assert emb.shape == (3, NUM_AREAS, 64)

    def test_parameter_count(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=512, num_gat_layers=6, num_heads=8,
            decoder_dim=256, decoder_layers=2, decoder_heads=4,
        )
        total = model.count_parameters()
        enc = model.count_encoder_parameters()
        dec = model.count_decoder_parameters()

        # Total should be encoder + decoder
        # Allow small discrepancy from shared params
        assert abs(total - enc - dec) < 100, (
            f"Total {total:,} != enc {enc:,} + dec {dec:,}"
        )
        # Encoder should be ~15M (same as original model)
        assert 10_000_000 < enc < 20_000_000, f"Encoder params {enc:,} outside range"
        # Decoder should be ~1-3M
        assert 500_000 < dec < 5_000_000, f"Decoder params {dec:,} outside range"


class TestEncoderTransfer:
    """Test encoder weight transfer from pretrained model."""

    def test_load_encoder_from_policy(self):
        # Create pretrained independent model
        torch.manual_seed(42)
        pretrained = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)

        # Create AR model and transfer
        ar_model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        ar_model.load_encoder_from_policy(pretrained)

        # Encoder weights should match
        for (name1, p1), (name2, p2) in zip(
            pretrained.input_proj.named_parameters(),
            ar_model.input_proj.named_parameters(),
        ):
            assert torch.allclose(p1, p2), f"input_proj.{name1} mismatch"

        for (name1, p1), (name2, p2) in zip(
            pretrained.gat_blocks.named_parameters(),
            ar_model.gat_blocks.named_parameters(),
        ):
            assert torch.allclose(p1, p2), f"gat_blocks.{name1} mismatch"

    def test_encoder_outputs_match_after_transfer(self):
        torch.manual_seed(42)
        pretrained = DiplomacyPolicyNet(hidden_dim=64, num_gat_layers=2, num_heads=2)

        ar_model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        ar_model.load_encoder_from_policy(pretrained)

        pretrained.eval()
        ar_model.eval()

        torch.manual_seed(99)
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        adj = _make_dummy_adj()
        power_idx = torch.zeros(1, dtype=torch.long)

        with torch.no_grad():
            pretrained_emb = pretrained.encode(board, adj)
            power_emb = pretrained.power_embed(power_idx)
            pretrained_emb = pretrained_emb + power_emb.unsqueeze(1)

            ar_emb = ar_model.encode(board, adj, power_idx)

        max_diff = (pretrained_emb - ar_emb).abs().max().item()
        assert torch.allclose(pretrained_emb, ar_emb, atol=1e-4), (
            f"Encoder outputs should match after weight transfer (max_diff={max_diff:.2e})"
        )


class TestLossAndAccuracy:
    """Test loss and accuracy computation with AR model."""

    def test_loss_is_finite(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        adj = _make_dummy_adj()

        logits = model(
            batch["board"], adj, batch["unit_indices"], batch["power_idx"],
            target_orders=batch["order_labels"],
        )
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
        assert torch.isfinite(loss), f"Loss is not finite: {loss.item()}"
        assert loss.item() >= 0, "KL divergence loss should be non-negative"

    def test_loss_masked_properly(self):
        batch = _make_dummy_batch(batch_size=2, max_orders=5)
        logits = torch.randn(2, 5, ORDER_VOCAB_SIZE)
        batch["order_mask"] = torch.zeros(2, 5)
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])
        assert loss.item() == 0.0, "Loss should be zero when all orders are masked"

    def test_accuracy_perfect_prediction(self):
        batch = _make_dummy_batch(batch_size=4, max_orders=3)
        logits = batch["order_labels"] * 100.0
        acc = compute_accuracy(logits, batch["order_labels"], batch["order_mask"], top_k=1)
        assert acc == 1.0, f"Expected 1.0 for perfect prediction, got {acc}"


class TestCausality:
    """Test that the decoder respects causal ordering."""

    def test_later_targets_dont_affect_earlier_outputs(self):
        """Changing target orders at position j>i should not change logits at position i."""
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        model.eval()
        adj = _make_dummy_adj()
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        unit_indices = torch.randint(0, NUM_AREAS, (1, 4))
        power_idx = torch.zeros(1, dtype=torch.long)

        # Target set A
        target_a = torch.zeros(1, 4, ORDER_VOCAB_SIZE)
        target_a[0, 0, 0] = 1.0; target_a[0, 0, 7] = 1.0  # hold at 0
        target_a[0, 1, 1] = 1.0; target_a[0, 1, 8] = 1.0  # move from 1
        target_a[0, 2, 0] = 1.0; target_a[0, 2, 9] = 1.0  # hold at 2
        target_a[0, 3, 0] = 1.0; target_a[0, 3, 10] = 1.0  # hold at 3

        # Target set B: differs only at position 3
        target_b = target_a.clone()
        target_b[0, 3, 0] = 0.0; target_b[0, 3, 10] = 0.0
        target_b[0, 3, 1] = 1.0; target_b[0, 3, 11] = 1.0  # move from 4

        with torch.no_grad():
            logits_a = model(board, adj, unit_indices, power_idx, target_orders=target_a)
            logits_b = model(board, adj, unit_indices, power_idx, target_orders=target_b)

        # Positions 0, 1, 2 should be identical (only position 3 target changed)
        for pos in range(3):
            assert torch.allclose(logits_a[0, pos], logits_b[0, pos], atol=1e-5), (
                f"Position {pos} logits changed when only position 3 target was modified"
            )

        # Position 3 may differ (it sees its own shifted input from position 2,
        # but position 2's target is the same, so position 3 should also match)
        # Actually: target shift means position 3 sees target[2] which is the same.
        # So position 3 should also match.
        assert torch.allclose(logits_a[0, 3], logits_b[0, 3], atol=1e-5), (
            "Position 3 should match since it only sees target[0..2] which are identical"
        )

    def test_earlier_target_change_affects_later(self):
        """Changing target at position i should affect logits at position i+1."""
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        model.eval()
        adj = _make_dummy_adj()
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        unit_indices = torch.randint(0, NUM_AREAS, (1, 4))
        power_idx = torch.zeros(1, dtype=torch.long)

        # Target set A
        target_a = torch.zeros(1, 4, ORDER_VOCAB_SIZE)
        target_a[0, 0, 0] = 1.0; target_a[0, 0, 7] = 1.0
        target_a[0, 1, 0] = 1.0; target_a[0, 1, 8] = 1.0
        target_a[0, 2, 0] = 1.0; target_a[0, 2, 9] = 1.0
        target_a[0, 3, 0] = 1.0; target_a[0, 3, 10] = 1.0

        # Target set B: differs at position 0
        target_b = target_a.clone()
        target_b[0, 0, 0] = 0.0; target_b[0, 0, 7] = 0.0
        target_b[0, 0, 1] = 1.0; target_b[0, 0, 20] = 1.0  # move from 13

        with torch.no_grad():
            logits_a = model(board, adj, unit_indices, power_idx, target_orders=target_a)
            logits_b = model(board, adj, unit_indices, power_idx, target_orders=target_b)

        # Position 0 should be the same (sees only SOS, not its own target)
        assert torch.allclose(logits_a[0, 0], logits_b[0, 0], atol=1e-5), (
            "Position 0 should be identical (both see SOS)"
        )

        # Position 1 should differ (sees target[0] which changed)
        diff = (logits_a[0, 1] - logits_b[0, 1]).abs().max()
        assert diff > 1e-4, (
            f"Position 1 should differ when position 0 target changes (max diff={diff:.6f})"
        )


class TestEndToEnd:
    """Test training convergence and full pipeline."""

    def test_single_training_step(self):
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4, max_orders=3)

        model.train()
        logits = model(
            batch["board"], adj, batch["unit_indices"], batch["power_idx"],
            target_orders=batch["order_labels"],
        )
        loss = compute_loss(logits, batch["order_labels"], batch["order_mask"])

        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

        assert torch.isfinite(loss), f"Loss is not finite: {loss.item()}"

    def test_loss_decreases_over_steps(self):
        """Verify model can overfit a single batch."""
        model = DiplomacyAutoRegressivePolicyNet(
            hidden_dim=64, num_gat_layers=2, num_heads=2,
            decoder_dim=32, decoder_layers=1, decoder_heads=2,
        )
        optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)
        adj = _make_dummy_adj()
        batch = _make_dummy_batch(batch_size=4, max_orders=3)

        initial_loss = None
        for step in range(50):
            model.train()
            logits = model(
                batch["board"], adj, batch["unit_indices"], batch["power_idx"],
                target_orders=batch["order_labels"],
            )
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

    def test_dataset_loading(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=10, max_orders=4)
            ds = DiplomacyDataset(npz_path)
            assert len(ds) == 10
            sample = ds[0]
            assert sample["board"].shape == (NUM_AREAS, NUM_FEATURES)
            assert sample["order_labels"].shape[1] == ORDER_VOCAB_SIZE

    def test_collate_and_forward(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            npz_path = _make_dummy_npz(tmpdir, n_samples=8, max_orders=4)
            ds = DiplomacyDataset(npz_path)
            batch_items = [ds[i] for i in range(4)]
            batch = collate_fn(batch_items)

            model = DiplomacyAutoRegressivePolicyNet(
                hidden_dim=64, num_gat_layers=2, num_heads=2,
                decoder_dim=32, decoder_layers=1, decoder_heads=2,
            )
            adj = _make_dummy_adj()

            logits = model(
                batch["board"], adj, batch["unit_indices"], batch["power_idx"],
                target_orders=batch["order_labels"],
            )
            assert logits.shape[0] == 4
            assert logits.shape[2] == ORDER_VOCAB_SIZE

    def test_dataset_orders_sorted_by_province(self):
        """Verify dataset sorts orders by source province index."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create data where orders are intentionally unsorted
            n_samples = 5
            max_orders = 4
            boards = np.random.randn(n_samples, NUM_AREAS, NUM_FEATURES).astype(np.float32)
            order_labels = np.zeros((n_samples, max_orders, ORDER_VOCAB_SIZE), dtype=np.float32)
            order_masks = np.zeros((n_samples, max_orders), dtype=np.float32)
            power_indices = np.zeros(n_samples, dtype=np.int32)
            values = np.zeros((n_samples, 4), dtype=np.float32)

            # Sample 0: provinces [30, 10, 20] (unsorted)
            for j, src in enumerate([30, 10, 20]):
                order_labels[0, j, 0] = 1.0  # hold
                order_labels[0, j, 7 + src] = 1.0
                order_masks[0, j] = 1.0

            path = Path(tmpdir) / "test.npz"
            np.savez_compressed(
                path, boards=boards, order_labels=order_labels,
                order_masks=order_masks, power_indices=power_indices,
                values=values, years=np.full(n_samples, 1901, dtype=np.int32),
            )

            ds = DiplomacyDataset(path)
            sample = ds[0]
            unit_idx = sample["unit_indices"]
            valid = unit_idx[:3]

            # Should be sorted: [10, 20, 30]
            assert valid[0].item() == 10, f"Expected 10, got {valid[0].item()}"
            assert valid[1].item() == 20, f"Expected 20, got {valid[1].item()}"
            assert valid[2].item() == 30, f"Expected 30, got {valid[2].item()}"

            # Order labels should be reordered to match
            src_10 = sample["order_labels"][0, 7 + 10].item()
            src_20 = sample["order_labels"][1, 7 + 20].item()
            src_30 = sample["order_labels"][2, 7 + 30].item()
            assert src_10 == 1.0, "First order should have src=10"
            assert src_20 == 1.0, "Second order should have src=20"
            assert src_30 == 1.0, "Third order should have src=30"


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        TestOrderEmbedding,
        TestAutoRegressiveDecoder,
        TestFullModel,
        TestEncoderTransfer,
        TestLossAndAccuracy,
        TestCausality,
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
