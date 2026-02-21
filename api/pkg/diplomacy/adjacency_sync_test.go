package diplomacy

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// adjType encodes whether an adjacency allows army, fleet, or both.
type adjType int

const (
	adjArmy  adjType = 1
	adjFleet adjType = 2
	adjBoth  adjType = 3
)

// canonicalAdj describes a single directional adjacency in the standard Diplomacy map.
// For split-coast provinces, the coast field on the relevant side is non-empty.
type canonicalAdj struct {
	to      string  // target province (lowercase)
	toCoast string  // "" or "nc"/"sc"/"ec"
	aType   adjType // army, fleet, or both
}

// buildCanonicalAdjacencies returns the complete canonical adjacency map for
// standard Diplomacy, transcribed from the DPjudge/diplomacy reference.
//
// Each key is a province ID (lowercase, e.g. "bul"); the value is a slice of
// canonicalAdj entries. For split-coast provinces, fleet entries reference
// the coast variant (e.g. bul with toCoast "ec"), while army entries use "".
func buildCanonicalAdjacencies() map[string][]canonicalAdj {
	adj := make(map[string][]canonicalAdj, 80)

	add := func(from, fromCoast, to, toCoast string, at adjType) {
		_ = fromCoast // fromCoast is for documentation; we key by province only
		adj[from] = append(adj[from], canonicalAdj{to: to, toCoast: toCoast, aType: at})
	}

	f := func(from, fromCoast, to, toCoast string) { add(from, fromCoast, to, toCoast, adjFleet) }
	a := func(from, to string) { add(from, "", to, "", adjArmy) }
	b := func(from, to string) { add(from, "", to, "", adjBoth) }

	// =====================================================================
	// Sea zones (fleet adjacencies only)
	// =====================================================================

	// ADR
	f("adr", "", "alb", "")
	f("adr", "", "apu", "")
	f("adr", "", "ion", "")
	f("adr", "", "tri", "")
	f("adr", "", "ven", "")

	// AEG
	f("aeg", "", "bul", "sc")
	f("aeg", "", "con", "")
	f("aeg", "", "eas", "")
	f("aeg", "", "gre", "")
	f("aeg", "", "ion", "")
	f("aeg", "", "smy", "")

	// BAL
	f("bal", "", "ber", "")
	f("bal", "", "bot", "")
	f("bal", "", "den", "")
	f("bal", "", "lvn", "")
	f("bal", "", "kie", "")
	f("bal", "", "pru", "")
	f("bal", "", "swe", "")

	// BAR
	f("bar", "", "nrg", "")
	f("bar", "", "nwy", "")
	f("bar", "", "stp", "nc")

	// BLA
	f("bla", "", "ank", "")
	f("bla", "", "arm", "")
	f("bla", "", "bul", "ec")
	f("bla", "", "con", "")
	f("bla", "", "rum", "")
	f("bla", "", "sev", "")

	// BOT
	f("bot", "", "bal", "")
	f("bot", "", "fin", "")
	f("bot", "", "lvn", "")
	f("bot", "", "stp", "sc")
	f("bot", "", "swe", "")

	// EAS
	f("eas", "", "aeg", "")
	f("eas", "", "ion", "")
	f("eas", "", "smy", "")
	f("eas", "", "syr", "")

	// ENG
	f("eng", "", "bel", "")
	f("eng", "", "bre", "")
	f("eng", "", "iri", "")
	f("eng", "", "lon", "")
	f("eng", "", "mao", "")
	f("eng", "", "nth", "")
	f("eng", "", "pic", "")
	f("eng", "", "wal", "")

	// GOL
	f("gol", "", "mar", "")
	f("gol", "", "pie", "")
	f("gol", "", "spa", "sc")
	f("gol", "", "tus", "")
	f("gol", "", "tys", "")
	f("gol", "", "wes", "")

	// HEL
	f("hel", "", "den", "")
	f("hel", "", "hol", "")
	f("hel", "", "kie", "")
	f("hel", "", "nth", "")

	// ION
	f("ion", "", "adr", "")
	f("ion", "", "aeg", "")
	f("ion", "", "alb", "")
	f("ion", "", "apu", "")
	f("ion", "", "eas", "")
	f("ion", "", "gre", "")
	f("ion", "", "nap", "")
	f("ion", "", "tun", "")
	f("ion", "", "tys", "")

	// IRI
	f("iri", "", "eng", "")
	f("iri", "", "lvp", "")
	f("iri", "", "mao", "")
	f("iri", "", "nao", "")
	f("iri", "", "wal", "")

	// MAO
	f("mao", "", "bre", "")
	f("mao", "", "eng", "")
	f("mao", "", "gas", "")
	f("mao", "", "iri", "")
	f("mao", "", "naf", "")
	f("mao", "", "nao", "")
	f("mao", "", "por", "")
	f("mao", "", "spa", "nc")
	f("mao", "", "spa", "sc")
	f("mao", "", "wes", "")

	// NAO
	f("nao", "", "cly", "")
	f("nao", "", "iri", "")
	f("nao", "", "lvp", "")
	f("nao", "", "mao", "")
	f("nao", "", "nrg", "")

	// NTH
	f("nth", "", "bel", "")
	f("nth", "", "den", "")
	f("nth", "", "edi", "")
	f("nth", "", "eng", "")
	f("nth", "", "hel", "")
	f("nth", "", "hol", "")
	f("nth", "", "lon", "")
	f("nth", "", "nrg", "")
	f("nth", "", "nwy", "")
	f("nth", "", "ska", "")
	f("nth", "", "yor", "")

	// NRG
	f("nrg", "", "bar", "")
	f("nrg", "", "cly", "")
	f("nrg", "", "edi", "")
	f("nrg", "", "nao", "")
	f("nrg", "", "nth", "")
	f("nrg", "", "nwy", "")

	// SKA
	f("ska", "", "den", "")
	f("ska", "", "nth", "")
	f("ska", "", "nwy", "")
	f("ska", "", "swe", "")

	// TYS
	f("tys", "", "gol", "")
	f("tys", "", "ion", "")
	f("tys", "", "nap", "")
	f("tys", "", "rom", "")
	f("tys", "", "tun", "")
	f("tys", "", "tus", "")
	f("tys", "", "wes", "")

	// WES
	f("wes", "", "gol", "")
	f("wes", "", "mao", "")
	f("wes", "", "naf", "")
	f("wes", "", "spa", "sc")
	f("wes", "", "tun", "")
	f("wes", "", "tys", "")

	// =====================================================================
	// Inland provinces (army adjacencies only)
	// =====================================================================

	// BOH
	a("boh", "gal")
	a("boh", "mun")
	a("boh", "sil")
	a("boh", "tyr")
	a("boh", "vie")
	// BUD
	a("bud", "gal")
	a("bud", "rum")
	a("bud", "ser")
	a("bud", "tri")
	a("bud", "vie")
	// GAL
	a("gal", "boh")
	a("gal", "bud")
	a("gal", "rum")
	a("gal", "sil")
	a("gal", "ukr")
	a("gal", "vie")
	a("gal", "war")
	// MOS
	a("mos", "lvn")
	a("mos", "sev")
	a("mos", "stp")
	a("mos", "ukr")
	a("mos", "war")
	// MUN
	a("mun", "ber")
	a("mun", "boh")
	a("mun", "bur")
	a("mun", "kie")
	a("mun", "ruh")
	a("mun", "sil")
	a("mun", "tyr")
	// PAR
	a("par", "bre")
	a("par", "bur")
	a("par", "gas")
	a("par", "pic")
	// RUH
	a("ruh", "bel")
	a("ruh", "bur")
	a("ruh", "hol")
	a("ruh", "kie")
	a("ruh", "mun")
	// SER
	a("ser", "alb")
	a("ser", "bud")
	a("ser", "bul")
	a("ser", "gre")
	a("ser", "rum")
	a("ser", "tri")
	// SIL
	a("sil", "ber")
	a("sil", "boh")
	a("sil", "gal")
	a("sil", "mun")
	a("sil", "pru")
	a("sil", "war")
	// TYR
	a("tyr", "boh")
	a("tyr", "mun")
	a("tyr", "pie")
	a("tyr", "tri")
	a("tyr", "ven")
	a("tyr", "vie")
	// UKR
	a("ukr", "gal")
	a("ukr", "mos")
	a("ukr", "rum")
	a("ukr", "sev")
	a("ukr", "war")
	// VIE
	a("vie", "boh")
	a("vie", "bud")
	a("vie", "gal")
	a("vie", "tri")
	a("vie", "tyr")
	// WAR
	a("war", "gal")
	a("war", "lvn")
	a("war", "mos")
	a("war", "pru")
	a("war", "sil")
	a("war", "ukr")
	// BUR
	a("bur", "bel")
	a("bur", "gas")
	a("bur", "mar")
	a("bur", "mun")
	a("bur", "par")
	a("bur", "pic")
	a("bur", "ruh")

	// =====================================================================
	// Coastal provinces
	// =====================================================================

	// ALB
	f("alb", "", "adr", "")
	b("alb", "gre")
	f("alb", "", "ion", "")
	a("alb", "ser")
	b("alb", "tri")

	// ANK
	b("ank", "arm")
	f("ank", "", "bla", "")
	b("ank", "con")
	a("ank", "smy")

	// APU
	f("apu", "", "adr", "")
	f("apu", "", "ion", "")
	b("apu", "nap")
	a("apu", "rom")
	b("apu", "ven")

	// ARM
	b("arm", "ank")
	f("arm", "", "bla", "")
	b("arm", "sev")
	a("arm", "smy")
	a("arm", "syr")

	// BEL
	f("bel", "", "eng", "")
	b("bel", "hol")
	f("bel", "", "nth", "")
	b("bel", "pic")
	a("bel", "bur")
	a("bel", "ruh")

	// BER
	f("ber", "", "bal", "")
	b("ber", "kie")
	a("ber", "mun")
	b("ber", "pru")
	a("ber", "sil")

	// BRE
	f("bre", "", "eng", "")
	b("bre", "gas")
	f("bre", "", "mao", "")
	a("bre", "par")
	b("bre", "pic")

	// BUL (army to all neighbors; fleet via bul/ec or bul/sc)
	a("bul", "con")
	a("bul", "gre")
	a("bul", "rum")
	a("bul", "ser")
	// BUL/EC
	f("bul", "ec", "bla", "")
	f("bul", "ec", "con", "")
	f("bul", "ec", "rum", "")
	// BUL/SC
	f("bul", "sc", "aeg", "")
	f("bul", "sc", "con", "")
	f("bul", "sc", "gre", "")

	// CLY
	b("cly", "edi")
	b("cly", "lvp")
	f("cly", "", "nao", "")
	f("cly", "", "nrg", "")

	// CON
	f("con", "", "aeg", "")
	b("con", "ank")
	f("con", "", "bla", "")
	a("con", "bul")
	f("con", "", "bul", "ec")
	f("con", "", "bul", "sc")
	b("con", "smy")

	// DEN
	f("den", "", "bal", "")
	f("den", "", "hel", "")
	b("den", "kie")
	f("den", "", "nth", "")
	f("den", "", "ska", "")
	b("den", "swe")

	// EDI
	b("edi", "cly")
	a("edi", "lvp")
	f("edi", "", "nth", "")
	f("edi", "", "nrg", "")
	b("edi", "yor")

	// FIN
	f("fin", "", "bot", "")
	a("fin", "nwy")
	a("fin", "stp")
	f("fin", "", "stp", "sc")
	b("fin", "swe")

	// GAS
	b("gas", "bre")
	a("gas", "bur")
	f("gas", "", "mao", "")
	a("gas", "mar")
	a("gas", "par")
	a("gas", "spa")
	f("gas", "", "spa", "nc")

	// GRE
	f("gre", "", "aeg", "")
	b("gre", "alb")
	a("gre", "bul")
	f("gre", "", "bul", "sc")
	f("gre", "", "ion", "")
	a("gre", "ser")

	// HOL
	b("hol", "bel")
	f("hol", "", "hel", "")
	b("hol", "kie")
	f("hol", "", "nth", "")
	a("hol", "ruh")

	// KIE
	f("kie", "", "bal", "")
	b("kie", "ber")
	b("kie", "den")
	f("kie", "", "hel", "")
	b("kie", "hol")
	a("kie", "mun")
	a("kie", "ruh")

	// LON
	f("lon", "", "eng", "")
	f("lon", "", "nth", "")
	b("lon", "wal")
	b("lon", "yor")

	// LVN
	f("lvn", "", "bal", "")
	f("lvn", "", "bot", "")
	a("lvn", "mos")
	b("lvn", "pru")
	a("lvn", "stp")
	f("lvn", "", "stp", "sc")
	a("lvn", "war")

	// LVP
	b("lvp", "cly")
	a("lvp", "edi")
	f("lvp", "", "iri", "")
	f("lvp", "", "nao", "")
	b("lvp", "wal")
	a("lvp", "yor")

	// MAR
	a("mar", "bur")
	a("mar", "gas")
	f("mar", "", "gol", "")
	b("mar", "pie")
	a("mar", "spa")
	f("mar", "", "spa", "sc")

	// NAF
	f("naf", "", "mao", "")
	b("naf", "tun")
	f("naf", "", "wes", "")

	// NAP
	b("nap", "apu")
	f("nap", "", "ion", "")
	b("nap", "rom")
	f("nap", "", "tys", "")

	// NWY
	f("nwy", "", "bar", "")
	f("nwy", "", "nth", "")
	f("nwy", "", "nrg", "")
	f("nwy", "", "ska", "")
	a("nwy", "stp")
	f("nwy", "", "stp", "nc")
	b("nwy", "swe")
	a("nwy", "fin")

	// PIE
	f("pie", "", "gol", "")
	b("pie", "mar")
	b("pie", "tus")
	a("pie", "tyr")
	a("pie", "ven")

	// PIC
	b("pic", "bel")
	b("pic", "bre")
	a("pic", "bur")
	f("pic", "", "eng", "")
	a("pic", "par")

	// POR
	f("por", "", "mao", "")
	a("por", "spa")
	f("por", "", "spa", "nc")
	f("por", "", "spa", "sc")

	// PRU
	f("pru", "", "bal", "")
	b("pru", "ber")
	b("pru", "lvn")
	a("pru", "sil")
	a("pru", "war")

	// ROM
	a("rom", "apu")
	b("rom", "nap")
	b("rom", "tus")
	f("rom", "", "tys", "")
	a("rom", "ven")

	// RUM
	f("rum", "", "bla", "")
	a("rum", "bud")
	a("rum", "bul")
	f("rum", "", "bul", "ec")
	a("rum", "gal")
	a("rum", "ser")
	b("rum", "sev")
	a("rum", "ukr")

	// SEV
	b("sev", "arm")
	f("sev", "", "bla", "")
	a("sev", "mos")
	b("sev", "rum")
	a("sev", "ukr")

	// SMY
	f("smy", "", "aeg", "")
	a("smy", "ank")
	a("smy", "arm")
	b("smy", "con")
	f("smy", "", "eas", "")
	b("smy", "syr")

	// SPA (army only; fleet via spa/nc or spa/sc)
	a("spa", "gas")
	a("spa", "mar")
	a("spa", "por")
	// SPA/NC
	f("spa", "nc", "gas", "")
	f("spa", "nc", "mao", "")
	f("spa", "nc", "por", "")
	// SPA/SC
	f("spa", "sc", "gol", "")
	f("spa", "sc", "mao", "")
	f("spa", "sc", "mar", "")
	f("spa", "sc", "por", "")
	f("spa", "sc", "wes", "")

	// STP (army to all; fleet via stp/nc or stp/sc)
	a("stp", "fin")
	a("stp", "lvn")
	a("stp", "mos")
	a("stp", "nwy")
	// STP/NC
	f("stp", "nc", "bar", "")
	f("stp", "nc", "nwy", "")
	// STP/SC
	f("stp", "sc", "bot", "")
	f("stp", "sc", "fin", "")
	f("stp", "sc", "lvn", "")

	// SWE
	f("swe", "", "bal", "")
	f("swe", "", "bot", "")
	b("swe", "den")
	b("swe", "fin")
	b("swe", "nwy")
	f("swe", "", "ska", "")

	// SYR
	a("syr", "arm")
	f("syr", "", "eas", "")
	b("syr", "smy")

	// TRI
	f("tri", "", "adr", "")
	b("tri", "alb")
	a("tri", "bud")
	a("tri", "ser")
	a("tri", "tyr")
	b("tri", "ven")
	a("tri", "vie")

	// TUN
	f("tun", "", "ion", "")
	b("tun", "naf")
	f("tun", "", "tys", "")
	f("tun", "", "wes", "")

	// TUS
	f("tus", "", "gol", "")
	b("tus", "pie")
	b("tus", "rom")
	f("tus", "", "tys", "")
	a("tus", "ven")

	// VEN
	f("ven", "", "adr", "")
	b("ven", "apu")
	a("ven", "pie")
	a("ven", "rom")
	b("ven", "tri")
	a("ven", "tus")
	a("ven", "tyr")

	// WAL
	f("wal", "", "eng", "")
	f("wal", "", "iri", "")
	b("wal", "lon")
	b("wal", "lvp")
	a("wal", "yor")

	// YOR
	b("yor", "edi")
	b("yor", "lon")
	a("yor", "lvp")
	f("yor", "", "nth", "")
	a("yor", "wal")

	return adj
}

