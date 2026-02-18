# False Victory Declaration — Turkey Wins Spring 1902

## Bug
Turkey was declared the winner during Spring 1902 movement phase. This is impossible — Turkey starts with 3 SCs and can gain at most ~5-6 by end of 1901. Solo victory requires 18 SCs.

## Investigation Results

### 1. Engine Logic — CLEAN
- `IsGameOver()` in `api/pkg/diplomacy/phase.go:49` correctly checks `SupplyCenterCount(power) >= 18`
- `AdvanceState()` in `phase.go:61` only calls `updateSupplyCenterOwnership()` when `gs.Season == Fall && (gs.Phase == PhaseMovement || gs.Phase == PhaseRetreat)` — Spring is excluded
- `updateSupplyCenterOwnership()` in `phase.go:80` only iterates existing SC entries and transfers ownership to occupying units — cannot create new SCs
- Verified via simulation: running Spring 1901 → Fall 1901 → Build skip → Spring 1902 → Fall 1902 with all-hold orders produces correct SC counts at every step (Turkey stays at 3)

### 2. Phase Service Logic — CLEAN
- `advanceToNextPhase()` in `phase_service.go:580` calls `AdvanceState` then `IsGameOver` — correct order
- Per-game mutex (`gameLock`) prevents concurrent resolution from keyspace listener + poller + early resolution
- Build phase skip at line 627-633 correctly handles the second `AdvanceState` call
- `SubmitBotOrders` goroutine (line 698-704) runs after lock release, captures `game.ID` correctly
- JSON round-trip of GameState preserves SupplyCenters map exactly (tested)

### 3. Arena Game Loop — CLEAN
- `arena.go:137` calls `AdvanceState` then `IsGameOver` at line 140 — same pattern as phase_service
- Build skip at line 172-174 matches phase_service logic
- `resolveMovementPhase` properly copies results before `ApplyResolution`

### 4. Orchestrator — CLEAN
- `orchestrator.go` doesn't resolve phases directly; it submits orders via HTTP and waits for WebSocket events
- Winner detection comes from `game_ended` WebSocket event, not client-side logic

### 5. Resolver / ApplyResolution — CLEAN
- `ResolveOrders` / `Resolver.Resolve` never writes to `gs.SupplyCenters` (only reads `gs.UnitAt`)
- `ApplyResolution` only modifies `gs.Units` and `gs.Dislodged`, never `gs.SupplyCenters`
- Bot strategies properly `Clone()` or `CloneInto()` before speculative mutations

### 6. Redis / Postgres Serialization — CLEAN
- `GameState` has no JSON struct tags; uses PascalCase field names (Go default)
- JSON round-trip preserves all 34 SC entries with correct ownership
- Redis stores raw `json.RawMessage` bytes — no transformation
- `RecoverActiveGames` restores from `phase.StateBefore` — correct

### 7. UI / WebSocket — CLEAN
- `game_notifier.dart:222` handles `game_ended` by calling `load()` which fetches game from API
- Winner comes from `game.winner` field in API response (Postgres), not from GameState
- WebSocket routing in `ws_hub.go:101` scopes events to correct `gameID`
- No cross-game event leakage possible

### 8. Secondary Bug Found — `append` slice corruption in HardStrategy
- `strategy_hard.go:589` — `append(candOrders[sampled], opOrders...)` may corrupt `candOrders[sampled]` if the slice has spare capacity, since `append` writes into the backing array
- Same issue on line 604: `append(candOrders[j], opOrders...)`
- Impact: corrupts candidate order data across regret matching iterations, potentially producing invalid/random orders
- **Not the cause of the false victory**, but should be fixed

## Root Cause Analysis

**I could NOT reproduce the false victory through any code path.** The engine logic is sound:
- `IsGameOver` requires 18+ SCs
- SC ownership only updates after Fall movement/retreat
- All code paths are locked against concurrent mutation
- JSON serialization is lossless
- UI only displays winner from database

### Most Likely Explanations (in order of probability)

1. **Database corruption / stale data from a crashed prior game**: If the server crashed after `SetFinished` was called for a DIFFERENT game and the game ID was reused or the DB had a row-level corruption, a stale `winner` field could persist. Need to check: `SELECT winner, status, finished_at FROM games WHERE id = '<game-id>'` to verify.

2. **Redis state contamination on server restart**: If `RecoverActiveGames` ran with a stale Postgres state where SC data was already corrupted, the game would start from a bad state. Checking the actual `state_before` JSON in the phases table for this game would confirm.

3. **Cannot reproduce in code** — request the game ID to inspect Postgres directly.

## Recommended Actions
1. **Check the specific game's DB state** — `SELECT * FROM games WHERE winner = 'turkey'` and inspect the phase states
2. **Fix the `append` bug** in `strategy_hard.go:589,604` — use `slices.Concat` or copy before append
3. **Add defensive logging** — log SC counts when `IsGameOver` returns true in `advanceToNextPhase`
4. **Add a game state integrity check** — verify SC count < 18 at phase start in Spring, log error if violated
