package neural

import "github.com/freeeve/polite-betrayal/api/pkg/diplomacy"

// NumAreas is the total number of areas: 75 base provinces + 6 bicoastal variants.
const NumAreas = 81

// NumFeatures is the number of features per area in the board tensor.
const NumFeatures = 47

// NumPowers is the number of great powers.
const NumPowers = 7

// MaxUnits is the maximum number of units a power can have (17 SCs = 17 units).
const MaxUnits = 17

// Feature offset constants matching the Rust/Python encoding.
const (
	FeatUnitType      = 0  // [0:3]  unit type one-hot: army, fleet, empty
	FeatUnitOwner     = 3  // [3:11] unit owner one-hot: A,E,F,G,I,R,T,none
	FeatSCOwner       = 11 // [11:20] SC owner: A-T(0-6), neutral(7), none(8)
	FeatCanBuild      = 20
	FeatCanDisband    = 21
	FeatDislodgedType = 22 // [22:25] dislodged unit type: army, fleet, none
	FeatDislodgedOwn  = 25 // [25:33] dislodged owner: A-T(0-6), none(7)
	FeatProvinceType  = 33 // [33:36] province type: land, sea, coast
	FeatPrevUnitType  = 36 // [36:39] previous unit type: army, fleet, empty
	FeatPrevUnitOwner = 39 // [39:47] previous unit owner: A-T(0-6), none(7)
)

// Policy output constants.
const (
	NumOrderTypes  = 7   // hold, move, support, convoy, retreat, build, disband
	OrderVocabSize = 169 // 7 + 81 + 81
	SrcOffset      = NumOrderTypes
	DstOffset      = NumOrderTypes + NumAreas
)

// Order type indices matching Python ORDER_TYPES.
const (
	OrderTypeHold    = 0
	OrderTypeMove    = 1
	OrderTypeSupport = 2
	OrderTypeConvoy  = 3
	OrderTypeRetreat = 4
	OrderTypeBuild   = 5
	OrderTypeDisband = 6
)

// Bicoastal variant indices (appended after 75 base provinces).
const (
	BulEC = 75
	BulSC = 76
	SpaNC = 77
	SpaSC = 78
	StpNC = 79
	StpSC = 80
)

// AreaNames lists all 81 area IDs in index order (alphabetical base provinces,
// then bicoastal variants). This ordering matches the Rust engine and Python
// training pipeline.
var AreaNames = [NumAreas]string{
	"adr", "aeg", "alb", "ank", "apu", "arm", "bal", "bar", "bel", "ber",
	"bla", "boh", "bot", "bre", "bud", "bul", "bur", "cly", "con", "den",
	"eas", "edi", "eng", "fin", "gal", "gas", "gol", "gre", "hel", "hol",
	"ion", "iri", "kie", "lon", "lvn", "lvp", "mao", "mar", "mos", "mun",
	"naf", "nao", "nap", "nrg", "nth", "nwy", "par", "pic", "pie", "por",
	"pru", "rom", "ruh", "rum", "ser", "sev", "sil", "ska", "smy", "spa",
	"stp", "swe", "syr", "tri", "tun", "tus", "tyr", "tys", "ukr", "ven",
	"vie", "wal", "war", "wes", "yor",
	"bul_ec", "bul_sc", "spa_nc", "spa_sc", "stp_nc", "stp_sc",
}

// areaIndex maps area ID -> index (0..80). Built at init time.
var areaIndex map[string]int

func init() {
	areaIndex = make(map[string]int, NumAreas)
	for i, name := range AreaNames {
		areaIndex[name] = i
	}
}

// AreaIndex returns the area index for a province ID, or -1 if unknown.
func AreaIndex(id string) int {
	idx, ok := areaIndex[id]
	if !ok {
		return -1
	}
	return idx
}

// BicoastalIndex returns the bicoastal variant index for a split-coast
// province+coast, or -1 if not a bicoastal variant.
func BicoastalIndex(provID string, coast diplomacy.Coast) int {
	switch {
	case provID == "bul" && coast == diplomacy.EastCoast:
		return BulEC
	case provID == "bul" && coast == diplomacy.SouthCoast:
		return BulSC
	case provID == "spa" && coast == diplomacy.NorthCoast:
		return SpaNC
	case provID == "spa" && coast == diplomacy.SouthCoast:
		return SpaSC
	case provID == "stp" && coast == diplomacy.NorthCoast:
		return StpNC
	case provID == "stp" && coast == diplomacy.SouthCoast:
		return StpSC
	default:
		return -1
	}
}

// PowerIndex maps a Power to its feature index (0..6), or 7 for unknown/neutral.
func PowerIndex(p diplomacy.Power) int {
	switch p {
	case diplomacy.Austria:
		return 0
	case diplomacy.England:
		return 1
	case diplomacy.France:
		return 2
	case diplomacy.Germany:
		return 3
	case diplomacy.Italy:
		return 4
	case diplomacy.Russia:
		return 5
	case diplomacy.Turkey:
		return 6
	default:
		return 7
	}
}
