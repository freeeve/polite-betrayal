package bot

import (
	"log"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// Strategy generates orders for a bot player during each phase type.
type Strategy interface {
	Name() string
	GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput
	GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput
	GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput
}

// DrawVoter decides whether a bot should vote for a draw.
// Not all strategies support draw voting; use a type assertion to check.
type DrawVoter interface {
	ShouldVoteDraw(gs *diplomacy.GameState, power diplomacy.Power) bool
}

// DiplomaticStrategy extends Strategy with diplomatic message capabilities.
// Not all strategies support diplomacy; use a type assertion to check.
type DiplomaticStrategy interface {
	Strategy
	GenerateDiplomaticMessages(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, received []DiplomaticIntent) []DiplomaticIntent
}

// ExternalEnginePath is the path to the DUI engine binary used by the
// "impossible" and "external" difficulties. Set this at startup (e.g. from
// an environment variable) before creating strategies.
var ExternalEnginePath string

// StrategyForDifficulty returns the appropriate strategy for a bot difficulty level.
func StrategyForDifficulty(difficulty string) Strategy {
	switch difficulty {
	case "medium":
		return &TacticalStrategy{}
	case "hard":
		return &HardStrategy{}
	case "random":
		return &RandomStrategy{}
	case "impossible", "external":
		return newExternalOrFallback(difficulty)
	default:
		return &HeuristicStrategy{}
	}
}

// newExternalOrFallback attempts to create an ExternalStrategy. If the engine
// path is not configured or the engine fails to start, it falls back to
// HardStrategy so the game can proceed.
func newExternalOrFallback(difficulty string) Strategy {
	if ExternalEnginePath == "" {
		log.Printf("bot: %s difficulty requested but ExternalEnginePath not set; falling back to hard", difficulty)
		return &HardStrategy{}
	}
	// Power is set per-query via setpower, so we use a placeholder here.
	// The actual power is passed in each Generate* call.
	es, err := NewExternalStrategy(ExternalEnginePath, "")
	if err != nil {
		log.Printf("bot: failed to start external engine %q: %v; falling back to hard", ExternalEnginePath, err)
		return &HardStrategy{}
	}
	return es
}

// --- HoldStrategy ---

// HoldStrategy holds all units, disbands retreating units, and waives builds.
type HoldStrategy struct{}

func (HoldStrategy) Name() string { return "hold" }

func (HoldStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	var orders []OrderInput
	for _, u := range gs.UnitsOf(power) {
		orders = append(orders, OrderInput{
			UnitType:  u.Type.String(),
			Location:  u.Province,
			Coast:     string(u.Coast),
			OrderType: "hold",
		})
	}
	return orders
}

func (HoldStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	var orders []OrderInput
	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}
		orders = append(orders, OrderInput{
			UnitType:  d.Unit.Type.String(),
			Location:  d.DislodgedFrom,
			Coast:     string(d.Unit.Coast),
			OrderType: "retreat_disband",
		})
	}
	return orders
}

func (HoldStrategy) GenerateBuildOrders(_ *diplomacy.GameState, _ diplomacy.Power, _ *diplomacy.DiplomacyMap) []OrderInput {
	// Waive all builds; civil disorder handles forced disbands.
	return nil
}

// --- RandomStrategy ---

// RandomStrategy generates random but valid orders for testing.
type RandomStrategy struct{}

func (RandomStrategy) Name() string { return "random" }

// GenerateMovementOrders picks random moves for each unit: ~30% hold, ~70% move.
func (RandomStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	var orders []OrderInput
	for _, u := range gs.UnitsOf(power) {
		if botFloat64() < 0.3 {
			orders = append(orders, OrderInput{
				UnitType:  u.Type.String(),
				Location:  u.Province,
				Coast:     string(u.Coast),
				OrderType: "hold",
			})
			continue
		}

		isFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
		if len(adj) == 0 {
			orders = append(orders, OrderInput{
				UnitType:  u.Type.String(),
				Location:  u.Province,
				Coast:     string(u.Coast),
				OrderType: "hold",
			})
			continue
		}

		moved := false
		// Shuffle adjacencies and try each until one validates
		perm := botPerm(len(adj))
		for _, idx := range perm {
			target := adj[idx]
			prov := m.Provinces[target]
			if prov == nil {
				continue
			}
			// Skip invalid unit/province combos
			if isFleet && prov.Type == diplomacy.Land {
				continue
			}
			if !isFleet && prov.Type == diplomacy.Sea {
				continue
			}

			oi := OrderInput{
				UnitType:  u.Type.String(),
				Location:  u.Province,
				Coast:     string(u.Coast),
				OrderType: "move",
				Target:    target,
			}

			// Handle fleet coast specification
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 1 {
					oi.TargetCoast = string(coasts[0])
				} else if len(coasts) > 1 {
					oi.TargetCoast = string(coasts[botIntn(len(coasts))])
				} else {
					continue
				}
			}

			// Validate before committing
			o := diplomacy.Order{
				UnitType:    u.Type,
				Power:       power,
				Location:    u.Province,
				Coast:       u.Coast,
				Type:        diplomacy.OrderMove,
				Target:      target,
				TargetCoast: diplomacy.Coast(oi.TargetCoast),
			}
			if diplomacy.ValidateOrder(o, gs, m) == nil {
				orders = append(orders, oi)
				moved = true
				break
			}
		}

		if !moved {
			orders = append(orders, OrderInput{
				UnitType:  u.Type.String(),
				Location:  u.Province,
				Coast:     string(u.Coast),
				OrderType: "hold",
			})
		}
	}
	return orders
}

