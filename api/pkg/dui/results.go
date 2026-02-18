package dui

import (
	"fmt"
	"strconv"
	"strings"
)

// Info represents a single "info" line emitted by the engine during search.
type Info struct {
	Depth int
	Nodes int
	NPS   int
	Time  int
	Score int
	PV    string
}

// SearchResults holds the accumulated output from a Go command:
// all info lines received during search plus the final bestorders.
type SearchResults struct {
	BestOrders string
	Infos      []Info
}

// EngineID holds the engine identification received during handshake.
type EngineID struct {
	Name            string
	Author          string
	ProtocolVersion int
}

// EngineOption describes a configuration option advertised by the engine.
type EngineOption struct {
	Name    string
	Type    string
	Default string
	Min     string
	Max     string
	Vars    []string
}

// GoParams configures search constraints for the Go command.
type GoParams struct {
	MoveTime int  // milliseconds; 0 means use engine default
	Depth    int  // search depth limit; 0 means unlimited
	Nodes    int  // node count limit; 0 means unlimited
	Infinite bool // search until stop is sent
}

// String formats GoParams as a DUI "go" command suffix.
func (p GoParams) String() string {
	if p.Infinite {
		return "infinite"
	}
	var parts []string
	if p.MoveTime > 0 {
		parts = append(parts, fmt.Sprintf("movetime %d", p.MoveTime))
	}
	if p.Depth > 0 {
		parts = append(parts, fmt.Sprintf("depth %d", p.Depth))
	}
	if p.Nodes > 0 {
		parts = append(parts, fmt.Sprintf("nodes %d", p.Nodes))
	}
	return strings.Join(parts, " ")
}

// parseInfo parses an "info" line from the engine into an Info struct.
// Fields not present in the line are left as zero values.
func parseInfo(line string) Info {
	var info Info
	tokens := strings.Fields(line)
	for i := 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "info":
			continue
		case "depth":
			if i+1 < len(tokens) {
				info.Depth, _ = strconv.Atoi(tokens[i+1])
				i++
			}
		case "nodes":
			if i+1 < len(tokens) {
				info.Nodes, _ = strconv.Atoi(tokens[i+1])
				i++
			}
		case "nps":
			if i+1 < len(tokens) {
				info.NPS, _ = strconv.Atoi(tokens[i+1])
				i++
			}
		case "time":
			if i+1 < len(tokens) {
				info.Time, _ = strconv.Atoi(tokens[i+1])
				i++
			}
		case "score":
			if i+1 < len(tokens) {
				info.Score, _ = strconv.Atoi(tokens[i+1])
				i++
			}
		case "pv":
			// PV is the rest of the line after "pv ".
			info.PV = strings.Join(tokens[i+1:], " ")
			return info
		}
	}
	return info
}

// parseEngineOption parses an "option" line from the engine handshake.
// Format: option name <id> type <type> [default <x>] [min <x>] [max <x>] [var <x> ...]
func parseEngineOption(line string) EngineOption {
	var opt EngineOption
	tokens := strings.Fields(line)

	for i := 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "option":
			continue
		case "name":
			if i+1 < len(tokens) {
				i++
				opt.Name = tokens[i]
			}
		case "type":
			if i+1 < len(tokens) {
				i++
				opt.Type = tokens[i]
			}
		case "default":
			if i+1 < len(tokens) {
				i++
				opt.Default = tokens[i]
			}
		case "min":
			if i+1 < len(tokens) {
				i++
				opt.Min = tokens[i]
			}
		case "max":
			if i+1 < len(tokens) {
				i++
				opt.Max = tokens[i]
			}
		case "var":
			if i+1 < len(tokens) {
				i++
				opt.Vars = append(opt.Vars, tokens[i])
			}
		}
	}
	return opt
}
