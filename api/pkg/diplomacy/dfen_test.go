package diplomacy

import (
	"strings"
	"testing"
)

// expectedInitialDFEN is the canonical DFEN for Spring 1901 Movement.
// Units are sorted by power (A,E,F,G,I,R,T) then province alphabetically.
const expectedInitialDFEN = "1901sm/" +
	"Aabud,Aftri,Aavie," +
	"Efedi,Eflon,Ealvp," +
	"Ffbre,Famar,Fapar," +
	"Gaber,Gfkie,Gamun," +
	"Ifnap,Iarom,Iaven," +
	"Ramos,Rfsev,Rfstp.sc,Rawar," +
	"Tfank,Tacon,Tasmy/" +
	"Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar," +
	"Gber,Gkie,Gmun,Inap,Irom,Iven," +
	"Rmos,Rsev,Rstp,Rwar," +
	"Tank,Tcon,Tsmy," +
	"Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-"

func TestEncodeDFEN_InitialState(t *testing.T) {
	gs := NewInitialState()
	got := EncodeDFEN(gs)

	if got != expectedInitialDFEN {
		t.Errorf("EncodeDFEN(initial) mismatch\ngot:  %s\nwant: %s", got, expectedInitialDFEN)
	}
}

func TestDecodeDFEN_InitialState(t *testing.T) {
	gs, err := DecodeDFEN(expectedInitialDFEN)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	if gs.Year != 1901 {
		t.Errorf("year: got %d, want 1901", gs.Year)
	}
	if gs.Season != Spring {
		t.Errorf("season: got %q, want %q", gs.Season, Spring)
	}
	if gs.Phase != PhaseMovement {
		t.Errorf("phase: got %q, want %q", gs.Phase, PhaseMovement)
	}
	if len(gs.Units) != 22 {
		t.Errorf("units: got %d, want 22", len(gs.Units))
	}
	if len(gs.SupplyCenters) != 34 {
		t.Errorf("supply centers: got %d, want 34", len(gs.SupplyCenters))
	}
	if len(gs.Dislodged) != 0 {
		t.Errorf("dislodged: got %d, want 0", len(gs.Dislodged))
	}

	// Verify Russia's fleet at stp is on south coast
	for _, u := range gs.Units {
		if u.Province == "stp" {
			if u.Type != Fleet {
				t.Errorf("stp: expected Fleet, got %v", u.Type)
			}
			if u.Coast != SouthCoast {
				t.Errorf("stp coast: got %q, want %q", u.Coast, SouthCoast)
			}
			if u.Power != Russia {
				t.Errorf("stp power: got %q, want %q", u.Power, Russia)
			}
		}
	}
}

func TestDFEN_RoundTrip_InitialState(t *testing.T) {
	original := NewInitialState()
	encoded := EncodeDFEN(original)
	decoded, err := DecodeDFEN(encoded)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	// Re-encode should be identical (deterministic)
	reencoded := EncodeDFEN(decoded)
	if encoded != reencoded {
		t.Errorf("round-trip not deterministic\nfirst:  %s\nsecond: %s", encoded, reencoded)
	}

	assertGameStatesEqual(t, original, decoded)
}

