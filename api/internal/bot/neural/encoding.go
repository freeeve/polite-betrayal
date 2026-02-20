package neural

import "github.com/freeeve/polite-betrayal/api/pkg/diplomacy"

// EncodeBoard encodes a GameState into a flat [81*47] float32 array (row-major)
// matching the Rust/Python board encoding. Previous-state features (channels
// 36..47) are filled from prevState if non-nil, otherwise marked as empty.
func EncodeBoard(gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, prevState *diplomacy.GameState) []float32 {
	tensor := make([]float32, NumAreas*NumFeatures)

	// Static province type features.
	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		ptype := provinceTypeVec(area, m)
		tensor[base+FeatProvinceType] = ptype[0]
		tensor[base+FeatProvinceType+1] = ptype[1]
		tensor[base+FeatProvinceType+2] = ptype[2]
	}

	// Unit positions.
	for i := range gs.Units {
		u := &gs.Units[i]
		pi := PowerIndex(u.Power)
		areaIdx := AreaIndex(u.Province)
		if areaIdx < 0 {
			continue
		}
		setUnitFeatures(tensor, areaIdx, u.Type, pi)

		// Also set on the bicoastal variant if the unit has a coast.
		if u.Coast != diplomacy.NoCoast {
			if varIdx := BicoastalIndex(u.Province, u.Coast); varIdx >= 0 {
				setUnitFeatures(tensor, varIdx, u.Type, pi)
			}
		}
	}

	// Mark empty areas (no unit present).
	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		if tensor[base+FeatUnitType] == 0 && tensor[base+FeatUnitType+1] == 0 {
			tensor[base+FeatUnitType+2] = 1          // empty
			tensor[base+FeatUnitOwner+NumPowers] = 1 // owner = none
		}
	}

	// Supply center ownership.
	ownedSC := make(map[string]bool)
	for provID, power := range gs.SupplyCenters {
		if power == "" || power == diplomacy.Neutral {
			continue
		}
		prov := m.Provinces[provID]
		if prov == nil || !prov.IsSupplyCenter {
			continue
		}
		ownedSC[provID] = true
		pi := PowerIndex(power)
		areaIdx := AreaIndex(provID)
		if areaIdx < 0 {
			continue
		}
		tensor[areaIdx*NumFeatures+FeatSCOwner+pi] = 1

		// Also mark on bicoastal variants.
		if len(prov.Coasts) > 0 {
			for _, coast := range prov.Coasts {
				if varIdx := BicoastalIndex(provID, coast); varIdx >= 0 {
					tensor[varIdx*NumFeatures+FeatSCOwner+pi] = 1
				}
			}
		}
	}

	// Mark neutral SCs and non-SC areas.
	for area := 0; area < NumAreas; area++ {
		baseProv := baseProvince(area)
		abase := area * NumFeatures
		if isSupplyCenter(baseProv, m) {
			if !ownedSC[baseProv] {
				tensor[abase+FeatSCOwner+NumPowers] = 1 // neutral
			}
		} else {
			tensor[abase+FeatSCOwner+NumPowers+1] = 1 // none (not an SC)
		}
	}

	// Build/disband flags (adjustment phase).
	if gs.Phase == diplomacy.PhaseBuild {
		encodeBuildDisband(tensor, gs, m)
	}

	// Dislodged units.
	for i := range gs.Dislodged {
		d := &gs.Dislodged[i]
		areaIdx := AreaIndex(d.DislodgedFrom)
		if areaIdx < 0 {
			continue
		}
		base := areaIdx * NumFeatures
		switch d.Unit.Type {
		case diplomacy.Army:
			tensor[base+FeatDislodgedType] = 1
		case diplomacy.Fleet:
			tensor[base+FeatDislodgedType+1] = 1
		}
		tensor[base+FeatDislodgedOwn+PowerIndex(d.Unit.Power)] = 1
	}

	// Mark non-dislodged areas.
	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		if tensor[base+FeatDislodgedType] == 0 && tensor[base+FeatDislodgedType+1] == 0 {
			tensor[base+FeatDislodgedType+2] = 1        // none
			tensor[base+FeatDislodgedOwn+NumPowers] = 1 // owner = none
		}
	}

	// Previous-state unit features (channels 36..47).
	if prevState != nil {
		encodePrevState(tensor, prevState, m)
	} else {
		for area := 0; area < NumAreas; area++ {
			base := area * NumFeatures
			tensor[base+FeatPrevUnitType+2] = 1          // empty
			tensor[base+FeatPrevUnitOwner+NumPowers] = 1 // owner = none
		}
	}

	return tensor
}

