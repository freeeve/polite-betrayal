package diplomacy

import (
	"fmt"
	"strings"
)

// DSONOrderType enumerates the kinds of orders representable in DSON.
type DSONOrderType int

const (
	DSONHold        DSONOrderType = iota // A vie H
	DSONMove                             // A bud - rum
	DSONSupportHold                      // A tyr S A vie H
	DSONSupportMove                      // A gal S A bud - rum
	DSONConvoy                           // F mao C A bre - spa
	DSONRetreat                          // A vie R boh
	DSONDisband                          // F tri D  (retreat or build phase)
	DSONBuild                            // A vie B
	DSONWaive                            // W
)

// DSONOrder is a phase-agnostic order representation matching the DSON wire format.
// It can represent movement, retreat, and build phase orders.
type DSONOrder struct {
	Type DSONOrderType

	// Unit being ordered (all types except DSONWaive).
	UnitType UnitType
	Location string
	Coast    Coast

	// Target location (DSONMove, DSONRetreat, DSONBuild coasts).
	Target      string
	TargetCoast Coast

	// Supported/convoyed unit (DSONSupportHold, DSONSupportMove, DSONConvoy).
	AuxUnitType UnitType
	AuxLocation string
	AuxCoast    Coast

	// Destination of the supported/convoyed move (DSONSupportMove, DSONConvoy).
	AuxTarget      string
	AuxTargetCoast Coast
}

// FormatDSON serializes a slice of DSONOrders to a DSON string.
// Multiple orders are separated by " ; ".
func FormatDSON(orders []DSONOrder) string {
	parts := make([]string, 0, len(orders))
	for _, o := range orders {
		parts = append(parts, formatSingleDSON(o))
	}
	return strings.Join(parts, " ; ")
}

// formatSingleDSON formats one DSONOrder to its DSON text representation.
func formatSingleDSON(o DSONOrder) string {
	if o.Type == DSONWaive {
		return "W"
	}

	var b strings.Builder
	b.Grow(32)

	writeUnit(&b, o.UnitType, o.Location, o.Coast)

	switch o.Type {
	case DSONHold:
		b.WriteString(" H")

	case DSONMove:
		b.WriteString(" - ")
		writeDSONLocation(&b, o.Target, o.TargetCoast)

	case DSONSupportHold:
		b.WriteString(" S ")
		writeUnit(&b, o.AuxUnitType, o.AuxLocation, o.AuxCoast)
		b.WriteString(" H")

	case DSONSupportMove:
		b.WriteString(" S ")
		writeUnit(&b, o.AuxUnitType, o.AuxLocation, o.AuxCoast)
		b.WriteString(" - ")
		writeDSONLocation(&b, o.AuxTarget, o.AuxTargetCoast)

	case DSONConvoy:
		b.WriteString(" C A ")
		writeDSONLocation(&b, o.AuxLocation, o.AuxCoast)
		b.WriteString(" - ")
		writeDSONLocation(&b, o.AuxTarget, o.AuxTargetCoast)

	case DSONRetreat:
		b.WriteString(" R ")
		writeDSONLocation(&b, o.Target, o.TargetCoast)

	case DSONDisband:
		b.WriteString(" D")

	case DSONBuild:
		b.WriteString(" B")
	}

	return b.String()
}

// writeUnit writes "A vie" or "F stp/nc" to the builder.
func writeUnit(b *strings.Builder, ut UnitType, province string, coast Coast) {
	if ut == Army {
		b.WriteByte('A')
	} else {
		b.WriteByte('F')
	}
	b.WriteByte(' ')
	writeDSONLocation(b, province, coast)
}

// writeDSONLocation writes a DSON location like "vie" or "stp/nc".
func writeDSONLocation(b *strings.Builder, province string, coast Coast) {
	b.WriteString(province)
	if coast != NoCoast {
		b.WriteByte('/')
		b.WriteString(string(coast))
	}
}

