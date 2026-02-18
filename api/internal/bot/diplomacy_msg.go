package bot

import (
	"fmt"
	"strings"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// IntentType categorizes a diplomatic message.
type IntentType int

const (
	IntentRequestSupport IntentType = iota
	IntentProposeNonAggression
	IntentProposeAlliance
	IntentThreaten
	IntentOfferDeal
	IntentAccept
	IntentReject
)

// DiplomaticIntent is the structured interpretation of a diplomatic message.
type DiplomaticIntent struct {
	Type        IntentType
	From        diplomacy.Power
	To          diplomacy.Power
	Provinces   []string        // relevant provinces
	TargetPower diplomacy.Power // e.g. "alliance against Turkey"
}

// BotDiplomacyState tracks promises and trust for a single bot.
type BotDiplomacyState struct {
	ReceivedRequests []DiplomaticIntent
	SentPromises     []DiplomaticIntent
	TrustScores      map[diplomacy.Power]float64
}

// NewBotDiplomacyState creates an initial diplomacy state with neutral trust.
func NewBotDiplomacyState() *BotDiplomacyState {
	trust := make(map[diplomacy.Power]float64)
	for _, p := range diplomacy.AllPowers() {
		trust[p] = 0.5
	}
	return &BotDiplomacyState{
		TrustScores: trust,
	}
}

// IntentName returns the string name for an IntentType.
func (it IntentType) String() string {
	switch it {
	case IntentRequestSupport:
		return "request_support"
	case IntentProposeNonAggression:
		return "propose_non_aggression"
	case IntentProposeAlliance:
		return "propose_alliance"
	case IntentThreaten:
		return "threaten"
	case IntentOfferDeal:
		return "offer_deal"
	case IntentAccept:
		return "accept"
	case IntentReject:
		return "reject"
	default:
		return "unknown"
	}
}

// FormatCannedMessage converts a DiplomaticIntent into a human-readable message.
func FormatCannedMessage(intent DiplomaticIntent) string {
	switch intent.Type {
	case IntentRequestSupport:
		if len(intent.Provinces) >= 2 {
			return fmt.Sprintf("Request support from %s to %s", intent.Provinces[0], intent.Provinces[1])
		}
		if len(intent.Provinces) == 1 {
			return fmt.Sprintf("Request support at %s", intent.Provinces[0])
		}
		return "Request support"

	case IntentProposeNonAggression:
		if len(intent.Provinces) > 0 {
			return fmt.Sprintf("Please don't attack %s, I won't attack yours", intent.Provinces[0])
		}
		return "Let's agree not to attack each other"

	case IntentProposeAlliance:
		if intent.TargetPower != "" {
			return fmt.Sprintf("Let's work together against %s", powerLabel(intent.TargetPower))
		}
		return "Let's work together"

	case IntentThreaten:
		if len(intent.Provinces) > 0 {
			return fmt.Sprintf("I'm coming for %s — back off", intent.Provinces[0])
		}
		return "Back off or face consequences"

	case IntentOfferDeal:
		if len(intent.Provinces) >= 2 {
			return fmt.Sprintf("Deal: I take %s, you take %s", intent.Provinces[0], intent.Provinces[1])
		}
		return "I'd like to make a deal"

	case IntentAccept:
		return "Agreed"

	case IntentReject:
		return "No deal"
	}
	return ""
}

// ParseCannedMessage converts a canned message string into a DiplomaticIntent.
// Returns an error if the message format is not recognized.
func ParseCannedMessage(content string) (*DiplomaticIntent, error) {
	lower := strings.ToLower(strings.TrimSpace(content))

	if lower == "agreed" {
		return &DiplomaticIntent{Type: IntentAccept}, nil
	}
	if lower == "no deal" {
		return &DiplomaticIntent{Type: IntentReject}, nil
	}

	if rest, ok := strings.CutPrefix(lower, "request support from "); ok {
		parts := strings.SplitN(rest, " to ", 2)
		if len(parts) == 2 {
			return &DiplomaticIntent{
				Type:      IntentRequestSupport,
				Provinces: []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])},
			}, nil
		}
	}
	if prov, ok := strings.CutPrefix(lower, "request support at "); ok {
		return &DiplomaticIntent{
			Type:      IntentRequestSupport,
			Provinces: []string{strings.TrimSpace(prov)},
		}, nil
	}

	if rest, ok := strings.CutPrefix(lower, "please don't attack "); ok {
		prov := strings.SplitN(rest, ",", 2)[0]
		return &DiplomaticIntent{
			Type:      IntentProposeNonAggression,
			Provinces: []string{strings.TrimSpace(prov)},
		}, nil
	}
	if lower == "let's agree not to attack each other" {
		return &DiplomaticIntent{Type: IntentProposeNonAggression}, nil
	}

	if powerStr, ok := strings.CutPrefix(lower, "let's work together against "); ok {
		return &DiplomaticIntent{
			Type:        IntentProposeAlliance,
			TargetPower: diplomacy.Power(strings.TrimSpace(powerStr)),
		}, nil
	}
	if lower == "let's work together" {
		return &DiplomaticIntent{Type: IntentProposeAlliance}, nil
	}

	if rest, ok := strings.CutPrefix(lower, "i'm coming for "); ok {
		prov := strings.SplitN(rest, " ", 2)[0]
		prov = strings.TrimSuffix(prov, "—")
		return &DiplomaticIntent{
			Type:      IntentThreaten,
			Provinces: []string{strings.TrimSpace(prov)},
		}, nil
	}

	if rest, ok := strings.CutPrefix(lower, "deal: i take "); ok {
		parts := strings.SplitN(rest, ", you take ", 2)
		if len(parts) == 2 {
			return &DiplomaticIntent{
				Type:      IntentOfferDeal,
				Provinces: []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])},
			}, nil
		}
	}

	return nil, fmt.Errorf("unrecognized canned message: %s", content)
}

