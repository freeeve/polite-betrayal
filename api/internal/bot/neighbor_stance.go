package bot

import (
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// Stance represents a neighbor power's posture toward us.
type Stance string

const (
	StanceAggressive Stance = "aggressive" // units adjacent to our SCs
	StanceNeutral    Stance = "neutral"    // no significant border pressure
	StanceRetreating Stance = "retreating" // units moving away from our SCs
)

// ClassifyNeighborStances examines unit positions to determine each neighbor's
// posture toward the given power. Since GameState does not store order history,
// this uses a proximity heuristic: count how many of a neighbor's units are
// adjacent to our supply centers.
func ClassifyNeighborStances(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) map[diplomacy.Power]Stance {
	// Build set of provinces adjacent to our SCs (border zone).
	ourSCs := make(map[string]bool)
	for prov, owner := range gs.SupplyCenters {
		if owner == power {
			ourSCs[prov] = true
		}
	}

	borderZone := make(map[string]bool)
	for sc := range ourSCs {
		// Include both army and fleet adjacencies.
		for _, adj := range m.Adjacencies[sc] {
			if !ourSCs[adj.To] {
				borderZone[adj.To] = true
			}
		}
	}

	// Count each neighbor's units in our border zone vs total units.
	type neighborStats struct {
		adjacent int
		total    int
	}
	stats := make(map[diplomacy.Power]*neighborStats)

	for _, u := range gs.Units {
		if u.Power == power || u.Power == diplomacy.Neutral {
			continue
		}
		s, ok := stats[u.Power]
		if !ok {
			s = &neighborStats{}
			stats[u.Power] = s
		}
		s.total++
		if borderZone[u.Province] {
			s.adjacent++
		}
	}

	result := make(map[diplomacy.Power]Stance)
	for p, s := range stats {
		if s.total == 0 {
			continue
		}
		ratio := float64(s.adjacent) / float64(s.total)
		switch {
		case ratio >= 0.5:
			result[p] = StanceAggressive
		case ratio == 0:
			result[p] = StanceRetreating
		default:
			result[p] = StanceNeutral
		}
	}

	return result
}
