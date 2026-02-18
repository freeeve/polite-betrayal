# Collapse Hard and Extreme Tiers

## Goal
Delete the broken hard tier and rename extreme to hard. The search-based hard strategy has 0% win rate vs medium — it's unsalvageable. Rather than rebuild two broken tiers, collapse them into one.

## Changes

### 1. Delete `strategy_hard.go`
- Remove the file entirely

### 2. Rename extreme to hard
- In `strategy_extreme.go`: rename the struct from `ExtremeStrategy` (or whatever it's called) to `HardStrategy`
- Update all method receivers and any comments referencing "extreme"

### 3. Update `strategy.go` difficulty routing
- In `StrategyForDifficulty`: remove the `"hard"` case pointing to the old strategy
- Change `"extreme"` case to `"hard"` (pointing to the renamed strategy)
- Remove the `"extreme"` case entirely

### 4. Update UI difficulty lists
- `ui/lib/features/lobby/create_game_screen.dart`: remove "extreme" from `_difficulties` list
- `ui/lib/features/lobby/lobby_screen.dart`: remove "extreme" from difficulty dropdown
- `ui/lib/features/game/widgets/supply_center_table.dart`: remove "extreme" abbreviation, keep "hard" → 'H'

### 5. Update botmatch / arena references
- Check `api/cmd/botmatch/main.go` and `api/internal/bot/` for any references to "extreme" or the old hard strategy
- Update any test files that reference these tiers

### 6. Update task tracking
- Update `tasks/015_hard-vs-medium-gap.md` — this now tracks the renamed hard (formerly extreme) vs medium
- Delete or close `tasks/016_extreme-vs-hard-gap.md` — no longer applicable

## Acceptance Criteria
- [ ] Only 3 bot tiers remain: random, easy, medium, hard
- [ ] `go build ./...` passes in `api/`
- [ ] `go test ./...` passes in `api/`
- [ ] `flutter analyze` passes in `ui/`
- [ ] No references to "extreme" remain in codebase
