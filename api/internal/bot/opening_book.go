package bot

import (
	"math/rand"
	"sort"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// openingEntry represents a single opening: a named set of orders with a weight
// proportional to historical popularity.
type openingEntry struct {
	name   string
	weight float64
	orders []OrderInput
}

// fallCondition maps a set of unit positions to a weighted list of fall order sets.
type fallCondition struct {
	// positions maps province -> unit type ("army"/"fleet") for matching
	positions map[string]string
	entries   []openingEntry
}

// unitKey builds a sorted position fingerprint for the power's units.
func unitKey(gs *diplomacy.GameState, power diplomacy.Power) map[string]string {
	m := make(map[string]string)
	for _, u := range gs.UnitsOf(power) {
		m[u.Province] = u.Type.String()
	}
	return m
}

// positionsMatch returns true if every required position is present in actual.
func positionsMatch(required, actual map[string]string) bool {
	for prov, utype := range required {
		if actual[prov] != utype {
			return false
		}
	}
	return true
}

// weightedSelect picks an entry from a weighted list using random selection
// proportional to weights.
func weightedSelect(entries []openingEntry) *openingEntry {
	if len(entries) == 0 {
		return nil
	}
	total := 0.0
	for i := range entries {
		total += entries[i].weight
	}
	r := rand.Float64() * total
	cum := 0.0
	for i := range entries {
		cum += entries[i].weight
		if r < cum {
			return &entries[i]
		}
	}
	return &entries[len(entries)-1]
}

// validateOrders checks that all orders in the set are valid against the map
// and game state. Returns nil if any order fails validation.
func validateOrders(orders []OrderInput, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	for _, o := range orders {
		eng := orderInputToOrder(o, power)
		switch eng.Type {
		case diplomacy.OrderMove:
			if diplomacy.ValidateOrder(eng, gs, m) != nil {
				return nil
			}
		case diplomacy.OrderHold:
			if gs.UnitAt(o.Location) == nil {
				return nil
			}
		case diplomacy.OrderSupport:
			if diplomacy.ValidateOrder(eng, gs, m) != nil {
				return nil
			}
		case diplomacy.OrderConvoy:
			if diplomacy.ValidateOrder(eng, gs, m) != nil {
				return nil
			}
		}
	}
	return orders
}

// orderInputToOrder converts a single OrderInput to a diplomacy.Order.
func orderInputToOrder(o OrderInput, power diplomacy.Power) diplomacy.Order {
	return diplomacy.Order{
		UnitType:    parseUnitTypeStr(o.UnitType),
		Power:       power,
		Location:    o.Location,
		Coast:       diplomacy.Coast(o.Coast),
		Type:        parseOrderTypeStr(o.OrderType),
		Target:      o.Target,
		TargetCoast: diplomacy.Coast(o.TargetCoast),
		AuxLoc:      o.AuxLoc,
		AuxTarget:   o.AuxTarget,
		AuxUnitType: parseUnitTypeStr(o.AuxUnitType),
	}
}

// move is a shorthand constructor for a move OrderInput.
func mv(ut, loc, target string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "move", Target: target}
}

// mvC is a shorthand constructor for a move OrderInput with a target coast.
func mvC(ut, loc, coast, target, targetCoast string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, Coast: coast, OrderType: "move", Target: target, TargetCoast: targetCoast}
}

// hld is a shorthand constructor for a hold OrderInput.
func hld(ut, loc string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "hold"}
}

// sup is a shorthand constructor for a support-move OrderInput.
func sup(ut, loc, auxLoc, auxTarget, auxUt string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "support", AuxLoc: auxLoc, AuxTarget: auxTarget, AuxUnitType: auxUt}
}

// con is a shorthand constructor for a convoy OrderInput.
func con(loc, auxLoc, auxTarget string) OrderInput {
	return OrderInput{UnitType: "fleet", Location: loc, OrderType: "convoy", AuxLoc: auxLoc, AuxTarget: auxTarget, AuxUnitType: "army"}
}