// TestAdjacencyMatchesStandard verifies that every adjacency in the Go
// StandardMap matches the canonical standard Diplomacy adjacency list, and
// that the Go map has no extra or missing adjacencies.
func TestAdjacencyMatchesStandard(t *testing.T) {
	m := StandardMap()
	canonical := buildCanonicalAdjacencies()

	var errors []string

	// --- Check 1: For every canonical adjacency, verify the Go map agrees ---
	for from, adjs := range canonical {
		for _, ca := range adjs {
			fromCoast := Coast("")
			toCoast := Coast(ca.toCoast)

			switch ca.aType {
			case adjArmy:
				if !m.Adjacent(from, NoCoast, ca.to, NoCoast, false) {
					errors = append(errors, fmt.Sprintf("MISSING army: %s -> %s", from, ca.to))
				}

			case adjFleet:
				// For fleet adjacencies involving split coasts, we need to check
				// with the coast on the split-coast side.
				fromCoast = NoCoast
				if toCoast == "" {
					toCoast = NoCoast
				}
				if !m.Adjacent(from, fromCoast, ca.to, toCoast, true) {
					errors = append(errors, fmt.Sprintf("MISSING fleet: %s -> %s (toCoast=%s)",
						from, ca.to, ca.toCoast))
				}

			case adjBoth:
				if !m.Adjacent(from, NoCoast, ca.to, NoCoast, false) {
					errors = append(errors, fmt.Sprintf("MISSING army (both): %s -> %s", from, ca.to))
				}
				if !m.Adjacent(from, fromCoast, ca.to, toCoast, true) {
					errors = append(errors, fmt.Sprintf("MISSING fleet (both): %s -> %s", from, ca.to))
				}
			}
		}
	}

	// --- Check 2: For every Go adjacency, verify it exists in canonical ---
	// Build a lookup of what the canonical data says is valid.
	type canonLookupKey struct {
		from    string
		to      string
		toCoast string
		isArmy  bool
		isFleet bool
	}
	canonLookup := make(map[canonLookupKey]bool, 500)
	for from, adjs := range canonical {
		for _, ca := range adjs {
			if ca.aType == adjArmy || ca.aType == adjBoth {
				canonLookup[canonLookupKey{from, ca.to, "", true, false}] = true
			}
			if ca.aType == adjFleet || ca.aType == adjBoth {
				canonLookup[canonLookupKey{from, ca.to, ca.toCoast, false, true}] = true
			}
		}
	}

	for from, adjs := range m.Adjacencies {
		for _, adj := range adjs {
			if adj.ArmyOK {
				key := canonLookupKey{from, adj.To, "", true, false}
				if !canonLookup[key] {
					errors = append(errors, fmt.Sprintf("EXTRA army in Go: %s -> %s (fromCoast=%s, toCoast=%s)",
						from, adj.To, adj.FromCoast, adj.ToCoast))
				}
			}
			if adj.FleetOK {
				key := canonLookupKey{from, adj.To, string(adj.ToCoast), false, true}
				if !canonLookup[key] {
					errors = append(errors, fmt.Sprintf("EXTRA fleet in Go: %s -> %s (fromCoast=%s, toCoast=%s)",
						from, adj.To, adj.FromCoast, adj.ToCoast))
				}
			}
		}
	}

	if len(errors) > 0 {
		sort.Strings(errors)
		t.Errorf("Found %d adjacency discrepancies:\n%s", len(errors), strings.Join(errors, "\n"))
	}
}

