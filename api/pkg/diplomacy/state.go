package diplomacy

// Season represents a game season.
type Season string

const (
	Spring Season = "spring"
	Fall   Season = "fall"
)

// PhaseType represents the type of game phase.
type PhaseType string

const (
	PhaseMovement PhaseType = "movement"
	PhaseRetreat  PhaseType = "retreat"
	PhaseBuild    PhaseType = "build"
)

// GameStatus represents the overall game status.
type GameStatus string

const (
	StatusWaiting  GameStatus = "waiting"
	StatusActive   GameStatus = "active"
	StatusFinished GameStatus = "finished"
)

// GameState represents a complete snapshot of the board at a point in time.
type GameState struct {
	Year          int
	Season        Season
	Phase         PhaseType
	Units         []Unit
	SupplyCenters map[string]Power // province ID -> owning power
	Dislodged     []DislodgedUnit  // Units that need retreat orders (retreat phase only)
}

// DislodgedUnit is a unit that was dislodged and needs a retreat order.
type DislodgedUnit struct {
	Unit          Unit
	DislodgedFrom string // Province the unit was dislodged from (same as Unit.Province before dislodgement)
	AttackerFrom  string // Province the attacker came from (cannot retreat there)
}

// NewInitialState returns the standard Diplomacy starting position (Spring 1901 Movement).
func NewInitialState() *GameState {
	return &GameState{
		Year:          1901,
		Season:        Spring,
		Phase:         PhaseMovement,
		Units:         initialUnits(),
		SupplyCenters: initialSupplyCenters(),
	}
}

// UnitAt returns the unit at the given province, or nil if none.
func (gs *GameState) UnitAt(province string) *Unit {
	for i := range gs.Units {
		if gs.Units[i].Province == province {
			return &gs.Units[i]
		}
	}
	return nil
}

// SupplyCenterCount returns the number of supply centers owned by the given power.
func (gs *GameState) SupplyCenterCount(power Power) int {
	count := 0
	for _, owner := range gs.SupplyCenters {
		if owner == power {
			count++
		}
	}
	return count
}

// UnitCount returns the number of units belonging to the given power.
func (gs *GameState) UnitCount(power Power) int {
	count := 0
	for _, u := range gs.Units {
		if u.Power == power {
			count++
		}
	}
	return count
}

// UnitsOf returns all units belonging to the given power.
func (gs *GameState) UnitsOf(power Power) []Unit {
	var units []Unit
	for _, u := range gs.Units {
		if u.Power == power {
			units = append(units, u)
		}
	}
	return units
}

// PowerIsAlive returns true if the power still has at least one supply center or unit.
func (gs *GameState) PowerIsAlive(power Power) bool {
	return gs.SupplyCenterCount(power) > 0 || gs.UnitCount(power) > 0
}

// Clone returns a deep copy of the GameState. Mutations to the clone
// do not affect the original, which is needed for search-based bots
// that call ApplyResolution on speculative states.
func (gs *GameState) Clone() *GameState {
	c := &GameState{
		Year:   gs.Year,
		Season: gs.Season,
		Phase:  gs.Phase,
	}
	if gs.Units != nil {
		c.Units = make([]Unit, len(gs.Units))
		copy(c.Units, gs.Units)
	}
	if gs.SupplyCenters != nil {
		c.SupplyCenters = make(map[string]Power, len(gs.SupplyCenters))
		for k, v := range gs.SupplyCenters {
			c.SupplyCenters[k] = v
		}
	}
	if gs.Dislodged != nil {
		c.Dislodged = make([]DislodgedUnit, len(gs.Dislodged))
		copy(c.Dislodged, gs.Dislodged)
	}
	return c
}

// CloneInto copies gs into dst, reusing dst's allocated slices and map
// to avoid allocations. After calling, dst is a deep copy of gs.
func (gs *GameState) CloneInto(dst *GameState) {
	dst.Year = gs.Year
	dst.Season = gs.Season
	dst.Phase = gs.Phase

	if gs.Units != nil {
		if cap(dst.Units) >= len(gs.Units) {
			dst.Units = dst.Units[:len(gs.Units)]
		} else {
			dst.Units = make([]Unit, len(gs.Units))
		}
		copy(dst.Units, gs.Units)
	} else {
		dst.Units = nil
	}

	if gs.SupplyCenters != nil {
		if dst.SupplyCenters == nil {
			dst.SupplyCenters = make(map[string]Power, len(gs.SupplyCenters))
		} else {
			clear(dst.SupplyCenters)
		}
		for k, v := range gs.SupplyCenters {
			dst.SupplyCenters[k] = v
		}
	} else {
		dst.SupplyCenters = nil
	}

	if gs.Dislodged != nil {
		if cap(dst.Dislodged) >= len(gs.Dislodged) {
			dst.Dislodged = dst.Dislodged[:len(gs.Dislodged)]
		} else {
			dst.Dislodged = make([]DislodgedUnit, len(gs.Dislodged))
		}
		copy(dst.Dislodged, gs.Dislodged)
	} else {
		dst.Dislodged = nil
	}
}

func initialUnits() []Unit {
	return []Unit{
		// Austria
		{Army, Austria, "vie", NoCoast},
		{Army, Austria, "bud", NoCoast},
		{Fleet, Austria, "tri", NoCoast},
		// England
		{Fleet, England, "lon", NoCoast},
		{Fleet, England, "edi", NoCoast},
		{Army, England, "lvp", NoCoast},
		// France
		{Fleet, France, "bre", NoCoast},
		{Army, France, "par", NoCoast},
		{Army, France, "mar", NoCoast},
		// Germany
		{Fleet, Germany, "kie", NoCoast},
		{Army, Germany, "ber", NoCoast},
		{Army, Germany, "mun", NoCoast},
		// Italy
		{Fleet, Italy, "nap", NoCoast},
		{Army, Italy, "rom", NoCoast},
		{Army, Italy, "ven", NoCoast},
		// Russia
		{Fleet, Russia, "stp", SouthCoast},
		{Army, Russia, "mos", NoCoast},
		{Army, Russia, "war", NoCoast},
		{Fleet, Russia, "sev", NoCoast},
		// Turkey
		{Fleet, Turkey, "ank", NoCoast},
		{Army, Turkey, "con", NoCoast},
		{Army, Turkey, "smy", NoCoast},
	}
}

func initialSupplyCenters() map[string]Power {
	return map[string]Power{
		// Austria
		"vie": Austria, "bud": Austria, "tri": Austria,
		// England
		"lon": England, "edi": England, "lvp": England,
		// France
		"bre": France, "par": France, "mar": France,
		// Germany
		"kie": Germany, "ber": Germany, "mun": Germany,
		// Italy
		"nap": Italy, "rom": Italy, "ven": Italy,
		// Russia
		"stp": Russia, "mos": Russia, "war": Russia, "sev": Russia,
		// Turkey
		"ank": Turkey, "con": Turkey, "smy": Turkey,
		// Neutral supply centers
		"nwy": Neutral, "swe": Neutral, "den": Neutral,
		"hol": Neutral, "bel": Neutral, "spa": Neutral,
		"por": Neutral, "tun": Neutral, "gre": Neutral,
		"ser": Neutral, "bul": Neutral, "rum": Neutral,
	}
}
