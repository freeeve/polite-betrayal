# Fix Province Flood Fill Bleed Issues

## Status: Pending

## Dependencies
- None

## Description
Several provinces have flood fill bleed issues where selecting one province incorrectly highlights an adjacent one:

1. **MAO → NAF**: Selecting MAO (Mid-Atlantic Ocean) flood-fills NAF (North Africa)
2. **LON → WAL**: Selecting London triggers Wales highlight
3. **LVP/EDI → CLY**: Selecting Liverpool or Edinburgh triggers Clyde highlight

This is likely a polygon/SVG path issue where province boundaries overlap or aren't properly closed, causing flood fill to bleed through shared edges.

### Likely Cause
- SVG polygon paths for affected provinces share edges that aren't properly closed
- Hit detection polygons overlap
- Common pattern: sea/coastal province boundaries bleeding into adjacent land provinces

### Files to Investigate
- `ui/assets/map/diplomacy_map.svg` — SVG province paths
- `ui/lib/core/map/province_data.dart` — province coordinates/polygons
- Previous similar fix: tasks/062_fix-nth-ska-hel-polygons.done.md

## Acceptance Criteria
- Selecting MAO does not highlight NAF
- Selecting LON does not highlight WAL
- Selecting LVP/EDI does not highlight CLY
- Each province only highlights when directly selected
- No regression in other province highlighting

## Estimated Effort: S
