package diplomacy

import (
	"testing"
)

func TestGameState_Clone_Independent(t *testing.T) {
	gs := NewInitialState()
	c := gs.Clone()

	// Scalar fields match
	if c.Year != gs.Year || c.Season != gs.Season || c.Phase != gs.Phase {
		t.Fatal("cloned scalars do not match original")
	}

	// Mutate original units — clone must be unaffected
	origProvince := gs.Units[0].Province
	gs.Units[0].Province = "xxx"
	if c.Units[0].Province != origProvince {
		t.Error("clone units should be independent of original")
	}

	// Mutate clone SCs — original must be unaffected
	c.SupplyCenters["zzz"] = France
	if _, ok := gs.SupplyCenters["zzz"]; ok {
		t.Error("original SCs should be independent of clone")
	}

	// Delete from original SCs — clone must be unaffected
	delete(gs.SupplyCenters, "par")
	if _, ok := c.SupplyCenters["par"]; !ok {
		t.Error("clone SCs should retain 'par' after original deletes it")
	}
}

func TestGameState_Clone_WithDislodged(t *testing.T) {
	gs := &GameState{
		Year:   1902,
		Season: Fall,
		Phase:  PhaseRetreat,
		Units: []Unit{
			{Army, France, "par", NoCoast},
		},
		SupplyCenters: map[string]Power{"par": France},
		Dislodged: []DislodgedUnit{
			{
				Unit:          Unit{Fleet, England, "lon", NoCoast},
				DislodgedFrom: "lon",
				AttackerFrom:  "nth",
			},
		},
	}

	c := gs.Clone()

	if len(c.Dislodged) != 1 {
		t.Fatalf("expected 1 dislodged, got %d", len(c.Dislodged))
	}
	if c.Dislodged[0].DislodgedFrom != "lon" {
		t.Errorf("expected dislodged from lon, got %s", c.Dislodged[0].DislodgedFrom)
	}

	// Mutate original dislodged — clone must be unaffected
	gs.Dislodged[0].AttackerFrom = "yyy"
	if c.Dislodged[0].AttackerFrom != "nth" {
		t.Error("clone dislodged should be independent of original")
	}
}

func TestGameState_Clone_NilSlices(t *testing.T) {
	gs := &GameState{Year: 1901, Season: Spring, Phase: PhaseMovement}
	c := gs.Clone()

	if c.Units != nil {
		t.Error("clone of nil Units should be nil")
	}
	if c.SupplyCenters != nil {
		t.Error("clone of nil SupplyCenters should be nil")
	}
	if c.Dislodged != nil {
		t.Error("clone of nil Dislodged should be nil")
	}
}

func TestIsYearLimitReached(t *testing.T) {
	tests := []struct {
		year int
		want bool
	}{
		{1901, false},
		{2999, false},
		{3000, false},
		{3001, true},
		{4000, true},
	}
	for _, tt := range tests {
		gs := &GameState{Year: tt.year}
		if got := IsYearLimitReached(gs); got != tt.want {
			t.Errorf("IsYearLimitReached(year=%d) = %v, want %v", tt.year, got, tt.want)
		}
	}
}

func TestGameState_Clone_Counts(t *testing.T) {
	gs := NewInitialState()
	c := gs.Clone()

	for _, power := range AllPowers() {
		if c.SupplyCenterCount(power) != gs.SupplyCenterCount(power) {
			t.Errorf("%s: SC count mismatch", power)
		}
		if c.UnitCount(power) != gs.UnitCount(power) {
			t.Errorf("%s: unit count mismatch", power)
		}
	}
	if len(c.Units) != len(gs.Units) {
		t.Errorf("unit slice length: %d vs %d", len(c.Units), len(gs.Units))
	}
}
