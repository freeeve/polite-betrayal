package neural

import (
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func initialState() *diplomacy.GameState {
	return diplomacy.NewInitialState()
}

func TestAreaNamesAreSorted(t *testing.T) {
	// First 75 must be alphabetically sorted base provinces.
	for i := 1; i < 75; i++ {
		if AreaNames[i] <= AreaNames[i-1] {
			t.Errorf("AreaNames not sorted at [%d]=%q <= [%d]=%q", i, AreaNames[i], i-1, AreaNames[i-1])
		}
	}
}

func TestAreaNameCount(t *testing.T) {
	if len(AreaNames) != NumAreas {
		t.Errorf("expected %d area names, got %d", NumAreas, len(AreaNames))
	}
}

func TestAreaIndexRoundtrip(t *testing.T) {
	for i, name := range AreaNames {
		got := AreaIndex(name)
		if got != i {
			t.Errorf("AreaIndex(%q) = %d, want %d", name, got, i)
		}
	}
}

func TestAreaIndexUnknown(t *testing.T) {
	if AreaIndex("xxx") != -1 {
		t.Error("AreaIndex for unknown province should be -1")
	}
}

func TestBicoastalIndices(t *testing.T) {
	cases := []struct {
		prov  string
		coast diplomacy.Coast
		want  int
	}{
		{"bul", diplomacy.EastCoast, BulEC},
		{"bul", diplomacy.SouthCoast, BulSC},
		{"spa", diplomacy.NorthCoast, SpaNC},
		{"spa", diplomacy.SouthCoast, SpaSC},
		{"stp", diplomacy.NorthCoast, StpNC},
		{"stp", diplomacy.SouthCoast, StpSC},
		{"vie", diplomacy.NoCoast, -1},
		{"lon", diplomacy.NorthCoast, -1},
	}
	for _, tc := range cases {
		got := BicoastalIndex(tc.prov, tc.coast)
		if got != tc.want {
			t.Errorf("BicoastalIndex(%q, %q) = %d, want %d", tc.prov, tc.coast, got, tc.want)
		}
	}
}

func TestPowerIndex(t *testing.T) {
	powers := diplomacy.AllPowers()
	for i, p := range powers {
		if PowerIndex(p) != i {
			t.Errorf("PowerIndex(%q) = %d, want %d", p, PowerIndex(p), i)
		}
	}
	if PowerIndex(diplomacy.Neutral) != 7 {
		t.Error("PowerIndex for neutral should be 7")
	}
}

func TestEncodeBoardShape(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)
	if len(tensor) != NumAreas*NumFeatures {
		t.Errorf("tensor length = %d, want %d", len(tensor), NumAreas*NumFeatures)
	}
}

func TestEncodeBoardBinaryValues(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)
	for i, v := range tensor {
		if v != 0 && v != 1 {
			t.Errorf("tensor[%d] = %f, want 0 or 1", i, v)
		}
	}
}

func TestViennaHasAustrianArmy(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)
	vie := AreaIndex("vie")
	base := vie * NumFeatures

	if tensor[base+FeatUnitType] != 1 {
		t.Error("Vie should have army")
	}
	if tensor[base+FeatUnitType+1] != 0 {
		t.Error("Vie should not have fleet")
	}
	if tensor[base+FeatUnitType+2] != 0 {
		t.Error("Vie should not be empty")
	}
	if tensor[base+FeatUnitOwner] != 1 {
		t.Error("Austria (index 0) should own Vie unit")
	}
}

func TestLondonHasEnglishFleet(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)
	lon := AreaIndex("lon")
	base := lon * NumFeatures

	if tensor[base+FeatUnitType+1] != 1 {
		t.Error("Lon should have fleet")
	}
	if tensor[base+FeatUnitOwner+1] != 1 {
		t.Error("England (index 1) should own Lon unit")
	}
}

func TestStpSouthCoastFleet(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	// Base Stp province should have the fleet.
	stp := AreaIndex("stp")
	base := stp * NumFeatures
	if tensor[base+FeatUnitType+1] != 1 {
		t.Error("Stp should have fleet")
	}
	if tensor[base+FeatUnitOwner+5] != 1 {
		t.Error("Russia (index 5) should own Stp unit")
	}

	// Stp/sc variant should also show the fleet.
	varBase := StpSC * NumFeatures
	if tensor[varBase+FeatUnitType+1] != 1 {
		t.Error("Stp/sc should have fleet")
	}
	if tensor[varBase+FeatUnitOwner+5] != 1 {
		t.Error("Russia should own Stp/sc unit")
	}

	// Stp/nc should be empty.
	ncBase := StpNC * NumFeatures
	if tensor[ncBase+FeatUnitType+2] != 1 {
		t.Error("Stp/nc should be empty")
	}
}

