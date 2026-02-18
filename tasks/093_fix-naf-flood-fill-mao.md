# Fix NAF Flood Fill When Navigating to MAO

## Status: Pending

## Dependencies
- None

## Description
In the UI map, when navigating to or selecting MAO (Mid-Atlantic Ocean), the NAF (North Africa) province gets incorrectly flood-filled. This is likely a polygon/SVG path issue where NAF's region overlaps or shares a boundary with MAO, causing the flood fill highlight to bleed into NAF.

### Likely Cause
- SVG polygon for NAF or MAO has incorrect boundaries
- Shared edge between NAF and MAO not properly closed
- Hit detection treating MAO click/hover as NAF

### Files to Investigate
- `ui/assets/map/diplomacy_map.svg` — SVG province paths
- `ui/lib/core/map/province_data.dart` — province coordinates/polygons
- Previous similar fix: tasks/062_fix-nth-ska-hel-polygons.done.md

## Acceptance Criteria
- Selecting/hovering MAO does not highlight NAF
- NAF only highlights when directly selected
- No regression in other province highlighting

## Estimated Effort: S
