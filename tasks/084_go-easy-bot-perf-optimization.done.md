# Go Easy Bot Performance Optimization

## Status: Pending

## Dependencies
- 083 (medium bot support improvement — touches shared EvaluatePosition)

## Problem
The easy bot (HeuristicStrategy) is called frequently — the medium bot runs it 24 times per move for heuristic sampling, plus EvaluatePosition is called up to 50K times in the search loop. Reducing allocations and CPU here speeds up both easy and medium bots significantly.

## Optimization Opportunities (profiled)

### P0: Cache NearestUnownedSCByUnit per call (HIGH impact, LOW effort)
- Called ~50 times per scoreMoves (once per unit*target), plus in EvaluatePosition per unit
- 24 heuristic samples + 50K search combos = massive redundancy
- Fix: Cache results per (province, power, isFleet) at start of call. State doesn't change within one order-generation invocation.
- Files: strategy_easy.go:453,523; search_util.go:561

### P1: Pre-compute threat/defense map in EvaluatePosition (HIGH impact, MEDIUM effort)
- ProvinceThreat + ProvinceDefense scan ALL units for each owned SC
- O(ownedSCs * totalUnits * avgAdj) per eval call
- In 50K-combo search: ~72M adjacency checks just for vulnerability scoring
- Fix: Build threat map once per eval: O(totalUnits * adj), then O(1) per province lookup
- Files: search_util.go:587-602

### P2: Replace unitCanReach with distance matrix O(1) lookup (MEDIUM impact, LOW effort)
- Territorial cohesion loop is O(ownUnits² * adj) per EvaluatePosition
- Distance matrices already computed and cached
- Fix: `dm.Distance(other.Province, u.Province) == 1` instead of adjacency list scan
- Files: search_util.go:609-618

### P3: Avoid UnitsOf allocation (MEDIUM impact, LOW effort)
- Allocates new []Unit slice every call, called 48+ times per medium bot turn
- Fix: Accept pre-allocated buffer or cache on GameState
- Files: state.go:90-98

### P4: Skip redundant ValidateOrder in scoreMoves (MEDIUM impact, LOW effort)
- ~50 ValidateOrder calls per scoreMoves, 1500+ per medium bot turn
- Already filtered by adjacency and type — validation is redundant for direct moves
- Fix: Inline lightweight check, skip full ValidateOrder
- Files: strategy_easy.go:560-571

### P5: Pre-build unit location map for UnitAt (MEDIUM impact, LOW effort)
- UnitAt does linear scan of all units
- Called inside CanSupportMove, ValidateOrder, etc.
- Fix: Build map[string]*Unit once per GenerateMovementOrders call
- Files: search_util.go (CanSupportMove), strategy_easy.go

### P6: Replace map[string]bool with [PROVINCE_COUNT]bool arrays (LOW-MEDIUM impact, LOW effort)
- assignedUnits, assignedTargets, supportConverted, ownOccupied, etc. — all use string-keyed maps
- Province count is fixed (~75). Use fixed arrays indexed by province ID.
- Eliminates hash map allocation, hashing, GC pressure. 24 samples * 8 maps = 192 map allocs eliminated.
- Files: strategy_easy.go (throughout)

### P7: Pre-allocate slice capacities (LOW impact, LOW effort)
- candidates, moves, plans, options grow via append without capacity hints
- Fix: make([]T, 0, estimatedCap)
- Files: strategy_easy.go (throughout)

## Key Files
- `api/internal/bot/strategy_easy.go` — HeuristicStrategy
- `api/internal/bot/search_util.go` — NearestUnownedSCByUnit, EvaluatePosition, CanSupportMove, ProvinceThreat
- `api/pkg/diplomacy/state.go` — UnitsOf, UnitAt

## Acceptance Criteria
- All existing bot tests pass
- Measurable reduction in allocations (go test -benchmem if bench exists, or add one)
- No behavioral changes
- Run gofmt -s before committing

## Estimated Effort: M
