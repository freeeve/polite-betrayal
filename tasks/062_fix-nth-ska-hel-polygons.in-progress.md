# Fix NTH/SKA/HEL Province Polygon Overlaps

## Status: In Progress (agent ab93b9a running)

## Problem
The NTH (North Sea) polygon's eastern boundary uses simplified waypoints that don't follow the actual SVG visual borders. This causes:
1. NTH territory shading bleeds into HEL and SKA
2. NTH underfills near Norway's coast (doesn't reach NWY border)
3. BEL/RUH polygon overlap also reported

## Key Technical Context
- HQ SVG (`ui/assets/map/diplomacy_map_hq.svg`) has viewBox `0 0 1152 1152` - same as painter space
- White border paths in SVG layer2 use transform `matrix(1.8885246,0,0,1.8885246,0.9442593,48.157374)`
- Province polygons are in `ui/lib/core/map/province_polygons.dart`
- The NTH polygon needs MORE POINTS along its eastern boundary to follow the visual map borders

## Current NTH Eastern Boundary (problematic)
```
(448.5, 350.3) -> (441.0, 363.5) -> (415.0, 420.0) -> (425.0, 470.0) -> (463.6, 495.7) -> (415.0, 530.0) -> (399.4, 565.6)
```

## Approach
Agent is parsing the HQ SVG to find actual boundary paths and extract coordinates in 1152x1152 space, then updating NTH/SKA/HEL polygons to match.
