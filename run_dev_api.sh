#!/usr/bin/env bash
set -euo pipefail

export DEV_MODE=true
export PORT=8009
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable"
export LOG_LEVEL="${LOG_LEVEL:-debug}"
export LOG_FILE="${LOG_FILE:-/tmp/polite-betrayal.log}"

cd "$(dirname "$0")/api"
echo "Starting API server on :${PORT} (DEV_MODE=${DEV_MODE}, LOG_LEVEL=${LOG_LEVEL}, LOG_FILE=${LOG_FILE})"

if command -v air &>/dev/null; then
  air
else
  echo "air not found, falling back to go run (no hot-reload)"
  go run ./cmd/server
fi
