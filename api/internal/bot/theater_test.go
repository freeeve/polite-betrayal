package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestProvinceTheaterCoversAllProvinces(t *testing.T) {
	m := diplomacy.StandardMap()
	for id := range m.Provinces {
		th := ProvinceTheater(id)
		if th == "" {
			t.Errorf("province %q has no theater assignment", id)
		}
	}
}

func TestProvinceTheaterKnownValues(t *testing.T) {
	cases := []struct {
		province string
		want     Theater
	}{
		{"par", TheaterWest},
		{"bre", TheaterWest},
		{"lon", TheaterWest},
		{"nwy", TheaterScan},
		{"swe", TheaterScan},
		{"stp", TheaterScan},
		{"rom", TheaterMed},
		{"tun", TheaterMed},
		{"ven", TheaterMed},
		{"gre", TheaterBalkans},
		{"con", TheaterBalkans},
		{"ank", TheaterBalkans},
		{"mos", TheaterEast},
		{"war", TheaterEast},
		{"sev", TheaterEast},
		{"mun", TheaterCenter},
		{"ber", TheaterCenter},
		{"vie", TheaterCenter},
	}
	for _, tc := range cases {
		got := ProvinceTheater(tc.province)
		if got != tc.want {
			t.Errorf("ProvinceTheater(%q) = %q, want %q", tc.province, got, tc.want)
		}
	}
}

func TestProvinceTheaterUnknown(t *testing.T) {
	if th := ProvinceTheater("zzz"); th != "" {
		t.Errorf("ProvinceTheater(unknown) = %q, want empty", th)
	}
}

func TestTheaterPresenceInitialState(t *testing.T) {
	gs := diplomacy.NewInitialState()

	// England starts with units in lon (west), edi (west), lvp (west)
	eng := TheaterPresence(gs, diplomacy.England)
	if eng[TheaterWest] != 3 {
		t.Errorf("England west = %d, want 3", eng[TheaterWest])
	}

	// Germany starts with kie (center), ber (center), mun (center)
	ger := TheaterPresence(gs, diplomacy.Germany)
	if ger[TheaterCenter] != 3 {
		t.Errorf("Germany center = %d, want 3", ger[TheaterCenter])
	}

	// Russia starts with stp (scan), sev (east), mos (east), war (east)
	rus := TheaterPresence(gs, diplomacy.Russia)
	if rus[TheaterScan] != 1 {
		t.Errorf("Russia scan = %d, want 1", rus[TheaterScan])
	}
	if rus[TheaterEast] != 3 {
		t.Errorf("Russia east = %d, want 3", rus[TheaterEast])
	}

	// Italy starts with nap (med), rom (med), ven (med)
	ita := TheaterPresence(gs, diplomacy.Italy)
	if ita[TheaterMed] != 3 {
		t.Errorf("Italy med = %d, want 3", ita[TheaterMed])
	}

	// Turkey starts with ank (balkans), con (balkans), smy (balkans)
	tur := TheaterPresence(gs, diplomacy.Turkey)
	if tur[TheaterBalkans] != 3 {
		t.Errorf("Turkey balkans = %d, want 3", tur[TheaterBalkans])
	}

	// Austria starts with tri (center), vie (center), bud (center)
	aus := TheaterPresence(gs, diplomacy.Austria)
	if aus[TheaterCenter] != 3 {
		t.Errorf("Austria center = %d, want 3", aus[TheaterCenter])
	}

	// France starts with bre (west), par (west), mar (west)
	fra := TheaterPresence(gs, diplomacy.France)
	if fra[TheaterWest] != 3 {
		t.Errorf("France west = %d, want 3", fra[TheaterWest])
	}
}

func TestTheaterPresenceEmptyForNeutral(t *testing.T) {
	gs := diplomacy.NewInitialState()
	counts := TheaterPresence(gs, diplomacy.Neutral)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != 0 {
		t.Errorf("Neutral should have 0 units, got %d", total)
	}
}

func TestTheaterPresenceAfterMovement(t *testing.T) {
	gs := diplomacy.NewInitialState()
	// Manually move England's fleet from lon to nth (scan theater)
	for i := range gs.Units {
		if gs.Units[i].Province == "lon" && gs.Units[i].Power == diplomacy.England {
			gs.Units[i].Province = "nth"
			break
		}
	}
	eng := TheaterPresence(gs, diplomacy.England)
	if eng[TheaterWest] != 2 {
		t.Errorf("England west after move = %d, want 2", eng[TheaterWest])
	}
	if eng[TheaterScan] != 1 {
		t.Errorf("England scan after move = %d, want 1", eng[TheaterScan])
	}
}
