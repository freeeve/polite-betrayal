#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/ui"

# Use chrome if available, otherwise fall back to macOS desktop.
if flutter devices 2>/dev/null | grep -q chrome; then
  echo "Starting Flutter web on http://localhost:3009"
  flutter run -d chrome --web-port 3009
else
  echo "Chrome not found — running as macOS desktop app"
  flutter run -d macos
fi
