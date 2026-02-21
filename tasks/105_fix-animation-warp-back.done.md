# Fix Unit Warp-Back After Move Animation

## Status: In Progress

## Description
When a move animation finishes during gameplay, all units temporarily warp back to their starting positions before the next move begins. This creates a jarring visual glitch.

## Expected Behavior
Units should smoothly transition from their animated end position to their new game state position without any visual snap-back.

## Likely Area
- `ui/lib/features/game/` — animation state management
- `game_notifier.dart` or `game_screen.dart` — how animation completion triggers state updates
- `map_painter.dart` — how unit positions are interpolated during/after animation