// ParseDSON parses a DSON string (possibly with " ; " separators) into DSONOrders.
func ParseDSON(s string) ([]DSONOrder, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, " ; ")
	orders := make([]DSONOrder, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		o, err := parseSingleDSON(part)
		if err != nil {
			return nil, fmt.Errorf("dson: parsing %q: %w", part, err)
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// parseSingleDSON parses one DSON order string.
func parseSingleDSON(s string) (DSONOrder, error) {
	if s == "W" {
		return DSONOrder{Type: DSONWaive}, nil
	}

	tokens := strings.Fields(s)
	if len(tokens) < 2 {
		return DSONOrder{}, fmt.Errorf("too few tokens")
	}

	unitType, err := parseDSONUnitChar(tokens[0])
	if err != nil {
		return DSONOrder{}, err
	}
	prov, coast, err := parseDSONLocation(tokens[1])
	if err != nil {
		return DSONOrder{}, fmt.Errorf("unit location: %w", err)
	}

	if len(tokens) < 3 {
		return DSONOrder{}, fmt.Errorf("missing action")
	}

	o := DSONOrder{
		UnitType: unitType,
		Location: prov,
		Coast:    coast,
	}

	action := tokens[2]
	rest := tokens[3:]

	switch action {
	case "H":
		o.Type = DSONHold
		return o, nil

	case "-":
		o.Type = DSONMove
		if len(rest) < 1 {
			return DSONOrder{}, fmt.Errorf("move missing target")
		}
		o.Target, o.TargetCoast, err = parseDSONLocation(rest[0])
		if err != nil {
			return DSONOrder{}, fmt.Errorf("move target: %w", err)
		}
		return o, nil

	case "S":
		return parseSupportOrder(o, rest)

	case "C":
		return parseConvoyOrder(o, rest)

	case "R":
		o.Type = DSONRetreat
		if len(rest) < 1 {
			return DSONOrder{}, fmt.Errorf("retreat missing target")
		}
		o.Target, o.TargetCoast, err = parseDSONLocation(rest[0])
		if err != nil {
			return DSONOrder{}, fmt.Errorf("retreat target: %w", err)
		}
		return o, nil

	case "D":
		o.Type = DSONDisband
		return o, nil

	case "B":
		o.Type = DSONBuild
		return o, nil

	default:
		return DSONOrder{}, fmt.Errorf("unknown action %q", action)
	}
}

// parseSupportOrder parses the remainder of a support order after "S".
// Formats: "A vie H" (support hold) or "A bud - rum" (support move).
func parseSupportOrder(o DSONOrder, tokens []string) (DSONOrder, error) {
	if len(tokens) < 3 {
		return DSONOrder{}, fmt.Errorf("support order too short")
	}

	auxUnit, err := parseDSONUnitChar(tokens[0])
	if err != nil {
		return DSONOrder{}, fmt.Errorf("supported unit: %w", err)
	}
	auxLoc, auxCoast, err := parseDSONLocation(tokens[1])
	if err != nil {
		return DSONOrder{}, fmt.Errorf("supported unit location: %w", err)
	}

	o.AuxUnitType = auxUnit
	o.AuxLocation = auxLoc
	o.AuxCoast = auxCoast

	switch tokens[2] {
	case "H":
		o.Type = DSONSupportHold
		return o, nil
	case "-":
		o.Type = DSONSupportMove
		if len(tokens) < 4 {
			return DSONOrder{}, fmt.Errorf("support move missing destination")
		}
		o.AuxTarget, o.AuxTargetCoast, err = parseDSONLocation(tokens[3])
		if err != nil {
			return DSONOrder{}, fmt.Errorf("support move target: %w", err)
		}
		return o, nil
	default:
		return DSONOrder{}, fmt.Errorf("support: expected H or -, got %q", tokens[2])
	}
}

// parseConvoyOrder parses the remainder of a convoy order after "C".
// Format: "A loc - dst".
func parseConvoyOrder(o DSONOrder, tokens []string) (DSONOrder, error) {
	if len(tokens) < 4 {
		return DSONOrder{}, fmt.Errorf("convoy order too short")
	}
	if tokens[0] != "A" {
		return DSONOrder{}, fmt.Errorf("convoy: expected convoyed unit type A, got %q", tokens[0])
	}

	o.Type = DSONConvoy
	var err error
	o.AuxLocation, o.AuxCoast, err = parseDSONLocation(tokens[1])
	if err != nil {
		return DSONOrder{}, fmt.Errorf("convoy source: %w", err)
	}

	if tokens[2] != "-" {
		return DSONOrder{}, fmt.Errorf("convoy: expected '-', got %q", tokens[2])
	}

	o.AuxTarget, o.AuxTargetCoast, err = parseDSONLocation(tokens[3])
	if err != nil {
		return DSONOrder{}, fmt.Errorf("convoy target: %w", err)
	}

	o.AuxUnitType = Army
	return o, nil
}

// parseDSONUnitChar parses "A" or "F" into a UnitType.
func parseDSONUnitChar(s string) (UnitType, error) {
	switch s {
	case "A":
		return Army, nil
	case "F":
		return Fleet, nil
	default:
		return Army, fmt.Errorf("invalid unit type %q (expected A or F)", s)
	}
}

// parseDSONLocation parses "vie" or "stp/nc" into province and coast.
func parseDSONLocation(s string) (string, Coast, error) {
	parts := strings.SplitN(s, "/", 2)
	province := parts[0]
	if len(province) != 3 {
		return "", NoCoast, fmt.Errorf("invalid province %q (must be 3 lowercase letters)", province)
	}

	coast := NoCoast
	if len(parts) == 2 {
		c := Coast(parts[1])
		switch c {
		case NorthCoast, SouthCoast, EastCoast:
			coast = c
		default:
			return "", NoCoast, fmt.Errorf("invalid coast %q", parts[1])
		}
	}

	return province, coast, nil
}

// OrderToDSON converts a movement-phase Order to a DSONOrder.
func OrderToDSON(o Order) DSONOrder {
	d := DSONOrder{
		UnitType: o.UnitType,
		Location: o.Location,
		Coast:    o.Coast,
	}
	switch o.Type {
	case OrderHold:
		d.Type = DSONHold
	case OrderMove:
		d.Type = DSONMove
		d.Target = o.Target
		d.TargetCoast = o.TargetCoast
	case OrderSupport:
		if o.AuxTarget == "" {
			d.Type = DSONSupportHold
		} else {
			d.Type = DSONSupportMove
			d.AuxTarget = o.AuxTarget
		}
		d.AuxUnitType = o.AuxUnitType
		d.AuxLocation = o.AuxLoc
	case OrderConvoy:
		d.Type = DSONConvoy
		d.AuxUnitType = Army
		d.AuxLocation = o.AuxLoc
		d.AuxTarget = o.AuxTarget
	}
	return d
}

// RetreatOrderToDSON converts a RetreatOrder to a DSONOrder.
func RetreatOrderToDSON(o RetreatOrder) DSONOrder {
	d := DSONOrder{
		UnitType: o.UnitType,
		Location: o.Location,
		Coast:    o.Coast,
	}
	switch o.Type {
	case RetreatMove:
		d.Type = DSONRetreat
		d.Target = o.Target
		d.TargetCoast = o.TargetCoast
	case RetreatDisband:
		d.Type = DSONDisband
	}
	return d
}

// BuildOrderToDSON converts a BuildOrder to a DSONOrder.
func BuildOrderToDSON(o BuildOrder) DSONOrder {
	d := DSONOrder{
		UnitType: o.UnitType,
		Location: o.Location,
		Coast:    o.Coast,
	}
	switch o.Type {
	case BuildUnit:
		d.Type = DSONBuild
	case DisbandUnit:
		d.Type = DSONDisband
	}
	return d
}

// DSONToOrder converts a DSONOrder back to a movement-phase Order.
// Only valid for DSONHold, DSONMove, DSONSupportHold, DSONSupportMove, DSONConvoy.
func DSONToOrder(d DSONOrder, power Power) Order {
	o := Order{
		UnitType: d.UnitType,
		Power:    power,
		Location: d.Location,
		Coast:    d.Coast,
	}
	switch d.Type {
	case DSONHold:
		o.Type = OrderHold
	case DSONMove:
		o.Type = OrderMove
		o.Target = d.Target
		o.TargetCoast = d.TargetCoast
	case DSONSupportHold:
		o.Type = OrderSupport
		o.AuxUnitType = d.AuxUnitType
		o.AuxLoc = d.AuxLocation
	case DSONSupportMove:
		o.Type = OrderSupport
		o.AuxUnitType = d.AuxUnitType
		o.AuxLoc = d.AuxLocation
		o.AuxTarget = d.AuxTarget
	case DSONConvoy:
		o.Type = OrderConvoy
		o.AuxLoc = d.AuxLocation
		o.AuxTarget = d.AuxTarget
		o.AuxUnitType = Army
	}
	return o
}

// DSONToRetreatOrder converts a DSONOrder back to a RetreatOrder.
// Only valid for DSONRetreat and DSONDisband (retreat phase).
func DSONToRetreatOrder(d DSONOrder, power Power) RetreatOrder {
	o := RetreatOrder{
		UnitType: d.UnitType,
		Power:    power,
		Location: d.Location,
		Coast:    d.Coast,
	}
	switch d.Type {
	case DSONRetreat:
		o.Type = RetreatMove
		o.Target = d.Target
		o.TargetCoast = d.TargetCoast
	case DSONDisband:
		o.Type = RetreatDisband
	}
	return o
}

// DSONToBuildOrder converts a DSONOrder back to a BuildOrder.
// Only valid for DSONBuild, DSONDisband (build phase), and DSONWaive.
func DSONToBuildOrder(d DSONOrder, power Power) BuildOrder {
	o := BuildOrder{
		Power:    power,
		UnitType: d.UnitType,
		Location: d.Location,
		Coast:    d.Coast,
	}
	switch d.Type {
	case DSONBuild:
		o.Type = BuildUnit
	case DSONDisband:
		o.Type = DisbandUnit
	case DSONWaive:
		o.Type = WaiveBuild
	}
	return o
}
