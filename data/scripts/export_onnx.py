#!/usr/bin/env python3
"""Export trained PyTorch policy and value networks to ONNX format.

Converts DiplomacyPolicyNet and DiplomacyValueNet checkpoints into ONNX
models suitable for inference via the Rust onnxruntime crate.

Usage:
    # Export both models from checkpoints
    python export_onnx.py --policy-ckpt policy.pt --value-ckpt value.pt --out-dir engine/models

    # Export with INT8 quantization
    python export_onnx.py --policy-ckpt policy.pt --value-ckpt value.pt --out-dir engine/models --quantize

    # Export with dummy weights (for testing)
    python export_onnx.py --dummy --out-dir engine/models

    # Validate only (compare PyTorch vs ONNX outputs)
    python export_onnx.py --validate --out-dir engine/models
"""

import argparse
import sys
import time
from pathlib import Path

import numpy as np
import torch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "models"))
from gnn import DiplomacyPolicyNet
from value_net import DiplomacyValueNet

NUM_AREAS = 81
NUM_FEATURES = 47
NUM_POWERS = 7
ORDER_VOCAB_SIZE = 169
OPSET_VERSION = 17


def make_dummy_adj() -> torch.Tensor:
    """Create a simple adjacency matrix for testing."""
    adj = torch.eye(NUM_AREAS)
    for i in range(NUM_AREAS - 1):
        adj[i, i + 1] = 1.0
        adj[i + 1, i] = 1.0
    return adj


class PolicyWrapper(torch.nn.Module):
    """Wraps DiplomacyPolicyNet for clean ONNX export.

    Flattens the forward signature so all inputs are explicit tensors
    (no keyword args or optional parameters), which is required by
    torch.onnx.export.
    """

    def __init__(self, model: DiplomacyPolicyNet):
        super().__init__()
        self.model = model

    def forward(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
    ) -> torch.Tensor:
        return self.model(board, adj, unit_indices, power_indices)


class ValueWrapper(torch.nn.Module):
    """Wraps DiplomacyValueNet for clean ONNX export."""

    def __init__(self, model: DiplomacyValueNet):
        super().__init__()
        self.model = model

    def forward(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        power_indices: torch.Tensor,
    ) -> torch.Tensor:
        return self.model(board, adj, power_indices)


def export_policy(
    model: DiplomacyPolicyNet, out_path: Path, max_units: int = 17
) -> None:
    """Export the policy network to ONNX format.

    Args:
        model: Trained DiplomacyPolicyNet instance.
        out_path: Output .onnx file path.
        max_units: Maximum number of units per batch element.
    """
    model.eval()
    wrapper = PolicyWrapper(model)

    batch_size = 1
    board = torch.randn(batch_size, NUM_AREAS, NUM_FEATURES)
    adj = make_dummy_adj()
    unit_indices = torch.zeros(batch_size, max_units, dtype=torch.long)
    power_indices = torch.zeros(batch_size, dtype=torch.long)

    dynamic_axes = {
        "board": {0: "batch"},
        "adj": {},
        "unit_indices": {0: "batch"},
        "power_indices": {0: "batch"},
        "order_logits": {0: "batch"},
    }

    with torch.no_grad():
        torch.onnx.export(
            wrapper,
            (board, adj, unit_indices, power_indices),
            str(out_path),
            opset_version=OPSET_VERSION,
            input_names=["board", "adj", "unit_indices", "power_indices"],
            output_names=["order_logits"],
            dynamic_axes=dynamic_axes,
            do_constant_folding=True,
            dynamo=False,
        )
    print(f"Policy ONNX exported to {out_path} ({out_path.stat().st_size / 1e6:.1f} MB)")


def export_value(model: DiplomacyValueNet, out_path: Path) -> None:
    """Export the value network to ONNX format.

    Args:
        model: Trained DiplomacyValueNet instance.
        out_path: Output .onnx file path.
    """
    model.eval()
    wrapper = ValueWrapper(model)

    batch_size = 1
    board = torch.randn(batch_size, NUM_AREAS, NUM_FEATURES)
    adj = make_dummy_adj()
    power_indices = torch.zeros(batch_size, dtype=torch.long)

    dynamic_axes = {
        "board": {0: "batch"},
        "adj": {},
        "power_indices": {0: "batch"},
        "value_preds": {0: "batch"},
    }

    with torch.no_grad():
        torch.onnx.export(
            wrapper,
            (board, adj, power_indices),
            str(out_path),
            opset_version=OPSET_VERSION,
            input_names=["board", "adj", "power_indices"],
            output_names=["value_preds"],
            dynamic_axes=dynamic_axes,
            do_constant_folding=True,
            dynamo=False,
        )
    print(f"Value ONNX exported to {out_path} ({out_path.stat().st_size / 1e6:.1f} MB)")


