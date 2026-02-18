# Arena Unit Tests

## File
`api/internal/bot/arena_test.go`

## Description
Unit tests for the arena game loop. Tests run in dry-run mode (no DB needed)
and with deterministic seeds for reproducibility.

## Test Cases
1. **TestRunGameDryRun**: Run a single game to completion in dry-run mode.
   Verify it terminates, returns a result with winner or draw, and respects max year.
2. **TestRunGameDeterministic**: Run two games with the same seed, verify identical results.
3. **TestRunGameMaxYear**: Set max year to 1902, verify game ends as draw.
4. **TestRunGameAllDifficulties**: Run one game per difficulty level, verify all complete.
5. **TestPowerConfigParsing**: Test parsing of power config strings.

## Dependencies
- `api/internal/bot` (RunGame, ArenaConfig)
- `api/pkg/diplomacy` (engine)

## Acceptance Criteria
- All tests pass with `go test ./api/internal/bot/ -run TestArena -v`
- Tests complete in under 30 seconds each
- No database connection required for dry-run tests
