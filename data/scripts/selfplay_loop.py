#!/usr/bin/env python3
"""Self-play RL loop orchestration script.

Iteratively generates self-play games, converts to training data, trains
policy and value networks, exports ONNX models, deploys to the Rust engine,
and optionally evaluates the new model against the previous iteration.

Usage:
    python3 data/scripts/selfplay_loop.py \
      --iterations 10 \
      --games-per-iter 200 \
      --movetime 1000 \
      --supervised-data data/processed/train.npz \
      --supervised-checkpoint checkpoints/best_policy.pt \
      --output-dir data/selfplay/

See --help for all options.
"""

import argparse
import json
import logging
import shutil
import subprocess
import sys
import time
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)


def find_project_root() -> Path:
    """Walk up from this script to find the project root (contains engine/ and data/)."""
    p = Path(__file__).resolve().parent
    for _ in range(10):
        if (p / "engine").is_dir() and (p / "data").is_dir():
            return p
        p = p.parent
    log.error("Could not find project root (expected engine/ and data/ directories)")
    sys.exit(1)


def run_cmd(cmd: list[str], label: str, dry_run: bool = False, cwd: str | None = None) -> int:
    """Run a subprocess command, logging the invocation and timing.

    Returns the process return code (0 on success).
    """
    cmd_str = " ".join(str(c) for c in cmd)
    log.info("[%s] %s", label, cmd_str)
    if dry_run:
        log.info("[%s] (dry run, skipping)", label)
        return 0

    t0 = time.time()
    result = subprocess.run(cmd, cwd=cwd)
    elapsed = time.time() - t0
    if result.returncode != 0:
        log.error("[%s] FAILED (exit code %d) after %.1fs", label, result.returncode, elapsed)
    else:
        log.info("[%s] completed in %.1fs", label, elapsed)
    return result.returncode


def step_completed(marker: Path) -> bool:
    """Check whether a step has already produced its output file."""
    return marker.exists() and marker.stat().st_size > 0