// AdjustOrdersForDiplomacy modifies order scores based on diplomatic state.
// Higher compliance means the bot honors more requests.
func AdjustOrdersForDiplomacy(scores map[string]float64, dipState *BotDiplomacyState, power diplomacy.Power, compliance float64) {
	if dipState == nil {
		return
	}

	for _, req := range dipState.ReceivedRequests {
		trust := dipState.TrustScores[req.From]

		switch req.Type {
		case IntentProposeNonAggression:
			// Penalize moves into provinces covered by non-aggression pact
			for _, prov := range req.Provinces {
				key := moveKey(power, prov)
				if _, ok := scores[key]; ok {
					penalty := -5.0 * compliance * trust
					scores[key] += penalty
				}
			}

		case IntentRequestSupport:
			// Boost support orders that align with the request
			if len(req.Provinces) >= 2 {
				key := supportKey(power, req.Provinces[0], req.Provinces[1])
				if _, ok := scores[key]; ok {
					bonus := 6.0 * compliance * trust
					scores[key] += bonus
				}
			}

		case IntentThreaten:
			// Boost defense of threatened provinces
			for _, prov := range req.Provinces {
				key := holdKey(power, prov)
				if _, ok := scores[key]; ok {
					scores[key] += 3.0
				}
			}
		}
	}
}

func moveKey(power diplomacy.Power, target string) string {
	return fmt.Sprintf("%s:move:%s", power, target)
}

func supportKey(power diplomacy.Power, from, to string) string {
	return fmt.Sprintf("%s:support:%s:%s", power, from, to)
}

func holdKey(power diplomacy.Power, loc string) string {
	return fmt.Sprintf("%s:hold:%s", power, loc)
}

func powerLabel(p diplomacy.Power) string {
	s := string(p)
	if len(s) > 0 {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// CannedMessageTemplates returns the list of available canned message types for UI display.
func CannedMessageTemplates() []string {
	return []string{
		"Request support from {province} to {province}",
		"Please don't attack {province}, I won't attack yours",
		"Let's work together against {power}",
		"I'm coming for {province} — back off",
		"Deal: I take {province}, you take {province}",
		"Agreed",
		"No deal",
	}
}
