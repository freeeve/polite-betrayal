# Show Season and Phase During Game Replay

## Goal
Display the current season and phase (e.g., "Spring 1901 — Movement") at the top of the replay controls during game replay.

## Resolution
- Added phase history watcher to replay_controls.dart
- Built formatted label from current phase index: "Spring 1901 — Movement"
- Displayed as bold titleSmall header above the existing "Phase 1 / 5" counter
- Falls back to "No phases" when no data available

## Key Files
- `ui/lib/features/game/widgets/replay_controls.dart`