// BuildAdjacencyMatrix builds the 81x81 adjacency matrix with self-loops
// and bicoastal variant inheritance, matching the Rust/Python format.
func BuildAdjacencyMatrix(m *diplomacy.DiplomacyMap) []float32 {
	adj := make([]float32, NumAreas*NumAreas)

	// Add edges from the map adjacency table (over base provinces only).
	for from, adjs := range m.Adjacencies {
		fi := AreaIndex(from)
		if fi < 0 || fi >= 75 {
			continue
		}
		for _, a := range adjs {
			ti := AreaIndex(a.To)
			if ti < 0 || ti >= 75 {
				continue
			}
			adj[fi*NumAreas+ti] = 1
			adj[ti*NumAreas+fi] = 1
		}
	}

	// Connect bicoastal variants to their base and propagate base adjacencies.
	type splitCoast struct {
		baseID string
		coasts []struct {
			coast  diplomacy.Coast
			varIdx int
		}
	}
	splits := []splitCoast{
		{"bul", []struct {
			coast  diplomacy.Coast
			varIdx int
		}{{diplomacy.EastCoast, BulEC}, {diplomacy.SouthCoast, BulSC}}},
		{"spa", []struct {
			coast  diplomacy.Coast
			varIdx int
		}{{diplomacy.NorthCoast, SpaNC}, {diplomacy.SouthCoast, SpaSC}}},
		{"stp", []struct {
			coast  diplomacy.Coast
			varIdx int
		}{{diplomacy.NorthCoast, StpNC}, {diplomacy.SouthCoast, StpSC}}},
	}

	for _, sp := range splits {
		baseIdx := AreaIndex(sp.baseID)
		if baseIdx < 0 {
			continue
		}
		for _, cv := range sp.coasts {
			varIdx := cv.varIdx
			// Variant <-> base.
			adj[baseIdx*NumAreas+varIdx] = 1
			adj[varIdx*NumAreas+baseIdx] = 1
			// Variant inherits all base adjacencies.
			for k := 0; k < NumAreas; k++ {
				if adj[baseIdx*NumAreas+k] == 1 {
					adj[varIdx*NumAreas+k] = 1
					adj[k*NumAreas+varIdx] = 1
				}
			}
		}
	}

	// Self-loops.
	for i := 0; i < NumAreas; i++ {
		adj[i*NumAreas+i] = 1
	}

	return adj
}

// CollectUnitIndices returns province indices (area indices) of units belonging
// to the given power, padded to MaxUnits with zeros.
func CollectUnitIndices(gs *diplomacy.GameState, power diplomacy.Power) []int64 {
	indices := make([]int64, 0, MaxUnits)
	for i := range gs.Units {
		u := &gs.Units[i]
		if u.Power == power {
			idx := AreaIndex(u.Province)
			if idx >= 0 && len(indices) < MaxUnits {
				indices = append(indices, int64(idx))
			}
		}
	}
	for len(indices) < MaxUnits {
		indices = append(indices, 0)
	}
	return indices
}

// setUnitFeatures sets unit type and owner features for an area.
func setUnitFeatures(tensor []float32, area int, unitType diplomacy.UnitType, powerIdx int) {
	base := area * NumFeatures
	switch unitType {
	case diplomacy.Army:
		tensor[base+FeatUnitType] = 1
	case diplomacy.Fleet:
		tensor[base+FeatUnitType+1] = 1
	}
	tensor[base+FeatUnitOwner+powerIdx] = 1
}