def quantize_model(fp32_path: Path, int8_path: Path) -> None:
    """Apply INT8 dynamic quantization to an ONNX model.

    Args:
        fp32_path: Path to the FP32 ONNX model.
        int8_path: Output path for the quantized model.
    """
    from onnxruntime.quantization import QuantType, quantize_dynamic

    quantize_dynamic(
        str(fp32_path),
        str(int8_path),
        weight_type=QuantType.QInt8,
    )
    print(
        f"Quantized model saved to {int8_path} ({int8_path.stat().st_size / 1e6:.1f} MB)"
    )


def validate_policy(
    pt_model: DiplomacyPolicyNet,
    onnx_path: Path,
    n_samples: int = 100,
    atol: float = 1e-5,
) -> tuple[float, float]:
    """Compare PyTorch and ONNX policy model outputs.

    Returns:
        Tuple of (max_abs_diff, mean_abs_diff).
    """
    import onnxruntime as ort

    pt_model.eval()
    session = ort.InferenceSession(str(onnx_path))
    adj = make_dummy_adj()

    max_diff = 0.0
    sum_diff = 0.0

    # Read max_units dimension from the ONNX model's unit_indices input
    unit_input = [i for i in session.get_inputs() if i.name == "unit_indices"][0]
    max_units = unit_input.shape[1]

    for _ in range(n_samples):
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        unit_indices = torch.randint(0, NUM_AREAS, (1, max_units))
        power_indices = torch.randint(0, NUM_POWERS, (1,))

        with torch.no_grad():
            pt_out = pt_model(board, adj, unit_indices, power_indices).numpy()

        ort_out = session.run(
            None,
            {
                "board": board.numpy(),
                "adj": adj.numpy(),
                "unit_indices": unit_indices.numpy(),
                "power_indices": power_indices.numpy(),
            },
        )[0]

        diff = np.abs(pt_out - ort_out)
        max_diff = max(max_diff, diff.max())
        sum_diff += diff.mean()

    mean_diff = sum_diff / n_samples
    status = "PASS" if max_diff < atol else "FAIL"
    print(f"Policy validation ({status}): max_diff={max_diff:.2e}, mean_diff={mean_diff:.2e}")
    return max_diff, mean_diff


def validate_value(
    pt_model: DiplomacyValueNet,
    onnx_path: Path,
    n_samples: int = 100,
    atol: float = 1e-5,
) -> tuple[float, float]:
    """Compare PyTorch and ONNX value model outputs.

    Returns:
        Tuple of (max_abs_diff, mean_abs_diff).
    """
    import onnxruntime as ort

    pt_model.eval()
    session = ort.InferenceSession(str(onnx_path))
    adj = make_dummy_adj()

    max_diff = 0.0
    sum_diff = 0.0

    for _ in range(n_samples):
        board = torch.randn(1, NUM_AREAS, NUM_FEATURES)
        power_indices = torch.randint(0, NUM_POWERS, (1,))

        with torch.no_grad():
            pt_out = pt_model(board, adj, power_indices).numpy()

        ort_out = session.run(
            None,
            {
                "board": board.numpy(),
                "adj": adj.numpy(),
                "power_indices": power_indices.numpy(),
            },
        )[0]

        diff = np.abs(pt_out - ort_out)
        max_diff = max(max_diff, diff.max())
        sum_diff += diff.mean()

    mean_diff = sum_diff / n_samples
    status = "PASS" if max_diff < atol else "FAIL"
    print(f"Value validation ({status}): max_diff={max_diff:.2e}, mean_diff={mean_diff:.2e}")
    return max_diff, mean_diff


