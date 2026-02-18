package diplomacy

// Power represents one of the seven great powers.
type Power string

const (
	Austria Power = "austria"
	England Power = "england"
	France  Power = "france"
	Germany Power = "germany"
	Italy   Power = "italy"
	Russia  Power = "russia"
	Turkey  Power = "turkey"
	Neutral Power = ""
)

// AllPowers returns the seven great powers in standard order.
func AllPowers() []Power {
	return []Power{Austria, England, France, Germany, Italy, Russia, Turkey}
}

// UnitType represents the type of a military unit.
type UnitType int

const (
	Army UnitType = iota
	Fleet
)

func (u UnitType) String() string {
	if u == Army {
		return "army"
	}
	return "fleet"
}

// Unit represents a single military unit on the board.
type Unit struct {
	Type     UnitType
	Power    Power
	Province string
	Coast    Coast // Only relevant for fleets on split-coast provinces
}

// UnitPosition identifies a unit's location including coast.
type UnitPosition struct {
	Province string
	Coast    Coast
}
