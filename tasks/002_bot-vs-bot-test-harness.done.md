# Bot-vs-Bot Test Harness

## Goal

Build a CLI tool that runs full Diplomacy games with configurable bot matchups and time controls, saving results to Postgres so games are reviewable in the UI.

## Requirements

- Run games **without HTTP server or Redis** — use the game engine directly + write to Postgres
- **Save every game to the database**: `games`, `game_players`, `phases`, `orders` tables
- Games appear in the UI for full phase-by-phase replay (state snapshots, orders with results)
- Configurable **time control per season**: 10 seconds and 60 seconds
- Configurable **matchups**: specify difficulty per power (e.g., `france=hard,*=easy`)
- Run **N games** in parallel (configurable concurrency)
- Output **results summary**: wins/draws/eliminations per power/difficulty, SC counts over time
- **Deterministic seeding** option for reproducibility
- Cap games at a max year (e.g., 1920) to avoid infinite stalemates — record as draw

## Design Sketch

```
api/cmd/botmatch/main.go          — CLI entry point (flags, DB connect, run arena)
api/internal/bot/arena.go          — game loop: init state → generate orders → resolve → save phase → repeat
api/internal/bot/arena_test.go     — unit tests
```

### Database Integration

Each game run must write:

1. **`games`** row — `status=finished`, `winner` set, `name` like `"botmatch: hard-vs-easy #1"`
2. **`game_players`** rows — 7 entries, `is_bot=true`, `bot_difficulty` set, powers assigned
3. **`phases`** rows — one per phase with `state_before` (JSONB) and `state_after` (JSONB)
4. **`orders`** rows — all orders with `result` populated (succeeded/failed/bounced/etc.)

State JSONB uses PascalCase keys and `UnitType` as int (0=Army, 1=Fleet) to match existing convention.

### CLI Interface

```bash
# 1 hard France vs 6 easy bots, 10s time control, 20 games
go run ./api/cmd/botmatch -p france=hard,*=easy -time 10s -n 20 -workers 4

# Shorthand: tier-vs-tier
go run ./api/cmd/botmatch -matchup medium-vs-easy -time 10s -n 50

# Specify DB connection (or use DATABASE_URL env var)
go run ./api/cmd/botmatch -db "postgres://localhost:5432/diplomacy?sslmode=disable" \
  -p france=hard,*=easy -time 10s -n 20
```

### Output

```
Results (20 games, 10s/season):
  france (hard):  18 wins, 1 draw, 1 survived  — avg SCs: 14.2
  england (easy):  0 wins, 1 draw, 12 survived  — avg SCs: 2.1
  ...

Games saved to database — review in UI under "botmatch: hard-vs-easy #1" through "#20"
```

## Key Dependencies

- `api/pkg/diplomacy` — GameState, Resolver, map data
- `api/internal/bot` — strategy implementations
- `api/internal/repo` — Postgres repositories (GameRepo, PhaseRepo)
- `lib/pq` — Postgres driver (already a project dependency)
- No HTTP, no Redis

## Notes

- The arena should call each bot's `GenerateOrders` with a context that enforces the time budget
- Need to handle all phase types: Movement, Retreat, Build
- Need to handle game-end detection (solo win at 18 SCs, or max year draw)
- Reuse existing repo layer to write games/phases/orders — keeps DB writes consistent with the main server
- Consider a `-dry-run` flag that skips DB writes for quick local testing
- Consider JSON output mode for automated analysis
- Bot user IDs can be synthetic UUIDs (e.g., UUID v5 from "bot-easy-france")
