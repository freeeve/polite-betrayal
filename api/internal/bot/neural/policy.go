package neural

import (
	"math"
	"sort"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// ScoredOrder pairs a legal order description with its neural network score.
type ScoredOrder struct {
	OrderType   string // hold, move, support, convoy, retreat, build, disband
	Location    string // unit source province
	Coast       string // unit coast (empty for most)
	Target      string // move/support target province
	TargetCoast string // target coast for fleet on split-coast
	AuxLoc      string // support: supported unit's location
	AuxTarget   string // support: supported move's target
	AuxUnitType string // support: supported unit type
	UnitType    string // army or fleet
	Score       float32
}

// DecodePolicyLogits takes the [maxUnits, 169] flattened policy logits and
// the power's unit list, generates all legal orders for each unit, scores
// them using the additive type+source+dest decomposition, and returns the
// top-K orders per unit sorted by descending score.
func DecodePolicyLogits(
	logits []float32,
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	k int,
) [][]ScoredOrder {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	result := make([][]ScoredOrder, 0, len(units))

	for ui, u := range units {
		logitStart := ui * OrderVocabSize
		logitEnd := logitStart + OrderVocabSize
		if logitEnd > len(logits) {
			break
		}
		unitLogits := logits[logitStart:logitEnd]

		scored := scoreUnitOrders(unitLogits, u, gs, power, m)
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].Score > scored[j].Score
		})
		if len(scored) > k {
			scored = scored[:k]
		}
		result = append(result, scored)
	}

	return result
}

// scoreUnitOrders generates and scores all legal orders for a single unit.
func scoreUnitOrders(
	logits []float32,
	u diplomacy.Unit,
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
) []ScoredOrder {
	srcArea := AreaIndex(u.Province)
	if srcArea < 0 {
		return nil
	}
	isFleet := u.Type == diplomacy.Fleet

	var orders []ScoredOrder

	// Hold order.
	holdScore := logits[OrderTypeHold] + logits[SrcOffset+srcArea]
	orders = append(orders, ScoredOrder{
		OrderType: "hold",
		Location:  u.Province,
		Coast:     string(u.Coast),
		UnitType:  u.Type.String(),
		Score:     holdScore,
	})

	// Move orders.
	adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
	for _, target := range adj {
		prov := m.Provinces[target]
		if prov == nil {
			continue
		}
		if isFleet && prov.Type == diplomacy.Land {
			continue
		}
		if !isFleet && prov.Type == diplomacy.Sea {
			continue
		}

		targetCoast := ""
		if isFleet && m.HasCoasts(target) {
			coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
			if len(coasts) == 0 {
				continue
			}
			// Pick the coast with the best score.
			bestCoast := coasts[0]
			bestScore := float32(math.Inf(-1))
			for _, c := range coasts {
				varIdx := BicoastalIndex(target, c)
				if varIdx >= 0 {
					s := logits[DstOffset+varIdx]
					if s > bestScore {
						bestScore = s
						bestCoast = c
					}
				}
			}
			targetCoast = string(bestCoast)
		}

		o := diplomacy.Order{
			UnitType: u.Type, Power: power, Location: u.Province,
			Coast: u.Coast, Type: diplomacy.OrderMove,
			Target: target, TargetCoast: diplomacy.Coast(targetCoast),
		}
		if diplomacy.ValidateOrder(o, gs, m) != nil {
			continue
		}

		dstArea := areaForTarget(target, targetCoast)
		score := logits[OrderTypeMove] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
		orders = append(orders, ScoredOrder{
			OrderType:   "move",
			Location:    u.Province,
			Coast:       string(u.Coast),
			Target:      target,
			TargetCoast: targetCoast,
			UnitType:    u.Type.String(),
			Score:       score,
		})
	}

	// Support orders: support-hold and support-move for adjacent friendly units.
	for _, other := range gs.Units {
		if other.Province == u.Province {
			continue
		}
		otherArea := AreaIndex(other.Province)
		if otherArea < 0 {
			continue
		}

		// Support-hold: can we support this unit holding in place?
		suppHold := diplomacy.Order{
			UnitType: u.Type, Power: power, Location: u.Province,
			Coast: u.Coast, Type: diplomacy.OrderSupport,
			AuxLoc: other.Province, AuxUnitType: other.Type,
		}
		if diplomacy.ValidateOrder(suppHold, gs, m) == nil {
			dstArea := otherArea
			score := logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
			orders = append(orders, ScoredOrder{
				OrderType:   "support",
				Location:    u.Province,
				Coast:       string(u.Coast),
				AuxLoc:      other.Province,
				AuxUnitType: other.Type.String(),
				UnitType:    u.Type.String(),
				Score:       score,
			})
		}

		// Support-move: support other unit moving to each adjacent target.
		otherIsFleet := other.Type == diplomacy.Fleet
		otherAdj := m.ProvincesAdjacentTo(other.Province, other.Coast, otherIsFleet)
		for _, target := range otherAdj {
			if target == u.Province {
				continue // Cannot support a move to our own location
			}
			suppMove := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province,
				Coast: u.Coast, Type: diplomacy.OrderSupport,
				AuxLoc: other.Province, AuxTarget: target,
				AuxUnitType: other.Type,
			}
			if diplomacy.ValidateOrder(suppMove, gs, m) == nil {
				dstArea := AreaIndex(target)
				if dstArea < 0 {
					continue
				}
				score := logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
				orders = append(orders, ScoredOrder{
					OrderType:   "support",
					Location:    u.Province,
					Coast:       string(u.Coast),
					Target:      target,
					AuxLoc:      other.Province,
					AuxTarget:   target,
					AuxUnitType: other.Type.String(),
					UnitType:    u.Type.String(),
					Score:       score,
				})
			}
		}
	}

	// Convoy orders (army convoys handled separately - fleet convoys army).
	if isFleet {
		for _, army := range gs.Units {
			if army.Type != diplomacy.Army || army.Province == u.Province {
				continue
			}
			armyAdj := m.ProvincesAdjacentTo(army.Province, army.Coast, false)
			for _, target := range armyAdj {
				convoyOrder := diplomacy.Order{
					UnitType: u.Type, Power: power, Location: u.Province,
					Coast: u.Coast, Type: diplomacy.OrderConvoy,
					AuxLoc: army.Province, AuxTarget: target,
					AuxUnitType: diplomacy.Army,
				}
				if diplomacy.ValidateOrder(convoyOrder, gs, m) == nil {
					dstArea := AreaIndex(target)
					if dstArea < 0 {
						continue
					}
					score := logits[OrderTypeConvoy] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
					orders = append(orders, ScoredOrder{
						OrderType:   "convoy",
						Location:    u.Province,
						Coast:       string(u.Coast),
						AuxLoc:      army.Province,
						AuxTarget:   target,
						AuxUnitType: "army",
						UnitType:    u.Type.String(),
						Score:       score,
					})
				}
			}
		}
	}

	return orders
}

