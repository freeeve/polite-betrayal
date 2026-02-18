package diplomacy

// Resolution state constants for the Kruijswijk algorithm.
type resolutionState int

const (
	rsUnresolved resolutionState = iota
	rsGuessing
	rsResolved
)

// adjResult tracks the resolution of a single order in the dependency graph.
type adjResult struct {
	order        Order
	state        resolutionState
	resolution   bool // true = succeeds, false = fails
	provIdx      int16
	targetIdx    int16
	auxLocIdx    int16
	auxTargetIdx int16
	attackStr    int
	holdStr      int
	preventStr   int
}

// ResolveOrders adjudicates a set of validated orders against the game state.
// Returns the list of resolved orders with outcomes, and a list of dislodged units.
func ResolveOrders(orders []Order, gs *GameState, m *DiplomacyMap) ([]ResolvedOrder, []DislodgedUnit) {
	r := newResolver(orders, gs, m)
	return r.resolve()
}

type resolver struct {
	lookup    [ProvinceCount]int16 // province index -> adjBuf offset (-1 = no order)
	adjBuf    []adjResult          // dense storage for iteration
	orderList []Order
	gs        *GameState
	m         *DiplomacyMap
}

// orderAt returns the adjResult for the given province index, or nil if no order exists.
func (r *resolver) orderAt(provIdx int16) *adjResult {
	if provIdx < 0 {
		return nil
	}
	idx := r.lookup[provIdx]
	if idx < 0 {
		return nil
	}
	return &r.adjBuf[idx]
}

// orderAtLoc returns the adjResult for the given province string, or nil if no order exists.
func (r *resolver) orderAtLoc(loc string) *adjResult {
	return r.orderAt(int16(r.m.ProvinceIndex(loc)))
}

// initLookup populates the lookup array and adjBuf province indices from the order list.
func (r *resolver) initLookup() {
	for i := range r.lookup {
		r.lookup[i] = -1
	}
	for i, o := range r.orderList {
		pIdx := int16(r.m.ProvinceIndex(o.Location))
		tIdx := int16(-1)
		if o.Target != "" {
			tIdx = int16(r.m.ProvinceIndex(o.Target))
		}
		aLIdx := int16(-1)
		if o.AuxLoc != "" {
			aLIdx = int16(r.m.ProvinceIndex(o.AuxLoc))
		}
		aTIdx := int16(-1)
		if o.AuxTarget != "" {
			aTIdx = int16(r.m.ProvinceIndex(o.AuxTarget))
		}
		r.adjBuf[i] = adjResult{
			order:        o,
			provIdx:      pIdx,
			targetIdx:    tIdx,
			auxLocIdx:    aLIdx,
			auxTargetIdx: aTIdx,
		}
		if pIdx >= 0 {
			r.lookup[pIdx] = int16(i)
		}
	}
}

func newResolver(orders []Order, gs *GameState, m *DiplomacyMap) *resolver {
	r := &resolver{
		adjBuf:    make([]adjResult, len(orders)),
		orderList: orders,
		gs:        gs,
		m:         m,
	}
	r.initLookup()
	return r
}

func (r *resolver) resolve() ([]ResolvedOrder, []DislodgedUnit) {
	for i := range r.adjBuf {
		r.adjudicate(r.adjBuf[i].provIdx)
	}
	return r.buildResults()
}

// adjudicate resolves the order at the given province index.
// Uses the Kruijswijk approach: when encountering a cycle,
// guess a resolution, check consistency, back off if inconsistent.
func (r *resolver) adjudicate(provIdx int16) bool {
	ar := r.orderAt(provIdx)
	if ar == nil {
		return false
	}

	switch ar.state {
	case rsResolved:
		return ar.resolution
	case rsGuessing:
		return ar.resolution
	}

	// Mark as guessing with initial guess = succeeds.
	ar.state = rsGuessing
	ar.resolution = true

	result := r.resolveOrder(provIdx)

	if ar.state == rsGuessing && result != ar.resolution {
		ar.resolution = result
		result = r.resolveOrder(provIdx)
	}

	ar.state = rsResolved
	ar.resolution = result
	return result
}

