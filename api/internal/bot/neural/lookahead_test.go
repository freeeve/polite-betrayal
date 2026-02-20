package neural

import (
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestScoreMoveFast(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Neutral SC should score 10
	score := scoreMoveFast("nwy", diplomacy.Russia, gs, m)
	if score != 10 {
		t.Errorf("neutral SC: got %f, want 10", score)
	}

	// Enemy SC should score 7
	score = scoreMoveFast("ank", diplomacy.Russia, gs, m)
	if score != 7 {
		t.Errorf("enemy SC: got %f, want 7", score)
	}

	// Own SC (unoccupied) should score 1 — use war (Russian SC, army there)
	// war has a Russian unit so it gets -15 penalty. Use a custom state instead.
	gsNoUnit := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units:  nil,
		SupplyCenters: map[string]diplomacy.Power{
			"sev": diplomacy.Russia,
		},
	}
	score = scoreMoveFast("sev", diplomacy.Russia, gsNoUnit, m)
	if score != 1 {
		t.Errorf("own SC (unoccupied): got %f, want 1", score)
	}

	// Non-SC should score 0
	score = scoreMoveFast("ukr", diplomacy.Russia, gs, m)
	if score != 0 {
		t.Errorf("non-SC: got %f, want 0", score)
	}

	// Own-occupied province should get -15 penalty
	score = scoreMoveFast("mos", diplomacy.Russia, gs, m)
	// mos is not an SC in the initial map data, let me check
	// Actually mos IS an SC. Own SC + own unit = 1 - 15 = -14
	if score != -14 {
		t.Errorf("own-occupied own SC: got %f, want -14", score)
	}
}

func TestGenerateGreedyOrdersFastInitialState(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	orders := GenerateGreedyOrdersFast(gs, m)

	// Should have one order per unit (22 units in initial state)
	if len(orders) != 22 {
		t.Fatalf("expected 22 orders, got %d", len(orders))
	}

	// Every order should be hold or move
	for _, o := range orders {
		if o.Type != diplomacy.OrderHold && o.Type != diplomacy.OrderMove {
			t.Errorf("unexpected order type: %v for unit at %s", o.Type, o.Location)
		}
	}

	// No two units of the same power should move to the same destination
	type powerTarget struct {
		power  diplomacy.Power
		target string
	}
	seen := make(map[powerTarget]string)
	for _, o := range orders {
		if o.Type != diplomacy.OrderMove {
			continue
		}
		key := powerTarget{o.Power, o.Target}
		if prev, exists := seen[key]; exists {
			t.Errorf("%s: two units move to %s (from %s and %s)", o.Power, o.Target, prev, o.Location)
		}
		seen[key] = o.Location
	}
}

func TestGenerateGreedyOrdersFastCollisionResolution(t *testing.T) {
	// Create a state where two units of the same power are adjacent to
	// the same high-value target (neutral SC), forcing collision resolution.
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "boh", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "gal", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"ser": diplomacy.Neutral,
			"rum": diplomacy.Neutral,
			"bud": diplomacy.Austria,
		},
	}
	m := diplomacy.StandardMap()

	orders := GenerateGreedyOrdersFast(gs, m)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// Both should be moves (not both to the same target)
	moveTargets := make(map[string]bool)
	for _, o := range orders {
		if o.Type == diplomacy.OrderMove {
			if moveTargets[o.Target] {
				t.Errorf("collision not resolved: both units move to %s", o.Target)
			}
			moveTargets[o.Target] = true
		}
	}
}

func TestGreedyOrderCacheHitMiss(t *testing.T) {
	cache := NewGreedyOrderCache()

	orders := []diplomacy.Order{
		{Type: diplomacy.OrderHold, Location: "par", Power: diplomacy.France},
	}

	// Miss
	_, ok := cache.Get(42)
	if ok {
		t.Error("expected cache miss")
	}

	// Put and hit
	cache.Put(42, orders)
	got, ok := cache.Get(42)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].Location != "par" {
		t.Errorf("unexpected cached orders: %v", got)
	}

	// Verify length
	if cache.Len() != 1 {
		t.Errorf("expected length 1, got %d", cache.Len())
	}
}

