# Arena Game Loop

## File
`api/internal/bot/arena.go`

## Description
Core game loop that runs a single Diplomacy game from start to finish using
the engine directly (no HTTP, no Redis). Writes every phase and order to Postgres
via the existing repository layer.

## Interface
```go
type ArenaConfig struct {
    GameName    string
    PowerConfig map[diplomacy.Power]string // power -> difficulty
    MaxYear     int                        // cap year for draw (e.g. 1920)
    Seed        int64                      // 0 = random
    DryRun      bool                       // skip DB writes
}

type ArenaResult struct {
    GameID      string
    Winner      string // power name or "" for draw
    FinalYear   int
    TotalPhases int
    SCCounts    map[string]int // power -> final SC count
}

func RunGame(ctx context.Context, cfg ArenaConfig, gameRepo repository.GameRepository, phaseRepo repository.PhaseRepository, userRepo repository.UserRepository) (*ArenaResult, error)
```

## Game Loop Pseudocode
1. Create game row + 7 bot game_player rows in Postgres (or skip if dry-run)
2. Initialize `diplomacy.NewInitialState()` and `diplomacy.StandardMap()`
3. Create a `diplomacy.NewResolver(34)` for reuse
4. Loop:
   a. Save phase (state_before) to Postgres
   b. For each power: generate orders via `StrategyForDifficulty(difficulty)`
   c. Convert bot.OrderInput -> engine orders (reuse `toEngineOrder` / `toRetreatOrder` / `toBuildOrder` logic)
   d. Resolve: movement -> apply -> check retreat -> retreat -> check build -> build
   e. Save orders + state_after to Postgres
   f. Check game over (18 SCs or max year)
   g. Advance state
5. Set game status to finished, return result

## Dependencies
- `api/pkg/diplomacy` (engine)
- `api/internal/bot` (strategies)
- `api/internal/repository` (GameRepository, PhaseRepository, UserRepository)
- `api/internal/model` (Game, Phase, Order)

## Acceptance Criteria
- A single game can be run from initial state to completion
- All phases and orders are persisted correctly
- Game appears in UI with full phase-by-phase replay
- Dry-run mode works (no DB writes)
- Deterministic seeding produces identical games