// GenerateRetreatOrders picks a random valid retreat destination, or disbands.
func (RandomStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	var orders []OrderInput
	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}

		isFleet := d.Unit.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(d.DislodgedFrom, d.Unit.Coast, isFleet)

		retreated := false
		perm := botPerm(len(adj))
		for _, idx := range perm {
			target := adj[idx]
			// Cannot retreat to attacker's origin
			if target == d.AttackerFrom {
				continue
			}
			// Cannot retreat to occupied province
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

			oi := OrderInput{
				UnitType:  d.Unit.Type.String(),
				Location:  d.DislodgedFrom,
				Coast:     string(d.Unit.Coast),
				OrderType: "retreat_move",
				Target:    target,
			}
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(d.DislodgedFrom, d.Unit.Coast, target)
				if len(coasts) == 1 {
					oi.TargetCoast = string(coasts[0])
				} else if len(coasts) > 1 {
					oi.TargetCoast = string(coasts[botIntn(len(coasts))])
				} else {
					continue
				}
			}

			ro := diplomacy.RetreatOrder{
				UnitType:    d.Unit.Type,
				Power:       power,
				Location:    d.DislodgedFrom,
				Coast:       d.Unit.Coast,
				Type:        diplomacy.RetreatMove,
				Target:      target,
				TargetCoast: diplomacy.Coast(oi.TargetCoast),
			}
			if diplomacy.ValidateRetreatOrder(ro, gs, m) == nil {
				orders = append(orders, oi)
				retreated = true
				break
			}
		}

		if !retreated {
			orders = append(orders, OrderInput{
				UnitType:  d.Unit.Type.String(),
				Location:  d.DislodgedFrom,
				Coast:     string(d.Unit.Coast),
				OrderType: "retreat_disband",
			})
		}
	}
	return orders
}

// GenerateBuildOrders builds units on open home SCs or disbands excess units.
func (RandomStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	var orders []OrderInput

	if diff > 0 {
		// Need builds — find unoccupied home SCs we still own
		homes := diplomacy.HomeCenters(power)
		var available []string
		for _, h := range homes {
			if gs.SupplyCenters[h] == power && gs.UnitAt(h) == nil {
				available = append(available, h)
			}
		}
		botShuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })

		built := 0
		for _, loc := range available {
			if built >= diff {
				break
			}
			prov := m.Provinces[loc]
			if prov == nil {
				continue
			}

			// Choose unit type
			unitType := diplomacy.Army
			if prov.Type == diplomacy.Sea {
				unitType = diplomacy.Fleet
			} else if prov.Type == diplomacy.Coastal && botFloat64() < 0.3 {
				unitType = diplomacy.Fleet
			}

			oi := OrderInput{
				UnitType:  unitType.String(),
				Location:  loc,
				OrderType: "build",
			}

			// Fleet on split-coast needs coast
			if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
				oi.Coast = string(prov.Coasts[botIntn(len(prov.Coasts))])
			}

			bo := diplomacy.BuildOrder{
				Power:    power,
				Type:     diplomacy.BuildUnit,
				UnitType: unitType,
				Location: loc,
				Coast:    diplomacy.Coast(oi.Coast),
			}
			if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
				orders = append(orders, oi)
				built++
			}
		}
	} else if diff < 0 {
		// Need disbands
		needed := -diff
		units := gs.UnitsOf(power)
		botShuffle(len(units), func(i, j int) { units[i], units[j] = units[j], units[i] })
		for i := 0; i < needed && i < len(units); i++ {
			orders = append(orders, OrderInput{
				UnitType:  units[i].Type.String(),
				Location:  units[i].Province,
				Coast:     string(units[i].Coast),
				OrderType: "disband",
			})
		}
	}

	return orders
}
