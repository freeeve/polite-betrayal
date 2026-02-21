# Scale "Time Running Out" Warning by Move Time

## Status: In Progress

## Description
The "time is running out" warning always shows during 1-minute games, making it annoying since the entire game has short move times. The warning threshold should scale based on the configured move time rather than using a fixed threshold.

## Expected Behavior
Warning should only trigger when a meaningful fraction of the move time remains (e.g., last 10-15% of the move timer), not at a fixed absolute threshold.

## Likely Area
- `ui/lib/features/game/` — timer display or warning logic
- Look for hardcoded time thresholds for the warning
