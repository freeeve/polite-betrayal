#!/usr/bin/env bash
# Runs the Playwright E2E visual tests end-to-end.
#
# Prereqs: backend on :8009, Postgres, Redis (docker compose up + run_dev_api.sh).
#
# This script:
#   1. Builds Flutter web (release)
#   2. Serves the build on :3009
#   3. Runs Playwright tests
#   4. Cleans up the server
#
# Screenshots saved to test/e2e/screenshots/

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Building Flutter web ==="
cd "$ROOT/ui"
flutter build web --release --quiet

echo "=== Serving on :3009 ==="
python3 -m http.server 3009 --directory "$ROOT/ui/build/web" &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null" EXIT

sleep 1
if ! curl -sf http://localhost:3009/ > /dev/null; then
  echo "ERROR: web server failed to start"
  exit 1
fi

echo "=== Running Playwright tests ==="
cd "$ROOT/test/e2e"
npx playwright test "$@"

echo ""
echo "=== Screenshots ==="
ls -1 "$ROOT/test/e2e/screenshots/"*.png 2>/dev/null || echo "(none)"
