#!/usr/bin/env python3
"""Tests for ONNX export of policy and value networks.

Verifies that exported models load correctly, produce outputs matching
the PyTorch originals within tolerance, and meet size/latency targets.
"""

import sys
import tempfile
import time
from pathlib import Path

import numpy as np
import onnx
import onnxruntime as ort
import torch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from gnn import DiplomacyPolicyNet
from value_net import DiplomacyValueNet

from export_onnx import (
    NUM_AREAS,
    NUM_FEATURES,
    NUM_POWERS,
    ORDER_VOCAB_SIZE,
    export_policy,
    export_value,
    make_dummy_adj,
    quantize_model,
    validate_policy,
    validate_value,
)

HIDDEN_DIM = 64
NUM_GAT_LAYERS = 2
NUM_HEADS = 2


def _make_policy() -> DiplomacyPolicyNet:
    """Create a small policy model for testing."""
    return DiplomacyPolicyNet(
        hidden_dim=HIDDEN_DIM, num_gat_layers=NUM_GAT_LAYERS, num_heads=NUM_HEADS
    )


def _make_value() -> DiplomacyValueNet:
    """Create a small value model for testing."""
    return DiplomacyValueNet(
        hidden_dim=HIDDEN_DIM, num_gat_layers=NUM_GAT_LAYERS, num_heads=NUM_HEADS
    )


class TestPolicyExport:
    """Test ONNX export of the policy network."""

    def test_export_creates_file(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out)
            assert out.exists(), "ONNX file should exist after export"
            assert out.stat().st_size > 0, "ONNX file should not be empty"

    def test_onnx_model_is_valid(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out)
            onnx_model = onnx.load(str(out))
            onnx.checker.check_model(onnx_model)

    def test_onnx_loads_in_ort(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out)
            session = ort.InferenceSession(str(out))
            inputs = session.get_inputs()
            assert len(inputs) == 4, f"Expected 4 inputs, got {len(inputs)}"
            input_names = {inp.name for inp in inputs}
            assert input_names == {"board", "adj", "unit_indices", "power_indices"}

    def test_output_shape(self):
        model = _make_policy()
        max_units = 5
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out, max_units=max_units)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            board = np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32)
            unit_idx = np.zeros((1, max_units), dtype=np.int64)
            power_idx = np.zeros((1,), dtype=np.int64)

            result = session.run(None, {
                "board": board,
                "adj": adj,
                "unit_indices": unit_idx,
                "power_indices": power_idx,
            })
            assert result[0].shape == (1, max_units, ORDER_VOCAB_SIZE), (
                f"Expected (1, {max_units}, {ORDER_VOCAB_SIZE}), got {result[0].shape}"
            )

    def test_batch_dimension(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out, max_units=5)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            for bs in [1, 4, 8]:
                board = np.random.randn(bs, NUM_AREAS, NUM_FEATURES).astype(np.float32)
                unit_idx = np.zeros((bs, 5), dtype=np.int64)
                power_idx = np.zeros((bs,), dtype=np.int64)

                result = session.run(None, {
                    "board": board,
                    "adj": adj,
                    "unit_indices": unit_idx,
                    "power_indices": power_idx,
                })
                assert result[0].shape[0] == bs, f"Batch {bs}: got shape {result[0].shape}"

    def test_numerical_match(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out, max_units=5)
            max_diff, mean_diff = validate_policy(model, out, n_samples=50, atol=1e-4)
            assert max_diff < 1e-4, f"Max diff {max_diff:.2e} exceeds tolerance 1e-4"


