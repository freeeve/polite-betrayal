package bot

import (
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders for the "medium" difficulty bot.
// Uses the opening book for known positions, then falls back to the same
// heuristic logic as the easy bot for everything else. This serves as a
// clean baseline for incremental improvements.
type TacticalStrategy struct{}

func (TacticalStrategy) Name() string { return "medium" }

// ShouldVoteDraw always accepts a draw (same as easy).
func (TacticalStrategy) ShouldVoteDraw(_ *diplomacy.GameState, _ diplomacy.Power) bool {
	return true
}

// GenerateMovementOrders checks the opening book first, then delegates to
// the easy bot's heuristic logic.
func (TacticalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	if opening := LookupOpening(gs, power, m); opening != nil {
		return opening
	}
	return HeuristicStrategy{}.GenerateMovementOrders(gs, power, m)
}

// GenerateRetreatOrders delegates to the easy bot's retreat logic.
func (TacticalStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return HeuristicStrategy{}.GenerateRetreatOrders(gs, power, m)
}

// GenerateBuildOrders delegates to the easy bot's build logic.
func (TacticalStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return HeuristicStrategy{}.GenerateBuildOrders(gs, power, m)
}

// GenerateDiplomaticMessages proposes non-aggression pacts to bordering powers
// and responds to incoming diplomatic messages with simple accept/reject logic.
func (TacticalStrategy) GenerateDiplomaticMessages(
	gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap,
	received []DiplomaticIntent,
) []DiplomaticIntent {
	var messages []DiplomaticIntent

	for _, req := range received {
		switch req.Type {
		case IntentRequestSupport, IntentProposeNonAggression, IntentProposeAlliance:
			messages = append(messages, DiplomaticIntent{
				Type: IntentAccept,
				From: power,
				To:   req.From,
			})
		case IntentThreaten:
			messages = append(messages, DiplomaticIntent{
				Type: IntentReject,
				From: power,
				To:   req.From,
			})
		}
	}

	ourReach := make(map[string]bool)
	for _, u := range gs.UnitsOf(power) {
		isFleet := u.Type == diplomacy.Fleet
		for _, adj := range m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet) {
			ourReach[adj] = true
		}
	}
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		bordering := false
		for _, u := range gs.UnitsOf(p) {
			isFleet := u.Type == diplomacy.Fleet
			for _, adj := range m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet) {
				if ourReach[adj] {
					bordering = true
					break
				}
			}
			if bordering {
				break
			}
		}
		if bordering {
			messages = append(messages, DiplomaticIntent{
				Type: IntentProposeNonAggression,
				From: power,
				To:   p,
			})
		}
	}

	return messages
}
