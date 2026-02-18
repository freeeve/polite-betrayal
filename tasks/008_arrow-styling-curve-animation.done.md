# Improve Arrow Styling and Curve-Following Unit Animation

## Goal
1. Units should follow the Bezier curve of their move arrows during animation (instead of straight lines)
2. Make arrowheads pointier and more stylized

## Resolution
- Added `_arrowControlPoint()` and `_bezierPoint()` helpers for quadratic Bezier evaluation
- Successful moves: units now follow the same Bezier curve as their arrows
- Bounced moves: sine-wave amplitude along Bezier curve path
- Arrow size increased from 15px to 19px, angle narrowed from 2.7 to 2.5 radians

## Key Files
- `ui/lib/features/game/widgets/map_painter.dart`