def measure_latency(
    onnx_path: Path, input_feed: dict, n_warmup: int = 10, n_runs: int = 100
) -> float:
    """Measure single-position inference latency in milliseconds.

    Returns:
        Median latency in milliseconds.
    """
    import onnxruntime as ort

    session = ort.InferenceSession(str(onnx_path))

    for _ in range(n_warmup):
        session.run(None, input_feed)

    times = []
    for _ in range(n_runs):
        t0 = time.perf_counter()
        session.run(None, input_feed)
        times.append((time.perf_counter() - t0) * 1000)

    times.sort()
    median_ms = times[len(times) // 2]
    print(f"Latency for {onnx_path.name}: median={median_ms:.2f}ms, p95={times[int(0.95*len(times))]:.2f}ms")
    return median_ms


def load_or_create_policy(ckpt_path: str | None, hidden_dim: int = 256) -> DiplomacyPolicyNet:
    """Load a policy model from checkpoint or create one with random weights."""
    model = DiplomacyPolicyNet(
        hidden_dim=hidden_dim, num_gat_layers=3, num_heads=4
    )
    if ckpt_path:
        ckpt = torch.load(ckpt_path, weights_only=True, map_location="cpu")
        state = ckpt.get("model_state_dict", ckpt)
        model.load_state_dict(state)
        print(f"Loaded policy checkpoint from {ckpt_path}")
    else:
        print("Using random policy weights (dummy mode)")
    return model


def load_or_create_value(ckpt_path: str | None, hidden_dim: int = 256) -> DiplomacyValueNet:
    """Load a value model from checkpoint or create one with random weights."""
    model = DiplomacyValueNet(
        hidden_dim=hidden_dim, num_gat_layers=3, num_heads=4
    )
    if ckpt_path:
        ckpt = torch.load(ckpt_path, weights_only=True, map_location="cpu")
        state = ckpt.get("model_state_dict", ckpt)
        model.load_state_dict(state)
        print(f"Loaded value checkpoint from {ckpt_path}")
    else:
        print("Using random value weights (dummy mode)")
    return model


def main():
    parser = argparse.ArgumentParser(description="Export Diplomacy models to ONNX")
    parser.add_argument("--policy-ckpt", type=str, default=None, help="Path to policy .pt checkpoint")
    parser.add_argument("--value-ckpt", type=str, default=None, help="Path to value .pt checkpoint")
    parser.add_argument("--out-dir", type=str, default="engine/models", help="Output directory")
    parser.add_argument("--dummy", action="store_true", help="Export with random weights for testing")
    parser.add_argument("--quantize", action="store_true", help="Also export INT8 quantized models")
    parser.add_argument("--validate", action="store_true", help="Validate ONNX vs PyTorch outputs")
    parser.add_argument("--hidden-dim", type=int, default=256, help="Model hidden dimension")
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    policy_fp32 = out_dir / "policy_v1.onnx"
    value_fp32 = out_dir / "value_v1.onnx"

    policy_model = None
    value_model = None

    if not args.validate:
        policy_ckpt = None if args.dummy else args.policy_ckpt
        value_ckpt = None if args.dummy else args.value_ckpt

        if not args.dummy and not policy_ckpt and not value_ckpt:
            parser.error("Provide --policy-ckpt / --value-ckpt or use --dummy")

        # Export policy
        if args.dummy or policy_ckpt:
            policy_model = load_or_create_policy(policy_ckpt, args.hidden_dim)
            export_policy(policy_model, policy_fp32)

        # Export value
        if args.dummy or value_ckpt:
            value_model = load_or_create_value(value_ckpt, args.hidden_dim)
            export_value(value_model, value_fp32)

        # Quantize
        if args.quantize:
            if policy_fp32.exists():
                quantize_model(policy_fp32, out_dir / "policy_v1_int8.onnx")
            if value_fp32.exists():
                quantize_model(value_fp32, out_dir / "value_v1_int8.onnx")

    # Validate
    import onnx

    all_pass = True

    if policy_fp32.exists():
        onnx_model = onnx.load(str(policy_fp32))
        onnx.checker.check_model(onnx_model)
        print(f"Policy ONNX model is valid (opset {onnx_model.opset_import[0].version})")

        if policy_model is None:
            policy_model = load_or_create_policy(args.policy_ckpt, args.hidden_dim)
        max_diff, _ = validate_policy(policy_model, policy_fp32)
        if max_diff > 1e-4:
            all_pass = False
            print(f"WARNING: Policy max diff {max_diff:.2e} exceeds 1e-4")

        adj = make_dummy_adj().numpy()
        feed = {
            "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
            "adj": adj.astype(np.float32),
            "unit_indices": np.zeros((1, 17), dtype=np.int64),
            "power_indices": np.zeros((1,), dtype=np.int64),
        }
        measure_latency(policy_fp32, feed)

    if value_fp32.exists():
        onnx_model = onnx.load(str(value_fp32))
        onnx.checker.check_model(onnx_model)
        print(f"Value ONNX model is valid (opset {onnx_model.opset_import[0].version})")

        if value_model is None:
            value_model = load_or_create_value(args.value_ckpt, args.hidden_dim)
        max_diff, _ = validate_value(value_model, value_fp32)
        if max_diff > 1e-4:
            all_pass = False
            print(f"WARNING: Value max diff {max_diff:.2e} exceeds 1e-4")

        adj = make_dummy_adj().numpy()
        feed = {
            "board": np.random.randn(1, NUM_AREAS, NUM_FEATURES).astype(np.float32),
            "adj": adj.astype(np.float32),
            "power_indices": np.zeros((1,), dtype=np.int64),
        }
        measure_latency(value_fp32, feed)

    # Validate INT8 if present
    for name in ["policy_v1_int8.onnx", "value_v1_int8.onnx"]:
        int8_path = out_dir / name
        if int8_path.exists():
            onnx_model = onnx.load(str(int8_path))
            onnx.checker.check_model(onnx_model)
            print(f"{name} is valid")

    if all_pass:
        print("\nAll validations passed.")
    else:
        print("\nSome validations had warnings - check output above.")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
