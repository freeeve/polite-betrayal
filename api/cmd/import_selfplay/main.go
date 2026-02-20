// Command import_selfplay reads self-play JSONL game data and imports it
// into the Postgres database so games are viewable in the UI.
//
// Usage:
//
//	go run ./cmd/import_selfplay/ --input games.jsonl --db postgres://...
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/freeeve/polite-betrayal/api/internal/model"
	"github.com/freeeve/polite-betrayal/api/internal/repository/postgres"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// powerOrder matches the Rust ALL_POWERS ordering used for sc_counts/values arrays.
var powerOrder = []diplomacy.Power{
	diplomacy.Austria, diplomacy.England, diplomacy.France,
	diplomacy.Germany, diplomacy.Italy, diplomacy.Russia, diplomacy.Turkey,
}

// jsonGameRecord is the JSON representation of a GameRecord from the Rust selfplay binary.
type jsonGameRecord struct {
	GameID       int              `json:"game_id"`
	Winner       *string          `json:"winner"` // null for draw
	FinalYear    int              `json:"final_year"`
	FinalSCCount []int            `json:"final_sc_counts"`
	Quality      json.RawMessage  `json:"quality"`
	Phases       []jsonPhaseEntry `json:"phases"`
}

// jsonPhaseEntry is the JSON representation of a PhaseRecord.
type jsonPhaseEntry struct {
	DFEN     string            `json:"dfen"`
	Year     int               `json:"year"`
	Season   string            `json:"season"`
	Phase    string            `json:"phase"`
	Orders   map[string]string `json:"orders"` // power -> DSON
	Values   []float64         `json:"values"`
	SCCounts []int             `json:"sc_counts"`
}

func main() {
	inputFile := flag.String("input", "", "Path to JSONL file")
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres connection URL")
	namePrefix := flag.String("name-prefix", "selfplay", "Game name prefix")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("--input is required")
	}
	if *dbURL == "" {
		log.Fatal("--db or DATABASE_URL is required")
	}

	db, err := postgres.Connect(*dbURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer db.Close()

	gameRepo := postgres.NewGameRepo(db)
	phaseRepo := postgres.NewPhaseRepo(db)
	userRepo := postgres.NewUserRepo(db)

	f, err := os.Open(*inputFile)
	if err != nil {
		log.Fatalf("open input: %v", err)
	}
	defer f.Close()

	ctx := context.Background()
	scanner := bufio.NewScanner(f)
	// Allow large lines (self-play games can be large).
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	imported := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var rec jsonGameRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			log.Printf("WARN: skip line (bad JSON): %v", err)
			continue
		}

		gameName := fmt.Sprintf("%s-%03d", *namePrefix, rec.GameID)
		gameID, err := importGame(ctx, gameRepo, phaseRepo, userRepo, rec, gameName)
		if err != nil {
			log.Printf("ERROR: import game %d: %v", rec.GameID, err)
			continue
		}

		imported++
		log.Printf("imported game %d -> %s (id=%s, %d phases)", rec.GameID, gameName, gameID, len(rec.Phases))
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("read input: %v", err)
	}

	log.Printf("done: imported %d games", imported)
}