func TestGreedyOrderCacheEviction(t *testing.T) {
	cache := &GreedyOrderCache{
		m:        make(map[uint64][]diplomacy.Order, 4),
		capacity: 4,
	}

	orders := []diplomacy.Order{{Type: diplomacy.OrderHold}}

	// Fill to capacity
	for i := uint64(0); i < 4; i++ {
		cache.Put(i, orders)
	}
	if cache.Len() != 4 {
		t.Fatalf("expected 4 entries, got %d", cache.Len())
	}

	// Next insert should clear all and add new
	cache.Put(100, orders)
	if cache.Len() != 1 {
		t.Errorf("expected 1 entry after eviction, got %d", cache.Len())
	}

	// Old keys should be gone
	_, ok := cache.Get(0)
	if ok {
		t.Error("expected old key to be evicted")
	}

	// New key should exist
	_, ok = cache.Get(100)
	if !ok {
		t.Error("expected new key to be present")
	}
}

func TestHashBoardForMovegen(t *testing.T) {
	gs1 := diplomacy.NewInitialState()
	gs2 := diplomacy.NewInitialState()

	h1 := HashBoardForMovegen(gs1)
	h2 := HashBoardForMovegen(gs2)

	if h1 != h2 {
		t.Errorf("identical states should have same hash: %d != %d", h1, h2)
	}

	// Different year should not change the hash (year is excluded)
	gs2.Year = 1905
	h2 = HashBoardForMovegen(gs2)
	if h1 != h2 {
		t.Errorf("different year should not change hash: %d != %d", h1, h2)
	}

	// Different season should change the hash
	gs2.Year = 1901
	gs2.Season = diplomacy.Fall
	h2 = HashBoardForMovegen(gs2)
	if h1 == h2 {
		t.Error("different season should change hash")
	}

	// Different units should change the hash
	gs3 := diplomacy.NewInitialState()
	gs3.Units = gs3.Units[:len(gs3.Units)-1] // remove last unit
	h3 := HashBoardForMovegen(gs3)
	if h1 == h3 {
		t.Error("different units should change hash")
	}
}

func TestSimulateNPhasesMovement(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cache := NewGreedyOrderCache()

	// Simulate 1 phase (Spring 1901 Movement)
	result := SimulateNPhases(gs, m, 1, gs.Year, cache)

	// Original state should be unchanged
	if gs.Year != 1901 || gs.Season != diplomacy.Spring {
		t.Error("original state was mutated")
	}

	// Result should have advanced past Spring movement
	// After Spring movement with no dislodgements, goes to Fall movement
	if result.Season != diplomacy.Fall || result.Phase != diplomacy.PhaseMovement {
		t.Errorf("expected Fall Movement, got %s %s", result.Season, result.Phase)
	}
	if result.Year != 1901 {
		t.Errorf("expected year 1901, got %d", result.Year)
	}

	// Units should still exist (greedy moves don't annihilate everyone)
	if len(result.Units) == 0 {
		t.Error("all units disappeared")
	}
}

func TestSimulateNPhasesMultiple(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cache := NewGreedyOrderCache()

	// Simulate 4 phases to go through full year cycle
	result := SimulateNPhases(gs, m, 4, gs.Year, cache)

	// Should have progressed beyond start
	if result.Year == 1901 && result.Season == diplomacy.Spring && result.Phase == diplomacy.PhaseMovement {
		t.Error("simulation did not advance")
	}

	// Units should still exist
	if len(result.Units) == 0 {
		t.Error("all units disappeared")
	}

	// Cache should have been populated
	if cache.Len() == 0 {
		t.Error("cache should have entries after simulation")
	}
}

