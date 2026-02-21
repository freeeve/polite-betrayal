#!/usr/bin/env bash
set -euo pipefail

# Usage: ./run_benchmark.sh [engine] [opponent] [games]
#   engine:   rust, gonnx (default: rust)
#   opponent: easy, medium, hard (default: medium)
#   games:    number of games (default: 100)
#
# Examples:
#   ./run_benchmark.sh                    # rust vs medium, 100 games
#   ./run_benchmark.sh rust hard 50       # rust vs hard, 50 games
#   ./run_benchmark.sh gonnx medium 100   # gonnx vs medium, 100 games
#   ./run_benchmark.sh gonnx hard 50      # gonnx vs hard, 50 games

ENGINE="${1:-rust}"
OPPONENT="${2:-medium}"
GAMES="${3:-100}"

MODELS_REPO="../polite-betrayal-models"
CURRENT_MODEL="$MODELS_REPO/current"
ENGINE_MODELS="engine/models"

# Map engine + opponent to test function name
case "$ENGINE" in
  rust)
    case "$OPPONENT" in
      easy)   TEST_FUNC="TestBenchmark_RustVsEasyAllPowers" ;;
      medium) TEST_FUNC="TestBenchmark_RustVsMediumAllPowers" ;;
      hard)   TEST_FUNC="TestBenchmark_RustVsHardAllPowers" ;;
      *)      echo "Unknown opponent: $OPPONENT (use easy, medium, or hard)"; exit 1 ;;
    esac
    ;;
  gonnx)
    case "$OPPONENT" in
      medium) TEST_FUNC="TestBenchmark_GonnxVsMedium" ;;
      *)      echo "gonnx currently only supports medium opponent"; exit 1 ;;
    esac
    ;;
  *)
    echo "Unknown engine: $ENGINE (use rust or gonnx)"; exit 1 ;;
esac

# Deploy latest model from models repo
if [ ! -L "$CURRENT_MODEL" ]; then
  echo "Error: $CURRENT_MODEL symlink not found. Clone the models repo first."
  exit 1
fi

echo "=== Code version ==="
echo "  Commit: $(git log -1 --format='%h %s')"
DIRTY="$(git status --porcelain)"
if [ -n "$DIRTY" ]; then
  echo "  WARNING: uncommitted changes:"
  echo "$DIRTY" | sed 's/^/    /'
else
  echo "  Working tree clean"
fi

echo "=== Deploying latest model ==="
MODEL_TAG="$(basename "$(readlink "$CURRENT_MODEL")")"
echo "  Registry tag: $MODEL_TAG"
if [ -f "$CURRENT_MODEL/metadata.json" ]; then
  echo "  Metadata: $(cat "$CURRENT_MODEL/metadata.json" | python3 -c "import sys,json; m=json.load(sys.stdin); print(f\"training={m.get('training','?')} params={m.get('params','?')}\")" 2>/dev/null || echo "parse error")"
fi

for f in "$CURRENT_MODEL"/*.onnx; do
  name="$(basename "$f")"
  hash="$(shasum -a 256 "$f" | head -c 8)"
  ln -sfn "$(cd "$(dirname "$f")" && pwd)/$name" "$ENGINE_MODELS/$name"
  echo "  Linked $name (sha256: $hash)"
done

# Build Rust engine (needed for rust engine, doesn't hurt for gonnx)
if [ "$ENGINE" = "rust" ]; then
  echo "=== Building Rust engine ==="
  cd engine && cargo build --release
  cd ..
fi

# Set up log file
mkdir -p /tmp/benchmarks
TIMESTAMP="$(date +%Y-%m-%d-%H%M)"
MODEL_HASH="$(shasum -a 256 "$CURRENT_MODEL/policy_v2.onnx" 2>/dev/null | head -c 8 || echo "unknown")"
LOGFILE="/tmp/benchmarks/${TIMESTAMP}-${ENGINE}-vs-${OPPONENT}-${GAMES}g-${MODEL_HASH}.log"

# Run benchmark
echo "=== Running benchmark: $ENGINE vs $OPPONENT ($GAMES games) ==="
echo "  Log: $LOGFILE"
cd api && \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable" \
REALPOLITIK_PATH=/Users/efreeman/polite-betrayal/engine/target/release/realpolitik \
BENCH_GAMES="$GAMES" \
BENCH_SAVE=1 \
go test -v -tags integration -run "$TEST_FUNC" ./internal/bot/ -timeout 1800m 2>&1 | tee "$LOGFILE"
