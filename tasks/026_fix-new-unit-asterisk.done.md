# Fix New Unit Asterisk Indicator (Reopened)

## Goal
The * indicator on newly built units is not showing. This was previously "fixed" in task 011 but is still broken.

## Current Implementation
In `ui/lib/features/game/game_screen.dart` (lines 89-95):
```dart
final oldState = _orderNotifier.gameState;
final newState = gameViewState.gameState;
if (oldState != null && newState != null) {
  final oldProvinces = {for (final u in oldState.units) u.province};
  final newProvinces = {for (final u in newState.units) u.province};
  final added = newProvinces.difference(oldProvinces);
  _newUnitProvinces = added.isNotEmpty ? added : {};
}
```

The `_newUnitProvinces` set is passed to `MapPainter` which draws a star at `map_painter.dart:247-249`.

## Likely Root Cause
Timing issue: `_orderNotifier.gameState` may already be updated to the new state before this diff runs, so `oldState == newState` and `added` is always empty. OR the notifier's game state hasn't been set yet, so `oldState` is null.

## Investigation Steps
1. Add debug logging to confirm whether `oldState` and `newState` are actually different when a build phase completes
2. Check when `_orderNotifier.gameState` gets updated relative to the phase ID change detection
3. Verify the `_drawNewUnitStar` method in map_painter.dart actually renders visibly

## Possible Fixes
- Store the previous phase's unit list explicitly (don't rely on `_orderNotifier.gameState` timing)
- Use the phase history to get the previous phase's game state for comparison
- Track build orders from the resolved phase to know which provinces got new units

## Key Files
- `ui/lib/features/game/game_screen.dart` — detection logic (lines 80-108)
- `ui/lib/features/game/widgets/map_painter.dart` — star rendering (line 247)
- `ui/lib/features/game/game_notifier.dart` — game state provider
- `ui/lib/features/game/order_notifier.dart` — order state management

## Acceptance Criteria
- [ ] After a build phase, newly built units show a * for 1 phase
- [ ] * does not appear on units that merely moved
- [ ] * clears after the next phase transition
- [ ] Works in both live games and replay mode
