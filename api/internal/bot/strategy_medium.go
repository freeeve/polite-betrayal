package bot

import (
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders for the "medium" difficulty bot.
// Uses the opening book for known positions, then generates multiple
// candidate order sets and picks the best via 1-ply lookahead.
type TacticalStrategy struct{}

func (TacticalStrategy) Name() string { return "medium" }

// ShouldVoteDraw always accepts a draw (same as easy).
func (TacticalStrategy) ShouldVoteDraw(_ *diplomacy.GameState, _ diplomacy.Power) bool {
	return true
}

// GenerateMovementOrders checks the opening book first, then generates 16
// candidate order sets using the easy bot (which has built-in randomness)
// and picks the one that produces the best evaluated position after
// 1-ply resolution.
func (TacticalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	if opening := LookupOpening(gs, power, m); opening != nil {
		return opening
	}

	const numCandidates = 16
	easy := HeuristicStrategy{}

	// Generate opponent orders once (predicted via easy heuristic).
	var opponentOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		opponentOrders = append(opponentOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	// Generate N candidate order sets and evaluate each via lookahead.
	rv := diplomacy.NewResolver(34)
	clone := gs.Clone()
	orderBuf := make([]diplomacy.Order, 0, 34)

	bestScore := -1e9
	var bestOrders []OrderInput

	for range numCandidates {
		candidate := easy.GenerateMovementOrders(gs, power, m)
		myOrders := OrderInputsToOrders(candidate, power)

		orderBuf = orderBuf[:0]
		orderBuf = append(orderBuf, myOrders...)
		orderBuf = append(orderBuf, opponentOrders...)

		rv.Resolve(orderBuf, gs, m)
		gs.CloneInto(clone)
		rv.Apply(clone, m)
		score := EvaluatePosition(clone, power, m)

		if score > bestScore {
			bestScore = score
			bestOrders = candidate
		}
	}

	return bestOrders
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
