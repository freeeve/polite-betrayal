package bot

import (
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// Theater represents a strategic region of the Diplomacy map.
type Theater string

const (
	TheaterWest    Theater = "west"    // France, Iberia, Low Countries, British Isles
	TheaterScan    Theater = "scan"    // Scandinavia, North Sea area
	TheaterMed     Theater = "med"     // Mediterranean, North Africa, Italy
	TheaterBalkans Theater = "balkans" // Balkans, Turkey, Black Sea
	TheaterEast    Theater = "east"    // Russia, Eastern Europe
	TheaterCenter  Theater = "center"  // Central Europe (Germany, Austria core)
)

var provinceTheaters = map[string]Theater{
	// West: France, Iberia, Low Countries, British Isles
	"bre": TheaterWest, "par": TheaterWest, "mar": TheaterWest,
	"gas": TheaterWest, "bur": TheaterWest, "pic": TheaterWest,
	"spa": TheaterWest, "por": TheaterWest, "bel": TheaterWest,
	"mao": TheaterWest, "eng": TheaterWest, "iri": TheaterWest,
	"naf": TheaterWest, "nao": TheaterWest,
	"lon": TheaterWest, "lvp": TheaterWest, "wal": TheaterWest,
	"yor": TheaterWest, "edi": TheaterWest, "cly": TheaterWest,

	// Scan: Scandinavia, North Sea area
	"nwy": TheaterScan, "swe": TheaterScan, "den": TheaterScan,
	"ska": TheaterScan, "nth": TheaterScan, "nrg": TheaterScan,
	"bar": TheaterScan, "fin": TheaterScan, "stp": TheaterScan,

	// Med: Mediterranean, Italy
	"tun": TheaterMed, "tys": TheaterMed, "wes": TheaterMed,
	"gol": TheaterMed, "ion": TheaterMed, "aeg": TheaterMed,
	"eas": TheaterMed, "rom": TheaterMed, "nap": TheaterMed,
	"apu": TheaterMed, "tus": TheaterMed, "pie": TheaterMed,
	"ven": TheaterMed,

	// Balkans: Balkans, Turkey, Black Sea
	"gre": TheaterBalkans, "ser": TheaterBalkans, "bul": TheaterBalkans,
	"rum": TheaterBalkans, "alb": TheaterBalkans, "con": TheaterBalkans,
	"smy": TheaterBalkans, "ank": TheaterBalkans, "arm": TheaterBalkans,
	"syr": TheaterBalkans, "bla": TheaterBalkans, "adr": TheaterBalkans,

	// East: Russia, Eastern Europe
	"mos": TheaterEast, "war": TheaterEast, "ukr": TheaterEast,
	"sev": TheaterEast, "lvn": TheaterEast, "pru": TheaterEast,
	"sil": TheaterEast, "gal": TheaterEast, "bot": TheaterEast,

	// Center: Central Europe (Germany, Austria core)
	"mun": TheaterCenter, "ber": TheaterCenter, "kie": TheaterCenter,
	"ruh": TheaterCenter, "hol": TheaterCenter, "tyr": TheaterCenter,
	"boh": TheaterCenter, "vie": TheaterCenter, "tri": TheaterCenter,
	"bud": TheaterCenter, "hel": TheaterCenter, "bal": TheaterCenter,
}

// ProvinceTheater returns the theater a province belongs to.
// Returns empty string if the province is unknown.
func ProvinceTheater(province string) Theater {
	return provinceTheaters[province]
}

// TheaterPresence counts units per theater for a given power.
func TheaterPresence(gs *diplomacy.GameState, power diplomacy.Power) map[Theater]int {
	counts := make(map[Theater]int)
	for _, u := range gs.UnitsOf(power) {
		t := ProvinceTheater(u.Province)
		if t != "" {
			counts[t]++
		}
	}
	return counts
}
