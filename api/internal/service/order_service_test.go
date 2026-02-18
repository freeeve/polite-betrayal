package service

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestParseUnitType(t *testing.T) {
	tests := []struct {
		input string
		want  diplomacy.UnitType
	}{
		{"army", diplomacy.Army},
		{"fleet", diplomacy.Fleet},
		{"", diplomacy.Army},
		{"invalid", diplomacy.Army},
	}
	for _, tt := range tests {
		got := parseUnitType(tt.input)
		if got != tt.want {
			t.Errorf("parseUnitType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseOrderType(t *testing.T) {
	tests := []struct {
		input string
		want  diplomacy.OrderType
	}{
		{"hold", diplomacy.OrderHold},
		{"move", diplomacy.OrderMove},
		{"support", diplomacy.OrderSupport},
		{"convoy", diplomacy.OrderConvoy},
		{"", diplomacy.OrderHold},
		{"invalid", diplomacy.OrderHold},
	}
	for _, tt := range tests {
		got := parseOrderType(tt.input)
		if got != tt.want {
			t.Errorf("parseOrderType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestToEngineOrder(t *testing.T) {
	input := OrderInput{
		UnitType:    "fleet",
		Location:    "nth",
		OrderType:   "convoy",
		Target:      "nwy",
		AuxLoc:      "lon",
		AuxTarget:   "nwy",
		AuxUnitType: "army",
	}
	order := toEngineOrder(input, diplomacy.England)
	if order.UnitType != diplomacy.Fleet {
		t.Errorf("expected Fleet, got %v", order.UnitType)
	}
	if order.Power != diplomacy.England {
		t.Errorf("expected England, got %v", order.Power)
	}
	if order.Location != "nth" {
		t.Errorf("expected nth, got %s", order.Location)
	}
	if order.Type != diplomacy.OrderConvoy {
		t.Errorf("expected Convoy, got %v", order.Type)
	}
	if order.Target != "nwy" {
		t.Errorf("expected nwy, got %s", order.Target)
	}
	if order.AuxUnitType != diplomacy.Army {
		t.Errorf("expected Army for aux, got %v", order.AuxUnitType)
	}
}

func TestToEngineOrderWithCoast(t *testing.T) {
	input := OrderInput{
		UnitType:    "fleet",
		Location:    "stp",
		Coast:       "nc",
		OrderType:   "move",
		Target:      "bar",
		TargetCoast: "",
	}
	order := toEngineOrder(input, diplomacy.Russia)
	if order.Coast != diplomacy.Coast("nc") {
		t.Errorf("expected coast nc, got %v", order.Coast)
	}
}
