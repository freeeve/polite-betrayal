package diplomacy

import (
	"testing"
)

func TestFormatDSON_MovementOrders(t *testing.T) {
	tests := []struct {
		name   string
		orders []DSONOrder
		want   string
	}{
		{
			name: "hold",
			orders: []DSONOrder{
				{Type: DSONHold, UnitType: Army, Location: "vie"},
			},
			want: "A vie H",
		},
		{
			name: "move",
			orders: []DSONOrder{
				{Type: DSONMove, UnitType: Army, Location: "bud", Target: "rum"},
			},
			want: "A bud - rum",
		},
		{
			name: "fleet move",
			orders: []DSONOrder{
				{Type: DSONMove, UnitType: Fleet, Location: "tri", Target: "adr"},
			},
			want: "F tri - adr",
		},
		{
			name: "support hold",
			orders: []DSONOrder{
				{Type: DSONSupportHold, UnitType: Army, Location: "tyr",
					AuxUnitType: Army, AuxLocation: "vie"},
			},
			want: "A tyr S A vie H",
		},
		{
			name: "support move",
			orders: []DSONOrder{
				{Type: DSONSupportMove, UnitType: Army, Location: "gal",
					AuxUnitType: Army, AuxLocation: "bud",
					AuxTarget: "rum"},
			},
			want: "A gal S A bud - rum",
		},
		{
			name: "convoy",
			orders: []DSONOrder{
				{Type: DSONConvoy, UnitType: Fleet, Location: "mao",
					AuxUnitType: Army, AuxLocation: "bre",
					AuxTarget: "spa"},
			},
			want: "F mao C A bre - spa",
		},
		{
			name: "fleet move to split coast",
			orders: []DSONOrder{
				{Type: DSONMove, UnitType: Fleet, Location: "nrg",
					Target: "stp", TargetCoast: NorthCoast},
			},
			want: "F nrg - stp/nc",
		},
		{
			name: "multiple orders",
			orders: []DSONOrder{
				{Type: DSONMove, UnitType: Army, Location: "vie", Target: "tri"},
				{Type: DSONMove, UnitType: Army, Location: "bud", Target: "ser"},
				{Type: DSONMove, UnitType: Fleet, Location: "tri", Target: "alb"},
			},
			want: "A vie - tri ; A bud - ser ; F tri - alb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDSON(tt.orders)
			if got != tt.want {
				t.Errorf("FormatDSON:\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestFormatDSON_RetreatOrders(t *testing.T) {
	tests := []struct {
		name   string
		orders []DSONOrder
		want   string
	}{
		{
			name: "retreat move",
			orders: []DSONOrder{
				{Type: DSONRetreat, UnitType: Army, Location: "vie", Target: "boh"},
			},
			want: "A vie R boh",
		},
		{
			name: "retreat disband",
			orders: []DSONOrder{
				{Type: DSONDisband, UnitType: Fleet, Location: "tri"},
			},
			want: "F tri D",
		},
		{
			name: "fleet retreat with coast",
			orders: []DSONOrder{
				{Type: DSONRetreat, UnitType: Fleet, Location: "stp", Coast: NorthCoast,
					Target: "nwy"},
			},
			want: "F stp/nc R nwy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDSON(tt.orders)
			if got != tt.want {
				t.Errorf("FormatDSON:\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestFormatDSON_BuildOrders(t *testing.T) {
	tests := []struct {
		name   string
		orders []DSONOrder
		want   string
	}{
		{
			name: "build army",
			orders: []DSONOrder{
				{Type: DSONBuild, UnitType: Army, Location: "vie"},
			},
			want: "A vie B",
		},
		{
			name: "build fleet split coast",
			orders: []DSONOrder{
				{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
			},
			want: "F stp/sc B",
		},
		{
			name: "disband build phase",
			orders: []DSONOrder{
				{Type: DSONDisband, UnitType: Army, Location: "war"},
			},
			want: "A war D",
		},
		{
			name: "waive",
			orders: []DSONOrder{
				{Type: DSONWaive},
			},
			want: "W",
		},
		{
			name: "multiple builds",
			orders: []DSONOrder{
				{Type: DSONBuild, UnitType: Army, Location: "vie"},
				{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
			},
			want: "A vie B ; F stp/sc B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDSON(tt.orders)
			if got != tt.want {
				t.Errorf("FormatDSON:\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestParseDSON_MovementOrders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  DSONOrder
	}{
		{
			name:  "hold",
			input: "A vie H",
			want:  DSONOrder{Type: DSONHold, UnitType: Army, Location: "vie"},
		},
		{
			name:  "move",
			input: "A bud - rum",
			want:  DSONOrder{Type: DSONMove, UnitType: Army, Location: "bud", Target: "rum"},
		},
		{
			name:  "fleet move",
			input: "F tri - adr",
			want:  DSONOrder{Type: DSONMove, UnitType: Fleet, Location: "tri", Target: "adr"},
		},
		{
			name:  "support hold",
			input: "A tyr S A vie H",
			want:  DSONOrder{Type: DSONSupportHold, UnitType: Army, Location: "tyr", AuxUnitType: Army, AuxLocation: "vie"},
		},
		{
			name:  "support move",
			input: "A gal S A bud - rum",
			want:  DSONOrder{Type: DSONSupportMove, UnitType: Army, Location: "gal", AuxUnitType: Army, AuxLocation: "bud", AuxTarget: "rum"},
		},
		{
			name:  "convoy",
			input: "F mao C A bre - spa",
			want:  DSONOrder{Type: DSONConvoy, UnitType: Fleet, Location: "mao", AuxUnitType: Army, AuxLocation: "bre", AuxTarget: "spa"},
		},
		{
			name:  "fleet move split coast",
			input: "F nrg - stp/nc",
			want:  DSONOrder{Type: DSONMove, UnitType: Fleet, Location: "nrg", Target: "stp", TargetCoast: NorthCoast},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders, err := ParseDSON(tt.input)
			if err != nil {
				t.Fatalf("ParseDSON(%q) error: %v", tt.input, err)
			}
			if len(orders) != 1 {
				t.Fatalf("expected 1 order, got %d", len(orders))
			}
			assertDSONOrderEqual(t, tt.want, orders[0])
		})
	}
}

func TestParseDSON_RetreatOrders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  DSONOrder
	}{
		{
			name:  "retreat move",
			input: "A vie R boh",
			want:  DSONOrder{Type: DSONRetreat, UnitType: Army, Location: "vie", Target: "boh"},
		},
		{
			name:  "disband",
			input: "F tri D",
			want:  DSONOrder{Type: DSONDisband, UnitType: Fleet, Location: "tri"},
		},
		{
			name:  "fleet retreat with coast",
			input: "F stp/nc R nwy",
			want:  DSONOrder{Type: DSONRetreat, UnitType: Fleet, Location: "stp", Coast: NorthCoast, Target: "nwy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders, err := ParseDSON(tt.input)
			if err != nil {
				t.Fatalf("ParseDSON(%q) error: %v", tt.input, err)
			}
			if len(orders) != 1 {
				t.Fatalf("expected 1 order, got %d", len(orders))
			}
			assertDSONOrderEqual(t, tt.want, orders[0])
		})
	}
}

func TestParseDSON_BuildOrders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  DSONOrder
	}{
		{
			name:  "build army",
			input: "A vie B",
			want:  DSONOrder{Type: DSONBuild, UnitType: Army, Location: "vie"},
		},
		{
			name:  "build fleet split coast",
			input: "F stp/sc B",
			want:  DSONOrder{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
		},
		{
			name:  "build disband",
			input: "A war D",
			want:  DSONOrder{Type: DSONDisband, UnitType: Army, Location: "war"},
		},
		{
			name:  "waive",
			input: "W",
			want:  DSONOrder{Type: DSONWaive},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders, err := ParseDSON(tt.input)
			if err != nil {
				t.Fatalf("ParseDSON(%q) error: %v", tt.input, err)
			}
			if len(orders) != 1 {
				t.Fatalf("expected 1 order, got %d", len(orders))
			}
			assertDSONOrderEqual(t, tt.want, orders[0])
		})
	}
}

func TestParseDSON_MultipleOrders(t *testing.T) {
	input := "A vie - tri ; A bud - ser ; F tri - alb"
	orders, err := ParseDSON(input)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}

	assertDSONOrderEqual(t, DSONOrder{Type: DSONMove, UnitType: Army, Location: "vie", Target: "tri"}, orders[0])
	assertDSONOrderEqual(t, DSONOrder{Type: DSONMove, UnitType: Army, Location: "bud", Target: "ser"}, orders[1])
	assertDSONOrderEqual(t, DSONOrder{Type: DSONMove, UnitType: Fleet, Location: "tri", Target: "alb"}, orders[2])
}

func TestParseDSON_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid unit type", "X vie H"},
		{"too short", "A"},
		{"missing action", "A vie"},
		{"bad province", "A vien H"},
		{"bad move target", "A vie - xxxx"},
		{"support too short", "A gal S A"},
		{"convoy no dash", "F mao C A bre = spa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDSON(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestDSON_RoundTrip_Movement(t *testing.T) {
	orders := []DSONOrder{
		{Type: DSONHold, UnitType: Army, Location: "vie"},
		{Type: DSONMove, UnitType: Army, Location: "bud", Target: "rum"},
		{Type: DSONSupportHold, UnitType: Army, Location: "tyr", AuxUnitType: Army, AuxLocation: "vie"},
		{Type: DSONSupportMove, UnitType: Army, Location: "gal", AuxUnitType: Army, AuxLocation: "bud", AuxTarget: "rum"},
		{Type: DSONConvoy, UnitType: Fleet, Location: "mao", AuxUnitType: Army, AuxLocation: "bre", AuxTarget: "spa"},
		{Type: DSONMove, UnitType: Fleet, Location: "nrg", Target: "stp", TargetCoast: NorthCoast},
	}

	formatted := FormatDSON(orders)
	parsed, err := ParseDSON(formatted)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(parsed) != len(orders) {
		t.Fatalf("count: got %d, want %d", len(parsed), len(orders))
	}
	for i := range orders {
		assertDSONOrderEqual(t, orders[i], parsed[i])
	}
}

func TestDSON_RoundTrip_Retreat(t *testing.T) {
	orders := []DSONOrder{
		{Type: DSONRetreat, UnitType: Army, Location: "vie", Target: "boh"},
		{Type: DSONDisband, UnitType: Fleet, Location: "tri"},
		{Type: DSONRetreat, UnitType: Fleet, Location: "stp", Coast: NorthCoast, Target: "nwy"},
	}

	formatted := FormatDSON(orders)
	parsed, err := ParseDSON(formatted)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(parsed) != len(orders) {
		t.Fatalf("count: got %d, want %d", len(parsed), len(orders))
	}
	for i := range orders {
		assertDSONOrderEqual(t, orders[i], parsed[i])
	}
}

func TestDSON_RoundTrip_Build(t *testing.T) {
	orders := []DSONOrder{
		{Type: DSONBuild, UnitType: Army, Location: "vie"},
		{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
		{Type: DSONDisband, UnitType: Army, Location: "war"},
		{Type: DSONWaive},
	}

	formatted := FormatDSON(orders)
	parsed, err := ParseDSON(formatted)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(parsed) != len(orders) {
		t.Fatalf("count: got %d, want %d", len(parsed), len(orders))
	}
	for i := range orders {
		assertDSONOrderEqual(t, orders[i], parsed[i])
	}
}

func TestParseDSON_EmptyInput(t *testing.T) {
	orders, err := ParseDSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestOrderToDSON_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		order Order
		want  DSONOrder
	}{
		{
			name:  "hold",
			order: Order{UnitType: Army, Power: Austria, Location: "vie", Type: OrderHold},
			want:  DSONOrder{Type: DSONHold, UnitType: Army, Location: "vie"},
		},
		{
			name:  "move",
			order: Order{UnitType: Army, Power: Austria, Location: "bud", Type: OrderMove, Target: "rum"},
			want:  DSONOrder{Type: DSONMove, UnitType: Army, Location: "bud", Target: "rum"},
		},
		{
			name:  "support hold",
			order: Order{UnitType: Army, Power: Austria, Location: "tyr", Type: OrderSupport, AuxUnitType: Army, AuxLoc: "vie"},
			want:  DSONOrder{Type: DSONSupportHold, UnitType: Army, Location: "tyr", AuxUnitType: Army, AuxLocation: "vie"},
		},
		{
			name:  "support move",
			order: Order{UnitType: Army, Power: Austria, Location: "gal", Type: OrderSupport, AuxUnitType: Army, AuxLoc: "bud", AuxTarget: "rum"},
			want:  DSONOrder{Type: DSONSupportMove, UnitType: Army, Location: "gal", AuxUnitType: Army, AuxLocation: "bud", AuxTarget: "rum"},
		},
		{
			name:  "convoy",
			order: Order{UnitType: Fleet, Power: France, Location: "mao", Type: OrderConvoy, AuxLoc: "bre", AuxTarget: "spa"},
			want:  DSONOrder{Type: DSONConvoy, UnitType: Fleet, Location: "mao", AuxUnitType: Army, AuxLocation: "bre", AuxTarget: "spa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrderToDSON(tt.order)
			assertDSONOrderEqual(t, tt.want, got)
		})
	}
}

func TestRetreatOrderToDSON(t *testing.T) {
	tests := []struct {
		name  string
		order RetreatOrder
		want  DSONOrder
	}{
		{
			name:  "retreat move",
			order: RetreatOrder{UnitType: Army, Power: Austria, Location: "vie", Type: RetreatMove, Target: "boh"},
			want:  DSONOrder{Type: DSONRetreat, UnitType: Army, Location: "vie", Target: "boh"},
		},
		{
			name:  "retreat disband",
			order: RetreatOrder{UnitType: Fleet, Power: England, Location: "tri", Type: RetreatDisband},
			want:  DSONOrder{Type: DSONDisband, UnitType: Fleet, Location: "tri"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RetreatOrderToDSON(tt.order)
			assertDSONOrderEqual(t, tt.want, got)
		})
	}
}

func TestBuildOrderToDSON(t *testing.T) {
	tests := []struct {
		name  string
		order BuildOrder
		want  DSONOrder
	}{
		{
			name:  "build army",
			order: BuildOrder{Power: Austria, Type: BuildUnit, UnitType: Army, Location: "vie"},
			want:  DSONOrder{Type: DSONBuild, UnitType: Army, Location: "vie"},
		},
		{
			name:  "build fleet split coast",
			order: BuildOrder{Power: Russia, Type: BuildUnit, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
			want:  DSONOrder{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast},
		},
		{
			name:  "disband",
			order: BuildOrder{Power: Russia, Type: DisbandUnit, UnitType: Army, Location: "war"},
			want:  DSONOrder{Type: DSONDisband, UnitType: Army, Location: "war"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildOrderToDSON(tt.order)
			assertDSONOrderEqual(t, tt.want, got)
		})
	}
}

func TestDSONToOrder(t *testing.T) {
	d := DSONOrder{Type: DSONMove, UnitType: Army, Location: "bud", Target: "rum"}
	o := DSONToOrder(d, Austria)

	if o.Type != OrderMove {
		t.Errorf("type: got %v, want move", o.Type)
	}
	if o.Power != Austria {
		t.Errorf("power: got %v, want austria", o.Power)
	}
	if o.Location != "bud" {
		t.Errorf("location: got %v, want bud", o.Location)
	}
	if o.Target != "rum" {
		t.Errorf("target: got %v, want rum", o.Target)
	}
}

func TestDSONToRetreatOrder(t *testing.T) {
	d := DSONOrder{Type: DSONRetreat, UnitType: Army, Location: "vie", Target: "boh"}
	o := DSONToRetreatOrder(d, Austria)

	if o.Type != RetreatMove {
		t.Errorf("type: got %v, want RetreatMove", o.Type)
	}
	if o.Power != Austria {
		t.Errorf("power: got %v, want austria", o.Power)
	}
	if o.Target != "boh" {
		t.Errorf("target: got %v, want boh", o.Target)
	}
}

func TestDSONToBuildOrder(t *testing.T) {
	d := DSONOrder{Type: DSONBuild, UnitType: Fleet, Location: "stp", Coast: SouthCoast}
	o := DSONToBuildOrder(d, Russia)

	if o.Type != BuildUnit {
		t.Errorf("type: got %v, want BuildUnit", o.Type)
	}
	if o.Power != Russia {
		t.Errorf("power: got %v, want russia", o.Power)
	}
	if o.Location != "stp" {
		t.Errorf("location: got %v, want stp", o.Location)
	}
	if o.Coast != SouthCoast {
		t.Errorf("coast: got %v, want sc", o.Coast)
	}
}

func TestDSON_SupportFleetHold(t *testing.T) {
	input := "A tyr S F tri H"
	orders, err := ParseDSON(input)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	o := orders[0]
	if o.Type != DSONSupportHold {
		t.Errorf("type: got %v, want DSONSupportHold", o.Type)
	}
	if o.AuxUnitType != Fleet {
		t.Errorf("aux unit type: got %v, want Fleet", o.AuxUnitType)
	}
	if o.AuxLocation != "tri" {
		t.Errorf("aux location: got %v, want tri", o.AuxLocation)
	}
}

func TestDSON_SupportFleetMove(t *testing.T) {
	input := "A pie S F mar - spa/sc"
	orders, err := ParseDSON(input)
	if err != nil {
		t.Fatalf("ParseDSON error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	o := orders[0]
	if o.Type != DSONSupportMove {
		t.Errorf("type: got %v, want DSONSupportMove", o.Type)
	}
	if o.AuxUnitType != Fleet {
		t.Errorf("aux unit type: got %v, want Fleet", o.AuxUnitType)
	}
	if o.AuxTarget != "spa" {
		t.Errorf("aux target: got %v, want spa", o.AuxTarget)
	}
	if o.AuxTargetCoast != SouthCoast {
		t.Errorf("aux target coast: got %v, want sc", o.AuxTargetCoast)
	}
}

func FuzzDSON_RoundTrip(f *testing.F) {
	f.Add("A vie H")
	f.Add("A bud - rum")
	f.Add("F nrg - stp/nc")
	f.Add("A gal S A bud - rum")
	f.Add("A tyr S A vie H")
	f.Add("F mao C A bre - spa")
	f.Add("A vie R boh")
	f.Add("F tri D")
	f.Add("A vie B")
	f.Add("F stp/sc B")
	f.Add("W")
	f.Add("A vie - tri ; A bud - ser ; F tri - alb")

	f.Fuzz(func(t *testing.T, dson string) {
		orders, err := ParseDSON(dson)
		if err != nil {
			return
		}

		formatted := FormatDSON(orders)
		orders2, err := ParseDSON(formatted)
		if err != nil {
			t.Fatalf("second parse failed: %v (formatted=%q)", err, formatted)
		}

		formatted2 := FormatDSON(orders2)
		if formatted != formatted2 {
			t.Fatalf("round-trip not stable:\nfirst:  %s\nsecond: %s", formatted, formatted2)
		}
	})
}

// assertDSONOrderEqual compares two DSONOrders field by field.
func assertDSONOrderEqual(t *testing.T, want, got DSONOrder) {
	t.Helper()
	if want.Type != got.Type {
		t.Errorf("Type: want %v, got %v", want.Type, got.Type)
	}
	if want.UnitType != got.UnitType {
		t.Errorf("UnitType: want %v, got %v", want.UnitType, got.UnitType)
	}
	if want.Location != got.Location {
		t.Errorf("Location: want %q, got %q", want.Location, got.Location)
	}
	if want.Coast != got.Coast {
		t.Errorf("Coast: want %q, got %q", want.Coast, got.Coast)
	}
	if want.Target != got.Target {
		t.Errorf("Target: want %q, got %q", want.Target, got.Target)
	}
	if want.TargetCoast != got.TargetCoast {
		t.Errorf("TargetCoast: want %q, got %q", want.TargetCoast, got.TargetCoast)
	}
	if want.AuxUnitType != got.AuxUnitType {
		t.Errorf("AuxUnitType: want %v, got %v", want.AuxUnitType, got.AuxUnitType)
	}
	if want.AuxLocation != got.AuxLocation {
		t.Errorf("AuxLocation: want %q, got %q", want.AuxLocation, got.AuxLocation)
	}
	if want.AuxCoast != got.AuxCoast {
		t.Errorf("AuxCoast: want %q, got %q", want.AuxCoast, got.AuxCoast)
	}
	if want.AuxTarget != got.AuxTarget {
		t.Errorf("AuxTarget: want %q, got %q", want.AuxTarget, got.AuxTarget)
	}
	if want.AuxTargetCoast != got.AuxTargetCoast {
		t.Errorf("AuxTargetCoast: want %q, got %q", want.AuxTargetCoast, got.AuxTargetCoast)
	}
}
