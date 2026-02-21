package diplomacy

import (
	"sort"
	"sync"
)

var (
	stdMapOnce sync.Once
	stdMapInst *DiplomacyMap
)

// StandardMap returns the standard 75-province Diplomacy map with all
// provinces and adjacencies. The map is built once and cached; subsequent
// calls return the same pointer. Callers must not mutate the returned map.
func StandardMap() *DiplomacyMap {
	stdMapOnce.Do(func() {
		stdMapInst = buildStandardMap()
	})
	return stdMapInst
}

func buildStandardMap() *DiplomacyMap {
	m := &DiplomacyMap{
		Provinces:   make(map[string]*Province, 75),
		Adjacencies: make(map[string][]Adjacency, 150),
	}

	prov := func(id, name string, pt ProvinceType, sc bool, hp Power, coasts ...Coast) {
		m.Provinces[id] = &Province{
			ID:             id,
			Name:           name,
			Type:           pt,
			IsSupplyCenter: sc,
			HomePower:      hp,
			Coasts:         coasts,
		}
	}

	// addAdj adds a single directed adjacency entry.
	addAdj := func(from string, fromCoast Coast, to string, toCoast Coast, armyOK, fleetOK bool) {
		m.Adjacencies[from] = append(m.Adjacencies[from], Adjacency{
			From:      from,
			FromCoast: fromCoast,
			To:        to,
			ToCoast:   toCoast,
			ArmyOK:    armyOK,
			FleetOK:   fleetOK,
		})
	}

	// addArmyAdj adds a bidirectional army-only adjacency between two provinces.
	addArmyAdj := func(from, to string) {
		addAdj(from, NoCoast, to, NoCoast, true, false)
		addAdj(to, NoCoast, from, NoCoast, true, false)
	}

	// addFleetAdj adds a bidirectional fleet-only adjacency with optional coast specifiers.
	addFleetAdj := func(from string, fromCoast Coast, to string, toCoast Coast) {
		addAdj(from, fromCoast, to, toCoast, false, true)
		addAdj(to, toCoast, from, fromCoast, false, true)
	}

	// addBothAdj adds a bidirectional adjacency for both armies and fleets (no coast).
	addBothAdj := func(from, to string) {
		addAdj(from, NoCoast, to, NoCoast, true, true)
		addAdj(to, NoCoast, from, NoCoast, true, true)
	}

	// =========================================================================
	// Provinces: 14 inland + 39 coastal + 3 split-coast + 19 sea = 75
	// =========================================================================

	// --- Inland provinces (14) ---
	prov("boh", "Bohemia", Land, false, Neutral)
	prov("bud", "Budapest", Land, true, Austria)
	prov("bur", "Burgundy", Land, false, Neutral)
	prov("gal", "Galicia", Land, false, Neutral)
	prov("mos", "Moscow", Land, true, Russia)
	prov("mun", "Munich", Land, true, Germany)
	prov("par", "Paris", Land, true, France)
	prov("ruh", "Ruhr", Land, false, Neutral)
	prov("ser", "Serbia", Land, true, Neutral)
	prov("sil", "Silesia", Land, false, Neutral)
	prov("tyr", "Tyrolia", Land, false, Neutral)
	prov("ukr", "Ukraine", Land, false, Neutral)
	prov("vie", "Vienna", Land, true, Austria)
	prov("war", "Warsaw", Land, true, Russia)

	// --- Coastal provinces without split coasts (39) ---
	prov("alb", "Albania", Coastal, false, Neutral)
	prov("ank", "Ankara", Coastal, true, Turkey)
	prov("apu", "Apulia", Coastal, false, Neutral)
	prov("arm", "Armenia", Coastal, false, Neutral)
	prov("bel", "Belgium", Coastal, true, Neutral)
	prov("ber", "Berlin", Coastal, true, Germany)
	prov("bre", "Brest", Coastal, true, France)
	prov("cly", "Clyde", Coastal, false, Neutral)
	prov("con", "Constantinople", Coastal, true, Turkey)
	prov("den", "Denmark", Coastal, true, Neutral)
	prov("edi", "Edinburgh", Coastal, true, England)
	prov("fin", "Finland", Coastal, false, Neutral)
	prov("gas", "Gascony", Coastal, false, Neutral)
	prov("gre", "Greece", Coastal, true, Neutral)
	prov("hol", "Holland", Coastal, true, Neutral)
	prov("kie", "Kiel", Coastal, true, Germany)
	prov("lon", "London", Coastal, true, England)
	prov("lvn", "Livonia", Coastal, false, Neutral)
	prov("lvp", "Liverpool", Coastal, true, England)
	prov("mar", "Marseilles", Coastal, true, France)
	prov("naf", "North Africa", Coastal, false, Neutral)
	prov("nap", "Naples", Coastal, true, Italy)
	prov("nwy", "Norway", Coastal, true, Neutral)
	prov("pic", "Picardy", Coastal, false, Neutral)
	prov("pie", "Piedmont", Coastal, false, Neutral)
	prov("por", "Portugal", Coastal, true, Neutral)
	prov("pru", "Prussia", Coastal, false, Neutral)
	prov("rom", "Rome", Coastal, true, Italy)
	prov("rum", "Rumania", Coastal, true, Neutral)
	prov("sev", "Sevastopol", Coastal, true, Russia)
	prov("smy", "Smyrna", Coastal, true, Turkey)
	prov("swe", "Sweden", Coastal, true, Neutral)
	prov("syr", "Syria", Coastal, false, Neutral)
	prov("tri", "Trieste", Coastal, true, Austria)
	prov("tun", "Tunisia", Coastal, true, Neutral)
	prov("tus", "Tuscany", Coastal, false, Neutral)
	prov("ven", "Venice", Coastal, true, Italy)
	prov("wal", "Wales", Coastal, false, Neutral)
	prov("yor", "Yorkshire", Coastal, false, Neutral)

	// --- Split-coast provinces (3) ---
	prov("bul", "Bulgaria", Coastal, true, Neutral, EastCoast, SouthCoast)
	prov("spa", "Spain", Coastal, true, Neutral, NorthCoast, SouthCoast)
	prov("stp", "St. Petersburg", Coastal, true, Russia, NorthCoast, SouthCoast)

	// --- Sea provinces (19) ---
	prov("adr", "Adriatic Sea", Sea, false, Neutral)
	prov("aeg", "Aegean Sea", Sea, false, Neutral)
	prov("bal", "Baltic Sea", Sea, false, Neutral)
	prov("bar", "Barents Sea", Sea, false, Neutral)
	prov("bla", "Black Sea", Sea, false, Neutral)
	prov("bot", "Gulf of Bothnia", Sea, false, Neutral)
	prov("eas", "Eastern Mediterranean", Sea, false, Neutral)
	prov("eng", "English Channel", Sea, false, Neutral)
	prov("gol", "Gulf of Lyon", Sea, false, Neutral)
	prov("hel", "Heligoland Bight", Sea, false, Neutral)
	prov("ion", "Ionian Sea", Sea, false, Neutral)
	prov("iri", "Irish Sea", Sea, false, Neutral)
	prov("mao", "Mid-Atlantic Ocean", Sea, false, Neutral)
	prov("nao", "North Atlantic Ocean", Sea, false, Neutral)
	prov("nrg", "Norwegian Sea", Sea, false, Neutral)
	prov("nth", "North Sea", Sea, false, Neutral)
	prov("ska", "Skagerrak", Sea, false, Neutral)
	prov("tys", "Tyrrhenian Sea", Sea, false, Neutral)
	prov("wes", "Western Mediterranean", Sea, false, Neutral)

	// =========================================================================
	// Adjacencies
	// =========================================================================
	// Each pair appears exactly once. For split-coast provinces, fleet adjacencies
	// specify the relevant coast; army adjacencies use addArmyAdj (no coast needed).
	//
	// Categories:
	//   addFleetAdj  - sea<->sea, sea<->coastal, or coastal<->coastal with ONLY sea border
	//   addArmyAdj   - involves at least one inland province, or coastal<->coastal ONLY land
	//   addBothAdj   - coastal<->coastal sharing both a land border and a sea border
	//
	// For split-coast provinces (spa, stp, bul):
	//   Army connections use addArmyAdj (armies don't care about coasts).
	//   Fleet connections use addFleetAdj with the specific coast.

	// ---- Sea-to-sea (fleet only) ----
	addFleetAdj("adr", NoCoast, "ion", NoCoast)
	addFleetAdj("aeg", NoCoast, "eas", NoCoast)
	addFleetAdj("aeg", NoCoast, "ion", NoCoast)
	addFleetAdj("bal", NoCoast, "bot", NoCoast)
	addFleetAdj("eng", NoCoast, "iri", NoCoast)
	addFleetAdj("eng", NoCoast, "mao", NoCoast)
	addFleetAdj("eng", NoCoast, "nth", NoCoast)
	addFleetAdj("gol", NoCoast, "tys", NoCoast)
	addFleetAdj("gol", NoCoast, "wes", NoCoast)
	addFleetAdj("hel", NoCoast, "nth", NoCoast)
	addFleetAdj("ion", NoCoast, "eas", NoCoast)
	addFleetAdj("ion", NoCoast, "tys", NoCoast)
	addFleetAdj("iri", NoCoast, "mao", NoCoast)
	addFleetAdj("iri", NoCoast, "nao", NoCoast)
	addFleetAdj("mao", NoCoast, "nao", NoCoast)
	addFleetAdj("mao", NoCoast, "wes", NoCoast)
	addFleetAdj("nao", NoCoast, "nrg", NoCoast)
	addFleetAdj("nth", NoCoast, "nrg", NoCoast)
	addFleetAdj("nth", NoCoast, "ska", NoCoast)
	addFleetAdj("nrg", NoCoast, "bar", NoCoast)
	addFleetAdj("tys", NoCoast, "wes", NoCoast)

	// ---- Sea-to-coastal (fleet only) ----

	// Adriatic Sea
	addFleetAdj("adr", NoCoast, "alb", NoCoast)
	addFleetAdj("adr", NoCoast, "apu", NoCoast)
	addFleetAdj("adr", NoCoast, "tri", NoCoast)
	addFleetAdj("adr", NoCoast, "ven", NoCoast)

	// Aegean Sea
	addFleetAdj("aeg", NoCoast, "bul", SouthCoast)
	addFleetAdj("aeg", NoCoast, "con", NoCoast)
	addFleetAdj("aeg", NoCoast, "gre", NoCoast)
	addFleetAdj("aeg", NoCoast, "smy", NoCoast)

	// Baltic Sea
	addFleetAdj("bal", NoCoast, "ber", NoCoast)
	addFleetAdj("bal", NoCoast, "den", NoCoast)
	addFleetAdj("bal", NoCoast, "kie", NoCoast)
	addFleetAdj("bal", NoCoast, "lvn", NoCoast)
	addFleetAdj("bal", NoCoast, "pru", NoCoast)
	addFleetAdj("bal", NoCoast, "swe", NoCoast)

	// Barents Sea
	addFleetAdj("bar", NoCoast, "nwy", NoCoast)
	addFleetAdj("bar", NoCoast, "stp", NorthCoast)

	// Black Sea
	addFleetAdj("bla", NoCoast, "ank", NoCoast)
	addFleetAdj("bla", NoCoast, "arm", NoCoast)
	addFleetAdj("bla", NoCoast, "bul", EastCoast)
	addFleetAdj("bla", NoCoast, "con", NoCoast)
	addFleetAdj("bla", NoCoast, "rum", NoCoast)
	addFleetAdj("bla", NoCoast, "sev", NoCoast)

	// Gulf of Bothnia
	addFleetAdj("bot", NoCoast, "fin", NoCoast)
	addFleetAdj("bot", NoCoast, "lvn", NoCoast)
	addFleetAdj("bot", NoCoast, "stp", SouthCoast)
	addFleetAdj("bot", NoCoast, "swe", NoCoast)

	// Eastern Mediterranean
	addFleetAdj("eas", NoCoast, "smy", NoCoast)
	addFleetAdj("eas", NoCoast, "syr", NoCoast)

	// English Channel
	addFleetAdj("eng", NoCoast, "bel", NoCoast)
	addFleetAdj("eng", NoCoast, "bre", NoCoast)
	addFleetAdj("eng", NoCoast, "lon", NoCoast)
	addFleetAdj("eng", NoCoast, "pic", NoCoast)
	addFleetAdj("eng", NoCoast, "wal", NoCoast)

	// Gulf of Lyon
	addFleetAdj("gol", NoCoast, "mar", NoCoast)
	addFleetAdj("gol", NoCoast, "pie", NoCoast)
	addFleetAdj("gol", NoCoast, "spa", SouthCoast)
	addFleetAdj("gol", NoCoast, "tus", NoCoast)

	// Heligoland Bight
	addFleetAdj("hel", NoCoast, "den", NoCoast)
	addFleetAdj("hel", NoCoast, "hol", NoCoast)
	addFleetAdj("hel", NoCoast, "kie", NoCoast)

	// Ionian Sea
	addFleetAdj("ion", NoCoast, "alb", NoCoast)
	addFleetAdj("ion", NoCoast, "apu", NoCoast)
	addFleetAdj("ion", NoCoast, "gre", NoCoast)
	addFleetAdj("ion", NoCoast, "nap", NoCoast)
	addFleetAdj("ion", NoCoast, "tun", NoCoast)

	// Irish Sea
	addFleetAdj("iri", NoCoast, "lvp", NoCoast)
	addFleetAdj("iri", NoCoast, "wal", NoCoast)

	// Mid-Atlantic Ocean
	addFleetAdj("mao", NoCoast, "bre", NoCoast)
	addFleetAdj("mao", NoCoast, "gas", NoCoast)
	addFleetAdj("mao", NoCoast, "naf", NoCoast)
	addFleetAdj("mao", NoCoast, "por", NoCoast)
	addFleetAdj("mao", NoCoast, "spa", NorthCoast)
	addFleetAdj("mao", NoCoast, "spa", SouthCoast)

	// North Atlantic Ocean
	addFleetAdj("nao", NoCoast, "cly", NoCoast)
	addFleetAdj("nao", NoCoast, "lvp", NoCoast)

	// North Sea
	addFleetAdj("nth", NoCoast, "bel", NoCoast)
	addFleetAdj("nth", NoCoast, "den", NoCoast)
	addFleetAdj("nth", NoCoast, "edi", NoCoast)
	addFleetAdj("nth", NoCoast, "hol", NoCoast)
	addFleetAdj("nth", NoCoast, "lon", NoCoast)
	addFleetAdj("nth", NoCoast, "nwy", NoCoast)
	addFleetAdj("nth", NoCoast, "yor", NoCoast)

	// Norwegian Sea
	addFleetAdj("nrg", NoCoast, "cly", NoCoast)
	addFleetAdj("nrg", NoCoast, "edi", NoCoast)
	addFleetAdj("nrg", NoCoast, "nwy", NoCoast)

	// Skagerrak
	addFleetAdj("ska", NoCoast, "den", NoCoast)
	addFleetAdj("ska", NoCoast, "nwy", NoCoast)
	addFleetAdj("ska", NoCoast, "swe", NoCoast)

	// Tyrrhenian Sea
	addFleetAdj("tys", NoCoast, "nap", NoCoast)
	addFleetAdj("tys", NoCoast, "rom", NoCoast)
	addFleetAdj("tys", NoCoast, "tun", NoCoast)
	addFleetAdj("tys", NoCoast, "tus", NoCoast)

	// Western Mediterranean
	addFleetAdj("wes", NoCoast, "naf", NoCoast)
	addFleetAdj("wes", NoCoast, "spa", SouthCoast)
	addFleetAdj("wes", NoCoast, "tun", NoCoast)

	// ---- Inland-to-inland adjacencies (army only) ----
	addArmyAdj("boh", "gal")
	addArmyAdj("boh", "mun")
	addArmyAdj("boh", "sil")
	addArmyAdj("boh", "tyr")
	addArmyAdj("boh", "vie")
	addArmyAdj("bud", "gal")
	addArmyAdj("bud", "vie")
	addArmyAdj("bur", "mun")
	addArmyAdj("bur", "par")
	addArmyAdj("bur", "ruh")
	addArmyAdj("gal", "sil")
	addArmyAdj("gal", "ukr")
	addArmyAdj("gal", "vie")
	addArmyAdj("gal", "war")
	addArmyAdj("mos", "ukr")
	addArmyAdj("mos", "war")
	addArmyAdj("mun", "ruh")
	addArmyAdj("mun", "sil")
	addArmyAdj("mun", "tyr")
	addArmyAdj("sil", "war")
	addArmyAdj("tyr", "vie")
	addArmyAdj("ukr", "war")

	// ---- Inland-to-coastal adjacencies (army only) ----
	addArmyAdj("bud", "rum")
	addArmyAdj("bud", "ser")
	addArmyAdj("bud", "tri")
	addArmyAdj("bur", "bel")
	addArmyAdj("bur", "gas")
	addArmyAdj("bur", "mar")
	addArmyAdj("bur", "pic")
	addArmyAdj("gal", "rum")
	addArmyAdj("gas", "mar")
	addArmyAdj("mos", "lvn")
	addArmyAdj("mos", "sev")
	addArmyAdj("mos", "stp")
	addArmyAdj("mun", "ber")
	addArmyAdj("mun", "kie")
	addArmyAdj("par", "bre")
	addArmyAdj("par", "gas")
	addArmyAdj("par", "pic")
	addArmyAdj("ruh", "bel")
	addArmyAdj("ruh", "hol")
	addArmyAdj("ruh", "kie")
	addArmyAdj("ser", "alb")
	addArmyAdj("ser", "bul")
	addArmyAdj("ser", "gre")
	addArmyAdj("ser", "rum")
	addArmyAdj("ser", "tri")
	addArmyAdj("sil", "ber")
	addArmyAdj("sil", "pru")
	addArmyAdj("tyr", "pie")
	addArmyAdj("tyr", "tri")
	addArmyAdj("tyr", "ven")
	addArmyAdj("ukr", "rum")
	addArmyAdj("ukr", "sev")
	addArmyAdj("vie", "tri")
	addArmyAdj("war", "lvn")
	addArmyAdj("war", "pru")

	// ---- Coastal-to-coastal adjacencies: both army and fleet ----
	// These pairs share BOTH a land border and a sea/coast border.
	addBothAdj("alb", "gre")
	addBothAdj("alb", "tri")
	addBothAdj("ank", "arm")
	addBothAdj("ank", "con")
	addBothAdj("apu", "nap")
	addBothAdj("apu", "ven")
	addBothAdj("bel", "hol")
	addBothAdj("bel", "pic")
	addBothAdj("ber", "kie")
	addBothAdj("ber", "pru")
	addBothAdj("bre", "gas")
	addBothAdj("bre", "pic")
	addBothAdj("cly", "edi")
	addBothAdj("cly", "lvp")
	addBothAdj("con", "smy")
	addBothAdj("den", "kie")
	addBothAdj("den", "swe")
	addBothAdj("edi", "yor")
	addBothAdj("fin", "swe")
	addBothAdj("hol", "kie")
	addBothAdj("lon", "wal")
	addBothAdj("lon", "yor")
	addBothAdj("lvp", "wal")
	addBothAdj("mar", "pie")
	addBothAdj("naf", "tun")
	addBothAdj("nwy", "swe")
	addBothAdj("pie", "tus")
	addBothAdj("pru", "lvn")
	addBothAdj("rom", "nap")
	addBothAdj("rom", "tus")
	addBothAdj("sev", "arm")
	addBothAdj("sev", "rum")
	addBothAdj("smy", "syr")
	addBothAdj("tri", "ven")

	// Coastal-to-coastal army-only: provinces share land border but face different seas.
	addArmyAdj("ank", "smy")
	addArmyAdj("apu", "rom")
	addArmyAdj("arm", "smy")
	addArmyAdj("arm", "syr")
	addArmyAdj("edi", "lvp")
	addArmyAdj("fin", "nwy")
	addArmyAdj("lvp", "yor")
	addArmyAdj("pie", "ven")
	addArmyAdj("rom", "ven")
	addArmyAdj("tus", "ven")
	addArmyAdj("wal", "yor")

	// ---- Coastal-to-coastal: fleet only (sea border but no shared land border) ----
	addFleetAdj("con", NoCoast, "bul", EastCoast)
	addFleetAdj("con", NoCoast, "bul", SouthCoast)
	addFleetAdj("gre", NoCoast, "bul", SouthCoast)
	addFleetAdj("rum", NoCoast, "bul", EastCoast)
	addFleetAdj("gas", NoCoast, "spa", NorthCoast)
	addFleetAdj("mar", NoCoast, "spa", SouthCoast)
	addFleetAdj("por", NoCoast, "spa", NorthCoast)
	addFleetAdj("por", NoCoast, "spa", SouthCoast)
	addFleetAdj("fin", NoCoast, "stp", SouthCoast)
	addFleetAdj("lvn", NoCoast, "stp", SouthCoast)
	addFleetAdj("nwy", NoCoast, "stp", NorthCoast)

	// ---- Coastal-to-coastal/split-coast: army only (land border, no shared sea) ----
	// These are pairs of coastal (or split-coast) provinces that share a land border
	// but have no fleet passage between them.
	addArmyAdj("con", "bul")
	addArmyAdj("gre", "bul")
	addArmyAdj("rum", "bul")
	addArmyAdj("gas", "spa")
	addArmyAdj("mar", "spa")
	addArmyAdj("por", "spa")
	addArmyAdj("fin", "stp")
	addArmyAdj("lvn", "stp")
	addArmyAdj("nwy", "stp")

	// Build dense province index (sorted for deterministic ordering).
	keys := make([]string, 0, len(m.Provinces))
	for id := range m.Provinces {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	m.provIndex = make(map[string]int, len(keys))
	for i, id := range keys {
		m.provIndex[id] = i
		m.provNames[i] = id
	}

	m.precomputeAdjCache()

	return m
}