def generate_selfplay(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 1: Generate self-play games using the Rust engine."""
    output_file = iter_dir / "games.jsonl"
    if not args.force and step_completed(output_file):
        log.info("Skipping selfplay generation (output exists): %s", output_file)
        return True
    if args.skip_generate:
        if not step_completed(output_file):
            log.error("--skip-generate but no games.jsonl found at %s", output_file)
            return False
        return True

    iter_dir.mkdir(parents=True, exist_ok=True)
    selfplay_bin = root / args.selfplay_bin

    cmd = [
        str(selfplay_bin),
        "--games", str(args.games_per_iter),
        "--movetime", str(args.movetime),
        "--strength", str(args.strength),
        "--temperature", str(args.temperature),
        "--threads", str(args.threads),
        "--output", str(output_file),
    ]
    if args.seed:
        cmd.extend(["--seed", str(args.seed + iteration)])

    rc = run_cmd(cmd, f"iter {iteration} selfplay", dry_run=args.dry_run)
    return rc == 0


def convert_data(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 2: Convert JSONL to NPZ training data."""
    train_file = iter_dir / "train.npz"
    if not args.force and step_completed(train_file):
        log.info("Skipping conversion (output exists): %s", train_file)
        return True

    script = root / "data" / "scripts" / "convert_selfplay.py"
    cmd = [
        sys.executable, str(script),
        "--input", str(iter_dir / "games.jsonl"),
        "--output-dir", str(iter_dir),
    ]

    rc = run_cmd(cmd, f"iter {iteration} convert", dry_run=args.dry_run)
    return rc == 0


def find_previous_policy(args, root: Path, iteration: int) -> str | None:
    """Find the best policy checkpoint from the previous iteration (or init checkpoint)."""
    if iteration <= args.start_iter:
        if args.init_checkpoint:
            return str(root / args.init_checkpoint)
        return None

    prev_dir = Path(args.output_dir) / f"iter_{iteration - 1}" / "checkpoints"
    prev_best = prev_dir / "best_rl_policy.pt"
    if prev_best.exists():
        return str(prev_best)

    if args.init_checkpoint:
        return str(root / args.init_checkpoint)
    return None


def train_policy(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 3: Train policy network with REINFORCE + supervised mix."""
    ckpt_dir = iter_dir / "checkpoints"
    best_policy = ckpt_dir / "best_rl_policy.pt"
    if not args.force and step_completed(best_policy):
        log.info("Skipping policy training (output exists): %s", best_policy)
        return True

    ckpt_dir.mkdir(parents=True, exist_ok=True)
    script = root / "data" / "scripts" / "train_policy_rl.py"

    # Adjacency matrix from converted data
    adj_path = iter_dir / "adjacency.npy"
    if not adj_path.exists():
        # Fallback to processed features adjacency
        adj_path = root / "data" / "processed" / "features" / "adjacency.npy"

    cmd = [
        sys.executable, str(script),
        "--selfplay-data", str(iter_dir / "train.npz"),
        "--adj-path", str(adj_path),
        "--checkpoint-dir", str(ckpt_dir),
        "--epochs", str(args.policy_epochs),
    ]

    # Validation data
    val_file = iter_dir / "val.npz"
    if val_file.exists():
        cmd.extend(["--val-data", str(val_file)])

    # Supervised mixing
    if args.supervised_data:
        cmd.extend([
            "--supervised-data", str(root / args.supervised_data),
            "--supervised-mix", str(args.supervised_mix),
        ])

    # Initial checkpoint (from previous iteration or specified)
    prev_policy = find_previous_policy(args, root, iteration)
    if prev_policy:
        cmd.extend(["--init-checkpoint", prev_policy])

    # Reference checkpoint for KL regularization
    if args.supervised_checkpoint:
        cmd.extend(["--supervised-checkpoint", str(root / args.supervised_checkpoint)])

    rc = run_cmd(cmd, f"iter {iteration} train_policy", dry_run=args.dry_run)
    return rc == 0


def train_value(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 4: Train value network on self-play data."""
    ckpt_dir = iter_dir / "checkpoints"
    best_value = ckpt_dir / "best_value.pt"
    if not args.force and step_completed(best_value):
        log.info("Skipping value training (output exists): %s", best_value)
        return True

    ckpt_dir.mkdir(parents=True, exist_ok=True)
    script = root / "data" / "scripts" / "train_value.py"

    cmd = [
        sys.executable, str(script),
        "--data-dir", str(iter_dir),
        "--checkpoint-dir", str(ckpt_dir),
        "--epochs", str(args.value_epochs),
    ]

    # Transfer encoder weights from policy checkpoint if available
    policy_ckpt = ckpt_dir / "best_rl_policy.pt"
    if policy_ckpt.exists():
        cmd.extend(["--policy-checkpoint", str(policy_ckpt)])

    rc = run_cmd(cmd, f"iter {iteration} train_value", dry_run=args.dry_run)
    return rc == 0


def export_onnx(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 5: Export trained models to ONNX format."""
    models_dir = iter_dir / "models"
    policy_onnx = models_dir / "policy_v2.onnx"
    value_onnx = models_dir / "value_v2.onnx"
    if not args.force and step_completed(policy_onnx) and step_completed(value_onnx):
        log.info("Skipping ONNX export (outputs exist): %s", models_dir)
        return True

    models_dir.mkdir(parents=True, exist_ok=True)
    script = root / "data" / "scripts" / "export_onnx.py"

    ckpt_dir = iter_dir / "checkpoints"
    cmd = [
        sys.executable, str(script),
        "--policy-ckpt", str(ckpt_dir / "best_rl_policy.pt"),
        "--value-ckpt", str(ckpt_dir / "best_value.pt"),
        "--out-dir", str(models_dir),
    ]

    rc = run_cmd(cmd, f"iter {iteration} export_onnx", dry_run=args.dry_run)
    return rc == 0


def deploy_to_engine(args, root: Path, iter_dir: Path, iteration: int) -> bool:
    """Step 6: Copy ONNX models to engine/models/ and rebuild the Rust engine."""
    models_dir = iter_dir / "models"
    engine_models = root / "engine" / "models"
    engine_models.mkdir(parents=True, exist_ok=True)

    for name in ["policy_v2.onnx", "value_v2.onnx"]:
        src = models_dir / name
        dst = engine_models / name
        if src.exists():
            log.info("Copying %s -> %s", src, dst)
            if not args.dry_run:
                shutil.copy2(src, dst)
        else:
            log.warning("ONNX model not found: %s", src)

    # Rebuild the engine
    engine_dir = root / "engine"
    cmd = ["cargo", "build", "--release"]
    rc = run_cmd(cmd, f"iter {iteration} cargo build", dry_run=args.dry_run, cwd=str(engine_dir))
    return rc == 0


def evaluate_model(args, root: Path, iter_dir: Path, iteration: int) -> dict | None:
    """Step 7: Evaluate the new model by running self-play games and logging results."""
    if args.skip_eval:
        log.info("Skipping evaluation (--skip-eval)")
        return None

    eval_file = iter_dir / "eval_results.txt"
    if not args.force and step_completed(eval_file):
        log.info("Skipping evaluation (output exists): %s", eval_file)
        return None

    selfplay_bin = root / args.selfplay_bin

    # Run a small set of self-play games with the new model
    eval_output = iter_dir / "eval_games.jsonl"
    cmd = [
        str(selfplay_bin),
        "--games", str(args.eval_games),
        "--movetime", str(args.movetime),
        "--strength", "100",
        "--temperature", "0.5",
        "--threads", str(args.threads),
        "--output", str(eval_output),
    ]

    rc = run_cmd(cmd, f"iter {iteration} eval", dry_run=args.dry_run)
    if rc != 0:
        return None

    if args.dry_run:
        return {"status": "dry_run"}

    # Parse eval games for basic stats
    results = parse_eval_results(eval_output)

    # Write results
    with open(eval_file, "w") as f:
        f.write(f"Iteration {iteration} evaluation\n")
        f.write(f"{'=' * 40}\n")
        f.write(f"Games played: {results.get('total_games', 0)}\n")
        f.write(f"Solo wins:    {results.get('solo_wins', 0)}\n")
        f.write(f"Draws:        {results.get('draws', 0)}\n")
        f.write(f"Avg phases:   {results.get('avg_phases', 0):.1f}\n")
        f.write(f"Avg year:     {results.get('avg_final_year', 0):.1f}\n")
    log.info("Evaluation results written to %s", eval_file)

    return results


def parse_eval_results(jsonl_path: Path) -> dict:
    """Parse evaluation game JSONL and compute summary statistics."""
    if not jsonl_path.exists():
        return {}

    games = []
    with open(jsonl_path) as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    games.append(json.loads(line))
                except json.JSONDecodeError:
                    continue

    if not games:
        return {"total_games": 0}

    solo_wins = sum(1 for g in games if g.get("winner"))
    draws = sum(1 for g in games if not g.get("winner"))
    total_phases = sum(len(g.get("phases", [])) for g in games)
    avg_phases = total_phases / len(games)

    # Estimate final year from last phase DFEN
    years = []
    for g in games:
        phases = g.get("phases", [])
        if phases:
            last_dfen = phases[-1].get("dfen", "")
            try:
                year_str = last_dfen.split("/")[0]
                year = int(year_str[:-2])
                years.append(year)
            except (ValueError, IndexError):
                pass

    avg_year = sum(years) / len(years) if years else 0

    return {
        "total_games": len(games),
        "solo_wins": solo_wins,
        "draws": draws,
        "avg_phases": avg_phases,
        "avg_final_year": avg_year,
    }


def print_iteration_summary(iteration: int, timings: dict, eval_results: dict | None):
    """Print a summary of a completed iteration."""
    print(f"\n{'=' * 60}")
    print(f"Iteration {iteration} Summary")
    print(f"{'=' * 60}")
    for step_name, elapsed in timings.items():
        print(f"  {step_name:20s}: {elapsed:7.1f}s")
    total = sum(timings.values())
    print(f"  {'TOTAL':20s}: {total:7.1f}s")

    if eval_results and eval_results.get("total_games", 0) > 0:
        print(f"\n  Evaluation:")
        print(f"    Games:      {eval_results['total_games']}")
        print(f"    Solo wins:  {eval_results['solo_wins']}")
        print(f"    Draws:      {eval_results['draws']}")
        print(f"    Avg phases: {eval_results['avg_phases']:.1f}")
        print(f"    Avg year:   {eval_results['avg_final_year']:.1f}")
    print(f"{'=' * 60}\n")


def run_iteration(args, root: Path, iteration: int) -> bool:
    """Run a single iteration of the self-play loop."""
    iter_dir = Path(args.output_dir) / f"iter_{iteration}"
    iter_dir.mkdir(parents=True, exist_ok=True)
    timings = {}

    log.info("=" * 60)
    log.info("Starting iteration %d", iteration)
    log.info("=" * 60)

    # Step 1: Generate self-play games
    t0 = time.time()
    if not generate_selfplay(args, root, iter_dir, iteration):
        return False
    timings["selfplay"] = time.time() - t0

    # Step 2: Convert to NPZ
    t0 = time.time()
    if not convert_data(args, root, iter_dir, iteration):
        return False
    timings["convert"] = time.time() - t0

    # Step 3: Train policy
    t0 = time.time()
    if not train_policy(args, root, iter_dir, iteration):
        return False
    timings["train_policy"] = time.time() - t0

    # Step 4: Train value
    t0 = time.time()
    if not train_value(args, root, iter_dir, iteration):
        return False
    timings["train_value"] = time.time() - t0

    # Step 5: Export ONNX
    t0 = time.time()
    if not export_onnx(args, root, iter_dir, iteration):
        return False
    timings["export_onnx"] = time.time() - t0

    # Step 6: Deploy to engine
    t0 = time.time()
    if not deploy_to_engine(args, root, iter_dir, iteration):
        return False
    timings["deploy"] = time.time() - t0

    # Step 7: Evaluate
    t0 = time.time()
    eval_results = evaluate_model(args, root, iter_dir, iteration)
    timings["evaluate"] = time.time() - t0

    # Save iteration metadata
    meta = {
        "iteration": iteration,
        "timings": timings,
        "eval_results": eval_results,
        "args": {k: str(v) for k, v in vars(args).items()},
    }
    meta_path = iter_dir / "iteration_meta.json"
    if not args.dry_run:
        with open(meta_path, "w") as f:
            json.dump(meta, f, indent=2)

    print_iteration_summary(iteration, timings, eval_results)
    return True


def main():
    parser = argparse.ArgumentParser(
        description="Self-play RL loop orchestration for Diplomacy engine",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    # Loop control
    parser.add_argument("--iterations", type=int, default=10,
                        help="Number of self-play iterations to run")
    parser.add_argument("--start-iter", type=int, default=1,
                        help="Starting iteration number (for resuming)")

    # Self-play generation
    parser.add_argument("--games-per-iter", type=int, default=200,
                        help="Number of self-play games per iteration")
    parser.add_argument("--movetime", type=int, default=1000,
                        help="Search time per move in milliseconds")
    parser.add_argument("--strength", type=int, default=100,
                        help="Engine strength 1-100")
    parser.add_argument("--temperature", type=float, default=1.0,
                        help="Exploration temperature for self-play")
    parser.add_argument("--threads", type=int, default=4,
                        help="Number of parallel threads for self-play")
    parser.add_argument("--seed", type=int, default=0,
                        help="Random seed (0 for entropy). Per-iteration seed = seed + iteration.")

    # Training
    parser.add_argument("--policy-epochs", type=int, default=10,
                        help="Training epochs for policy network per iteration")
    parser.add_argument("--value-epochs", type=int, default=10,
                        help="Training epochs for value network per iteration")
    parser.add_argument("--supervised-data", type=str, default="",
                        help="Path to supervised NPZ data for mixed training (relative to project root)")
    parser.add_argument("--supervised-checkpoint", type=str, default="",
                        help="Frozen supervised checkpoint for KL regularization (relative to project root)")
    parser.add_argument("--supervised-mix", type=float, default=0.3,
                        help="Fraction of supervised data in mixed training")
    parser.add_argument("--init-checkpoint", type=str, default="",
                        help="Initial policy checkpoint for the first iteration (relative to project root)")

    # Evaluation
    parser.add_argument("--eval-games", type=int, default=20,
                        help="Number of evaluation games per iteration")
    parser.add_argument("--skip-eval", action="store_true",
                        help="Skip evaluation step for faster iteration")

    # Paths
    parser.add_argument("--output-dir", type=str, default="data/selfplay",
                        help="Base output directory for all iterations")
    parser.add_argument("--selfplay-bin", type=str, default="engine/target/release/selfplay",
                        help="Path to selfplay binary (relative to project root)")
    parser.add_argument("--project-root", type=str, default="",
                        help="Project root directory (auto-detected if empty)")

    # Execution control
    parser.add_argument("--dry-run", action="store_true",
                        help="Print commands without executing them")
    parser.add_argument("--force", action="store_true",
                        help="Force re-run of steps even if output files exist")
    parser.add_argument("--skip-generate", action="store_true",
                        help="Skip selfplay generation (use existing JSONL)")

    args = parser.parse_args()

    # Resolve project root
    if args.project_root:
        root = Path(args.project_root).resolve()
    else:
        root = find_project_root()
    log.info("Project root: %s", root)

    # Make output-dir absolute
    if not Path(args.output_dir).is_absolute():
        args.output_dir = str(root / args.output_dir)

    # Validate selfplay binary exists (unless dry run)
    selfplay_path = root / args.selfplay_bin
    if not args.dry_run and not selfplay_path.exists():
        log.error("Selfplay binary not found: %s", selfplay_path)
        log.error("Build it first: cd engine && cargo build --release")
        sys.exit(1)

    # Print configuration
    end_iter = args.start_iter + args.iterations - 1
    log.info("Self-play RL loop: iterations %d-%d", args.start_iter, end_iter)
    log.info("  Games/iter:  %d", args.games_per_iter)
    log.info("  Movetime:    %d ms", args.movetime)
    log.info("  Strength:    %d", args.strength)
    log.info("  Temperature: %.2f", args.temperature)
    log.info("  Threads:     %d", args.threads)
    log.info("  Output:      %s", args.output_dir)
    if args.supervised_data:
        log.info("  Supervised:  %s (mix=%.1f%%)", args.supervised_data, args.supervised_mix * 100)
    if args.dry_run:
        log.info("  DRY RUN MODE")

    # Run iterations
    loop_start = time.time()
    completed = 0
    for iteration in range(args.start_iter, end_iter + 1):
        if not run_iteration(args, root, iteration):
            log.error("Iteration %d failed, stopping loop", iteration)
            break
        completed += 1

    loop_elapsed = time.time() - loop_start

    # Final summary
    print(f"\n{'=' * 60}")
    print("Self-Play RL Loop Complete")
    print(f"{'=' * 60}")
    print(f"Iterations completed: {completed}/{args.iterations}")
    print(f"Total time:           {loop_elapsed:.1f}s ({loop_elapsed / 3600:.1f}h)")
    if completed > 0:
        print(f"Avg time/iter:        {loop_elapsed / completed:.1f}s")
    print(f"Output directory:     {args.output_dir}")
    print(f"{'=' * 60}")


if __name__ == "__main__":
    main()