func TestSCOwnershipInitial(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	// Vienna is Austrian SC.
	vieBase := AreaIndex("vie") * NumFeatures
	if tensor[vieBase+FeatSCOwner] != 1 {
		t.Error("Vie SC should be owned by Austria")
	}

	// Serbia is neutral SC.
	serBase := AreaIndex("ser") * NumFeatures
	if tensor[serBase+FeatSCOwner+NumPowers] != 1 {
		t.Error("Ser should be neutral SC")
	}

	// Bohemia is not an SC.
	bohBase := AreaIndex("boh") * NumFeatures
	if tensor[bohBase+FeatSCOwner+NumPowers+1] != 1 {
		t.Error("Boh should not be an SC (none)")
	}
}

func TestProvinceTypesCorrect(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	// Bohemia is inland.
	bohBase := AreaIndex("boh") * NumFeatures
	if tensor[bohBase+FeatProvinceType] != 1 {
		t.Error("Boh should be land")
	}

	// North Sea is sea.
	nthBase := AreaIndex("nth") * NumFeatures
	if tensor[nthBase+FeatProvinceType+1] != 1 {
		t.Error("Nth should be sea")
	}

	// London is coastal.
	lonBase := AreaIndex("lon") * NumFeatures
	if tensor[lonBase+FeatProvinceType+2] != 1 {
		t.Error("Lon should be coast")
	}

	// Bicoastal variant is coastal.
	bulECBase := BulEC * NumFeatures
	if tensor[bulECBase+FeatProvinceType+2] != 1 {
		t.Error("Bul/ec should be coast")
	}
}

func TestEmptyProvincesMarkedCorrectly(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	// Galicia has no unit.
	galBase := AreaIndex("gal") * NumFeatures
	if tensor[galBase+FeatUnitType+2] != 1 {
		t.Error("Gal should be empty")
	}
	if tensor[galBase+FeatUnitOwner+NumPowers] != 1 {
		t.Error("Gal owner should be none")
	}
}

func TestNoDislodgedInInitial(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		if tensor[base+FeatDislodgedType+2] != 1 {
			t.Errorf("Area %d (%s) should have no dislodged unit", area, AreaNames[area])
		}
	}
}

func TestNoPrevStateFillsEmpty(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	tensor := EncodeBoard(gs, m, nil)

	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		if tensor[base+FeatPrevUnitType+2] != 1 {
			t.Errorf("Area %d prev should be empty", area)
		}
		if tensor[base+FeatPrevUnitOwner+NumPowers] != 1 {
			t.Errorf("Area %d prev owner should be none", area)
		}
	}
}

func TestPrevStateEncodesUnits(t *testing.T) {
	m := diplomacy.StandardMap()

	current := &diplomacy.GameState{
		Year: 1901, Season: diplomacy.Fall, Phase: diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
		},
		SupplyCenters: map[string]diplomacy.Power{"ser": diplomacy.Neutral},
	}

	prev := &diplomacy.GameState{
		Year: 1901, Season: diplomacy.Spring, Phase: diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
		},
		SupplyCenters: map[string]diplomacy.Power{"bud": diplomacy.Austria},
	}

	tensor := EncodeBoard(current, m, prev)

	// Current: Ser has army.
	serBase := AreaIndex("ser") * NumFeatures
	if tensor[serBase+FeatUnitType] != 1 {
		t.Error("Ser should have current army")
	}

	// Previous: Bud had army.
	budBase := AreaIndex("bud") * NumFeatures
	if tensor[budBase+FeatPrevUnitType] != 1 {
		t.Error("Bud should have prev army")
	}
	if tensor[budBase+FeatPrevUnitOwner] != 1 {
		t.Error("Bud prev army should be Austrian")
	}

	// Ser was empty in prev state.
	if tensor[serBase+FeatPrevUnitType+2] != 1 {
		t.Error("Ser should be empty in prev state")
	}
}

func TestAdjacencyMatrixShape(t *testing.T) {
	m := diplomacy.StandardMap()
	adj := BuildAdjacencyMatrix(m)
	if len(adj) != NumAreas*NumAreas {
		t.Errorf("adj length = %d, want %d", len(adj), NumAreas*NumAreas)
	}
}

func TestAdjacencyHasSelfLoops(t *testing.T) {
	m := diplomacy.StandardMap()
	adj := BuildAdjacencyMatrix(m)
	for i := 0; i < NumAreas; i++ {
		if adj[i*NumAreas+i] != 1 {
			t.Errorf("Self-loop missing for area %d", i)
		}
	}
}

func TestAdjacencyIsSymmetric(t *testing.T) {
	m := diplomacy.StandardMap()
	adj := BuildAdjacencyMatrix(m)
	for i := 0; i < NumAreas; i++ {
		for j := 0; j < NumAreas; j++ {
			if adj[i*NumAreas+j] != adj[j*NumAreas+i] {
				t.Errorf("Asymmetric at (%d, %d)", i, j)
			}
		}
	}
}

