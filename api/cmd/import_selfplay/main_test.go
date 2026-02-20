package main

import (
	"testing"
)

func TestExpandSeason(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"s", "spring"},
		{"f", "fall"},
		{"x", "spring"}, // default
	}
	for _, tt := range tests {
		got := expandSeason(tt.in)
		if got != tt.want {
			t.Errorf("expandSeason(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandPhase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"m", "movement"},
		{"r", "retreat"},
		{"b", "build"},
		{"x", "movement"}, // default
	}
	for _, tt := range tests {
		got := expandPhase(tt.in)
		if got != tt.want {
			t.Errorf("expandPhase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSplitLocation(t *testing.T) {
	tests := []struct {
		in        string
		wantProv  string
		wantCoast string
	}{
		{"vie", "vie", ""},
		{"stp/nc", "stp", "nc"},
		{"spa/sc", "spa", "sc"},
		{"bul/ec", "bul", "ec"},
	}
	for _, tt := range tests {
		prov, coast := splitLocation(tt.in)
		if prov != tt.wantProv || coast != tt.wantCoast {
			t.Errorf("splitLocation(%q) = (%q, %q), want (%q, %q)", tt.in, prov, coast, tt.wantProv, tt.wantCoast)
		}
	}
}

func TestDsonUnitType(t *testing.T) {
	if got := dsonUnitType("A"); got != "army" {
		t.Errorf("dsonUnitType(A) = %q", got)
	}
	if got := dsonUnitType("F"); got != "fleet" {
		t.Errorf("dsonUnitType(F) = %q", got)
	}
}

func TestParseSingleDSON_Hold(t *testing.T) {
	o, ok := parseSingleDSON("A vie H", "austria", "phase-1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.UnitType != "army" || o.Location != "vie" || o.OrderType != "hold" {
		t.Errorf("hold: got %+v", o)
	}
	if o.Power != "austria" || o.PhaseID != "phase-1" {
		t.Errorf("metadata: got %+v", o)
	}
}

func TestParseSingleDSON_Move(t *testing.T) {
	o, ok := parseSingleDSON("A bud - rum", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "move" || o.Location != "bud" || o.Target != "rum" {
		t.Errorf("move: got %+v", o)
	}
}

func TestParseSingleDSON_MoveWithCoast(t *testing.T) {
	o, ok := parseSingleDSON("F nrg - stp/nc", "russia", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "move" || o.Location != "nrg" || o.Target != "stp" {
		t.Errorf("move coast: got %+v", o)
	}
}

func TestParseSingleDSON_SupportHold(t *testing.T) {
	o, ok := parseSingleDSON("A tyr S A vie H", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "support" || o.Location != "tyr" || o.AuxLoc != "vie" || o.AuxUnitType != "army" {
		t.Errorf("support hold: got %+v", o)
	}
}

func TestParseSingleDSON_SupportMove(t *testing.T) {
	o, ok := parseSingleDSON("A gal S A bud - rum", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "support" || o.Location != "gal" || o.AuxLoc != "bud" || o.Target != "rum" || o.AuxTarget != "rum" {
		t.Errorf("support move: got %+v", o)
	}
}

func TestParseSingleDSON_Convoy(t *testing.T) {
	o, ok := parseSingleDSON("F nth C A lon - nwy", "england", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "convoy" || o.Location != "nth" || o.AuxLoc != "lon" || o.AuxTarget != "nwy" {
		t.Errorf("convoy: got %+v", o)
	}
}

func TestParseSingleDSON_Retreat(t *testing.T) {
	o, ok := parseSingleDSON("A vie R boh", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "retreat_move" || o.Location != "vie" || o.Target != "boh" {
		t.Errorf("retreat: got %+v", o)
	}
}

func TestParseSingleDSON_Disband(t *testing.T) {
	o, ok := parseSingleDSON("F tri D", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "retreat_disband" || o.Location != "tri" || o.UnitType != "fleet" {
		t.Errorf("disband: got %+v", o)
	}
}

func TestParseSingleDSON_Build(t *testing.T) {
	o, ok := parseSingleDSON("A vie B", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "build" || o.Location != "vie" || o.UnitType != "army" {
		t.Errorf("build: got %+v", o)
	}
}

func TestParseSingleDSON_BuildFleetCoast(t *testing.T) {
	o, ok := parseSingleDSON("F stp/sc B", "russia", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "build" || o.Location != "stp" || o.UnitType != "fleet" {
		t.Errorf("build fleet coast: got %+v", o)
	}
}

func TestParseSingleDSON_Waive(t *testing.T) {
	o, ok := parseSingleDSON("W", "austria", "p1")
	if !ok {
		t.Fatal("expected ok")
	}
	if o.OrderType != "waive" {
		t.Errorf("waive: got %+v", o)
	}
}

func TestParseDSONOrders_Multi(t *testing.T) {
	orders := parseDSONOrders("A vie - tri ; A bud - ser ; F tri - alb", "austria", "p1")
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
	if orders[0].OrderType != "move" || orders[0].Target != "tri" {
		t.Errorf("order 0: got %+v", orders[0])
	}
	if orders[1].OrderType != "move" || orders[1].Target != "ser" {
		t.Errorf("order 1: got %+v", orders[1])
	}
	if orders[2].OrderType != "move" || orders[2].Target != "alb" {
		t.Errorf("order 2: got %+v", orders[2])
	}
}

func TestParseDSONOrders_Empty(t *testing.T) {
	orders := parseDSONOrders("", "austria", "p1")
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestParseDSONOrders_Complex(t *testing.T) {
	// France opening
	dson := "F bre - mao ; A par - bur ; A mar S A par - bur"
	orders := parseDSONOrders(dson, "france", "p1")
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}

	if orders[0].OrderType != "move" || orders[0].Location != "bre" || orders[0].Target != "mao" {
		t.Errorf("order 0: got %+v", orders[0])
	}
	if orders[1].OrderType != "move" || orders[1].Location != "par" || orders[1].Target != "bur" {
		t.Errorf("order 1: got %+v", orders[1])
	}
	if orders[2].OrderType != "support" || orders[2].Location != "mar" || orders[2].AuxLoc != "par" || orders[2].AuxTarget != "bur" {
		t.Errorf("order 2: got %+v", orders[2])
	}
}