class TestValueExport:
    """Test ONNX export of the value network."""

    def test_export_creates_file(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            assert out.exists(), "ONNX file should exist after export"
            assert out.stat().st_size > 0, "ONNX file should not be empty"

    def test_onnx_model_is_valid(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            onnx_model = onnx.load(str(out))
            onnx.checker.check_model(onnx_model)

    def test_onnx_loads_in_ort(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            session = ort.InferenceSession(str(out))
            inputs = session.get_inputs()
            assert len(inputs) == 3, f"Expected 3 inputs, got {len(inputs)}"
            input_names = {inp.name for inp in inputs}
            assert input_names == {"board", "adj", "power_indices"}

    def test_output_shape(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            board = np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32)
            power_idx = np.zeros((1,), dtype=np.int64)

            result = session.run(None, {
                "board": board,
                "adj": adj,
                "power_indices": power_idx,
            })
            assert result[0].shape == (1, 4), f"Expected (1, 4), got {result[0].shape}"

    def test_output_range(self):
        """Value outputs should be in [0, 1] due to sigmoid."""
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            for _ in range(10):
                board = np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32)
                power_idx = np.random.randint(0, NUM_POWERS, (1,)).astype(np.int64)
                result = session.run(None, {
                    "board": board,
                    "adj": adj,
                    "power_indices": power_idx,
                })
                assert (result[0] >= 0).all() and (result[0] <= 1).all(), (
                    f"Values out of [0,1]: {result[0]}"
                )

    def test_batch_dimension(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            for bs in [1, 4, 8]:
                board = np.random.randn(bs, NUM_AREAS, NUM_FEATURES).astype(np.float32)
                power_idx = np.zeros((bs,), dtype=np.int64)
                result = session.run(None, {
                    "board": board,
                    "adj": adj,
                    "power_indices": power_idx,
                })
                assert result[0].shape == (bs, 4), f"Batch {bs}: got {result[0].shape}"

    def test_numerical_match(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            max_diff, mean_diff = validate_value(model, out, n_samples=50, atol=1e-4)
            assert max_diff < 1e-4, f"Max diff {max_diff:.2e} exceeds tolerance 1e-4"


class TestQuantization:
    """Test INT8 quantization of exported models."""

    def test_policy_quantization(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            fp32_path = Path(tmpdir) / "policy.onnx"
            int8_path = Path(tmpdir) / "policy_int8.onnx"
            export_policy(model, fp32_path)
            quantize_model(fp32_path, int8_path)

            assert int8_path.exists()
            assert int8_path.stat().st_size < fp32_path.stat().st_size, (
                "INT8 model should be smaller than FP32"
            )

            # Should still load and run
            session = ort.InferenceSession(str(int8_path))
            adj = make_dummy_adj().numpy()
            result = session.run(None, {
                "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
                "adj": adj,
                "unit_indices": np.zeros((1, 17), dtype=np.int64),
                "power_indices": np.zeros((1,), dtype=np.int64),
            })
            assert result[0].shape[0] == 1

    def test_value_quantization(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            fp32_path = Path(tmpdir) / "value.onnx"
            int8_path = Path(tmpdir) / "value_int8.onnx"
            export_value(model, fp32_path)
            quantize_model(fp32_path, int8_path)

            assert int8_path.exists()
            assert int8_path.stat().st_size < fp32_path.stat().st_size

            session = ort.InferenceSession(str(int8_path))
            adj = make_dummy_adj().numpy()
            result = session.run(None, {
                "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
                "adj": adj,
                "power_indices": np.zeros((1,), dtype=np.int64),
            })
            assert result[0].shape == (1, 4)

    def test_int8_value_output_reasonable(self):
        """INT8 quantized value model should produce reasonable outputs."""
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            fp32_path = Path(tmpdir) / "value.onnx"
            int8_path = Path(tmpdir) / "value_int8.onnx"
            export_value(model, fp32_path)
            quantize_model(fp32_path, int8_path)

            fp32_sess = ort.InferenceSession(str(fp32_path))
            int8_sess = ort.InferenceSession(str(int8_path))

            adj = make_dummy_adj().numpy()
            diffs = []
            for _ in range(20):
                feed = {
                    "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
                    "adj": adj,
                    "power_indices": np.random.randint(0, NUM_POWERS, (1,)).astype(np.int64),
                }
                fp32_out = fp32_sess.run(None, feed)[0]
                int8_out = int8_sess.run(None, feed)[0]
                diffs.append(np.abs(fp32_out - int8_out).max())

            max_quant_diff = max(diffs)
            # INT8 tolerance is more relaxed
            assert max_quant_diff < 0.1, (
                f"INT8 vs FP32 max diff {max_quant_diff:.4f} exceeds 0.1"
            )


class TestLatency:
    """Test inference latency meets targets."""

    def test_policy_latency(self):
        model = _make_policy()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "policy.onnx"
            export_policy(model, out)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            feed = {
                "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
                "adj": adj,
                "unit_indices": np.zeros((1, 17), dtype=np.int64),
                "power_indices": np.zeros((1,), dtype=np.int64),
            }

            # Warmup
            for _ in range(5):
                session.run(None, feed)

            times = []
            for _ in range(50):
                t0 = time.perf_counter()
                session.run(None, feed)
                times.append((time.perf_counter() - t0) * 1000)

            times.sort()
            median = times[len(times) // 2]
            # Small model should be fast; full-size model target is <5ms
            assert median < 50, f"Policy latency {median:.1f}ms exceeds 50ms (small model)"

    def test_value_latency(self):
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "value.onnx"
            export_value(model, out)
            session = ort.InferenceSession(str(out))

            adj = make_dummy_adj().numpy()
            feed = {
                "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
                "adj": adj,
                "power_indices": np.zeros((1,), dtype=np.int64),
            }

            for _ in range(5):
                session.run(None, feed)

            times = []
            for _ in range(50):
                t0 = time.perf_counter()
                session.run(None, feed)
                times.append((time.perf_counter() - t0) * 1000)

            times.sort()
            median = times[len(times) // 2]
            assert median < 50, f"Value latency {median:.1f}ms exceeds 50ms (small model)"


class TestReproducibility:
    """Test that export is reproducible."""

    def test_same_model_same_onnx(self):
        """Exporting the same model twice should produce identical ONNX files."""
        torch.manual_seed(42)
        model = _make_policy()

        with tempfile.TemporaryDirectory() as tmpdir:
            out1 = Path(tmpdir) / "policy1.onnx"
            out2 = Path(tmpdir) / "policy2.onnx"
            export_policy(model, out1)
            export_policy(model, out2)

            # Files should be identical byte-for-byte
            b1 = out1.read_bytes()
            b2 = out2.read_bytes()
            assert b1 == b2, "Same model should produce identical ONNX exports"

    def test_checkpoint_roundtrip(self):
        """Export from checkpoint, reload, re-export: should match."""
        model = _make_value()
        with tempfile.TemporaryDirectory() as tmpdir:
            # Save checkpoint
            ckpt_path = Path(tmpdir) / "value.pt"
            torch.save({"model_state_dict": model.state_dict()}, ckpt_path)

            # Export original
            out1 = Path(tmpdir) / "value1.onnx"
            export_value(model, out1)

            # Reload and re-export
            model2 = _make_value()
            ckpt = torch.load(ckpt_path, weights_only=True)
            model2.load_state_dict(ckpt["model_state_dict"])
            out2 = Path(tmpdir) / "value2.onnx"
            export_value(model2, out2)

            b1 = out1.read_bytes()
            b2 = out2.read_bytes()
            assert b1 == b2, "Checkpoint roundtrip should produce identical ONNX"


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        TestPolicyExport,
        TestValueExport,
        TestQuantization,
        TestLatency,
        TestReproducibility,
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