func TestAdjacencyKnownEdges(t *testing.T) {
	m := diplomacy.StandardMap()
	adj := BuildAdjacencyMatrix(m)

	// Vienna <-> Bohemia should be connected.
	vie := AreaIndex("vie")
	boh := AreaIndex("boh")
	if adj[vie*NumAreas+boh] != 1 {
		t.Error("Vie-Boh should be adjacent")
	}

	// Vienna <-> Venice should NOT be connected.
	ven := AreaIndex("ven")
	if adj[vie*NumAreas+ven] != 0 {
		t.Error("Vie-Ven should not be adjacent")
	}

	// Smyrna <-> Ankara should be connected.
	smy := AreaIndex("smy")
	ank := AreaIndex("ank")
	if adj[smy*NumAreas+ank] != 1 {
		t.Error("Smy-Ank should be adjacent")
	}
}

func TestBicoastalVariantsConnectedToBase(t *testing.T) {
	m := diplomacy.StandardMap()
	adj := BuildAdjacencyMatrix(m)

	bul := AreaIndex("bul")
	if adj[bul*NumAreas+BulEC] != 1 {
		t.Error("Bul should be connected to Bul/ec")
	}
	if adj[bul*NumAreas+BulSC] != 1 {
		t.Error("Bul should be connected to Bul/sc")
	}
}

func TestCollectUnitIndicesAustria(t *testing.T) {
	gs := initialState()
	indices := CollectUnitIndices(gs, diplomacy.Austria)
	if len(indices) != MaxUnits {
		t.Errorf("indices length = %d, want %d", len(indices), MaxUnits)
	}

	// Austria has 3 units: Vie, Bud, Tri.
	active := indices[:3]
	hasProvince := func(idx int) bool {
		for _, a := range active {
			if int(a) == idx {
				return true
			}
		}
		return false
	}
	for _, prov := range []string{"vie", "bud", "tri"} {
		if !hasProvince(AreaIndex(prov)) {
			t.Errorf("Austria should have unit at %s", prov)
		}
	}

	// Remaining slots should be zero-padded.
	for i := 3; i < MaxUnits; i++ {
		if indices[i] != 0 {
			t.Errorf("indices[%d] = %d, want 0", i, indices[i])
		}
	}
}

func TestMapProvincesCoverAllBaseAreas(t *testing.T) {
	m := diplomacy.StandardMap()
	for i := 0; i < 75; i++ {
		name := AreaNames[i]
		if m.Provinces[name] == nil {
			t.Errorf("Province %q (area %d) not found in standard map", name, i)
		}
	}
}

// ---------------------------------------------------------------------------
// encodeBuildDisband tests
// ---------------------------------------------------------------------------

func TestEncodeBuildDisband_CanBuild(t *testing.T) {
	m := diplomacy.StandardMap()
	// Austria has 4 SCs but only 1 unit -> needs 3 builds.
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria,
			"tri": diplomacy.Austria, "ser": diplomacy.Austria,
		},
	}

	tensor := EncodeBoard(gs, m, nil)

	// Vienna, Budapest, Trieste are home centers with no unit -> can build.
	for _, prov := range []string{"vie", "bud", "tri"} {
		base := AreaIndex(prov) * NumFeatures
		if tensor[base+FeatCanBuild] != 1 {
			t.Errorf("expected FeatCanBuild=1 for %s", prov)
		}
	}

	// Ser has a unit, so it should not be marked can-build.
	serBase := AreaIndex("ser") * NumFeatures
	if tensor[serBase+FeatCanBuild] != 0 {
		t.Errorf("Ser (occupied) should not be marked can-build")
	}
}

func TestEncodeBuildDisband_MustDisband(t *testing.T) {
	m := diplomacy.StandardMap()
	// Austria has 1 SC but 3 units -> needs 2 disbands.
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
			{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria,
		},
	}

	tensor := EncodeBoard(gs, m, nil)

	// All Austrian units should be marked can-disband.
	for _, prov := range []string{"vie", "bud", "tri"} {
		base := AreaIndex(prov) * NumFeatures
		if tensor[base+FeatCanDisband] != 1 {
			t.Errorf("expected FeatCanDisband=1 for %s", prov)
		}
	}
}

func TestEncodeBuildDisband_Balanced(t *testing.T) {
	m := diplomacy.StandardMap()
	// Austria has 3 SCs and 3 units -> no builds or disbands.
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
			{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
		},
	}

	tensor := EncodeBoard(gs, m, nil)

	// No can-build or can-disband flags.
	for _, prov := range []string{"vie", "bud", "tri"} {
		base := AreaIndex(prov) * NumFeatures
		if tensor[base+FeatCanBuild] != 0 {
			t.Errorf("%s should not have FeatCanBuild", prov)
		}
		if tensor[base+FeatCanDisband] != 0 {
			t.Errorf("%s should not have FeatCanDisband", prov)
		}
	}
}
