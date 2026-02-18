# Fix Missing * Indicator on Newly Created Units

## Goal
Newly created units should display a * marker for 1 season after they are built. This was working before but has stopped.

## Investigation Areas
- How is the * drawn in map_painter.dart?
- How are "new" units identified (previous state comparison, flag, set)?
- Was the code lost during recent map_painter.dart edits (arrow styling, icon improvements, curve animation)?

## Key Files
- `ui/lib/features/game/widgets/map_painter.dart` — unit rendering
- `ui/lib/core/models/game_state.dart` — unit state tracking
- `ui/lib/features/game/phase_history_notifier.dart` — state management
