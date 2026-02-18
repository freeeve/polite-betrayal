#!/bin/bash
# Run integration tests: build the Rust realpolitik engine, then run Go
# integration tests that launch the real engine binary and exchange DUI
# messages to play a Diplomacy game.
#
# Usage:
#   ./test/run_integration.sh            # run all integration tests
#   ./test/run_integration.sh -run Smoke # run a specific test

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENGINE_DIR="$REPO_ROOT/engine"
API_DIR="$REPO_ROOT/api"
ENGINE_BIN="$ENGINE_DIR/target/release/realpolitik"

echo "=== Building Rust engine (release) ==="
cd "$ENGINE_DIR"
cargo build --release 2>&1 | tail -5

if [ ! -x "$ENGINE_BIN" ]; then
    echo "ERROR: engine binary not found at $ENGINE_BIN"
    exit 1
fi
echo "Engine binary: $ENGINE_BIN"

echo ""
echo "=== Running Go integration tests ==="
cd "$API_DIR"
REALPOLITIK_PATH="$ENGINE_BIN" go test \
    ./internal/bot/ \
    -tags=integration \
    -run TestIntegration \
    -v \
    -count=1 \
    -timeout=300s \
    "$@"

echo ""
echo "=== Integration tests passed ==="
