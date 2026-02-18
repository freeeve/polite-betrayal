package diplomacy

import "fmt"

// OrderType represents the type of order a unit can be given.
type OrderType int

const (
	OrderHold    OrderType = iota // Unit holds position
	OrderMove                     // Unit moves to adjacent province
	OrderSupport                  // Unit supports another unit's hold or move
	OrderConvoy                   // Fleet convoys army across sea
)

func (o OrderType) String() string {
	switch o {
	case OrderHold:
		return "hold"
	case OrderMove:
		return "move"
	case OrderSupport:
		return "support"
	case OrderConvoy:
		return "convoy"
	default:
		return "unknown"
	}
}

// Order represents a single order issued to a unit.
type Order struct {
	// Unit being ordered
	UnitType UnitType
	Power    Power
	Location string
	Coast    Coast // Coast of the unit being ordered (for fleets on split coasts)

	// Order details
	Type OrderType

	// Target province (for move, support, convoy)
	Target      string
	TargetCoast Coast // Coast of target (for fleet moves to split-coast provinces)

	// Aux fields for support and convoy:
	// For support: the province the supported unit is in
	// For convoy: the province the convoyed army is in
	AuxLoc string
	// For support: the destination the supported unit is moving to (empty if support-hold)
	// For convoy: the destination the convoyed army is moving to
	AuxTarget string
	// For support: the type of the supported unit
	AuxUnitType UnitType
}

// OrderResult describes the outcome of adjudicating an order.
type OrderResult int

const (
	ResultSucceeded OrderResult = iota // Order carried out
	ResultFailed                       // Move bounced or support failed to enable
	ResultDislodged                    // Unit was dislodged
	ResultBounced                      // Move bounced
	ResultCut                          // Support was cut
	ResultVoid                         // Order was invalid, treated as hold
)

func (r OrderResult) String() string {
	switch r {
	case ResultSucceeded:
		return "succeeded"
	case ResultFailed:
		return "failed"
	case ResultDislodged:
		return "dislodged"
	case ResultBounced:
		return "bounced"
	case ResultCut:
		return "cut"
	case ResultVoid:
		return "void"
	default:
		return "unknown"
	}
}

// ResolvedOrder pairs an order with its adjudication result.
type ResolvedOrder struct {
	Order  Order
	Result OrderResult
}

// Describe returns a human-readable description of the order.
func (o *Order) Describe() string {
	unitStr := "A"
	if o.UnitType == Fleet {
		unitStr = "F"
	}
	loc := o.Location
	if o.Coast != NoCoast {
		loc += "/" + string(o.Coast)
	}

	switch o.Type {
	case OrderHold:
		return fmt.Sprintf("%s %s Hold", unitStr, loc)
	case OrderMove:
		target := o.Target
		if o.TargetCoast != NoCoast {
			target += "/" + string(o.TargetCoast)
		}
		return fmt.Sprintf("%s %s -> %s", unitStr, loc, target)
	case OrderSupport:
		auxUnit := "A"
		if o.AuxUnitType == Fleet {
			auxUnit = "F"
		}
		if o.AuxTarget == "" {
			return fmt.Sprintf("%s %s S %s %s Hold", unitStr, loc, auxUnit, o.AuxLoc)
		}
		return fmt.Sprintf("%s %s S %s %s -> %s", unitStr, loc, auxUnit, o.AuxLoc, o.AuxTarget)
	case OrderConvoy:
		return fmt.Sprintf("%s %s C A %s -> %s", unitStr, loc, o.AuxLoc, o.AuxTarget)
	default:
		return fmt.Sprintf("%s %s ???", unitStr, loc)
	}
}
