package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestFormatAndParseCannedMessage_RequestSupport(t *testing.T) {
	intent := DiplomaticIntent{
		Type:      IntentRequestSupport,
		Provinces: []string{"bur", "mun"},
	}
	msg := FormatCannedMessage(intent)
	if msg != "Request support from bur to mun" {
		t.Errorf("unexpected format: %s", msg)
	}

	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentRequestSupport {
		t.Errorf("expected RequestSupport, got %d", parsed.Type)
	}
	if len(parsed.Provinces) != 2 || parsed.Provinces[0] != "bur" || parsed.Provinces[1] != "mun" {
		t.Errorf("unexpected provinces: %v", parsed.Provinces)
	}
}

func TestFormatAndParseCannedMessage_RequestSupportAt(t *testing.T) {
	intent := DiplomaticIntent{
		Type:      IntentRequestSupport,
		Provinces: []string{"mun"},
	}
	msg := FormatCannedMessage(intent)
	if msg != "Request support at mun" {
		t.Errorf("unexpected format: %s", msg)
	}

	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentRequestSupport {
		t.Errorf("expected RequestSupport, got %d", parsed.Type)
	}
}

func TestFormatAndParseCannedMessage_NonAggression(t *testing.T) {
	intent := DiplomaticIntent{
		Type:      IntentProposeNonAggression,
		Provinces: []string{"bur"},
	}
	msg := FormatCannedMessage(intent)
	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentProposeNonAggression {
		t.Errorf("expected NonAggression, got %d", parsed.Type)
	}
	if len(parsed.Provinces) != 1 || parsed.Provinces[0] != "bur" {
		t.Errorf("unexpected provinces: %v", parsed.Provinces)
	}
}

func TestFormatAndParseCannedMessage_Alliance(t *testing.T) {
	intent := DiplomaticIntent{
		Type:        IntentProposeAlliance,
		TargetPower: diplomacy.Turkey,
	}
	msg := FormatCannedMessage(intent)
	if msg != "Let's work together against Turkey" {
		t.Errorf("unexpected format: %s", msg)
	}

	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentProposeAlliance {
		t.Errorf("expected Alliance, got %d", parsed.Type)
	}
	if parsed.TargetPower != diplomacy.Turkey {
		t.Errorf("expected Turkey, got %s", parsed.TargetPower)
	}
}

func TestFormatAndParseCannedMessage_Threaten(t *testing.T) {
	intent := DiplomaticIntent{
		Type:      IntentThreaten,
		Provinces: []string{"war"},
	}
	msg := FormatCannedMessage(intent)
	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentThreaten {
		t.Errorf("expected Threaten, got %d", parsed.Type)
	}
}

func TestFormatAndParseCannedMessage_Deal(t *testing.T) {
	intent := DiplomaticIntent{
		Type:      IntentOfferDeal,
		Provinces: []string{"bel", "hol"},
	}
	msg := FormatCannedMessage(intent)
	parsed, err := ParseCannedMessage(msg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentOfferDeal {
		t.Errorf("expected OfferDeal, got %d", parsed.Type)
	}
	if len(parsed.Provinces) != 2 {
		t.Errorf("expected 2 provinces, got %d", len(parsed.Provinces))
	}
}

func TestParseCannedMessage_Agreed(t *testing.T) {
	parsed, err := ParseCannedMessage("Agreed")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentAccept {
		t.Errorf("expected Accept, got %d", parsed.Type)
	}
}

func TestParseCannedMessage_NoDeal(t *testing.T) {
	parsed, err := ParseCannedMessage("No deal")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed.Type != IntentReject {
		t.Errorf("expected Reject, got %d", parsed.Type)
	}
}

func TestParseCannedMessage_Unrecognized(t *testing.T) {
	_, err := ParseCannedMessage("Hello world, how are you?")
	if err == nil {
		t.Error("expected error for unrecognized message")
	}
}

func TestIntentType_String(t *testing.T) {
	if IntentRequestSupport.String() != "request_support" {
		t.Errorf("unexpected string: %s", IntentRequestSupport.String())
	}
	if IntentAccept.String() != "accept" {
		t.Errorf("unexpected string: %s", IntentAccept.String())
	}
}

func TestBotDiplomacyState_NewDefaults(t *testing.T) {
	state := NewBotDiplomacyState()
	for _, p := range diplomacy.AllPowers() {
		if state.TrustScores[p] != 0.5 {
			t.Errorf("expected initial trust 0.5 for %s, got %.1f", p, state.TrustScores[p])
		}
	}
}

func TestAdjustOrdersForDiplomacy_NonAggression(t *testing.T) {
	state := NewBotDiplomacyState()
	state.ReceivedRequests = []DiplomaticIntent{
		{
			Type:      IntentProposeNonAggression,
			From:      diplomacy.Germany,
			Provinces: []string{"bur"},
		},
	}

	scores := map[string]float64{
		moveKey(diplomacy.France, "bur"): 5.0,
		moveKey(diplomacy.France, "pic"): 3.0,
	}

	AdjustOrdersForDiplomacy(scores, state, diplomacy.France, 0.7)

	// Score for moving to bur should be reduced
	if scores[moveKey(diplomacy.France, "bur")] >= 5.0 {
		t.Errorf("non-aggression should penalize move to bur, got %.1f", scores[moveKey(diplomacy.France, "bur")])
	}
	// Score for moving to pic should be unchanged
	if scores[moveKey(diplomacy.France, "pic")] != 3.0 {
		t.Errorf("non-aggression should not affect move to pic, got %.1f", scores[moveKey(diplomacy.France, "pic")])
	}
}

func TestAdjustOrdersForDiplomacy_NilState(t *testing.T) {
	scores := map[string]float64{"foo": 1.0}
	AdjustOrdersForDiplomacy(scores, nil, diplomacy.France, 1.0)
	if scores["foo"] != 1.0 {
		t.Error("nil state should not modify scores")
	}
}

func TestCannedMessageTemplates(t *testing.T) {
	templates := CannedMessageTemplates()
	if len(templates) != 7 {
		t.Errorf("expected 7 templates, got %d", len(templates))
	}
}