// importGame creates a game, players, and phases in the database.
func importGame(
	ctx context.Context,
	gameRepo *postgres.GameRepo,
	phaseRepo *postgres.PhaseRepo,
	userRepo *postgres.UserRepo,
	rec jsonGameRecord,
	gameName string,
) (string, error) {
	// Create bot users for each power.
	type botInfo struct {
		userID string
		power  diplomacy.Power
	}
	var bots []botInfo
	for _, power := range powerOrder {
		providerID := fmt.Sprintf("selfplay-%s", power)
		displayName := fmt.Sprintf("Selfplay %s", power)
		user, err := userRepo.Upsert(ctx, "bot", providerID, displayName, "")
		if err != nil {
			return "", fmt.Errorf("upsert bot %s: %w", power, err)
		}
		bots = append(bots, botInfo{userID: user.ID, power: power})
	}

	// Create the game.
	game, err := gameRepo.Create(ctx, gameName, bots[0].userID, "1 hours", "1 hours", "1 hours", "manual")
	if err != nil {
		return "", fmt.Errorf("create game: %w", err)
	}

	// Join all bots.
	for _, b := range bots {
		if err := gameRepo.JoinGameAsBot(ctx, game.ID, b.userID, "realpolitik"); err != nil {
			return "", fmt.Errorf("join bot %s: %w", b.power, err)
		}
	}

	// Assign powers.
	assignments := make(map[string]string)
	for _, b := range bots {
		assignments[b.userID] = string(b.power)
	}
	if err := gameRepo.AssignPowers(ctx, game.ID, assignments); err != nil {
		return "", fmt.Errorf("assign powers: %w", err)
	}

	// Import phases.
	for i, pe := range rec.Phases {
		if err := importPhase(ctx, phaseRepo, game.ID, pe, rec.Phases, i); err != nil {
			return "", fmt.Errorf("import phase %d: %w", i, err)
		}
	}

	// Mark game finished.
	winner := ""
	if rec.Winner != nil {
		winner = *rec.Winner
	}
	if err := gameRepo.SetFinished(ctx, game.ID, winner); err != nil {
		return "", fmt.Errorf("set finished: %w", err)
	}

	return game.ID, nil
}

// importPhase creates a single phase record with state_before, state_after, and orders.
func importPhase(
	ctx context.Context,
	phaseRepo *postgres.PhaseRepo,
	gameID string,
	pe jsonPhaseEntry,
	allPhases []jsonPhaseEntry,
	idx int,
) error {
	// Decode DFEN to get state_before.
	gsBefore, err := diplomacy.DecodeDFEN(pe.DFEN)
	if err != nil {
		return fmt.Errorf("decode DFEN: %w", err)
	}
	stateBefore, err := json.Marshal(gsBefore)
	if err != nil {
		return fmt.Errorf("marshal state_before: %w", err)
	}

	season := expandSeason(pe.Season)
	phaseType := expandPhase(pe.Phase)

	deadline := time.Now().Add(-24 * time.Hour) // dummy past deadline
	phase, err := phaseRepo.CreatePhase(ctx, gameID, pe.Year, season, phaseType, stateBefore, deadline)
	if err != nil {
		return fmt.Errorf("create phase: %w", err)
	}

	// Compute state_after: use the next phase's DFEN, or for the last phase use the same DFEN.
	var stateAfter json.RawMessage
	if idx+1 < len(allPhases) {
		gsAfter, err := diplomacy.DecodeDFEN(allPhases[idx+1].DFEN)
		if err != nil {
			// Fall back to current state.
			stateAfter = stateBefore
		} else {
			stateAfter, err = json.Marshal(gsAfter)
			if err != nil {
				stateAfter = stateBefore
			}
		}
	} else {
		// Last phase: state_after = state_before (game ended).
		stateAfter = stateBefore
	}

	if err := phaseRepo.ResolvePhase(ctx, phase.ID, stateAfter); err != nil {
		return fmt.Errorf("resolve phase: %w", err)
	}

	// Parse and save orders.
	var modelOrders []model.Order
	for power, dsonStr := range pe.Orders {
		orders := parseDSONOrders(dsonStr, power, phase.ID)
		modelOrders = append(modelOrders, orders...)
	}
	if len(modelOrders) > 0 {
		if err := phaseRepo.SaveOrders(ctx, modelOrders); err != nil {
			return fmt.Errorf("save orders: %w", err)
		}
	}

	return nil
}

// parseDSONOrders parses a DSON string (semicolon-separated) into model.Order entries.
// DSON format examples:
//
//	"A vie - tri ; A bud - ser ; F tri - alb"
//	"A vie H"
//	"A vie S A bud - rum"
//	"F mao C A bre - spa"
//	"A vie R boh"  (retreat)
//	"A vie B"      (build)
//	"A vie D"      (disband)
//	"W"            (waive)
func parseDSONOrders(dson, power, phaseID string) []model.Order {
	dson = strings.TrimSpace(dson)
	if dson == "" {
		return nil
	}

	parts := strings.Split(dson, " ; ")
	var orders []model.Order
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		o, ok := parseSingleDSON(part, power, phaseID)
		if ok {
			orders = append(orders, o)
		}
	}
	return orders
}