// TestAdjacencyCountSanity checks the total number of directed adjacency
// entries matches the expected count.
func TestAdjacencyCountSanity(t *testing.T) {
	m := StandardMap()
	total := 0
	armyOnly := 0
	fleetOnly := 0
	bothCount := 0
	for _, adjs := range m.Adjacencies {
		for _, adj := range adjs {
			total++
			switch {
			case adj.ArmyOK && adj.FleetOK:
				bothCount++
			case adj.ArmyOK:
				armyOnly++
			case adj.FleetOK:
				fleetOnly++
			}
		}
	}
	// Expected: 218 unique bidirectional pairs = 436 directed entries.
	// Breakdown: 107 fleet-only pairs (214) + 77 army-only pairs (154)
	//            + 34 both pairs (68) = 436 total.
	if total != 436 {
		t.Errorf("expected 436 directed adjacency entries, got %d", total)
	}
	if armyOnly != 154 {
		t.Errorf("expected 154 army-only entries, got %d", armyOnly)
	}
	if fleetOnly != 214 {
		t.Errorf("expected 214 fleet-only entries, got %d", fleetOnly)
	}
	if bothCount != 68 {
		t.Errorf("expected 68 both-army-and-fleet entries, got %d", bothCount)
	}
}

// TestCanonicalAdjacencySymmetry verifies that the canonical adjacency
// reference itself is symmetric: for every A->B, there is a matching B->A.
func TestCanonicalAdjacencySymmetry(t *testing.T) {
	canonical := buildCanonicalAdjacencies()

	// Build reverse lookup.
	type symKey struct {
		from    string
		to      string
		toCoast string
		aType   adjType
	}

	entries := make(map[symKey]bool)
	for from, adjs := range canonical {
		for _, ca := range adjs {
			entries[symKey{from, ca.to, ca.toCoast, ca.aType}] = true
		}
	}

	var errors []string
	for from, adjs := range canonical {
		for _, ca := range adjs {
			// For the reverse, the "from" of the original becomes "to",
			// and what was the toCoast becomes the fromCoast of the reverse.
			// But our canonical data doesn't track fromCoast in the lookup,
			// so we check a simpler reverse: does the target province have
			// an entry back to the source?

			// For split-coast provinces, the reverse adjacency may have a
			// different coast on the "from" side. E.g., canonical has:
			//   bla -> bul/ec (fleet)
			//   bul/ec -> bla (fleet)
			// The first is under "bla" with toCoast="ec".
			// The second is under "bul" with toCoast="" (bla has no coast).
			// So the reverse key is {from:"bul", to:"bla", toCoast:"", fleet}.

			// For "both" adjacencies, they are always NoCoast on both sides.
			reverseToCoast := "" // the original "from" has no coast in the canonical lookup
			switch ca.aType {
			case adjArmy:
				rk := symKey{ca.to, from, reverseToCoast, adjArmy}
				if !entries[rk] {
					errors = append(errors, fmt.Sprintf("canonical asymmetry: %s->%s (army) has no reverse", from, ca.to))
				}
			case adjFleet:
				// The reverse of "from -> to/toCoast" is "to -> from".
				// But if "from" has a coast, it would appear as toCoast in the reverse.
				// Our canonical data doesn't encode fromCoast in the key.
				// For fleet: check if there's any fleet entry from ca.to to from.
				found := false
				if reverseAdjs, ok := canonical[ca.to]; ok {
					for _, ra := range reverseAdjs {
						if ra.to == from && (ra.aType == adjFleet || ra.aType == adjBoth) {
							found = true
							break
						}
					}
				}
				if !found {
					errors = append(errors, fmt.Sprintf("canonical asymmetry: %s->%s/%s (fleet) has no reverse",
						from, ca.to, ca.toCoast))
				}
			case adjBoth:
				rk := symKey{ca.to, from, "", adjBoth}
				if !entries[rk] {
					errors = append(errors, fmt.Sprintf("canonical asymmetry: %s->%s (both) has no reverse", from, ca.to))
				}
			}
		}
	}

	if len(errors) > 0 {
		sort.Strings(errors)
		t.Errorf("Found %d canonical symmetry issues:\n%s", len(errors), strings.Join(errors, "\n"))
	}
}

