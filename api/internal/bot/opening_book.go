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

// BookCondition holds the matching criteria for an entry. Multiple fields are AND-ed.
type BookCondition struct {
	Positions  map[string]string `json:"positions,omitempty"`
	OwnedSCs   []string          `json:"owned_scs,omitempty"`
	SCCountMin int               `json:"sc_count_min,omitempty"`
	SCCountMax int               `json:"sc_count_max,omitempty"`
	Theaters   map[string]int    `json:"theaters,omitempty"`
	FleetCount int               `json:"fleet_count,omitempty"`
	ArmyCount  int               `json:"army_count,omitempty"`
}

// BookOption is a named, weighted set of orders to choose from.
type BookOption struct {
	Name   string       `json:"name"`
	Weight float64      `json:"weight"`
	Orders []OrderInput `json:"orders"`
}

// conditionSpecificity returns the number of non-zero fields in a condition,
// used to rank matches from most to least specific.
func conditionSpecificity(c *BookCondition) int {
	score := 0
	if len(c.Positions) > 0 {
		score += len(c.Positions) * 2
	}
	if len(c.OwnedSCs) > 0 {
		score += len(c.OwnedSCs)
	}
	if c.SCCountMin > 0 || c.SCCountMax > 0 {
		score++
	}
	if len(c.Theaters) > 0 {
		score += len(c.Theaters)
	}
	if c.FleetCount > 0 {
		score++
	}
	if c.ArmyCount > 0 {
		score++
	}
	return score
}

// matchCondition returns true if all non-zero fields in the condition match the game state.
func matchCondition(cond *BookCondition, gs *diplomacy.GameState, power diplomacy.Power) bool {
	if len(cond.Positions) > 0 {
		actual := unitKey(gs, power)
		if !positionsMatch(cond.Positions, actual) {
			return false
		}
	}

	if len(cond.OwnedSCs) > 0 {
		for _, sc := range cond.OwnedSCs {
			if gs.SupplyCenters[sc] != power {
				return false
			}
		}
	}

	if cond.SCCountMin > 0 || cond.SCCountMax > 0 {
		count := gs.SupplyCenterCount(power)
		if cond.SCCountMin > 0 && count < cond.SCCountMin {
			return false
		}
		if cond.SCCountMax > 0 && count > cond.SCCountMax {
			return false
		}
	}

	if len(cond.Theaters) > 0 {
		presence := TheaterPresence(gs, power)
		for theater, expected := range cond.Theaters {
			if presence[Theater(theater)] != expected {
				return false
			}
		}
	}

	if cond.FleetCount > 0 || cond.ArmyCount > 0 {
		fleets, armies := 0, 0
		for _, u := range gs.UnitsOf(power) {
			if u.Type == diplomacy.Fleet {
				fleets++
			} else {
				armies++
			}
		}
		if cond.FleetCount > 0 && fleets != cond.FleetCount {
			return false
		}
		if cond.ArmyCount > 0 && armies != cond.ArmyCount {
			return false
		}
	}

	return true
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

	// Evaluate conditions and collect matching entries with specificity.
	type match struct {
		entry       *BookEntry
		specificity int
	}
	var matches []match
	for i := range candidates {
		if matchCondition(&candidates[i].Condition, gs, power) {
			matches = append(matches, match{
				entry:       &candidates[i],
				specificity: conditionSpecificity(&candidates[i].Condition),
			})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by specificity descending; pick the highest tier.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].specificity > matches[j].specificity
	})

	// Collect all entries at the top specificity level.
	topSpec := matches[0].specificity
	var topOptions []BookOption
	for _, mt := range matches {
		if mt.specificity < topSpec {
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
