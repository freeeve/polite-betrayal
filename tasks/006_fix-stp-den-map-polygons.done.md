# Fix STP and DEN Map Polygons in SVG (Reopened)

## Goal
Properly fix the STP and DEN polygons in the SVG map. Previous fix (v1) was insufficient — polygons are still visually incomplete.

## Root Cause
The SVG polygons are severely simplified compared to the canonical data in `province_polygons.dart`:

| Province | SVG Vertices | Dart Vertices | Coverage |
|----------|-------------|---------------|----------|
| **DEN** | 8 | 16 (2 rings) | 50% — 2nd ring completely missing |
| **STP** | 12 | 83 (2 rings) | 14% — both rings missing, oversimplified |

The SVG uses a ~610x560 coordinate space while the Dart polygon data uses ~1200x1000. The previous fix only added a single vertex to STP and repositioned DEN, but didn't address the fundamental coordinate mismatch or missing rings.

## Approach
1. Convert polygon vertices from `province_polygons.dart` coordinate space to SVG viewBox space (scale factor ~0.5x)
2. Rebuild both `<polygon>` elements in the SVG with the full vertex set
3. Handle multi-ring provinces — DEN has 2 rings (mainland + island), STP has 2 rings (main territory + coast)
4. For multi-ring polygons, use separate `<polygon>` elements or a single `<path>` with multiple subpaths
5. Verify visually that the polygons align with neighboring provinces

## Coordinate Transformation
The SVG viewBox is `0 0 610 560`. The Dart data appears to use a different scale. Determine the exact transform by comparing known matching vertices between the SVG and Dart data for a province that IS correct, then apply the same transform to STP and DEN.

## Key Files
- `ui/assets/map/diplomacy_map.svg` — SVG to fix (lines ~138 for DEN, ~143 for STP)
- `ui/lib/core/map/province_polygons.dart` — canonical polygon data (source of truth)
- `ui/lib/core/map/province_data.dart` — province metadata
- `ui/tool/gen_polygons.dart` — polygon generation tool (may help understand coordinate system)

## Acceptance Criteria
- [ ] DEN polygon covers full territory including both rings (mainland + islands)
- [ ] STP polygon covers full territory including both rings
- [ ] Polygons align properly with neighboring provinces (no gaps or overlaps)
- [ ] No regressions to other province polygons
- [ ] `flutter analyze` passes
