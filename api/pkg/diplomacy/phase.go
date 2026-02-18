package diplomacy

// NextPhase computes the next phase after the current one.
// Movement -> Retreat (if dislodgements) or straight to Fall Movement / Build.
// Retreat -> Fall Movement or Build (if Fall).
// Build -> Spring Movement of next year.
func NextPhase(gs *GameState, hasDislodgements bool) (Season, PhaseType) {
	switch gs.Phase {
	case PhaseMovement:
		if hasDislodgements {
			return gs.Season, PhaseRetreat
		}
		return afterMovement(gs.Season)
	case PhaseRetreat:
		return afterMovement(gs.Season)
	case PhaseBuild:
		return Spring, PhaseMovement
	}
	return Spring, PhaseMovement
}

func afterMovement(season Season) (Season, PhaseType) {
	if season == Spring {
		return Fall, PhaseMovement
	}
	// After Fall movement, always go to Build phase for adjustments
	return Fall, PhaseBuild
}

// NeedsBuildPhase returns true if any power has a unit/SC mismatch requiring adjustments.
func NeedsBuildPhase(gs *GameState) bool {
	for _, power := range AllPowers() {
		if gs.SupplyCenterCount(power) != gs.UnitCount(power) {
			return true
		}
	}
	return false
}

// MaxYear is the highest year a game can reach before ending as a draw.
const MaxYear = 3000

// IsYearLimitReached returns true if the game has exceeded the maximum year.
func IsYearLimitReached(gs *GameState) bool {
	return gs.Year > MaxYear
}

// IsGameOver checks if any single power controls 18+ supply centers (solo victory).
func IsGameOver(gs *GameState) (bool, Power) {
	for _, power := range AllPowers() {
		if gs.SupplyCenterCount(power) >= 18 {
			return true, power
		}
	}
	return false, Neutral
}

// AdvanceState transitions the game state to the next phase.
// For movement: updates year/season/phase, updates SC ownership after Fall.
// Callers must apply resolution results to units before calling this.
func AdvanceState(gs *GameState, hasDislodgements bool) {
	nextSeason, nextPhase := NextPhase(gs, hasDislodgements)

	// After Fall movement or Fall retreat, update SC ownership
	if gs.Season == Fall && (gs.Phase == PhaseMovement || gs.Phase == PhaseRetreat) {
		updateSupplyCenterOwnership(gs)
	}

	if nextSeason == Spring && nextPhase == PhaseMovement {
		gs.Year++
	}
	gs.Season = nextSeason
	gs.Phase = nextPhase
	if nextPhase != PhaseRetreat {
		gs.Dislodged = nil
	}
}

// updateSupplyCenterOwnership assigns SCs to the power whose unit occupies them.
func updateSupplyCenterOwnership(gs *GameState) {
	stdMap := StandardMap()
	for provID := range gs.SupplyCenters {
		prov := stdMap.Provinces[provID]
		if prov == nil || !prov.IsSupplyCenter {
			continue
		}
		if unit := gs.UnitAt(provID); unit != nil {
			gs.SupplyCenters[provID] = unit.Power
		}
		// If no unit present, ownership stays with current owner
	}
}

// HomeCenters returns the home supply center IDs for a given power.
func HomeCenters(power Power) []string {
	stdMap := StandardMap()
	var centers []string
	for _, prov := range stdMap.Provinces {
		if prov.HomePower == power && prov.IsSupplyCenter {
			centers = append(centers, prov.ID)
		}
	}
	return centers
}