func (r *resolver) resolveOrder(provIdx int16) bool {
	ar := r.orderAt(provIdx)
	switch ar.order.Type {
	case OrderHold:
		return true
	case OrderMove:
		return r.resolveMove(provIdx)
	case OrderSupport:
		return r.resolveSupport(provIdx)
	case OrderConvoy:
		return r.resolveConvoy(provIdx)
	default:
		return false
	}
}

// resolveMove determines if a move order succeeds.
func (r *resolver) resolveMove(provIdx int16) bool {
	ar := r.orderAt(provIdx)

	if r.needsConvoy(ar.order) && !r.hasConvoyPath(ar.order) {
		return false
	}

	attackStr := r.attackStrength(provIdx)
	holdStr := r.holdStrength(ar.targetIdx)

	if attackStr <= holdStr {
		return false
	}

	// Head-to-head battle: if the defender is moving to our province,
	// our attack must also exceed the defender's attack strength.
	defender := r.orderAt(ar.targetIdx)
	if defender != nil && defender.order.Type == OrderMove && defender.targetIdx == provIdx {
		defendAttack := r.attackStrength(ar.targetIdx)
		if attackStr <= defendAttack {
			return false
		}
	}

	// Attack must exceed all other prevent strengths at the target.
	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.provIdx == provIdx {
			continue
		}
		if other.order.Type == OrderMove && other.targetIdx == ar.targetIdx {
			preventStr := r.preventStrength(other.provIdx)
			if attackStr <= preventStr {
				return false
			}
		}
	}

	return true
}

// resolveSupport determines if support is successfully given (not cut).
func (r *resolver) resolveSupport(provIdx int16) bool {
	ar := r.orderAt(provIdx)

	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.order.Type != OrderMove {
			continue
		}
		if other.targetIdx != provIdx {
			continue
		}

		// Support cannot be cut by the unit being supported against.
		if ar.auxTargetIdx >= 0 && other.provIdx == ar.auxTargetIdx {
			continue
		}

		// Support cannot be cut by a unit of the same power.
		if other.order.Power == ar.order.Power {
			continue
		}

		// For a convoyed attack, the convoy must succeed for the support to be cut.
		if r.needsConvoy(other.order) && !r.adjudicate(other.provIdx) {
			continue
		}

		return false
	}

	return true
}

// resolveConvoy determines if a convoy order succeeds.
func (r *resolver) resolveConvoy(provIdx int16) bool {
	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.order.Type == OrderMove && other.targetIdx == provIdx {
			if r.adjudicate(other.provIdx) {
				return false
			}
		}
	}
	return true
}

// attackStrength computes the attack strength of a move order.
func (r *resolver) attackStrength(provIdx int16) int {
	ar := r.orderAt(provIdx)
	if ar.order.Type != OrderMove {
		return 0
	}

	strength := 1

	// A unit cannot attack a province occupied by a unit of the same power
	// UNLESS the occupying unit is moving away.
	occupier := r.gs.UnitAt(ar.order.Target)
	if occupier != nil && occupier.Power == ar.order.Power {
		occOrder := r.orderAt(ar.targetIdx)
		if occOrder == nil || occOrder.order.Type != OrderMove {
			return 0
		}
		if occOrder.targetIdx == provIdx {
			return 0
		}
	}

	// Count successful support for this move.
	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.order.Type != OrderSupport {
			continue
		}
		if other.auxLocIdx != provIdx {
			continue
		}
		if other.auxTargetIdx != ar.targetIdx {
			continue
		}
		if r.adjudicate(other.provIdx) {
			strength++
		}
	}

	return strength
}