// DecodeRetreatLogits scores retreat orders for dislodged units using policy logits.
func DecodeRetreatLogits(
	logits []float32,
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
) []ScoredOrder {
	var orders []ScoredOrder
	ui := 0
	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}
		logitStart := ui * OrderVocabSize
		logitEnd := logitStart + OrderVocabSize
		if logitEnd > len(logits) {
			break
		}
		unitLogits := logits[logitStart:logitEnd]
		srcArea := AreaIndex(d.DislodgedFrom)
		if srcArea < 0 {
			ui++
			continue
		}

		isFleet := d.Unit.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(d.DislodgedFrom, d.Unit.Coast, isFleet)

		bestOrder := ScoredOrder{
			OrderType: "retreat_disband",
			Location:  d.DislodgedFrom,
			Coast:     string(d.Unit.Coast),
			UnitType:  d.Unit.Type.String(),
			Score:     unitLogits[OrderTypeDisband] + unitLogits[SrcOffset+srcArea],
		}

		for _, target := range adj {
			if target == d.AttackerFrom {
				continue
			}
			if gs.UnitAt(target) != nil {
				continue
			}
			prov := m.Provinces[target]
			if prov == nil {
				continue
			}
			if isFleet && prov.Type == diplomacy.Land {
				continue
			}
			if !isFleet && prov.Type == diplomacy.Sea {
				continue
			}

			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(d.DislodgedFrom, d.Unit.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
			}

			ro := diplomacy.RetreatOrder{
				UnitType: d.Unit.Type, Power: power, Location: d.DislodgedFrom,
				Coast: d.Unit.Coast, Type: diplomacy.RetreatMove,
				Target: target, TargetCoast: diplomacy.Coast(targetCoast),
			}
			if diplomacy.ValidateRetreatOrder(ro, gs, m) != nil {
				continue
			}

			dstArea := areaForTarget(target, targetCoast)
			score := unitLogits[OrderTypeRetreat] + unitLogits[SrcOffset+srcArea] + unitLogits[DstOffset+dstArea]
			if score > bestOrder.Score {
				bestOrder = ScoredOrder{
					OrderType:   "retreat_move",
					Location:    d.DislodgedFrom,
					Coast:       string(d.Unit.Coast),
					Target:      target,
					TargetCoast: targetCoast,
					UnitType:    d.Unit.Type.String(),
					Score:       score,
				}
			}
		}
		orders = append(orders, bestOrder)
		ui++
	}
	return orders
}

