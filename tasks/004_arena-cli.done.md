# Arena CLI Entry Point

## File
`api/cmd/botmatch/main.go`

## Description
CLI binary that parses flags, connects to Postgres, and runs N bot-vs-bot games
in parallel using the arena game loop. Prints a results summary table.

## CLI Flags
- `-p` / `-matchup`: power config (e.g. `france=hard,*=easy` or `medium-vs-easy`)
- `-time`: time control per season (parsed but stored as MaxYear for now)
- `-n`: number of games (default 1)
- `-workers`: concurrency (default 1)
- `-db`: database URL (default from DATABASE_URL env or localhost default)
- `-max-year`: cap year (default 1920)
- `-seed`: base seed (0 = random; game i uses seed+i)
- `-dry-run`: skip DB writes
- `-json`: output results as JSON

## Behavior
1. Parse flags, build `ArenaConfig` for each game
2. Connect to Postgres via `postgres.Connect()`
3. Create repos (GameRepo, PhaseRepo, UserRepo)
4. Run games with worker pool (bounded goroutines)
5. Collect ArenaResults
6. Print summary table (or JSON)

## Dependencies
- `api/internal/bot` (RunGame)
- `api/internal/repository/postgres` (Connect, NewGameRepo, etc.)

## Acceptance Criteria
- `go run ./api/cmd/botmatch -n 1 -dry-run` completes successfully
- `-workers 4 -n 20` runs 20 games with 4 concurrent workers
- Summary table shows wins/draws/SCs per power
- JSON output mode works