// holdStrength computes the hold strength of a province.
func (r *resolver) holdStrength(provIdx int16) int {
	ar := r.orderAt(provIdx)
	if ar == nil {
		return 0
	}

	if ar.order.Type == OrderMove {
		if r.adjudicate(provIdx) {
			return 0
		}
		return 1
	}

	strength := 1
	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.order.Type != OrderSupport {
			continue
		}
		if other.auxLocIdx != provIdx || other.auxTargetIdx >= 0 {
			continue
		}
		if r.adjudicate(other.provIdx) {
			strength++
		}
	}
	return strength
}

// preventStrength computes the prevent strength of a move order.
func (r *resolver) preventStrength(provIdx int16) int {
	ar := r.orderAt(provIdx)
	if ar.order.Type != OrderMove {
		return 0
	}

	defender := r.orderAt(ar.targetIdx)
	if defender != nil && defender.order.Type == OrderMove && defender.targetIdx == provIdx {
		if !r.adjudicate(provIdx) {
			return 0
		}
	}

	strength := 1
	for i := range r.adjBuf {
		other := &r.adjBuf[i]
		if other.order.Type != OrderSupport {
			continue
		}
		if other.auxLocIdx != provIdx || other.auxTargetIdx != ar.targetIdx {
			continue
		}
		if r.adjudicate(other.provIdx) {
			strength++
		}
	}
	return strength
}

// needsConvoy returns true if the move requires a convoy chain.
func (r *resolver) needsConvoy(order Order) bool {
	if order.Type != OrderMove || order.UnitType != Army {
		return false
	}
	return !r.m.Adjacent(order.Location, order.Coast, order.Target, NoCoast, false)
}

