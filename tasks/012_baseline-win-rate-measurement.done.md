# Baseline Win Rate Measurement

## Goal
Run arena matches to establish current win rates for each difficulty tier gap. This data drives all subsequent tuning work.

## Matchups to Run
Use the `botmatch` CLI with `--dry-run` and `--n 20` for each:

| # | Matchup | Command |
|---|---------|---------|
| 1 | Easy (Fra) vs Random (6) | `-matchup Fra=easy,*=random -n 20 --dry-run` |
| 2 | Medium (Fra) vs Easy (6) | `-matchup Fra=medium,*=easy -n 20 --dry-run` |
| 3 | Hard (Fra) vs Medium (6) | `-matchup Fra=hard,*=medium -n 20 --dry-run --max-year 1915` |
| 4 | Extreme (Fra) vs Hard (6) | `-matchup Fra=extreme,*=hard -n 20 --dry-run --max-year 1912` |
| 5 | Mirror: all Easy (7) | `-matchup *=easy -n 10 --dry-run` |
| 6 | Mirror: all Hard (7) | `-matchup *=hard -n 5 --dry-run --max-year 1910` |

Also run with Turkey instead of France for matchups 1-4 to check power bias.

## Output
Record per-matchup:
- Win rate (solo victories / total games)
- Average SC count at game end
- Average phase count / final year
- Draw rate

## Acceptance Criteria
- Results documented in a markdown table in this file (or a results file)
- Identifies which tier gaps are already clean (~100% win rate) and which are weak
- Provides the baseline numbers that subsequent tuning tasks can measure against

## Key Files
- `api/cmd/botmatch/main.go` — CLI entry point
- `api/internal/bot/arena.go` — game loop
