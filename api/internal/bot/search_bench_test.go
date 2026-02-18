package bot

import (
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func BenchmarkResolveOrders(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Generate orders for all powers
	h := HeuristicStrategy{}
	var orders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		inputs := h.GenerateMovementOrders(gs, p, m)
		orders = append(orders, OrderInputsToOrders(inputs, p)...)
	}

	b.ResetTimer()
	for b.Loop() {
		diplomacy.ResolveOrders(orders, gs, m)
	}
}

func BenchmarkLegalOrdersForUnit(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("par")

	b.ResetTimer()
	for b.Loop() {
		LegalOrdersForUnit(unit, gs, m)
	}
}

func BenchmarkEvaluatePosition(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	b.ResetTimer()
	for b.Loop() {
		EvaluatePosition(gs, diplomacy.France, m)
	}
}

func BenchmarkSearchBestOrders_Small(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	power := diplomacy.France

	units := gs.UnitsOf(power)
	var unitOrders [][]diplomacy.Order
	for _, u := range units {
		all := LegalOrdersForUnit(u, gs, m)
		top := TopKOrders(all, 3, gs, power, m)
		unitOrders = append(unitOrders, top)
	}

	var oppOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		oppOrders = append(oppOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	b.ResetTimer()
	for b.Loop() {
		deadline := time.Now().Add(10 * time.Second)
		searchBestOrders(gs, power, m, unitOrders, oppOrders, deadline)
	}
}

func BenchmarkMediumBot_InitialState(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := &TacticalStrategy{}

	b.ResetTimer()
	for b.Loop() {
		s.GenerateMovementOrders(gs, diplomacy.France, m)
	}
}
