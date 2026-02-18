#!/usr/bin/env bash
# Seeds a test game with two human players and bot fill.
# Prereqs: backend on :8009 with DEV_MODE=true, Postgres, Redis.
#
# Usage: bash test/seed_game.sh
# Output: game ID and auth tokens for browser/test use.

set -euo pipefail

API="http://localhost:8009"

echo "=== Creating dev users ==="
RESP1=$(curl -sf "${API}/auth/dev?name=TestPlayer1")
TOKEN1=$(echo "$RESP1" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "Player1 token: ${TOKEN1:0:20}..."

RESP2=$(curl -sf "${API}/auth/dev?name=TestPlayer2")
TOKEN2=$(echo "$RESP2" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "Player2 token: ${TOKEN2:0:20}..."

echo ""
echo "=== Creating game ==="
GAME_RESP=$(curl -sf -X POST "${API}/api/v1/games" \
  -H "Authorization: Bearer ${TOKEN1}" \
  -H "Content-Type: application/json" \
  -d '{"name":"Visual Test Game"}')
GAME_ID=$(echo "$GAME_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Game ID: ${GAME_ID}"

echo ""
echo "=== Player2 joining ==="
curl -sf -X POST "${API}/api/v1/games/${GAME_ID}/join" \
  -H "Authorization: Bearer ${TOKEN2}" > /dev/null
echo "Player2 joined."

echo ""
echo "=== Starting game (fills 5 bot slots, assigns powers, places units) ==="
curl -sf -X POST "${API}/api/v1/games/${GAME_ID}/start" \
  -H "Authorization: Bearer ${TOKEN1}" > /dev/null
echo "Game started."

echo ""
echo "=== Summary ==="
echo "GAME_ID=${GAME_ID}"
echo "TOKEN1=${TOKEN1}"
echo "TOKEN2=${TOKEN2}"
echo ""
echo "Open in browser: http://localhost:3009/#/game/${GAME_ID}"
