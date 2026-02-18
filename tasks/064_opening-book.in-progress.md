# Opening Book for Medium/Hard Bots

## Status: In Progress (code complete, needs benchmarking)

## Description
Full opening book for all 7 powers covering Spring and Fall 1901, with weighted historical openings.

## Files Created
- `api/internal/bot/opening_book.go` (780 lines) - Spring + conditional Fall openings for all 7 powers
- `api/internal/bot/opening_book_test.go` (295 lines) - 11 tests all passing
- Integration in `strategy_medium.go` and `strategy_hard.go` - `LookupOpening()` called when year == 1901

## Design
- Queryable Go data structure mapping (power, season, year, unit positions) to weighted order sets
- Weighted random selection from historical openings
- Conditional Fall 1901 moves keyed by unit positions after Spring resolution
- Full order validation against game engine

## Note
Had to work around missing adjacencies (lvp-yor, kie-hol, rom-apu, smy-ank) which are now fixed in task 061.

## Next Step
- Task #74: Run 100-game benchmark (England medium vs 6 easy) to measure improvement