// hasConvoyPath checks if there's a successful convoy chain for the given move.
func (r *resolver) hasConvoyPath(order Order) bool {
	srcIdx := int16(r.m.ProvinceIndex(order.Location))
	tgtIdx := int16(r.m.ProvinceIndex(order.Target))

	visited := make(map[int16]bool)
	queue := []int16{}

	for i := range r.adjBuf {
		ar := &r.adjBuf[i]
		if ar.order.Type != OrderConvoy {
			continue
		}
		if ar.auxLocIdx != srcIdx || ar.auxTargetIdx != tgtIdx {
			continue
		}
		prov := r.m.Provinces[ar.order.Location]
		if prov == nil || prov.Type != Sea {
			continue
		}
		if r.m.Adjacent(order.Location, NoCoast, ar.order.Location, NoCoast, true) {
			if r.adjudicate(ar.provIdx) {
				visited[ar.provIdx] = true
				queue = append(queue, ar.provIdx)
			}
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		currentAr := r.orderAt(current)
		if r.m.Adjacent(currentAr.order.Location, NoCoast, order.Target, NoCoast, true) {
			return true
		}

		for i := range r.adjBuf {
			ar := &r.adjBuf[i]
			if visited[ar.provIdx] {
				continue
			}
			if ar.order.Type != OrderConvoy {
				continue
			}
			if ar.auxLocIdx != srcIdx || ar.auxTargetIdx != tgtIdx {
				continue
			}
			prov := r.m.Provinces[ar.order.Location]
			if prov == nil || prov.Type != Sea {
				continue
			}
			if r.m.Adjacent(currentAr.order.Location, NoCoast, ar.order.Location, NoCoast, true) {
				if r.adjudicate(ar.provIdx) {
					visited[ar.provIdx] = true
					queue = append(queue, ar.provIdx)
				}
			}
		}
	}

	return false
}

// buildResults converts internal adjudication state to the external result format.
func (r *resolver) buildResults() ([]ResolvedOrder, []DislodgedUnit) {
	var results []ResolvedOrder
	var dislodged []DislodgedUnit

	successfulMoves := make(map[string]string)
	for i := range r.adjBuf {
		ar := &r.adjBuf[i]
		if ar.order.Type == OrderMove && ar.resolution {
			successfulMoves[ar.order.Target] = ar.order.Location
		}
	}

	for _, o := range r.orderList {
		ar := r.orderAtLoc(o.Location)
		if ar == nil {
			continue
		}

		result := ResultSucceeded

		switch o.Type {
		case OrderMove:
			if !ar.resolution {
				result = ResultBounced
			}
		case OrderSupport:
			if !ar.resolution {
				result = ResultCut
			}
		case OrderConvoy:
			if !ar.resolution {
				result = ResultFailed
			}
		case OrderHold:
		}

		if attacker, ok := successfulMoves[o.Location]; ok {
			if o.Type != OrderMove || !ar.resolution {
				result = ResultDislodged
				dislodged = append(dislodged, DislodgedUnit{
					Unit: Unit{
						Type:     o.UnitType,
						Power:    o.Power,
						Province: o.Location,
						Coast:    o.Coast,
					},
					DislodgedFrom: o.Location,
					AttackerFrom:  attacker,
				})
			}
		}

		results = append(results, ResolvedOrder{Order: o, Result: result})
	}

	return results, dislodged
}

// applyUnitKey identifies a unit by power and province for resolution application.
type applyUnitKey struct {
	power    Power
	province string
}

// applyMoveEntry stores the result of a successful move for batch application.
type applyMoveEntry struct {
	target      string
	targetCoast Coast
	clearCoast  bool
}

// ApplyResolution updates the game state based on resolved orders.
// Moves successful units, removes dislodged units from the board.
func ApplyResolution(gs *GameState, m *DiplomacyMap, results []ResolvedOrder, dislodged []DislodgedUnit) {
	dislodgedSet := make(map[applyUnitKey]bool)
	for _, d := range dislodged {
		dislodgedSet[applyUnitKey{d.Unit.Power, d.DislodgedFrom}] = true
	}

	moves := make(map[applyUnitKey]applyMoveEntry)
	for _, ro := range results {
		if ro.Order.Type == OrderMove && ro.Result == ResultSucceeded {
			clearCoast := ro.Order.TargetCoast == NoCoast && !m.HasCoasts(ro.Order.Target)
			moves[applyUnitKey{ro.Order.Power, ro.Order.Location}] = applyMoveEntry{
				target:      ro.Order.Target,
				targetCoast: ro.Order.TargetCoast,
				clearCoast:  clearCoast,
			}
		}
	}
	applyMoves(gs, moves, dislodgedSet, dislodged)
}

// applyMoves applies move updates and removes dislodged units from the game state.
func applyMoves(gs *GameState, moves map[applyUnitKey]applyMoveEntry, dislodgedSet map[applyUnitKey]bool, dislodged []DislodgedUnit) {
	for i := range gs.Units {
		key := applyUnitKey{gs.Units[i].Power, gs.Units[i].Province}
		if mu, ok := moves[key]; ok {
			gs.Units[i].Province = mu.target
			if mu.targetCoast != NoCoast {
				gs.Units[i].Coast = mu.targetCoast
			} else if mu.clearCoast {
				gs.Units[i].Coast = NoCoast
			}
		}
	}

	remaining := gs.Units[:0]
	for _, u := range gs.Units {
		if !dislodgedSet[applyUnitKey{u.Power, u.Province}] {
			remaining = append(remaining, u)
		}
	}
	gs.Units = remaining
	gs.Dislodged = dislodged
}

// Resolver is a reusable order adjudicator that minimizes allocations.
// Allocate once with NewResolver and call Resolve repeatedly in hot loops.
// The returned slices are owned by the Resolver and overwritten on the next call.
type Resolver struct {
	r resolver

	// buildResults buffers
	resBuf  []ResolvedOrder
	disBuf  []DislodgedUnit
	moveMap map[string]string // target -> source for dislodgement detection

	// Apply buffers
	dislodgedSet map[applyUnitKey]bool
	movesMap     map[applyUnitKey]applyMoveEntry
}

// NewResolver creates a reusable resolver. capacity should be the
// expected number of orders per resolution (e.g. 34 for a full board).
func NewResolver(capacity int) *Resolver {
	rv := &Resolver{
		r: resolver{
			adjBuf: make([]adjResult, 0, capacity),
		},
		resBuf:       make([]ResolvedOrder, 0, capacity),
		disBuf:       make([]DislodgedUnit, 0, 4),
		moveMap:      make(map[string]string, capacity),
		dislodgedSet: make(map[applyUnitKey]bool, 4),
		movesMap:     make(map[applyUnitKey]applyMoveEntry, capacity),
	}
	for i := range rv.r.lookup {
		rv.r.lookup[i] = -1
	}
	return rv
}

// Resolve adjudicates orders and returns resolved results plus dislodged units.
// The returned slices are backed by internal buffers; they are valid until the
// next Resolve call.
func (rv *Resolver) Resolve(orders []Order, gs *GameState, m *DiplomacyMap) ([]ResolvedOrder, []DislodgedUnit) {
	rv.reset(orders, gs, m)

	for i := range rv.r.adjBuf {
		rv.r.adjudicate(rv.r.adjBuf[i].provIdx)
	}

	return rv.buildResults()
}

func (rv *Resolver) reset(orders []Order, gs *GameState, m *DiplomacyMap) {
	r := &rv.r
	n := len(orders)
	if cap(r.adjBuf) >= n {
		r.adjBuf = r.adjBuf[:n]
	} else {
		r.adjBuf = make([]adjResult, n)
	}
	r.orderList = orders
	r.gs = gs
	r.m = m
	r.initLookup()
}

func (rv *Resolver) buildResults() ([]ResolvedOrder, []DislodgedUnit) {
	rv.resBuf = rv.resBuf[:0]
	rv.disBuf = rv.disBuf[:0]
	clear(rv.moveMap)

	r := &rv.r
	for i := range r.adjBuf {
		ar := &r.adjBuf[i]
		if ar.order.Type == OrderMove && ar.resolution {
			rv.moveMap[ar.order.Target] = ar.order.Location
		}
	}

	for _, o := range r.orderList {
		ar := r.orderAtLoc(o.Location)
		if ar == nil {
			continue
		}

		result := ResultSucceeded

		switch o.Type {
		case OrderMove:
			if !ar.resolution {
				result = ResultBounced
			}
		case OrderSupport:
			if !ar.resolution {
				result = ResultCut
			}
		case OrderConvoy:
			if !ar.resolution {
				result = ResultFailed
			}
		case OrderHold:
		}

		if attacker, ok := rv.moveMap[o.Location]; ok {
			if o.Type != OrderMove || !ar.resolution {
				result = ResultDislodged
				rv.disBuf = append(rv.disBuf, DislodgedUnit{
					Unit: Unit{
						Type:     o.UnitType,
						Power:    o.Power,
						Province: o.Location,
						Coast:    o.Coast,
					},
					DislodgedFrom: o.Location,
					AttackerFrom:  attacker,
				})
			}
		}

		rv.resBuf = append(rv.resBuf, ResolvedOrder{Order: o, Result: result})
	}

	return rv.resBuf, rv.disBuf
}

// Apply updates the game state using the results from the most recent Resolve call.
// Moves successful units and removes dislodged units.
func (rv *Resolver) Apply(gs *GameState, m *DiplomacyMap) {
	clear(rv.dislodgedSet)
	clear(rv.movesMap)

	for _, d := range rv.disBuf {
		rv.dislodgedSet[applyUnitKey{d.Unit.Power, d.DislodgedFrom}] = true
	}

	for _, ro := range rv.resBuf {
		if ro.Order.Type == OrderMove && ro.Result == ResultSucceeded {
			clearCoast := ro.Order.TargetCoast == NoCoast && !m.HasCoasts(ro.Order.Target)
			rv.movesMap[applyUnitKey{ro.Order.Power, ro.Order.Location}] = applyMoveEntry{
				target:      ro.Order.Target,
				targetCoast: ro.Order.TargetCoast,
				clearCoast:  clearCoast,
			}
		}
	}
	applyMoves(gs, rv.movesMap, rv.dislodgedSet, rv.disBuf)
}

// HasDislodged returns true if the last Resolve call produced any dislodged units.
func (rv *Resolver) HasDislodged() bool {
	return len(rv.disBuf) > 0
}
