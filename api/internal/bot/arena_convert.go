package bot

import (
	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// Order conversion helpers (adapted from service layer, no service dependency)

func inputToEngineOrder(in OrderInput, power diplomacy.Power) diplomacy.Order {
	return diplomacy.Order{
		UnitType:    parseUnitType(in.UnitType),
		Power:       power,
		Location:    in.Location,
		Coast:       diplomacy.Coast(in.Coast),
		Type:        parseOrderType(in.OrderType),
		Target:      in.Target,
		TargetCoast: diplomacy.Coast(in.TargetCoast),
		AuxLoc:      in.AuxLoc,
		AuxTarget:   in.AuxTarget,
		AuxUnitType: parseUnitType(in.AuxUnitType),
	}
}

func inputToRetreatOrder(in OrderInput, power diplomacy.Power) diplomacy.RetreatOrder {
	rt := diplomacy.RetreatDisband
	if in.OrderType == "retreat_move" {
		rt = diplomacy.RetreatMove
	}
	return diplomacy.RetreatOrder{
		UnitType:    parseUnitType(in.UnitType),
		Power:       power,
		Location:    in.Location,
		Coast:       diplomacy.Coast(in.Coast),
		Type:        rt,
		Target:      in.Target,
		TargetCoast: diplomacy.Coast(in.TargetCoast),
	}
}

func inputToBuildOrder(in OrderInput, power diplomacy.Power) diplomacy.BuildOrder {
	bt := diplomacy.BuildUnit
	if in.OrderType == "disband" {
		bt = diplomacy.DisbandUnit
	}
	return diplomacy.BuildOrder{
		Power:    power,
		Type:     bt,
		UnitType: parseUnitType(in.UnitType),
		Location: in.Location,
		Coast:    diplomacy.Coast(in.Coast),
	}
}

func parseUnitType(s string) diplomacy.UnitType {
	if s == "fleet" {
		return diplomacy.Fleet
	}
	return diplomacy.Army
}

func parseOrderType(s string) diplomacy.OrderType {
	switch s {
	case "move":
		return diplomacy.OrderMove
	case "support":
		return diplomacy.OrderSupport
	case "convoy":
		return diplomacy.OrderConvoy
	default:
		return diplomacy.OrderHold
	}
}

// Model conversion helpers

func resolvedOrdersToModel(phaseID string, results []diplomacy.ResolvedOrder) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  r.Order.UnitType.String(),
			Location:  r.Order.Location,
			OrderType: orderTypeStr(r.Order.Type),
			Target:    r.Order.Target,
			AuxLoc:    r.Order.AuxLoc,
			AuxTarget: r.Order.AuxTarget,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func retreatResultsToModel(phaseID string, results []diplomacy.RetreatResult) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orderType := "retreat_move"
		if r.Order.Type == diplomacy.RetreatDisband {
			orderType = "retreat_disband"
		}
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  r.Order.UnitType.String(),
			Location:  r.Order.Location,
			OrderType: orderType,
			Target:    r.Order.Target,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func buildResultsToModel(phaseID string, results []diplomacy.BuildResult) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orderType := "build"
		if r.Order.Type == diplomacy.DisbandUnit {
			orderType = "disband"
		}
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  r.Order.UnitType.String(),
			Location:  r.Order.Location,
			OrderType: orderType,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func orderTypeStr(ot diplomacy.OrderType) string {
	switch ot {
	case diplomacy.OrderMove:
		return "move"
	case diplomacy.OrderSupport:
		return "support"
	case diplomacy.OrderConvoy:
		return "convoy"
	default:
		return "hold"
	}
}

func orderResultStr(r diplomacy.OrderResult) string {
	switch r {
	case diplomacy.ResultSucceeded:
		return "succeeds"
	case diplomacy.ResultFailed:
		return "fails"
	case diplomacy.ResultDislodged:
		return "dislodged"
	case diplomacy.ResultBounced:
		return "bounced"
	case diplomacy.ResultCut:
		return "cut"
	case diplomacy.ResultVoid:
		return "void"
	default:
		return "unknown"
	}
}

// ParsePowerConfig parses a power configuration string like "france=hard,*=easy".
func ParsePowerConfig(s string) map[diplomacy.Power]string {
	cfg := make(map[diplomacy.Power]string)
	if s == "" {
		return cfg
	}

	defaultDiff := "easy"
	parts := splitConfig(s)

	for _, part := range parts {
		if idx := indexOf(part, '='); idx >= 0 {
			key := part[:idx]
			val := part[idx+1:]
			if key == "*" {
				defaultDiff = val
			} else {
				cfg[diplomacy.Power(key)] = val
			}
		}
	}

	// Fill in defaults
	for _, p := range diplomacy.AllPowers() {
		if _, ok := cfg[p]; !ok {
			cfg[p] = defaultDiff
		}
	}

	return cfg
}

// ParseMatchup sets all 7 powers to the given difficulty string.
func ParseMatchup(s string) map[diplomacy.Power]string {
	cfg := make(map[diplomacy.Power]string)
	for _, p := range diplomacy.AllPowers() {
		cfg[p] = s
	}
	return cfg
}

func splitConfig(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
