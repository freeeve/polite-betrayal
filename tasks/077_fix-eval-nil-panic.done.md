# Fix Nil Pointer Dereference in NearestUnownedSCByUnit

## Status: Pending

## Description
Runtime panic at eval.go:366 — nil pointer dereference in `NearestUnownedSCByUnit` called from `predictEnemyTargets` (strategy_medium.go:1002). Occurs during hard bot multi-phase simulation when `gs.SupplyCenters` is nil.

### Failing Tests
- `TestHardVsMedium` — panic
- `TestTacticalStrategy_BetterThanHeuristic` — 0% (expects >=40%)
- `TestTacticalStrategy_DefendsOwnSCWhenEnemyAdjacent` — 0% (expects >=60%)

### Fix
Add nil guard for `gs.SupplyCenters` in `NearestUnownedSCByUnit`, or ensure simulation always initializes it.

## Estimated Effort: S
