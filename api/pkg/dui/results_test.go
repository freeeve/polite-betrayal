package dui

import (
	"testing"
)

func TestParseInfo(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Info
	}{
		{
			name: "full info line",
			line: "info depth 3 nodes 120000 nps 40000 score 12 time 3200",
			want: Info{Depth: 3, Nodes: 120000, NPS: 40000, Score: 12, Time: 3200},
		},
		{
			name: "partial info line",
			line: "info depth 1 nodes 100 time 50",
			want: Info{Depth: 1, Nodes: 100, Time: 50},
		},
		{
			name: "info with pv",
			line: "info depth 2 nodes 5000 score 5 time 300 pv A vie - tri ; A bud - ser",
			want: Info{Depth: 2, Nodes: 5000, Score: 5, Time: 300, PV: "A vie - tri ; A bud - ser"},
		},
		{
			name: "empty info",
			line: "info",
			want: Info{},
		},
		{
			name: "score only",
			line: "info score 15",
			want: Info{Score: 15},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInfo(tt.line)
			if got.Depth != tt.want.Depth {
				t.Errorf("Depth = %d, want %d", got.Depth, tt.want.Depth)
			}
			if got.Nodes != tt.want.Nodes {
				t.Errorf("Nodes = %d, want %d", got.Nodes, tt.want.Nodes)
			}
			if got.NPS != tt.want.NPS {
				t.Errorf("NPS = %d, want %d", got.NPS, tt.want.NPS)
			}
			if got.Time != tt.want.Time {
				t.Errorf("Time = %d, want %d", got.Time, tt.want.Time)
			}
			if got.Score != tt.want.Score {
				t.Errorf("Score = %d, want %d", got.Score, tt.want.Score)
			}
			if got.PV != tt.want.PV {
				t.Errorf("PV = %q, want %q", got.PV, tt.want.PV)
			}
		})
	}
}

func TestParseEngineOption(t *testing.T) {
	tests := []struct {
		name string
		line string
		want EngineOption
	}{
		{
			name: "spin option",
			line: "option name Threads type spin default 4 min 1 max 64",
			want: EngineOption{Name: "Threads", Type: "spin", Default: "4", Min: "1", Max: "64"},
		},
		{
			name: "string option",
			line: "option name ModelPath type string default models/v1.onnx",
			want: EngineOption{Name: "ModelPath", Type: "string", Default: "models/v1.onnx"},
		},
		{
			name: "combo option",
			line: "option name Personality type combo default balanced var aggressive var defensive var balanced",
			want: EngineOption{
				Name:    "Personality",
				Type:    "combo",
				Default: "balanced",
				Vars:    []string{"aggressive", "defensive", "balanced"},
			},
		},
		{
			name: "check option",
			line: "option name UseBook type check default true",
			want: EngineOption{Name: "UseBook", Type: "check", Default: "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEngineOption(tt.line)
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Default != tt.want.Default {
				t.Errorf("Default = %q, want %q", got.Default, tt.want.Default)
			}
			if got.Min != tt.want.Min {
				t.Errorf("Min = %q, want %q", got.Min, tt.want.Min)
			}
			if got.Max != tt.want.Max {
				t.Errorf("Max = %q, want %q", got.Max, tt.want.Max)
			}
			if len(got.Vars) != len(tt.want.Vars) {
				t.Errorf("Vars count = %d, want %d", len(got.Vars), len(tt.want.Vars))
			} else {
				for i, v := range got.Vars {
					if v != tt.want.Vars[i] {
						t.Errorf("Vars[%d] = %q, want %q", i, v, tt.want.Vars[i])
					}
				}
			}
		})
	}
}