func TestSimulateNPhasesYearLimit(t *testing.T) {
	gs := diplomacy.NewInitialState()
	gs.Year = 1910
	m := diplomacy.StandardMap()
	cache := NewGreedyOrderCache()

	// With startYear=1910 and depth=100, should stop at year > 1912
	result := SimulateNPhases(gs, m, 100, gs.Year, cache)
	if result.Year > 1913 {
		t.Errorf("simulation should stop near year 1912, got year %d", result.Year)
	}
}

func TestSimulateNPhasesWithRetreat(t *testing.T) {
	// Set up a state where a dislodgement will happen: two units attacking one.
	m := diplomacy.StandardMap()

	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			// France has two armies that can converge on Burgundy
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar", Coast: diplomacy.NoCoast},
			// Germany has an army in Burgundy
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bur", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
			"mar": diplomacy.France,
			"bre": diplomacy.France,
			"mun": diplomacy.Germany,
			"ber": diplomacy.Germany,
			"kie": diplomacy.Germany,
		},
	}

	cache := NewGreedyOrderCache()

	// Simulate a few phases - should not panic even with retreats
	result := SimulateNPhases(gs, m, 4, gs.Year, cache)
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestHeuristicRetreatOrders(t *testing.T) {
	m := diplomacy.StandardMap()

	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseRetreat,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
		},
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bur", Coast: diplomacy.NoCoast},
				DislodgedFrom: "bur",
				AttackerFrom:  "par",
			},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
			"mun": diplomacy.Germany,
		},
	}

	orders := heuristicRetreatOrders(gs, diplomacy.Germany, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d", len(orders))
	}

	ro := orders[0]
	if ro.Type != diplomacy.RetreatMove {
		t.Errorf("expected retreat move, got disband")
	}
	if ro.Target == "par" {
		t.Error("should not retreat to attacker's province")
	}
	if ro.Target == "bur" {
		t.Error("should not retreat to own province")
	}
}

func TestHeuristicBuildOrders(t *testing.T) {
	m := diplomacy.StandardMap()

	// France has 4 SCs but only 2 units -> needs 2 builds
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Fleet, Power: diplomacy.France, Province: "mao", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
			"mar": diplomacy.France,
			"bre": diplomacy.France,
			"spa": diplomacy.France,
		},
	}

	orders := heuristicBuildOrders(gs, diplomacy.France, m)
	if len(orders) == 0 {
		t.Fatal("expected build orders")
	}
	if len(orders) > 2 {
		t.Fatalf("expected at most 2 builds, got %d", len(orders))
	}

	for _, bo := range orders {
		if bo.Type != diplomacy.BuildUnit {
			t.Errorf("expected build, got %v", bo.Type)
		}
		if bo.Power != diplomacy.France {
			t.Errorf("expected France, got %s", bo.Power)
		}
	}
}

func TestHeuristicDisbandOrders(t *testing.T) {
	m := diplomacy.StandardMap()

	// France has 1 SC but 3 units -> needs 2 disbands
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "gas", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
		},
	}

	orders := heuristicBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 2 {
		t.Fatalf("expected 2 disband orders, got %d", len(orders))
	}

	for _, bo := range orders {
		if bo.Type != diplomacy.DisbandUnit {
			t.Errorf("expected disband, got %v", bo.Type)
		}
	}
}

func TestNearestUnownedSCDist(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// From nwy (neutral SC not owned by Russia), distance should be 0
	dist := nearestUnownedSCDist("nwy", diplomacy.Russia, gs, m, false)
	if dist != 0 {
		t.Errorf("expected 0 from neutral SC, got %d", dist)
	}

	// From mos (owned by Russia), distance should be > 0
	dist = nearestUnownedSCDist("mos", diplomacy.Russia, gs, m, false)
	if dist == 0 {
		t.Error("expected nonzero distance from own SC")
	}
	if dist < 0 {
		t.Error("expected reachable SC from mos")
	}
}

func BenchmarkGenerateGreedyOrdersFast(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateGreedyOrdersFast(gs, m)
	}
}

func BenchmarkSimulateNPhases(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache := NewGreedyOrderCache()
		SimulateNPhases(gs, m, LookaheadDepth, gs.Year, cache)
	}
}