// springOpenings returns the Spring 1901 opening entries for a power.
// All moves are validated against the actual map adjacencies in map_data.go.
func springOpenings(power diplomacy.Power) []openingEntry {
	switch power {
	case diplomacy.England:
		// lvp adjacent to: edi, wal, cly (both), iri, nao (fleet)
		return []openingEntry{
			{name: "Northern", weight: 37.6, orders: []OrderInput{
				mv("fleet", "edi", "nrg"), mv("fleet", "lon", "nth"), mv("army", "lvp", "edi"),
			}},
			{name: "Welsh", weight: 20.4, orders: []OrderInput{
				mv("fleet", "edi", "nrg"), mv("fleet", "lon", "nth"), mv("army", "lvp", "wal"),
			}},
			{name: "Channel", weight: 13.0, orders: []OrderInput{
				mv("fleet", "edi", "nth"), mv("fleet", "lon", "eng"), mv("army", "lvp", "wal"),
			}},
			{name: "Edinburgh", weight: 11.0, orders: []OrderInput{
				mv("fleet", "edi", "nth"), mv("fleet", "lon", "eng"), mv("army", "lvp", "edi"),
			}},
		}
	case diplomacy.France:
		return []openingEntry{
			{name: "Maginot", weight: 23.1, orders: []OrderInput{
				mv("fleet", "bre", "mao"), mv("army", "par", "bur"),
				sup("army", "mar", "par", "bur", "army"),
			}},
			{name: "Picardy", weight: 17.4, orders: []OrderInput{
				mv("fleet", "bre", "mao"), mv("army", "par", "pic"), mv("army", "mar", "spa"),
			}},
			{name: "Burgundy", weight: 17.0, orders: []OrderInput{
				mv("fleet", "bre", "mao"), mv("army", "par", "bur"), mv("army", "mar", "spa"),
			}},
			{name: "Guernsey", weight: 7.3, orders: []OrderInput{
				mv("fleet", "bre", "eng"), mv("army", "par", "pic"), mv("army", "mar", "spa"),
			}},
		}
	case diplomacy.Germany:
		// kie adjacent to: den, ber (both), mun, ruh (army), bal, hel (fleet)
		return []openingEntry{
			{name: "Danish Blitzkrieg", weight: 32.3, orders: []OrderInput{
				mv("fleet", "kie", "den"), mv("army", "ber", "kie"), mv("army", "mun", "ruh"),
			}},
			{name: "Burgundian Attack", weight: 18.6, orders: []OrderInput{
				mv("fleet", "kie", "den"), mv("army", "ber", "kie"), mv("army", "mun", "bur"),
			}},
			{name: "Baltic Opening", weight: 9.1, orders: []OrderInput{
				mv("fleet", "kie", "bal"), mv("army", "ber", "kie"), mv("army", "mun", "ruh"),
			}},
			{name: "Helgoland Opening", weight: 4.0, orders: []OrderInput{
				mv("fleet", "kie", "hel"), mv("army", "ber", "kie"), mv("army", "mun", "ruh"),
			}},
		}
	case diplomacy.Italy:
		// rom adjacent to: ven, nap, tus (both), tys (fleet)
		// ven adjacent to: apu, rom, pie, tri (both), tyr (army), adr (fleet)
		return []openingEntry{
			{name: "Trentino Attack", weight: 20.0, orders: []OrderInput{
				mv("fleet", "nap", "ion"), mv("army", "rom", "ven"), mv("army", "ven", "tyr"),
			}},
			{name: "Lepanto Preparation", weight: 19.1, orders: []OrderInput{
				mv("fleet", "nap", "ion"), mv("army", "rom", "ven"), mv("army", "ven", "apu"),
			}},
			{name: "Alpine Chicken", weight: 8.0, orders: []OrderInput{
				mv("fleet", "nap", "ion"), mv("army", "rom", "ven"), mv("army", "ven", "pie"),
			}},
			{name: "Trieste Strike", weight: 5.0, orders: []OrderInput{
				mv("fleet", "nap", "ion"), mv("army", "rom", "ven"), mv("army", "ven", "tri"),
			}},
		}
	case diplomacy.Austria:
		return []openingEntry{
			{name: "Slovenian", weight: 18.0, orders: []OrderInput{
				mv("fleet", "tri", "alb"), mv("army", "bud", "ser"), mv("army", "vie", "tri"),
			}},
			{name: "Galician", weight: 16.0, orders: []OrderInput{
				mv("fleet", "tri", "alb"), mv("army", "bud", "ser"), mv("army", "vie", "gal"),
			}},
			{name: "Balkan", weight: 14.0, orders: []OrderInput{
				mv("fleet", "tri", "alb"), mv("army", "bud", "ser"), mv("army", "vie", "bud"),
			}},
			{name: "Hungarian Houseboat", weight: 4.8, orders: []OrderInput{
				hld("fleet", "tri"), mv("army", "bud", "ser"), mv("army", "vie", "gal"),
			}},
		}
	case diplomacy.Russia:
		return []openingEntry{
			{name: "Southern Defence", weight: 22.4, orders: []OrderInput{
				mvC("fleet", "stp", "sc", "bot", ""),
				mv("fleet", "sev", "bla"), mv("army", "mos", "ukr"), mv("army", "war", "gal"),
			}},
			{name: "Ukrainian System", weight: 9.2, orders: []OrderInput{
				mvC("fleet", "stp", "sc", "bot", ""),
				mv("fleet", "sev", "rum"), mv("army", "mos", "ukr"), mv("army", "war", "gal"),
			}},
			{name: "The Squid", weight: 7.8, orders: []OrderInput{
				mvC("fleet", "stp", "sc", "bot", ""),
				mv("fleet", "sev", "bla"), mv("army", "mos", "stp"), mv("army", "war", "ukr"),
			}},
			{name: "The Octopus", weight: 5.0, orders: []OrderInput{
				mvC("fleet", "stp", "sc", "bot", ""),
				mv("fleet", "sev", "bla"), mv("army", "mos", "stp"), mv("army", "war", "gal"),
			}},
		}
	case diplomacy.Turkey:
		// smy adjacent to: con, arm, syr (both), aeg, eas (fleet)
		// ank adjacent to: con, arm (both), bla (fleet)
		return []openingEntry{
			{name: "Byzantine", weight: 47.5, orders: []OrderInput{
				mv("army", "con", "bul"), mv("army", "smy", "con"), mv("fleet", "ank", "bla"),
			}},
			{name: "Armenian Attack", weight: 13.0, orders: []OrderInput{
				mv("army", "con", "bul"), mv("army", "smy", "arm"), mv("fleet", "ank", "bla"),
			}},
			{name: "Anti-Lepanto", weight: 8.7, orders: []OrderInput{
				mv("army", "con", "bul"), mv("army", "smy", "arm"), mv("fleet", "ank", "con"),
			}},
			{name: "Boston Strangler", weight: 4.7, orders: []OrderInput{
				mv("army", "con", "bul"), mv("army", "smy", "con"), hld("fleet", "ank"),
			}},
		}
	}
	return nil
}

