package bot

import (
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

var (
	allocGS    = diplomacy.NewInitialState()
	allocMap   = diplomacy.StandardMap()
	allocPower = diplomacy.France
	allocUnits = allocGS.UnitsOf(allocPower)
)

func BenchmarkAlloc_SearchTopN(b *testing.B) {
	k := adaptiveK(len(allocUnits), 1000)
	var unitOrders [][]diplomacy.Order
	for _, u := range allocUnits {
		legal := LegalOrdersForUnit(u, allocGS, allocMap)
		top := TopKOrders(legal, k, allocGS, allocPower, allocMap)
		unitOrders = append(unitOrders, top)
	}
	opp := func() []diplomacy.Order {
		var orders []diplomacy.Order
		for _, p := range diplomacy.AllPowers() {
			if p == allocPower || !allocGS.PowerIsAlive(p) {
				continue
			}
			orders = append(orders, GenerateOpponentOrders(allocGS, p, allocMap)...)
		}
		return orders
	}()
	deadline := time.Now().Add(10 * time.Second)
	b.ReportAllocs()
	for b.Loop() {
		searchTopN(allocGS, allocPower, allocMap, unitOrders, opp, 3, 1000, deadline)
	}
}

func BenchmarkAlloc_EvaluatePosition(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		EvaluatePosition(allocGS, allocPower, allocMap)
	}
}

func BenchmarkAlloc_Clone(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		allocGS.Clone()
	}
}

func BenchmarkAlloc_ResolveOrders(b *testing.B) {
	k := adaptiveK(len(allocUnits), 1000)
	var unitOrders [][]diplomacy.Order
	for _, u := range allocUnits {
		legal := LegalOrdersForUnit(u, allocGS, allocMap)
		top := TopKOrders(legal, k, allocGS, allocPower, allocMap)
		unitOrders = append(unitOrders, top)
	}
	opp := func() []diplomacy.Order {
		var orders []diplomacy.Order
		for _, p := range diplomacy.AllPowers() {
			if p == allocPower || !allocGS.PowerIsAlive(p) {
				continue
			}
			orders = append(orders, GenerateOpponentOrders(allocGS, p, allocMap)...)
		}
		return orders
	}()
	var combo []diplomacy.Order
	for _, uo := range unitOrders {
		combo = append(combo, uo[0])
	}
	allOrders := append(combo, opp...)
	b.ReportAllocs()
	for b.Loop() {
		diplomacy.ResolveOrders(allOrders, allocGS, allocMap)
	}
}

func BenchmarkAlloc_NearestUnownedSC(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		NearestUnownedSC("par", allocPower, allocGS, allocMap)
	}
}
