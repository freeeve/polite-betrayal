package diplomacy

// ProvinceCount is the number of provinces on the standard Diplomacy map.
const ProvinceCount = 75

// ProvinceType classifies a province as land, sea, or coastal.
type ProvinceType int

const (
	Land    ProvinceType = iota // Inland province (armies only)
	Sea                         // Sea province (fleets only)
	Coastal                     // Coastal province (armies or fleets)
)

// Coast represents a specific coast of a province with split coasts.
type Coast string

const (
	NoCoast    Coast = ""
	NorthCoast Coast = "nc"
	SouthCoast Coast = "sc"
	EastCoast  Coast = "ec"
	WestCoast  Coast = "wc"
)

// Province represents a single province on the Diplomacy map.
type Province struct {
	ID             string
	Name           string
	Type           ProvinceType
	IsSupplyCenter bool
	HomePower      Power   // Power whose home SC this is ("" if not a home SC)
	Coasts         []Coast // Non-empty only for split-coast provinces (e.g. Spain)
}

// Adjacency describes a connection between two provinces.
// For provinces with split coasts, coastal adjacencies specify which coast.
type Adjacency struct {
	From      string
	FromCoast Coast
	To        string
	ToCoast   Coast
	ArmyOK    bool // Armies can traverse this adjacency
	FleetOK   bool // Fleets can traverse this adjacency
}

// DiplomacyMap holds the full province and adjacency graph.
type DiplomacyMap struct {
	Provinces   map[string]*Province
	Adjacencies map[string][]Adjacency // keyed by from province ID
	provIndex   map[string]int
	provNames   [ProvinceCount]string
}

// ProvinceIndex returns the dense index (0..ProvinceCount-1) for a province ID.
// Returns -1 if the province is not found.
func (m *DiplomacyMap) ProvinceIndex(id string) int {
	idx, ok := m.provIndex[id]
	if !ok {
		return -1
	}
	return idx
}

// ProvinceName returns the province ID for a given dense index.
func (m *DiplomacyMap) ProvinceName(idx int) string {
	return m.provNames[idx]
}

// Adjacent returns true if there is a valid adjacency from src to dst
// for the given unit type and coast constraints.
func (m *DiplomacyMap) Adjacent(src string, srcCoast Coast, dst string, dstCoast Coast, isFleet bool) bool {
	for _, adj := range m.Adjacencies[src] {
		if adj.To != dst {
			continue
		}
		if isFleet && !adj.FleetOK {
			continue
		}
		if !isFleet && !adj.ArmyOK {
			continue
		}
		if srcCoast != NoCoast && adj.FromCoast != NoCoast && adj.FromCoast != srcCoast {
			continue
		}
		if dstCoast != NoCoast && adj.ToCoast != NoCoast && adj.ToCoast != dstCoast {
			continue
		}
		return true
	}
	return false
}

// FleetCostsTo returns all coasts at the destination province reachable by fleet
// from the given source province and coast.
func (m *DiplomacyMap) FleetCoastsTo(src string, srcCoast Coast, dst string) []Coast {
	var coasts []Coast
	for _, adj := range m.Adjacencies[src] {
		if adj.To != dst || !adj.FleetOK {
			continue
		}
		if srcCoast != NoCoast && adj.FromCoast != NoCoast && adj.FromCoast != srcCoast {
			continue
		}
		coasts = append(coasts, adj.ToCoast)
	}
	return coasts
}

// ProvincesAdjacentTo returns all province IDs adjacent to the given province
// accessible by the given unit type.
func (m *DiplomacyMap) ProvincesAdjacentTo(provID string, coast Coast, isFleet bool) []string {
	seen := make(map[string]bool)
	var result []string
	for _, adj := range m.Adjacencies[provID] {
		if isFleet && !adj.FleetOK {
			continue
		}
		if !isFleet && !adj.ArmyOK {
			continue
		}
		if coast != NoCoast && adj.FromCoast != NoCoast && adj.FromCoast != coast {
			continue
		}
		if !seen[adj.To] {
			seen[adj.To] = true
			result = append(result, adj.To)
		}
	}
	return result
}

// HasCoasts returns true if the province has split coasts (e.g. Spain, St Petersburg, Bulgaria).
func (m *DiplomacyMap) HasCoasts(provID string) bool {
	p, ok := m.Provinces[provID]
	return ok && len(p.Coasts) > 0
}