// fallOpenings returns the Fall 1901 conditional opening data for a power.
// Conditions match unit positions resulting from spring openings above.
func fallOpenings(power diplomacy.Power) []fallCondition {
	switch power {
	case diplomacy.England:
		return []fallCondition{
			// Northern result: F nrg, F nth, A edi
			{positions: map[string]string{"nrg": "fleet", "nth": "fleet", "edi": "army"},
				entries: []openingEntry{
					{name: "Northern: convoy edi to nwy", weight: 60, orders: []OrderInput{
						con("nrg", "edi", "nwy"),
						mv("army", "edi", "nwy"),
						mv("fleet", "nth", "bel"),
					}},
					{name: "Northern: nrg to nwy, nth to bel", weight: 40, orders: []OrderInput{
						mv("fleet", "nrg", "nwy"),
						mv("fleet", "nth", "bel"),
						mv("army", "edi", "yor"),
					}},
				}},
			// Welsh result: F nrg, F nth, A wal
			{positions: map[string]string{"nrg": "fleet", "nth": "fleet", "wal": "army"},
				entries: []openingEntry{
					{name: "Welsh: convoy wal to nwy", weight: 60, orders: []OrderInput{
						mv("fleet", "nrg", "nwy"),
						con("nth", "wal", "bel"),
						mv("army", "wal", "bel"),
					}},
					{name: "Welsh: nrg to nwy, wal to lon", weight: 40, orders: []OrderInput{
						mv("fleet", "nrg", "nwy"),
						mv("fleet", "nth", "bel"),
						mv("army", "wal", "lon"),
					}},
				}},
			// Channel result: F nth, F eng, A wal
			{positions: map[string]string{"nth": "fleet", "eng": "fleet", "wal": "army"},
				entries: []openingEntry{
					{name: "Channel: eng to bel, nth to nwy", weight: 60, orders: []OrderInput{
						mv("fleet", "eng", "bel"),
						mv("fleet", "nth", "nwy"),
						mv("army", "wal", "lon"),
					}},
					{name: "Channel: eng to bre", weight: 40, orders: []OrderInput{
						mv("fleet", "eng", "bre"),
						mv("fleet", "nth", "nwy"),
						mv("army", "wal", "lon"),
					}},
				}},
			// Edinburgh result: F nth, F eng, A edi
			{positions: map[string]string{"nth": "fleet", "eng": "fleet", "edi": "army"},
				entries: []openingEntry{
					{name: "Edinburgh: eng to bel, nth to nwy", weight: 60, orders: []OrderInput{
						mv("fleet", "eng", "bel"),
						mv("fleet", "nth", "nwy"),
						mv("army", "edi", "yor"),
					}},
					{name: "Edinburgh: eng to bre", weight: 40, orders: []OrderInput{
						mv("fleet", "eng", "bre"),
						mv("fleet", "nth", "nwy"),
						mv("army", "edi", "yor"),
					}},
				}},
		}
	case diplomacy.France:
		return []fallCondition{
			// Maginot result: F mao, A bur, A mar (mar stays because it supported)
			{positions: map[string]string{"mao": "fleet", "bur": "army", "mar": "army"},
				entries: []openingEntry{
					{name: "Maginot: por+bel", weight: 50, orders: []OrderInput{
						mv("fleet", "mao", "por"),
						mv("army", "bur", "bel"),
						mv("army", "mar", "spa"),
					}},
					{name: "Maginot: spa+mun", weight: 50, orders: []OrderInput{
						mvC("fleet", "mao", "", "spa", "sc"),
						mv("army", "bur", "mun"),
						mv("army", "mar", "bur"),
					}},
				}},
			// Picardy result: F mao, A pic, A spa
			{positions: map[string]string{"mao": "fleet", "pic": "army", "spa": "army"},
				entries: []openingEntry{
					{name: "Picardy: pic to bel, spa to mar", weight: 60, orders: []OrderInput{
						mv("fleet", "mao", "por"),
						mv("army", "pic", "bel"),
						mv("army", "spa", "mar"),
					}},
					{name: "Picardy: mao to spa(sc)", weight: 40, orders: []OrderInput{
						mvC("fleet", "mao", "", "spa", "sc"),
						mv("army", "pic", "bel"),
						mv("army", "spa", "por"),
					}},
				}},
			// Burgundy result: F mao, A bur, A spa
			{positions: map[string]string{"mao": "fleet", "bur": "army", "spa": "army"},
				entries: []openingEntry{
					{name: "Burgundy: bel+por", weight: 60, orders: []OrderInput{
						mv("fleet", "mao", "por"),
						mv("army", "bur", "bel"),
						mv("army", "spa", "mar"),
					}},
					{name: "Burgundy: mun push", weight: 40, orders: []OrderInput{
						mv("fleet", "mao", "por"),
						mv("army", "bur", "mun"),
						mv("army", "spa", "mar"),
					}},
				}},
			// Guernsey result: F eng, A pic, A spa
			{positions: map[string]string{"eng": "fleet", "pic": "army", "spa": "army"},
				entries: []openingEntry{
					{name: "Guernsey: eng to bel, pic supports", weight: 60, orders: []OrderInput{
						mv("fleet", "eng", "bel"),
						sup("army", "pic", "eng", "bel", "fleet"),
						mv("army", "spa", "por"),
					}},
					{name: "Guernsey: eng to nth", weight: 40, orders: []OrderInput{
						mv("fleet", "eng", "nth"),
						mv("army", "pic", "bel"),
						mv("army", "spa", "por"),
					}},
				}},
		}
	case diplomacy.Germany:
		return []fallCondition{
			// Danish Blitzkrieg result: F den, A kie, A ruh
			{positions: map[string]string{"den": "fleet", "kie": "army", "ruh": "army"},
				entries: []openingEntry{
					{name: "Danish: swe+hol", weight: 50, orders: []OrderInput{
						mv("fleet", "den", "swe"),
						mv("army", "kie", "mun"),
						mv("army", "ruh", "hol"),
					}},
					{name: "Danish: swe+bel", weight: 50, orders: []OrderInput{
						mv("fleet", "den", "swe"),
						mv("army", "kie", "den"),
						mv("army", "ruh", "bel"),
					}},
				}},
			// Burgundian Attack result: F den, A kie, A bur (if succeeded)
			{positions: map[string]string{"den": "fleet", "kie": "army", "bur": "army"},
				entries: []openingEntry{
					{name: "Burgundian: bur to bel", weight: 60, orders: []OrderInput{
						mv("fleet", "den", "swe"),
						mv("army", "kie", "mun"),
						mv("army", "bur", "bel"),
					}},
					{name: "Burgundian: bur to mar", weight: 40, orders: []OrderInput{
						mv("fleet", "den", "swe"),
						mv("army", "kie", "mun"),
						mv("army", "bur", "mar"),
					}},
				}},
			// Burgundian bounce (A stayed in mun)
			{positions: map[string]string{"den": "fleet", "kie": "army", "mun": "army"},
				entries: []openingEntry{
					{name: "Burgundian bounce: fallback", weight: 100, orders: []OrderInput{
						mv("fleet", "den", "swe"),
						mv("army", "kie", "den"),
						mv("army", "mun", "ruh"),
					}},
				}},
			// Baltic Opening result: F bal, A kie, A ruh
			{positions: map[string]string{"bal": "fleet", "kie": "army", "ruh": "army"},
				entries: []openingEntry{
					{name: "Baltic: swe+hol", weight: 60, orders: []OrderInput{
						mv("fleet", "bal", "swe"),
						mv("army", "kie", "den"),
						mv("army", "ruh", "hol"),
					}},
					{name: "Baltic: den+bel", weight: 40, orders: []OrderInput{
						mv("fleet", "bal", "den"),
						mv("army", "kie", "mun"),
						mv("army", "ruh", "bel"),
					}},
				}},
			// Helgoland Opening result: F hel, A kie, A ruh
			{positions: map[string]string{"hel": "fleet", "kie": "army", "ruh": "army"},
				entries: []openingEntry{
					{name: "Helgoland: hol+den", weight: 60, orders: []OrderInput{
						mv("fleet", "hel", "hol"),
						mv("army", "kie", "den"),
						mv("army", "ruh", "bel"),
					}},
					{name: "Helgoland: nth+den", weight: 40, orders: []OrderInput{
						mv("fleet", "hel", "nth"),
						mv("army", "kie", "den"),
						mv("army", "ruh", "bel"),
					}},
				}},
		}
	case diplomacy.Italy:
		return []fallCondition{
			// Trentino result: F ion, A ven, A tyr
			{positions: map[string]string{"ion": "fleet", "ven": "army", "tyr": "army"},
				entries: []openingEntry{
					{name: "Trentino: tun+mun", weight: 40, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						hld("army", "ven"),
						mv("army", "tyr", "mun"),
					}},
					{name: "Trentino: tun+vie", weight: 30, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						hld("army", "ven"),
						mv("army", "tyr", "vie"),
					}},
					{name: "Trentino: tun+tri", weight: 30, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						mv("army", "ven", "tri"),
						sup("army", "tyr", "ven", "tri", "army"),
					}},
				}},
			// Lepanto result: F ion, A ven, A apu (rom->ven, ven->apu)
			{positions: map[string]string{"ion": "fleet", "ven": "army", "apu": "army"},
				entries: []openingEntry{
					{name: "Lepanto: convoy to tun", weight: 70, orders: []OrderInput{
						con("ion", "apu", "tun"),
						mv("army", "apu", "tun"),
						hld("army", "ven"),
					}},
					{name: "Lepanto: ion to tun direct", weight: 30, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						mv("army", "apu", "nap"),
						hld("army", "ven"),
					}},
				}},
			// Alpine Chicken result: F ion, A ven, A pie (rom->ven, ven->pie)
			{positions: map[string]string{"ion": "fleet", "ven": "army", "pie": "army"},
				entries: []openingEntry{
					{name: "Alpine: tun+mar", weight: 50, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						hld("army", "ven"),
						mv("army", "pie", "mar"),
					}},
					{name: "Alpine: tun+tyr", weight: 50, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						hld("army", "ven"),
						mv("army", "pie", "tyr"),
					}},
				}},
			// Trieste Strike result: F ion, A ven, A tri (rom->ven, ven->tri)
			{positions: map[string]string{"ion": "fleet", "ven": "army", "tri": "army"},
				entries: []openingEntry{
					{name: "Trieste: tun+ser", weight: 60, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						sup("army", "ven", "tri", "ser", "army"),
						mv("army", "tri", "ser"),
					}},
					{name: "Trieste: tun+alb", weight: 40, orders: []OrderInput{
						mv("fleet", "ion", "tun"),
						sup("army", "ven", "tri", "alb", "army"),
						mv("army", "tri", "alb"),
					}},
				}},
		}
	case diplomacy.Austria:
		return []fallCondition{
			// Slovenian result: F alb, A ser, A tri
			{positions: map[string]string{"alb": "fleet", "ser": "army", "tri": "army"},
				entries: []openingEntry{
					{name: "Slovenian: gre+vie", weight: 50, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						mv("army", "tri", "vie"),
					}},
					{name: "Slovenian: gre+ven", weight: 50, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						mv("army", "tri", "ven"),
					}},
				}},
			// Galician result: F alb, A ser, A gal
			{positions: map[string]string{"alb": "fleet", "ser": "army", "gal": "army"},
				entries: []openingEntry{
					{name: "Galician: gre+rum", weight: 60, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						mv("army", "gal", "rum"),
					}},
					{name: "Galician: gre, gal holds", weight: 40, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						hld("army", "gal"),
					}},
				}},
			// Balkan result: F alb, A ser, A bud
			{positions: map[string]string{"alb": "fleet", "ser": "army", "bud": "army"},
				entries: []openingEntry{
					{name: "Balkan: gre+rum", weight: 60, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						mv("army", "bud", "rum"),
					}},
					{name: "Balkan: gre+gal", weight: 40, orders: []OrderInput{
						mv("fleet", "alb", "gre"),
						sup("army", "ser", "alb", "gre", "fleet"),
						mv("army", "bud", "gal"),
					}},
				}},
			// Hungarian Houseboat result: F tri, A ser, A gal
			{positions: map[string]string{"tri": "fleet", "ser": "army", "gal": "army"},
				entries: []openingEntry{
					{name: "Houseboat: tri to alb, rum", weight: 60, orders: []OrderInput{
						mv("fleet", "tri", "alb"),
						mv("army", "ser", "gre"),
						mv("army", "gal", "rum"),
					}},
					{name: "Houseboat: tri to adr", weight: 40, orders: []OrderInput{
						mv("fleet", "tri", "adr"),
						mv("army", "ser", "gre"),
						mv("army", "gal", "rum"),
					}},
				}},
		}
	case diplomacy.Russia:
		return []fallCondition{
			// Southern Defence result: F bot, F bla, A ukr, A gal
			{positions: map[string]string{"bot": "fleet", "bla": "fleet", "ukr": "army", "gal": "army"},
				entries: []openingEntry{
					{name: "Southern: swe+rum", weight: 60, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						sup("fleet", "bla", "ukr", "rum", "army"),
						mv("army", "ukr", "rum"),
						sup("army", "gal", "ukr", "rum", "army"),
					}},
					{name: "Southern: swe, bla to rum", weight: 40, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mv("fleet", "bla", "rum"),
						mv("army", "ukr", "sev"),
						mv("army", "gal", "rum"),
					}},
				}},
			// Ukrainian System result: F bot, F rum, A ukr, A gal
			{positions: map[string]string{"bot": "fleet", "rum": "fleet", "ukr": "army", "gal": "army"},
				entries: []openingEntry{
					{name: "Ukrainian: swe, rum to bla", weight: 50, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mv("fleet", "rum", "bla"),
						mv("army", "ukr", "sev"),
						hld("army", "gal"),
					}},
					{name: "Ukrainian: swe, rum to bul(ec)", weight: 50, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mvC("fleet", "rum", "", "bul", "ec"),
						mv("army", "ukr", "rum"),
						hld("army", "gal"),
					}},
				}},
			// The Squid result: F bot, F bla, A stp, A ukr
			{positions: map[string]string{"bot": "fleet", "bla": "fleet", "stp": "army", "ukr": "army"},
				entries: []openingEntry{
					{name: "Squid: swe+nwy+rum", weight: 60, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mv("fleet", "bla", "rum"),
						mv("army", "stp", "nwy"),
						mv("army", "ukr", "rum"),
					}},
					{name: "Squid: swe+nwy, bla supports", weight: 40, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						sup("fleet", "bla", "ukr", "rum", "army"),
						mv("army", "stp", "nwy"),
						mv("army", "ukr", "rum"),
					}},
				}},
			// The Octopus result: F bot, F bla, A stp, A gal
			{positions: map[string]string{"bot": "fleet", "bla": "fleet", "stp": "army", "gal": "army"},
				entries: []openingEntry{
					{name: "Octopus: swe+nwy+rum", weight: 60, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mv("fleet", "bla", "rum"),
						mv("army", "stp", "nwy"),
						mv("army", "gal", "rum"),
					}},
					{name: "Octopus: swe+nwy, gal holds", weight: 40, orders: []OrderInput{
						mv("fleet", "bot", "swe"),
						mv("fleet", "bla", "rum"),
						mv("army", "stp", "nwy"),
						hld("army", "gal"),
					}},
				}},
		}
	case diplomacy.Turkey:
		return []fallCondition{
			// Byzantine result: A bul, A con, F bla
			{positions: map[string]string{"bul": "army", "con": "army", "bla": "fleet"},
				entries: []openingEntry{
					{name: "Byzantine: gre+rum", weight: 40, orders: []OrderInput{
						mv("army", "bul", "gre"),
						mv("army", "con", "bul"),
						sup("fleet", "bla", "bul", "rum", "army"),
					}},
					{name: "Byzantine: ser, bla to rum", weight: 30, orders: []OrderInput{
						mv("army", "bul", "ser"),
						mv("army", "con", "bul"),
						mv("fleet", "bla", "rum"),
					}},
					{name: "Byzantine: rum direct", weight: 30, orders: []OrderInput{
						mv("army", "bul", "rum"),
						mv("army", "con", "bul"),
						sup("fleet", "bla", "bul", "rum", "army"),
					}},
				}},
			// Armenian Attack result: A bul, A arm, F bla
			{positions: map[string]string{"bul": "army", "arm": "army", "bla": "fleet"},
				entries: []openingEntry{
					{name: "Armenian: bul to gre, arm to sev", weight: 50, orders: []OrderInput{
						mv("army", "bul", "gre"),
						mv("army", "arm", "sev"),
						sup("fleet", "bla", "arm", "sev", "army"),
					}},
					{name: "Armenian: bul to rum, arm holds", weight: 50, orders: []OrderInput{
						mv("army", "bul", "rum"),
						hld("army", "arm"),
						sup("fleet", "bla", "bul", "rum", "army"),
					}},
				}},
			// Anti-Lepanto result: A bul, A arm, F con
			{positions: map[string]string{"bul": "army", "arm": "army", "con": "fleet"},
				entries: []openingEntry{
					{name: "Anti-Lepanto: con to aeg, bul to gre", weight: 50, orders: []OrderInput{
						mv("army", "bul", "gre"),
						mv("army", "arm", "smy"),
						mv("fleet", "con", "aeg"),
					}},
					{name: "Anti-Lepanto: con to bla, arm to sev", weight: 50, orders: []OrderInput{
						mv("army", "bul", "gre"),
						mv("army", "arm", "sev"),
						mv("fleet", "con", "bla"),
					}},
				}},
			// Boston Strangler result: A bul, A con, F ank
			{positions: map[string]string{"bul": "army", "con": "army", "ank": "fleet"},
				entries: []openingEntry{
					{name: "Strangler: bul to gre, ank to bla", weight: 50, orders: []OrderInput{
						mv("army", "bul", "gre"),
						mv("army", "con", "bul"),
						mv("fleet", "ank", "bla"),
					}},
					{name: "Strangler: bul to ser, ank to con", weight: 50, orders: []OrderInput{
						mv("army", "bul", "ser"),
						mv("army", "con", "bul"),
						mv("fleet", "ank", "con"),
					}},
				}},
		}
	}
	return nil
}

