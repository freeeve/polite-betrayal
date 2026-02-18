package bot

import (
	_ "embed"
	"encoding/json"
	"log"
	"sort"
	"sync"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

//go:embed opening_book.json
var openingBookJSON []byte

var bookData *OpeningBook
var bookOnce sync.Once

// getBook lazily loads and caches the embedded opening book JSON.
func getBook() *OpeningBook {
	bookOnce.Do(func() {
		bookData = &OpeningBook{}
		if err := json.Unmarshal(openingBookJSON, bookData); err != nil {
			log.Printf("opening book: failed to parse JSON: %v", err)
			bookData = &OpeningBook{}
		}
	})
	return bookData
}

// OpeningBook holds the full set of opening book entries.
type OpeningBook struct {
	Entries []BookEntry `json:"entries"`
}

// BookEntry represents one conditional entry in the opening book.
type BookEntry struct {
	Power     string        `json:"power"`
	Year      int           `json:"year"`
	Season    string        `json:"season"`
	Phase     string        `json:"phase"`
	Condition BookCondition `json:"condition"`
	Options   []BookOption  `json:"options"`
}

// BookCondition holds the matching criteria for an entry.
// In "exact" mode, all non-zero fields are AND-ed (strict match).
// In scoring modes, each matching field contributes to a match score.
type BookCondition struct {
	// Tier 1: exact positions (1901)
	Positions map[string]string `json:"positions,omitempty"`

	// Tier 2: SC-based
	OwnedSCs   []string `json:"owned_scs,omitempty"`
	SCCountMin int      `json:"sc_count_min,omitempty"`
	SCCountMax int      `json:"sc_count_max,omitempty"`

	// Tier 3: neighbor behavior (key feature for 1902+)
	NeighborStance map[string]string `json:"neighbor_stance,omitempty"` // power -> "aggressive"/"neutral"/"retreating"
	BorderPressure int               `json:"border_pressure,omitempty"` // enemy units adjacent to our SCs

	// Tier 4: theater/composition
	Theaters   map[string]int `json:"theaters,omitempty"`
	FleetCount int            `json:"fleet_count,omitempty"`
	ArmyCount  int            `json:"army_count,omitempty"`
}

// BookOption is a named, weighted set of orders to choose from.
type BookOption struct {
	Name   string       `json:"name"`
	Weight float64      `json:"weight"`
	Orders []OrderInput `json:"orders"`
}

// MatchMode selects the book matching strategy.
type MatchMode string

const (
	MatchExact    MatchMode = "exact"    // strict AND of all conditions (1901 behavior)
	MatchNeighbor MatchMode = "neighbor" // primarily match on neighbor stances
	MatchSCBased  MatchMode = "sc_based" // primarily match on SC ownership
	MatchHybrid   MatchMode = "hybrid"   // weighted combination of all features
)

// BookMatchConfig holds tunable weights for the scoring system.
type BookMatchConfig struct {
	Mode              MatchMode
	MinScore          float64 // minimum score to accept a match
	PositionWeight    float64 // per-position match
	OwnedSCWeight     float64 // per-SC ownership match
	SCCountWeight     float64 // SC count in range
	NeighborWeight    float64 // per-neighbor stance match
	BorderPressWeight float64 // border pressure match
	TheaterWeight     float64 // per-theater match
	FleetArmyWeight   float64 // fleet/army count match
}

// DefaultBookConfig returns the default hybrid matching config.
func DefaultBookConfig() BookMatchConfig {
	return BookMatchConfig{
		Mode:              MatchHybrid,
		MinScore:          1.0,
		PositionWeight:    10.0,
		OwnedSCWeight:     3.0,
		SCCountWeight:     1.0,
		NeighborWeight:    5.0,
		BorderPressWeight: 2.0,
		TheaterWeight:     2.0,
		FleetArmyWeight:   1.5,
	}
}

// bookMatchMode is the active match config. Defaults to hybrid.
var bookMatchMode = DefaultBookConfig()

// SetBookMatchMode sets the match mode for opening book lookups.
func SetBookMatchMode(mode MatchMode) {
	bookMatchMode.Mode = mode
}

// SetBookMatchConfig replaces the full match config.
func SetBookMatchConfig(cfg BookMatchConfig) {
	bookMatchMode = cfg
}

// GetBookMatchConfig returns the current match config.
func GetBookMatchConfig() BookMatchConfig {
	return bookMatchMode
}

// borderPressure counts enemy units adjacent to the given power's SCs.
func borderPressure(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) int {
	ourSCs := make(map[string]bool)
	for prov, owner := range gs.SupplyCenters {
		if owner == power {
			ourSCs[prov] = true
		}
	}

	borderZone := make(map[string]bool)
	for sc := range ourSCs {
		for _, adj := range m.Adjacencies[sc] {
			if !ourSCs[adj.To] {
				borderZone[adj.To] = true
			}
		}
	}

	count := 0
	for _, u := range gs.Units {
		if u.Power != power && u.Power != diplomacy.Neutral && borderZone[u.Province] {
			count++
		}
	}
	return count
}

// scoreCondition computes a match score for a condition against the game state.
// Returns (score, maxPossible) so callers can check quality.
// A score of -1 means a hard mismatch (positions failed in exact mode).
func scoreCondition(cond *BookCondition, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, cfg *BookMatchConfig) (float64, float64) {
	score := 0.0
	maxScore := 0.0

	// Tier 1: exact positions
	if len(cond.Positions) > 0 {
		actual := unitKey(gs, power)
		matched := 0
		for prov, utype := range cond.Positions {
			if actual[prov] == utype {
				matched++
			}
		}
		posMax := float64(len(cond.Positions)) * cfg.PositionWeight
		maxScore += posMax

		if cfg.Mode == MatchExact {
			// In exact mode, positions must fully match
			if matched != len(cond.Positions) {
				return -1, maxScore
			}
			score += posMax
		} else {
			score += float64(matched) * cfg.PositionWeight
		}
	}

	// Tier 2: SC ownership
	if len(cond.OwnedSCs) > 0 {
		matched := 0
		for _, sc := range cond.OwnedSCs {
			if gs.SupplyCenters[sc] == power {
				matched++
			}
		}
		scMax := float64(len(cond.OwnedSCs)) * cfg.OwnedSCWeight
		maxScore += scMax

		if cfg.Mode == MatchExact || cfg.Mode == MatchSCBased {
			if matched != len(cond.OwnedSCs) {
				return -1, maxScore
			}
			score += scMax
		} else {
			score += float64(matched) * cfg.OwnedSCWeight
		}
	}

	// Tier 2: SC count range
	if cond.SCCountMin > 0 || cond.SCCountMax > 0 {
		count := gs.SupplyCenterCount(power)
		maxScore += cfg.SCCountWeight
		inRange := true
		if cond.SCCountMin > 0 && count < cond.SCCountMin {
			inRange = false
		}
		if cond.SCCountMax > 0 && count > cond.SCCountMax {
			inRange = false
		}

		if cfg.Mode == MatchExact {
			if !inRange {
				return -1, maxScore
			}
			score += cfg.SCCountWeight
		} else if inRange {
			score += cfg.SCCountWeight
		}
	}

	// Tier 3: neighbor stances
	if len(cond.NeighborStance) > 0 {
		stances := ClassifyNeighborStances(gs, power, m)
		matched := 0
		for powerStr, expectedStance := range cond.NeighborStance {
			neighborPower := parsePowerStr(powerStr)
			actual, ok := stances[neighborPower]
			if ok && string(actual) == expectedStance {
				matched++
			}
		}
		nMax := float64(len(cond.NeighborStance)) * cfg.NeighborWeight
		maxScore += nMax

		if cfg.Mode == MatchExact || cfg.Mode == MatchNeighbor {
			if matched != len(cond.NeighborStance) {
				return -1, maxScore
			}
			score += nMax
		} else {
			score += float64(matched) * cfg.NeighborWeight
		}
	}

	// Tier 3: border pressure
	if cond.BorderPressure > 0 {
		actual := borderPressure(gs, power, m)
		maxScore += cfg.BorderPressWeight

		// Match if actual pressure is within +/-1 of expected
		diff := actual - cond.BorderPressure
		if diff < 0 {
			diff = -diff
		}
		if diff <= 1 {
			score += cfg.BorderPressWeight
		} else if cfg.Mode == MatchExact {
			return -1, maxScore
		}
	}

	// Tier 4: theater distribution
	if len(cond.Theaters) > 0 {
		presence := TheaterPresence(gs, power)
		matched := 0
		for theater, expected := range cond.Theaters {
			if presence[Theater(theater)] == expected {
				matched++
			}
		}
		tMax := float64(len(cond.Theaters)) * cfg.TheaterWeight
		maxScore += tMax

		if cfg.Mode == MatchExact {
			if matched != len(cond.Theaters) {
				return -1, maxScore
			}
			score += tMax
		} else {
			score += float64(matched) * cfg.TheaterWeight
		}
	}

	// Tier 4: fleet/army counts
	faFields := 0
	if cond.FleetCount > 0 {
		faFields++
	}
	if cond.ArmyCount > 0 {
		faFields++
	}
	if faFields > 0 {
		fleets, armies := 0, 0
		for _, u := range gs.UnitsOf(power) {
			if u.Type == diplomacy.Fleet {
				fleets++
			} else {
				armies++
			}
		}
		faMax := float64(faFields) * cfg.FleetArmyWeight
		maxScore += faMax

		matched := 0
		if cond.FleetCount > 0 && fleets == cond.FleetCount {
			matched++
		}
		if cond.ArmyCount > 0 && armies == cond.ArmyCount {
			matched++
		}

		if cfg.Mode == MatchExact {
			if matched != faFields {
				return -1, maxScore
			}
			score += faMax
		} else {
			score += float64(matched) * cfg.FleetArmyWeight
		}
	}

	return score, maxScore
}

// unitKey builds a position fingerprint mapping province to unit type string.
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

// bookWeightedSelect picks an option from a weighted list using random selection.
func bookWeightedSelect(options []BookOption) *BookOption {
	if len(options) == 0 {
		return nil
	}
	total := 0.0
	for i := range options {
		total += options[i].Weight
	}
	r := botFloat64() * total
	cum := 0.0
	for i := range options {
		cum += options[i].Weight
		if r < cum {
			return &options[i]
		}
	}
	return &options[len(options)-1]
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

// validateBuildOrders checks that all build/disband orders are valid.
func validateBuildOrders(orders []OrderInput, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	for _, o := range orders {
		switch o.OrderType {
		case "build":
			bo := diplomacy.BuildOrder{
				Power:    power,
				Type:     diplomacy.BuildUnit,
				UnitType: parseUnitTypeStr(o.UnitType),
				Location: o.Location,
				Coast:    diplomacy.Coast(o.Coast),
			}
			if err := diplomacy.ValidateBuildOrder(bo, gs, m); err != nil {
				return nil
			}
		case "disband":
			if gs.UnitAt(o.Location) == nil {
				return nil
			}
			u := gs.UnitAt(o.Location)
			if u.Power != power {
				return nil
			}
		default:
			return nil
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

// Shorthand constructors for building OrderInputs in tests.

func mv(ut, loc, target string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "move", Target: target}
}

func mvC(ut, loc, coast, target, targetCoast string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, Coast: coast, OrderType: "move", Target: target, TargetCoast: targetCoast}
}

func hld(ut, loc string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "hold"}
}

func sup(ut, loc, auxLoc, auxTarget, auxUt string) OrderInput {
	return OrderInput{UnitType: ut, Location: loc, OrderType: "support", AuxLoc: auxLoc, AuxTarget: auxTarget, AuxUnitType: auxUt}
}

func con(loc, auxLoc, auxTarget string) OrderInput {
	return OrderInput{UnitType: "fleet", Location: loc, OrderType: "convoy", AuxLoc: auxLoc, AuxTarget: auxTarget, AuxUnitType: "army"}
}

// parsePhaseStr converts a phase string from JSON to the engine PhaseType.
func parsePhaseStr(s string) diplomacy.PhaseType {
	switch s {
	case "movement":
		return diplomacy.PhaseMovement
	case "retreat":
		return diplomacy.PhaseRetreat
	case "build":
		return diplomacy.PhaseBuild
	default:
		return diplomacy.PhaseMovement
	}
}

// parseSeasonStr converts a season string from JSON to the engine Season.
func parseSeasonStr(s string) diplomacy.Season {
	if s == "fall" {
		return diplomacy.Fall
	}
	return diplomacy.Spring
}

// parsePowerStr converts a power string from JSON to the engine Power.
func parsePowerStr(s string) diplomacy.Power {
	switch s {
	case "austria":
		return diplomacy.Austria
	case "england":
		return diplomacy.England
	case "france":
		return diplomacy.France
	case "germany":
		return diplomacy.Germany
	case "italy":
		return diplomacy.Italy
	case "russia":
		return diplomacy.Russia
	case "turkey":
		return diplomacy.Turkey
	default:
		return diplomacy.Power(s)
	}
}

// LookupOpening returns a validated set of opening book orders for the given
// power and game state, or nil if no opening matches.
func LookupOpening(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	book := getBook()
	cfg := bookMatchMode

	// Filter entries matching current (year, season, phase, power).
	var candidates []BookEntry
	for _, e := range book.Entries {
		if e.Year != gs.Year {
			continue
		}
		if parseSeasonStr(e.Season) != gs.Season {
			continue
		}
		if parsePhaseStr(e.Phase) != gs.Phase {
			continue
		}
		if parsePowerStr(e.Power) != power {
			continue
		}
		candidates = append(candidates, e)
	}

	if len(candidates) == 0 {
		return nil
	}

	// Score each candidate's condition.
	type scored struct {
		entry *BookEntry
		score float64
	}
	var matches []scored
	for i := range candidates {
		s, _ := scoreCondition(&candidates[i].Condition, gs, power, m, &cfg)
		if s < 0 {
			continue // hard mismatch
		}
		if s < cfg.MinScore {
			continue // below threshold
		}
		matches = append(matches, scored{entry: &candidates[i], score: s})
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by score descending.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	// Collect all entries at the top score level (within a small epsilon).
	topScore := matches[0].score
	var topOptions []BookOption
	for _, mt := range matches {
		if topScore-mt.score > 0.01 {
			break
		}
		topOptions = append(topOptions, mt.entry.Options...)
	}

	// Weighted select from the combined top-tier options.
	selected := bookWeightedSelect(topOptions)
	if selected == nil {
		return nil
	}

	// Validate based on phase type.
	if gs.Phase == diplomacy.PhaseBuild {
		return validateBuildOrders(selected.Orders, gs, power, m)
	}
	return validateOrders(selected.Orders, gs, power, m)
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
