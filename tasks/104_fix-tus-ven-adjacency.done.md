# Fix Missing Tuscany-Venice Adjacency

## Status: In Progress

## Description
Army (and fleet) cannot move from Tuscany to Venice. The tus-ven adjacency is missing from both the Go backend (`map_data.go`) and the Flutter UI (`adjacency_data.dart`). In standard Diplomacy, Tuscany and Venice share both a land and sea border.

## Root Cause
- Go backend `api/pkg/diplomacy/map_data.go`: no `addBothAdj("tus", "ven")` call
- Flutter UI `ui/lib/core/map/adjacency_data.dart`: no `Adjacency(from: 'tus', to: 'ven', type: AdjType.both)` entry

## Fix
1. Add `addBothAdj("tus", "ven")` in `map_data.go` near the other Italian coastal adjacencies
2. Add `Adjacency(from: 'tus', to: 'ven', type: AdjType.both)` in `adjacency_data.dart`
3. Run existing tests to verify no regressions
4. Add a test case specifically for tus->ven movement

## Acceptance Criteria
- Army in Tuscany can move to Venice (and vice versa)
- Fleet in Tuscany can move to Venice (and vice versa)
- All existing adjacency/resolver tests pass
- `flutter analyze` clean