// LookupOpening returns a validated set of opening book orders for the given
// power and game state, or nil if no opening matches. Only applies in 1901.
func LookupOpening(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	if gs.Year != 1901 || gs.Phase != diplomacy.PhaseMovement {
		return nil
	}

	if gs.Season == diplomacy.Spring {
		entries := springOpenings(power)
		if len(entries) == 0 {
			return nil
		}
		// Verify units are in expected starting positions
		units := gs.UnitsOf(power)
		expected := initialUnitPositions(power)
		if len(units) != len(expected) {
			return nil
		}
		actual := unitKey(gs, power)
		if !positionsMatch(expected, actual) {
			return nil
		}
		entry := weightedSelect(entries)
		if entry == nil {
			return nil
		}
		return validateOrders(entry.orders, gs, power, m)
	}

	if gs.Season == diplomacy.Fall {
		conditions := fallOpenings(power)
		if len(conditions) == 0 {
			return nil
		}
		actual := unitKey(gs, power)
		// Sort by number of positions descending for most-specific match first
		sort.Slice(conditions, func(i, j int) bool {
			return len(conditions[i].positions) > len(conditions[j].positions)
		})
		for _, cond := range conditions {
			if positionsMatch(cond.positions, actual) {
				entry := weightedSelect(cond.entries)
				if entry == nil {
					continue
				}
				result := validateOrders(entry.orders, gs, power, m)
				if result != nil {
					return result
				}
			}
		}
	}

	return nil
}

// initialUnitPositions returns the expected starting unit positions for a power.
func initialUnitPositions(power diplomacy.Power) map[string]string {
	switch power {
	case diplomacy.England:
		return map[string]string{"lon": "fleet", "edi": "fleet", "lvp": "army"}
	case diplomacy.France:
		return map[string]string{"bre": "fleet", "par": "army", "mar": "army"}
	case diplomacy.Germany:
		return map[string]string{"kie": "fleet", "ber": "army", "mun": "army"}
	case diplomacy.Italy:
		return map[string]string{"nap": "fleet", "rom": "army", "ven": "army"}
	case diplomacy.Austria:
		return map[string]string{"tri": "fleet", "vie": "army", "bud": "army"}
	case diplomacy.Russia:
		return map[string]string{"stp": "fleet", "sev": "fleet", "mos": "army", "war": "army"}
	case diplomacy.Turkey:
		return map[string]string{"ank": "fleet", "con": "army", "smy": "army"}
	}
	return nil
}