// provinceTypeVec returns [land, sea, coast] for an area.
func provinceTypeVec(area int, m *diplomacy.DiplomacyMap) [3]float32 {
	if area >= 75 {
		// Bicoastal variants are always coastal.
		return [3]float32{0, 0, 1}
	}
	provID := AreaNames[area]
	prov := m.Provinces[provID]
	if prov == nil {
		return [3]float32{0, 0, 0}
	}
	switch prov.Type {
	case diplomacy.Land:
		return [3]float32{1, 0, 0}
	case diplomacy.Sea:
		return [3]float32{0, 1, 0}
	default: // Coastal
		return [3]float32{0, 0, 1}
	}
}

// baseProvince returns the base province ID for an area index.
func baseProvince(area int) string {
	if area < 75 {
		return AreaNames[area]
	}
	switch area {
	case BulEC, BulSC:
		return "bul"
	case SpaNC, SpaSC:
		return "spa"
	case StpNC, StpSC:
		return "stp"
	default:
		return ""
	}
}

// isSupplyCenter checks whether a province is an SC.
func isSupplyCenter(provID string, m *diplomacy.DiplomacyMap) bool {
	prov := m.Provinces[provID]
	return prov != nil && prov.IsSupplyCenter
}

// encodeBuildDisband sets build/disband flags during adjustment phases.
func encodeBuildDisband(tensor []float32, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) {
	for _, power := range diplomacy.AllPowers() {
		numUnits := gs.UnitCount(power)
		numSCs := gs.SupplyCenterCount(power)

		if numSCs > numUnits {
			// Can build on owned home centers that are unoccupied.
			homes := diplomacy.HomeCenters(power)
			for _, h := range homes {
				if gs.SupplyCenters[h] == power && gs.UnitAt(h) == nil {
					areaIdx := AreaIndex(h)
					if areaIdx >= 0 {
						tensor[areaIdx*NumFeatures+FeatCanBuild] = 1
					}
				}
			}
		} else if numUnits > numSCs {
			// Must disband: mark all of this power's units.
			for i := range gs.Units {
				u := &gs.Units[i]
				if u.Power == power {
					areaIdx := AreaIndex(u.Province)
					if areaIdx >= 0 {
						tensor[areaIdx*NumFeatures+FeatCanDisband] = 1
					}
				}
			}
		}
	}
}

// encodePrevState encodes previous-state unit positions into channels 36..47.
func encodePrevState(tensor []float32, prev *diplomacy.GameState, m *diplomacy.DiplomacyMap) {
	for i := range prev.Units {
		u := &prev.Units[i]
		pi := PowerIndex(u.Power)
		areaIdx := AreaIndex(u.Province)
		if areaIdx < 0 {
			continue
		}
		setPrevUnitFeatures(tensor, areaIdx, u.Type, pi)

		if u.Coast != diplomacy.NoCoast {
			if varIdx := BicoastalIndex(u.Province, u.Coast); varIdx >= 0 {
				setPrevUnitFeatures(tensor, varIdx, u.Type, pi)
			}
		}
	}

	// Mark empty areas in previous-state channels.
	for area := 0; area < NumAreas; area++ {
		base := area * NumFeatures
		if tensor[base+FeatPrevUnitType] == 0 && tensor[base+FeatPrevUnitType+1] == 0 {
			tensor[base+FeatPrevUnitType+2] = 1          // empty
			tensor[base+FeatPrevUnitOwner+NumPowers] = 1 // owner = none
		}
	}
}

// setPrevUnitFeatures sets previous-turn unit type and owner features.
func setPrevUnitFeatures(tensor []float32, area int, unitType diplomacy.UnitType, powerIdx int) {
	base := area * NumFeatures
	switch unitType {
	case diplomacy.Army:
		tensor[base+FeatPrevUnitType] = 1
	case diplomacy.Fleet:
		tensor[base+FeatPrevUnitType+1] = 1
	}
	tensor[base+FeatPrevUnitOwner+powerIdx] = 1
}
