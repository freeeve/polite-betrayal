package bot

import (
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestBFSDistance_SameProvince(t *testing.T) {
	m := diplomacy.StandardMap()
	if d := BFSDistance("par", "par", m); d != 0 {
		t.Errorf("expected 0, got %d", d)
	}
}

func TestBFSDistance_Adjacent(t *testing.T) {
	m := diplomacy.StandardMap()
	if d := BFSDistance("par", "bur", m); d != 1 {
		t.Errorf("par->bur: expected 1, got %d", d)
	}
}

func TestBFSDistance_TwoHops(t *testing.T) {
	m := diplomacy.StandardMap()
	// par -> bur -> mun (2 hops by army)
	d := BFSDistance("par", "mun", m)
	if d != 2 {
		t.Errorf("par->mun: expected 2, got %d", d)
	}
}

func TestBFSDistance_Unreachable(t *testing.T) {
	m := diplomacy.StandardMap()
	// Sea provinces are not reachable by army BFS
	d := BFSDistance("par", "mid", m)
	if d != -1 {
		t.Errorf("par->mid (sea): expected -1, got %d", d)
	}
}

func TestNearestUnownedSC_Initial(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// France starts with par, bre, mar. Nearby unowned SCs: spa, bel, etc.
	prov, dist := NearestUnownedSC("par", diplomacy.France, gs, m)
	if dist < 0 {
		t.Fatal("expected to find an unowned SC")
	}
	if prov == "" {
		t.Fatal("expected non-empty province")
	}
	// par is adjacent to bur which is adjacent to bel (neutral SC, 2 hops)
	// or par -> pic -> bel (2 hops), par -> gas -> spa (2 hops)
	if dist > 3 {
		t.Errorf("expected distance <= 3 for nearest unowned SC from par, got %d (prov=%s)", dist, prov)
	}
}

func TestNearestUnownedSC_AllOwned(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Give France all SCs so nothing is "unowned by France"
	for sc := range gs.SupplyCenters {
		gs.SupplyCenters[sc] = diplomacy.France
	}
	// Also claim neutral SCs
	for id, prov := range m.Provinces {
		if prov.IsSupplyCenter {
			gs.SupplyCenters[id] = diplomacy.France
		}
	}

	_, dist := NearestUnownedSC("par", diplomacy.France, gs, m)
	if dist != -1 {
		t.Errorf("expected -1 when all SCs owned, got %d", dist)
	}
}

func TestProvinceThreat_Initial(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// bur is adjacent to French A par and A mar, but those are friendly.
	// No enemy units adjacent to bur at start.
	threat := ProvinceThreat("bur", diplomacy.France, gs, m)
	// Germany's A mun can reach bur
	if threat < 1 {
		t.Errorf("expected at least 1 threat to bur (from mun), got %d", threat)
	}
}

func TestProvinceThreat_NoThreats(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// mos (Moscow) - deep in Russia, not adjacent to any non-Russian units at start
	threat := ProvinceThreat("mos", diplomacy.Russia, gs, m)
	if threat != 0 {
		t.Errorf("expected 0 threats to mos for Russia, got %d", threat)
	}
}

func TestProvinceDefense(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// bur is adjacent to French A par and A mar
	defense := ProvinceDefense("bur", diplomacy.France, gs, m)
	if defense < 2 {
		t.Errorf("expected at least 2 French defenders for bur, got %d", defense)
	}
}

func TestCanSupportMove(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// French A par can support A mar -> bur
	parUnit := *gs.UnitAt("par")
	if !CanSupportMove("par", "mar", "bur", parUnit, gs, m) {
		t.Error("expected A par to be able to support A mar -> bur")
	}

	// French A par cannot support A mar -> spa (par not adjacent to spa... wait, it is via gas)
	// Actually par is not adjacent to spa via army. Let's check a clear negative.
	// A mar cannot support a move to mos (too far)
	marUnit := *gs.UnitAt("mar")
	if CanSupportMove("mar", "par", "mos", marUnit, gs, m) {
		t.Error("expected A mar to NOT be able to support a move to mos")
	}
}

func TestProvinceConnectivity(t *testing.T) {
	m := diplomacy.StandardMap()

	// bur is a well-connected inland province
	c := ProvinceConnectivity("bur", m)
	if c < 4 {
		t.Errorf("expected bur connectivity >= 4, got %d", c)
	}

	// lon has fewer army connections (it's coastal, connected via land to wal, yor)
	cLon := ProvinceConnectivity("lon", m)
	if cLon == 0 {
		t.Error("expected lon to have some army connectivity")
	}
}
