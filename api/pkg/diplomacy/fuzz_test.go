package diplomacy

import (
	"math/rand"
	"testing"
)

// FuzzResolveOrders verifies the resolver doesn't panic on random order combinations.
func FuzzResolveOrders(f *testing.F) {
	f.Add(int64(42))
	f.Add(int64(123456))
	f.Add(int64(0))

	f.Fuzz(func(t *testing.T, seed int64) {
		rng := rand.New(rand.NewSource(seed))
		m := StandardMap()
		gs := NewInitialState()

		// Generate random orders for each unit
		var orders []Order
		for _, unit := range gs.Units {
			order := randomOrder(rng, unit, gs, m)
			orders = append(orders, order)
		}

		// Should not panic
		validated, _ := ValidateAndDefaultOrders(orders, gs, m)
		results, dislodged := ResolveOrders(validated, gs, m)

		// Basic invariant checks
		if len(results) != len(validated) {
			t.Errorf("expected %d results, got %d", len(validated), len(results))
		}

		// No unit should appear in results and dislodged unless it was dislodged
		dislodgedProvs := make(map[string]bool)
		for _, d := range dislodged {
			dislodgedProvs[d.DislodgedFrom] = true
		}

		for _, r := range results {
			if r.Result == ResultDislodged && !dislodgedProvs[r.Order.Location] {
				t.Error("result says dislodged but unit not in dislodged list")
			}
		}
	})
}

func randomOrder(rng *rand.Rand, unit Unit, gs *GameState, m *DiplomacyMap) Order {
	order := Order{
		UnitType: unit.Type,
		Power:    unit.Power,
		Location: unit.Province,
		Coast:    unit.Coast,
	}

	isFleet := unit.Type == Fleet
	adj := m.ProvincesAdjacentTo(unit.Province, unit.Coast, isFleet)

	switch rng.Intn(4) {
	case 0: // Hold
		order.Type = OrderHold
	case 1: // Move
		order.Type = OrderMove
		if len(adj) > 0 {
			order.Target = adj[rng.Intn(len(adj))]
		} else {
			order.Type = OrderHold
		}
	case 2: // Support
		order.Type = OrderSupport
		if len(adj) > 0 {
			target := adj[rng.Intn(len(adj))]
			supported := gs.UnitAt(target)
			if supported != nil {
				order.AuxLoc = target
				order.AuxUnitType = supported.Type
				// 50% support hold, 50% support move
				if rng.Intn(2) == 0 {
					supportedAdj := m.ProvincesAdjacentTo(target, supported.Coast, supported.Type == Fleet)
					if len(supportedAdj) > 0 {
						order.AuxTarget = supportedAdj[rng.Intn(len(supportedAdj))]
					}
				}
			} else {
				order.Type = OrderHold
			}
		} else {
			order.Type = OrderHold
		}
	case 3: // Convoy (only for fleets in sea)
		prov := m.Provinces[unit.Province]
		if isFleet && prov != nil && prov.Type == Sea {
			order.Type = OrderConvoy
			// Pick a random army to convoy
			for _, u := range gs.Units {
				if u.Type == Army {
					uAdj := m.ProvincesAdjacentTo(u.Province, u.Coast, false)
					if len(uAdj) > 0 {
						order.AuxLoc = u.Province
						order.AuxTarget = uAdj[rng.Intn(len(uAdj))]
						break
					}
				}
			}
			if order.AuxLoc == "" {
				order.Type = OrderHold
			}
		} else {
			order.Type = OrderHold
		}
	}

	return order
}