func TestEncodeDFEN_RetreatPhase(t *testing.T) {
	gs := &GameState{
		Year:   1902,
		Season: Fall,
		Phase:  PhaseRetreat,
		Units: []Unit{
			{Army, Austria, "bud", NoCoast},
			{Army, Austria, "vie", NoCoast},
			{Fleet, Austria, "tri", NoCoast},
			{Army, Austria, "gre", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"bud": Austria, "gre": Austria, "tri": Austria, "vie": Austria,
			"edi": England, "lon": England, "lvp": England,
			"bre": France, "mar": France, "par": France,
			"ber": Germany, "kie": Germany, "mun": Germany,
			"nap": Italy, "rom": Italy, "ven": Italy,
			"mos": Russia, "sev": Russia, "stp": Russia, "war": Russia,
			"ank": Turkey, "con": Turkey, "smy": Turkey,
			"bel": Neutral, "bul": Neutral, "den": Neutral,
			"hol": Neutral, "nwy": Neutral, "por": Neutral,
			"rum": Neutral, "ser": Neutral, "spa": Neutral,
			"swe": Neutral, "tun": Neutral,
		},
		Dislodged: []DislodgedUnit{
			{
				Unit:          Unit{Army, Austria, "ser", NoCoast},
				DislodgedFrom: "ser",
				AttackerFrom:  "bul",
			},
			{
				Unit:          Unit{Fleet, Russia, "sev", NoCoast},
				DislodgedFrom: "sev",
				AttackerFrom:  "bla",
			},
		},
	}

	encoded := EncodeDFEN(gs)

	// Check phase info
	if !strings.HasPrefix(encoded, "1902fr/") {
		t.Errorf("expected 1902fr prefix, got: %s", encoded[:10])
	}

	// Check dislodged section is not "-"
	parts := strings.Split(encoded, "/")
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	dislodgedSection := parts[3]
	if dislodgedSection == "-" {
		t.Error("expected dislodged units, got -")
	}
	if !strings.Contains(dislodgedSection, "Aaser<bul") {
		t.Errorf("expected Austrian army dislodged from ser by bul, got: %s", dislodgedSection)
	}
	if !strings.Contains(dislodgedSection, "Rfsev<bla") {
		t.Errorf("expected Russian fleet dislodged from sev by bla, got: %s", dislodgedSection)
	}
}

func TestDFEN_RoundTrip_RetreatPhase(t *testing.T) {
	gs := &GameState{
		Year:   1902,
		Season: Fall,
		Phase:  PhaseRetreat,
		Units: []Unit{
			{Army, Austria, "bud", NoCoast},
			{Fleet, Turkey, "bla", NoCoast},
			{Army, Turkey, "bul", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"bud": Austria, "tri": Austria, "vie": Austria,
			"edi": England, "lon": England, "lvp": England,
			"bre": France, "mar": France, "par": France,
			"ber": Germany, "kie": Germany, "mun": Germany,
			"nap": Italy, "rom": Italy, "ven": Italy,
			"mos": Russia, "sev": Russia, "stp": Russia, "war": Russia,
			"ank": Turkey, "con": Turkey, "smy": Turkey,
			"bel": Neutral, "bul": Neutral, "den": Neutral,
			"gre": Neutral, "hol": Neutral, "nwy": Neutral,
			"por": Neutral, "rum": Neutral, "ser": Neutral,
			"spa": Neutral, "swe": Neutral, "tun": Neutral,
		},
		Dislodged: []DislodgedUnit{
			{
				Unit:          Unit{Fleet, Russia, "sev", NoCoast},
				DislodgedFrom: "sev",
				AttackerFrom:  "bla",
			},
		},
	}

	encoded := EncodeDFEN(gs)
	decoded, err := DecodeDFEN(encoded)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	reencoded := EncodeDFEN(decoded)
	if encoded != reencoded {
		t.Errorf("round-trip mismatch:\nfirst:  %s\nsecond: %s", encoded, reencoded)
	}

	assertGameStatesEqual(t, gs, decoded)
}

func TestEncodeDFEN_BuildPhase(t *testing.T) {
	gs := &GameState{
		Year:   1901,
		Season: Fall,
		Phase:  PhaseBuild,
		Units: []Unit{
			{Army, Austria, "tri", NoCoast},
			{Army, Austria, "rum", NoCoast},
			{Fleet, Austria, "gre", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"bud": Austria, "tri": Austria, "vie": Austria,
			"rum": Austria, "gre": Austria,
			"edi": England, "lon": England, "lvp": England,
			"bre": France, "mar": France, "par": France,
			"ber": Germany, "kie": Germany, "mun": Germany,
			"nap": Italy, "rom": Italy, "ven": Italy,
			"mos": Russia, "sev": Russia, "stp": Russia, "war": Russia,
			"ank": Turkey, "con": Turkey, "smy": Turkey,
			"bel": Neutral, "bul": Neutral, "den": Neutral,
			"hol": Neutral, "nwy": Neutral, "por": Neutral,
			"ser": Neutral, "spa": Neutral, "swe": Neutral,
			"tun": Neutral,
		},
	}

	encoded := EncodeDFEN(gs)

	if !strings.HasPrefix(encoded, "1901fb/") {
		t.Errorf("expected 1901fb prefix, got: %s", encoded)
	}

	// No dislodged in build phase
	parts := strings.Split(encoded, "/")
	if parts[3] != "-" {
		t.Errorf("expected no dislodged in build phase, got: %s", parts[3])
	}
}

func TestDFEN_RoundTrip_BuildPhase(t *testing.T) {
	gs := &GameState{
		Year:   1901,
		Season: Fall,
		Phase:  PhaseBuild,
		Units: []Unit{
			{Army, Austria, "tri", NoCoast},
			{Army, Austria, "rum", NoCoast},
			{Fleet, Austria, "gre", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"bud": Austria, "tri": Austria, "vie": Austria,
			"rum": Austria, "gre": Austria,
			"edi": England, "lon": England, "lvp": England,
			"bre": France, "mar": France, "par": France,
			"ber": Germany, "kie": Germany, "mun": Germany,
			"nap": Italy, "rom": Italy, "ven": Italy,
			"mos": Russia, "sev": Russia, "stp": Russia, "war": Russia,
			"ank": Turkey, "con": Turkey, "smy": Turkey,
			"bel": Neutral, "bul": Neutral, "den": Neutral,
			"hol": Neutral, "nwy": Neutral, "por": Neutral,
			"ser": Neutral, "spa": Neutral, "swe": Neutral,
			"tun": Neutral,
		},
	}

	encoded := EncodeDFEN(gs)
	decoded, err := DecodeDFEN(encoded)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	reencoded := EncodeDFEN(decoded)
	if encoded != reencoded {
		t.Errorf("round-trip mismatch:\nfirst:  %s\nsecond: %s", encoded, reencoded)
	}
}

func TestEncodeDFEN_EmptyUnits(t *testing.T) {
	gs := &GameState{
		Year:   1920,
		Season: Spring,
		Phase:  PhaseMovement,
		Units:  nil,
		SupplyCenters: map[string]Power{
			"vie": Austria,
		},
	}

	encoded := EncodeDFEN(gs)
	parts := strings.Split(encoded, "/")
	if parts[1] != "-" {
		t.Errorf("expected '-' for empty units, got: %s", parts[1])
	}
}

func TestEncodeDFEN_SplitCoastProvinces(t *testing.T) {
	gs := &GameState{
		Year:   1902,
		Season: Spring,
		Phase:  PhaseMovement,
		Units: []Unit{
			{Fleet, Russia, "stp", NorthCoast},
			{Fleet, Turkey, "bul", EastCoast},
			{Fleet, France, "spa", SouthCoast},
		},
		SupplyCenters: map[string]Power{
			"stp": Russia,
			"bul": Turkey,
			"spa": France,
		},
	}

	encoded := EncodeDFEN(gs)
	if !strings.Contains(encoded, "Rfstp.nc") {
		t.Errorf("expected stp.nc, got: %s", encoded)
	}
	if !strings.Contains(encoded, "Tfbul.ec") {
		t.Errorf("expected bul.ec, got: %s", encoded)
	}
	if !strings.Contains(encoded, "Ffspa.sc") {
		t.Errorf("expected spa.sc, got: %s", encoded)
	}
}

func TestDecodeDFEN_Errors(t *testing.T) {
	tests := []struct {
		name string
		dfen string
	}{
		{"too few sections", "1901sm/units/scs"},
		{"invalid year", "ABCsm/units/scs/-"},
		{"invalid season", "1901xm/units/scs/-"},
		{"invalid phase", "1901sx/units/scs/-"},
		{"invalid power in unit", "Xavie/scs/-"},
		{"invalid unit type", "Axvie/scs/-"},
		{"short phase info", "sm/-/-/-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDFEN(tt.dfen)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestDFEN_Deterministic(t *testing.T) {
	gs := NewInitialState()

	// Encode multiple times to verify determinism
	first := EncodeDFEN(gs)
	for i := range 10 {
		got := EncodeDFEN(gs)
		if got != first {
			t.Errorf("non-deterministic output on iteration %d\nfirst: %s\ngot:   %s", i, first, got)
		}
	}
}

func TestDFEN_AllSevenPowersHaveUnits(t *testing.T) {
	gs := NewInitialState()
	encoded := EncodeDFEN(gs)

	unitsPart := strings.Split(encoded, "/")[1]
	for _, p := range powerOrder {
		prefix := string(powerToChar[p])
		if !strings.Contains(unitsPart, prefix) {
			t.Errorf("missing units for power %s in units section", p)
		}
	}
}

func TestDFEN_NeutralSCs(t *testing.T) {
	gs := NewInitialState()
	encoded := EncodeDFEN(gs)

	scPart := strings.Split(encoded, "/")[2]
	neutralSCs := []string{"bel", "bul", "den", "gre", "hol", "nwy", "por", "rum", "ser", "spa", "swe", "tun"}
	for _, sc := range neutralSCs {
		entry := "N" + sc
		if !strings.Contains(scPart, entry) {
			t.Errorf("missing neutral SC %s in: %s", entry, scPart)
		}
	}
}

func TestDecodeDFEN_SpecExample_Retreat(t *testing.T) {
	// Example from Section 7.3 of the protocol spec
	dfen := "1902fr/" +
		"Aabud,Aavie,Aftri,Aagre," +
		"Efnth,Efnwy,Eabel,Eflon," +
		"Ffmao,Fabur,Fapar,Ffbre," +
		"Gaden,Gamun,Gfkie,Gaber," +
		"Ifnap,Iaven,Iarom," +
		"Ramos,Rawar,Ragal,Rfstp.sc," +
		"Tabul,Tfbla,Tacon,Tasmy,Tfank/" +
		"Abud,Agre,Atri,Avie," +
		"Ebel,Eedi,Elon,Elvp," +
		"Fbre,Fmar,Fpar," +
		"Gber,Gden,Gkie,Gmun," +
		"Inap,Irom,Iven," +
		"Rmos,Rsev,Rstp,Rwar," +
		"Tank,Tbul,Tcon,Tsmy," +
		"Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/" +
		"Aaser<bul,Rfsev<bla"

	gs, err := DecodeDFEN(dfen)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	if gs.Year != 1902 {
		t.Errorf("year: got %d, want 1902", gs.Year)
	}
	if gs.Season != Fall {
		t.Errorf("season: got %q, want fall", gs.Season)
	}
	if gs.Phase != PhaseRetreat {
		t.Errorf("phase: got %q, want retreat", gs.Phase)
	}
	if len(gs.Units) != 28 {
		t.Errorf("units: got %d, want 28", len(gs.Units))
	}
	if len(gs.Dislodged) != 2 {
		t.Errorf("dislodged: got %d, want 2", len(gs.Dislodged))
	}
	if len(gs.SupplyCenters) != 34 {
		t.Errorf("SCs: got %d, want 34", len(gs.SupplyCenters))
	}

	// Verify dislodged details
	for _, d := range gs.Dislodged {
		switch d.Unit.Province {
		case "ser":
			if d.Unit.Power != Austria || d.Unit.Type != Army || d.AttackerFrom != "bul" {
				t.Errorf("wrong dislodged ser entry: %+v", d)
			}
		case "sev":
			if d.Unit.Power != Russia || d.Unit.Type != Fleet || d.AttackerFrom != "bla" {
				t.Errorf("wrong dislodged sev entry: %+v", d)
			}
		default:
			t.Errorf("unexpected dislodged province: %s", d.Unit.Province)
		}
	}
}

func TestDFEN_RoundTrip_MidGame(t *testing.T) {
	// Mid-game position from spec section 7.2
	gs := &GameState{
		Year:   1903,
		Season: Fall,
		Phase:  PhaseMovement,
		Units: []Unit{
			{Army, Austria, "bud", NoCoast},
			{Army, Austria, "rum", NoCoast},
			{Fleet, Austria, "gre", NoCoast},
			{Army, Austria, "vie", NoCoast},
			{Fleet, England, "nth", NoCoast},
			{Fleet, England, "nwy", NoCoast},
			{Army, England, "yor", NoCoast},
			{Fleet, England, "lon", NoCoast},
			{Fleet, France, "mao", NoCoast},
			{Army, France, "bur", NoCoast},
			{Army, France, "mar", NoCoast},
			{Fleet, France, "por", NoCoast},
			{Army, Germany, "den", NoCoast},
			{Army, Germany, "hol", NoCoast},
			{Army, Germany, "mun", NoCoast},
			{Fleet, Germany, "kie", NoCoast},
			{Fleet, Germany, "ska", NoCoast},
			{Fleet, Italy, "tys", NoCoast},
			{Army, Italy, "ven", NoCoast},
			{Army, Italy, "rom", NoCoast},
			{Fleet, Russia, "sev", NoCoast},
			{Army, Russia, "mos", NoCoast},
			{Army, Russia, "war", NoCoast},
			{Fleet, Turkey, "ank", NoCoast},
			{Army, Turkey, "bul", NoCoast},
			{Army, Turkey, "con", NoCoast},
			{Army, Turkey, "smy", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"bud": Austria, "gre": Austria, "rum": Austria, "tri": Austria, "vie": Austria,
			"edi": England, "lon": England, "lvp": England, "nwy": England,
			"bre": France, "mar": France, "par": France, "spa": France,
			"ber": Germany, "den": Germany, "hol": Germany, "kie": Germany, "mun": Germany,
			"nap": Italy, "rom": Italy, "ven": Italy,
			"mos": Russia, "sev": Russia, "war": Russia,
			"ank": Turkey, "bul": Turkey, "con": Turkey, "smy": Turkey,
			"bel": Neutral, "por": Neutral, "ser": Neutral, "stp": Neutral, "swe": Neutral, "tun": Neutral,
		},
	}

	encoded := EncodeDFEN(gs)
	decoded, err := DecodeDFEN(encoded)
	if err != nil {
		t.Fatalf("DecodeDFEN failed: %v", err)
	}

	assertGameStatesEqual(t, gs, decoded)
}

// assertGameStatesEqual compares two game states structurally.
func assertGameStatesEqual(t *testing.T, want, got *GameState) {
	t.Helper()

	if want.Year != got.Year {
		t.Errorf("year: want %d, got %d", want.Year, got.Year)
	}
	if want.Season != got.Season {
		t.Errorf("season: want %q, got %q", want.Season, got.Season)
	}
	if want.Phase != got.Phase {
		t.Errorf("phase: want %q, got %q", want.Phase, got.Phase)
	}
	if len(want.Units) != len(got.Units) {
		t.Errorf("unit count: want %d, got %d", len(want.Units), len(got.Units))
	}
	if len(want.SupplyCenters) != len(got.SupplyCenters) {
		t.Errorf("SC count: want %d, got %d", len(want.SupplyCenters), len(got.SupplyCenters))
	}
	if len(want.Dislodged) != len(got.Dislodged) {
		t.Errorf("dislodged count: want %d, got %d", len(want.Dislodged), len(got.Dislodged))
	}

	// Check all SCs match
	for prov, wantPower := range want.SupplyCenters {
		gotPower, ok := got.SupplyCenters[prov]
		if !ok {
			t.Errorf("missing SC %s in decoded", prov)
			continue
		}
		if wantPower != gotPower {
			t.Errorf("SC %s: want %q, got %q", prov, wantPower, gotPower)
		}
	}

	// Build unit lookup for got
	gotUnits := make(map[string]Unit)
	for _, u := range got.Units {
		gotUnits[u.Province] = u
	}

	for _, wu := range want.Units {
		gu, ok := gotUnits[wu.Province]
		if !ok {
			t.Errorf("missing unit at %s in decoded", wu.Province)
			continue
		}
		if wu.Type != gu.Type || wu.Power != gu.Power || wu.Coast != gu.Coast {
			t.Errorf("unit at %s: want %+v, got %+v", wu.Province, wu, gu)
		}
	}

	// Check dislodged
	gotDislodged := make(map[string]DislodgedUnit)
	for _, d := range got.Dislodged {
		gotDislodged[d.Unit.Province] = d
	}

	for _, wd := range want.Dislodged {
		gd, ok := gotDislodged[wd.Unit.Province]
		if !ok {
			t.Errorf("missing dislodged at %s in decoded", wd.Unit.Province)
			continue
		}
		if wd.Unit.Type != gd.Unit.Type || wd.Unit.Power != gd.Unit.Power || wd.AttackerFrom != gd.AttackerFrom {
			t.Errorf("dislodged at %s: want %+v, got %+v", wd.Unit.Province, wd, gd)
		}
	}
}

func FuzzDFEN_RoundTrip(f *testing.F) {
	// Seed with the initial state DFEN
	f.Add(expectedInitialDFEN)

	// Seed with a retreat phase DFEN
	f.Add("1902fr/Aabud,Tfbla/" +
		"Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar," +
		"Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar," +
		"Tank,Tcon,Tsmy," +
		"Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/" +
		"Rfsev<bla")

	// Seed with build phase
	f.Add("1901fb/Aatri,Aarum,Afgre/" +
		"Abud,Atri,Avie,Arum,Agre,Eedi,Elon,Elvp,Fbre,Fmar,Fpar," +
		"Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar," +
		"Tank,Tcon,Tsmy," +
		"Nbel,Nbul,Nden,Nhol,Nnwy,Npor,Nser,Nspa,Nswe,Ntun/-")

	f.Fuzz(func(t *testing.T, dfen string) {
		gs, err := DecodeDFEN(dfen)
		if err != nil {
			// Invalid input is fine; just ensure no panic
			return
		}

		// Encode and decode again
		encoded := EncodeDFEN(gs)
		gs2, err := DecodeDFEN(encoded)
		if err != nil {
			t.Fatalf("second decode failed: %v (encoded=%q)", err, encoded)
		}

		// Third encode must match second
		encoded2 := EncodeDFEN(gs2)
		if encoded != encoded2 {
			t.Fatalf("round-trip not stable:\nfirst:  %s\nsecond: %s", encoded, encoded2)
		}
	})
}