// DecodeBuildLogits scores build/disband orders using policy logits.
func DecodeBuildLogits(
	logits []float32,
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
) []ScoredOrder {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	if diff == 0 {
		return nil
	}

	var orders []ScoredOrder

	if diff > 0 {
		// Need builds.
		homes := diplomacy.HomeCenters(power)
		type buildCandidate struct {
			loc      string
			unitType diplomacy.UnitType
			coast    string
			score    float32
		}
		var candidates []buildCandidate

		for _, h := range homes {
			if gs.SupplyCenters[h] != power || gs.UnitAt(h) != nil {
				continue
			}
			areaIdx := AreaIndex(h)
			if areaIdx < 0 {
				continue
			}
			prov := m.Provinces[h]
			if prov == nil {
				continue
			}

			// Try army.
			if prov.Type != diplomacy.Sea {
				bo := diplomacy.BuildOrder{Power: power, Type: diplomacy.BuildUnit, UnitType: diplomacy.Army, Location: h}
				if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
					score := logits[OrderTypeBuild] + logits[SrcOffset+areaIdx]
					candidates = append(candidates, buildCandidate{h, diplomacy.Army, "", score})
				}
			}

			// Try fleet.
			if prov.Type != diplomacy.Land {
				if len(prov.Coasts) > 0 {
					for _, c := range prov.Coasts {
						bo := diplomacy.BuildOrder{Power: power, Type: diplomacy.BuildUnit, UnitType: diplomacy.Fleet, Location: h, Coast: c}
						if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
							varIdx := BicoastalIndex(h, c)
							dstIdx := areaIdx
							if varIdx >= 0 {
								dstIdx = varIdx
							}
							score := logits[OrderTypeBuild] + logits[SrcOffset+dstIdx]
							candidates = append(candidates, buildCandidate{h, diplomacy.Fleet, string(c), score})
						}
					}
				} else {
					bo := diplomacy.BuildOrder{Power: power, Type: diplomacy.BuildUnit, UnitType: diplomacy.Fleet, Location: h}
					if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
						score := logits[OrderTypeBuild] + logits[SrcOffset+areaIdx]
						candidates = append(candidates, buildCandidate{h, diplomacy.Fleet, "", score})
					}
				}
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})

		built := 0
		usedLoc := make(map[string]bool)
		for _, c := range candidates {
			if built >= diff {
				break
			}
			if usedLoc[c.loc] {
				continue
			}
			usedLoc[c.loc] = true
			orders = append(orders, ScoredOrder{
				OrderType: "build",
				Location:  c.loc,
				Coast:     c.coast,
				UnitType:  c.unitType.String(),
				Score:     c.score,
			})
			built++
		}
	} else {
		// Need disbands: score each unit and disband the lowest-scored.
		type disbandCandidate struct {
			unit  diplomacy.Unit
			score float32
		}
		var candidates []disbandCandidate

		ui := 0
		for _, u := range gs.Units {
			if u.Power != power {
				continue
			}
			areaIdx := AreaIndex(u.Province)
			if areaIdx < 0 {
				continue
			}
			logitStart := ui * OrderVocabSize
			logitEnd := logitStart + OrderVocabSize
			score := float32(0)
			if logitEnd <= len(logits) {
				score = logits[logitStart+OrderTypeDisband] + logits[logitStart+SrcOffset+areaIdx]
			}
			candidates = append(candidates, disbandCandidate{u, score})
			ui++
		}

		// Sort by score descending - highest score means "most wants to disband".
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})

		needed := -diff
		for i := 0; i < needed && i < len(candidates); i++ {
			c := candidates[i]
			orders = append(orders, ScoredOrder{
				OrderType: "disband",
				Location:  c.unit.Province,
				Coast:     string(c.unit.Coast),
				UnitType:  c.unit.Type.String(),
				Score:     c.score,
			})
		}
	}

	return orders
}

// SoftmaxWeights converts scores to probability weights.
func SoftmaxWeights(scores []float32) []float64 {
	if len(scores) == 0 {
		return nil
	}
	maxS := float32(math.Inf(-1))
	for _, s := range scores {
		if s > maxS {
			maxS = s
		}
	}
	weights := make([]float64, len(scores))
	sum := 0.0
	for i, s := range scores {
		w := math.Exp(float64(s - maxS))
		weights[i] = w
		sum += w
	}
	if sum > 0 {
		for i := range weights {
			weights[i] /= sum
		}
	} else {
		uniform := 1.0 / float64(len(scores))
		for i := range weights {
			weights[i] = uniform
		}
	}
	return weights
}

// areaForTarget returns the area index for a target province+coast.
func areaForTarget(target string, targetCoast string) int {
	if targetCoast != "" {
		varIdx := BicoastalIndex(target, diplomacy.Coast(targetCoast))
		if varIdx >= 0 {
			return varIdx
		}
	}
	return AreaIndex(target)
}