// parseSingleDSON parses one DSON order string into a model.Order.
func parseSingleDSON(s, power, phaseID string) (model.Order, bool) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return model.Order{}, false
	}

	// Waive: standalone "W"
	if tokens[0] == "W" {
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  "army",
			Location:  "",
			OrderType: "waive",
			Result:    "succeeds",
		}, true
	}

	if len(tokens) < 3 {
		return model.Order{}, false
	}

	unitType := dsonUnitType(tokens[0])
	location, coast := splitLocation(tokens[1])
	_ = coast // coast stored as part of location for display

	action := tokens[2]

	switch action {
	case "H":
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  unitType,
			Location:  location,
			OrderType: "hold",
			Result:    "succeeds",
		}, true

	case "-":
		// Move: unit - dest
		if len(tokens) < 4 {
			return model.Order{}, false
		}
		target, _ := splitLocation(tokens[3])
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  unitType,
			Location:  location,
			OrderType: "move",
			Target:    target,
			Result:    "succeeds",
		}, true

	case "S":
		// Support: unit S supported_unit (H | - dest)
		if len(tokens) < 6 {
			return model.Order{}, false
		}
		auxUnitType := dsonUnitType(tokens[3])
		auxLoc, _ := splitLocation(tokens[4])
		subAction := tokens[5]
		if subAction == "H" {
			// Support hold
			return model.Order{
				PhaseID:     phaseID,
				Power:       power,
				UnitType:    unitType,
				Location:    location,
				OrderType:   "support",
				AuxUnitType: auxUnitType,
				AuxLoc:      auxLoc,
				AuxTarget:   auxLoc,
				Result:      "succeeds",
			}, true
		}
		if subAction == "-" && len(tokens) >= 7 {
			// Support move
			auxTarget, _ := splitLocation(tokens[6])
			return model.Order{
				PhaseID:     phaseID,
				Power:       power,
				UnitType:    unitType,
				Location:    location,
				OrderType:   "support",
				Target:      auxTarget,
				AuxUnitType: auxUnitType,
				AuxLoc:      auxLoc,
				AuxTarget:   auxTarget,
				Result:      "succeeds",
			}, true
		}
		return model.Order{}, false

	case "C":
		// Convoy: unit C A from - to
		if len(tokens) < 7 {
			return model.Order{}, false
		}
		auxLoc, _ := splitLocation(tokens[4])
		// tokens[5] should be "-"
		auxTarget, _ := splitLocation(tokens[6])
		return model.Order{
			PhaseID:     phaseID,
			Power:       power,
			UnitType:    unitType,
			Location:    location,
			OrderType:   "convoy",
			Target:      auxTarget,
			AuxLoc:      auxLoc,
			AuxTarget:   auxTarget,
			AuxUnitType: "army",
			Result:      "succeeds",
		}, true

	case "R":
		// Retreat: unit R dest
		if len(tokens) < 4 {
			return model.Order{}, false
		}
		target, _ := splitLocation(tokens[3])
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  unitType,
			Location:  location,
			OrderType: "retreat_move",
			Target:    target,
			Result:    "succeeds",
		}, true

	case "D":
		// Disband
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  unitType,
			Location:  location,
			OrderType: "retreat_disband",
			Result:    "succeeds",
		}, true

	case "B":
		// Build
		return model.Order{
			PhaseID:   phaseID,
			Power:     power,
			UnitType:  unitType,
			Location:  location,
			OrderType: "build",
			Result:    "succeeds",
		}, true
	}

	return model.Order{}, false
}

// dsonUnitType converts "A"/"F" to "army"/"fleet".
func dsonUnitType(s string) string {
	if s == "F" {
		return "fleet"
	}
	return "army"
}

// splitLocation splits "stp/nc" into ("stp", "nc") or "vie" into ("vie", "").
func splitLocation(s string) (string, string) {
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// expandSeason converts "s"/"f" to "spring"/"fall".
func expandSeason(s string) string {
	switch s {
	case "f":
		return "fall"
	default:
		return "spring"
	}
}

// expandPhase converts "m"/"r"/"b" to "movement"/"retreat"/"build".
func expandPhase(s string) string {
	switch s {
	case "r":
		return "retreat"
	case "b":
		return "build"
	default:
		return "movement"
	}
}
