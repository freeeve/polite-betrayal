# Arrow Scaling Improvements

## Status: Done

## Changes in `ui/lib/features/game/widgets/map_painter.dart`
- Arrowhead size scales with arrow length: `(len * 0.12).clamp(10.0, 19.0)`
- Curvature scales: short arrows get less curve, long arrows 18%
- Inset scales: `min(18.0, len * 0.2)`
- Minimum arrow length lowered from 30 to 20

## Pending
- User asked about arrow endpoints: "maybe the arrows should point from the label to the label, not sure if it is doing the centers instead?" - Currently arrows use province center coordinates. Labels are offset from centers based on unit presence. This could be improved.