// TestSplitCoastFleetReachability verifies that fleets on specific coasts of
// split-coast provinces can reach exactly the expected destinations.
func TestSplitCoastFleetReachability(t *testing.T) {
	m := StandardMap()

	tests := []struct {
		prov     string
		coast    Coast
		expected []string
	}{
		// Bulgaria
		{"bul", EastCoast, []string{"bla", "con", "rum"}},
		{"bul", SouthCoast, []string{"aeg", "con", "gre"}},
		// Spain
		{"spa", NorthCoast, []string{"gas", "mao", "por"}},
		{"spa", SouthCoast, []string{"gol", "mao", "mar", "por", "wes"}},
		// St. Petersburg
		{"stp", NorthCoast, []string{"bar", "nwy"}},
		{"stp", SouthCoast, []string{"bot", "fin", "lvn"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.prov, tt.coast), func(t *testing.T) {
			actual := m.ProvincesAdjacentTo(tt.prov, tt.coast, true)
			sort.Strings(actual)
			expected := make([]string, len(tt.expected))
			copy(expected, tt.expected)
			sort.Strings(expected)

			if len(actual) != len(expected) {
				t.Errorf("fleet from %s/%s: got %v, want %v", tt.prov, tt.coast, actual, expected)
				return
			}
			for i := range actual {
				if actual[i] != expected[i] {
					t.Errorf("fleet from %s/%s: got %v, want %v", tt.prov, tt.coast, actual, expected)
					return
				}
			}
		})
	}
}
