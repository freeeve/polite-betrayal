package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestClassifyNeighborStancesInitialState(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// At game start, most powers have units on their home SCs which are
	// adjacent to neighbors' home SCs. Check a few known relationships.
	stances := ClassifyNeighborStances(gs, diplomacy.Austria, m)

	// Italy has ven adjacent to tri (Austrian SC), so Italy should appear.
	if s, ok := stances[diplomacy.Italy]; !ok {
		t.Error("expected Italy in Austria's neighbor stances")
	} else if s != StanceAggressive && s != StanceNeutral {
		t.Errorf("Italy stance toward Austria = %q, expected aggressive or neutral", s)
	}

	// Turkey has con adjacent to bul which borders ser/rum area near Austria.
	// At start, Turkey's units may or may not be adjacent to Austrian SCs.
	// Just verify the function doesn't panic and returns valid stances.
	for p, s := range stances {
		if s != StanceAggressive && s != StanceNeutral && s != StanceRetreating {
			t.Errorf("invalid stance %q for %s", s, p)
		}
	}
}

func TestClassifyNeighborStancesAllAdjacent(t *testing.T) {
	// Create a scenario where all of Germany's units border French SCs.
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			// France
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar"},
			{Type: diplomacy.Fleet, Power: diplomacy.France, Province: "bre"},
			// Germany: all units adjacent to French SCs
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bur"},  // adj to par, mar
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "pic"},  // adj to par, bre
			{Type: diplomacy.Fleet, Power: diplomacy.Germany, Province: "eng"}, // adj to bre
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France, "mar": diplomacy.France, "bre": diplomacy.France,
			"mun": diplomacy.Germany, "ber": diplomacy.Germany, "kie": diplomacy.Germany,
		},
	}
	m := diplomacy.StandardMap()

	stances := ClassifyNeighborStances(gs, diplomacy.France, m)
	if stances[diplomacy.Germany] != StanceAggressive {
		t.Errorf("Germany stance toward France = %q, want aggressive", stances[diplomacy.Germany])
	}
}

func TestClassifyNeighborStancesNoAdjacent(t *testing.T) {
	// Create a scenario where Turkey's units are far from English SCs.
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			// England
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "lon"},
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "edi"},
			{Type: diplomacy.Army, Power: diplomacy.England, Province: "lvp"},
			// Turkey: far from England
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "con"},
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "smy"},
			{Type: diplomacy.Fleet, Power: diplomacy.Turkey, Province: "bla"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"lon": diplomacy.England, "edi": diplomacy.England, "lvp": diplomacy.England,
			"ank": diplomacy.Turkey, "con": diplomacy.Turkey, "smy": diplomacy.Turkey,
		},
	}
	m := diplomacy.StandardMap()

	stances := ClassifyNeighborStances(gs, diplomacy.England, m)
	if stances[diplomacy.Turkey] != StanceRetreating {
		t.Errorf("Turkey stance toward England = %q, want retreating", stances[diplomacy.Turkey])
	}
}

func TestClassifyNeighborStancesMixed(t *testing.T) {
	// Russia has some units near and some far from German SCs.
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			// Germany
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "mun"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
			{Type: diplomacy.Fleet, Power: diplomacy.Germany, Province: "kie"},
			// Russia: 1 adjacent (sil borders ber/mun), 3 far
			{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "sil"}, // adj to mun, ber
			{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "mos"},
			{Type: diplomacy.Fleet, Power: diplomacy.Russia, Province: "sev"},
			{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "ukr"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"mun": diplomacy.Germany, "ber": diplomacy.Germany, "kie": diplomacy.Germany,
			"mos": diplomacy.Russia, "war": diplomacy.Russia, "sev": diplomacy.Russia, "stp": diplomacy.Russia,
		},
	}
	m := diplomacy.StandardMap()

	stances := ClassifyNeighborStances(gs, diplomacy.Germany, m)
	// 1 out of 4 = 25%, should be neutral
	if stances[diplomacy.Russia] != StanceNeutral {
		t.Errorf("Russia stance toward Germany = %q, want neutral", stances[diplomacy.Russia])
	}
}

func TestClassifyNeighborStancesExcludesSelf(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	stances := ClassifyNeighborStances(gs, diplomacy.England, m)
	if _, ok := stances[diplomacy.England]; ok {
		t.Error("should not include self in neighbor stances")
	}
	if _, ok := stances[diplomacy.Neutral]; ok {
		t.Error("should not include neutral in neighbor stances")
	}
}

func TestClassifyNeighborStancesEliminatedPower(t *testing.T) {
	// A power with no units should not appear in results.
	gs := &diplomacy.GameState{
		Year:   1910,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
		},
	}
	m := diplomacy.StandardMap()

	stances := ClassifyNeighborStances(gs, diplomacy.France, m)
	if len(stances) != 0 {
		t.Errorf("expected empty stances when only one power, got %d entries", len(stances))
	}
}
