package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/bot"
	"github.com/efreeman/polite-betrayal/api/internal/repository/postgres"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func main() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	var (
		powerCfg string
		matchup  string
		numGames int
		workers  int
		dbURL    string
		maxYear  int
		seed     int64
		dryRun   bool
		jsonOut  bool
	)

	flag.StringVar(&powerCfg, "p", "", "Power config (e.g. france=hard,*=easy)")
	flag.StringVar(&matchup, "matchup", "", "Shorthand tier-vs-tier (e.g. hard-vs-easy)")
	flag.IntVar(&numGames, "n", 1, "Number of games to run")
	flag.IntVar(&workers, "workers", 1, "Concurrency (parallel games)")
	flag.StringVar(&dbURL, "db", "", "Database URL (or use DATABASE_URL env)")
	flag.IntVar(&maxYear, "max-year", 1920, "Max year before draw")
	flag.Int64Var(&seed, "seed", 0, "Base seed (0 = random)")
	flag.BoolVar(&dryRun, "dry-run", false, "Skip database writes")
	flag.BoolVar(&jsonOut, "json", false, "Output results as JSON")

	flag.Parse()

	// Resolve power config
	var powers map[diplomacy.Power]string
	switch {
	case powerCfg != "":
		powers = bot.ParsePowerConfig(powerCfg)
	case matchup != "":
		powers = parseTierVsTier(matchup)
	default:
		powers = bot.ParsePowerConfig("*=easy")
	}

	// Resolve DB URL
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable"
	}

	// Build game label
	label := buildLabel(powers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info().Msg("Shutting down...")
		cancel()
	}()

	// Connect to DB (unless dry-run)
	var gameRepo *postgres.GameRepo
	var phaseRepo *postgres.PhaseRepo
	var userRepo *postgres.UserRepo

	if !dryRun {
		db, err := postgres.Connect(dbURL)
		if err != nil {
			log.Fatal().Err(err).Msg("Database connection failed")
		}
		defer db.Close()
		gameRepo = postgres.NewGameRepo(db)
		phaseRepo = postgres.NewPhaseRepo(db)
		userRepo = postgres.NewUserRepo(db)
	}

	// Run games
	results := make([]*bot.ArenaResult, numGames)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	errCount := 0

	for i := 0; i < numGames; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			gameSeed := seed
			if seed != 0 {
				gameSeed = seed + int64(idx)
			}

			cfg := bot.ArenaConfig{
				GameName:    fmt.Sprintf("%s-%d", label, idx+1),
				PowerConfig: powers,
				MaxYear:     maxYear,
				Seed:        gameSeed,
				DryRun:      dryRun,
			}

			result, err := bot.RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
			if err != nil {
				log.Error().Err(err).Int("game", idx+1).Msg("Game failed")
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = result
			mu.Unlock()

			log.Info().Int("game", idx+1).Str("winner", result.Winner).Int("phases", result.TotalPhases).Int("year", result.FinalYear).Msg("Game completed")
		}(i)
	}

	wg.Wait()

	if jsonOut {
		printJSON(results, numGames, errCount)
	} else {
		printSummary(results, powers, maxYear, errCount, label, dryRun)
	}
}

// parseTierVsTier handles "hard-vs-easy" style matchup strings.
func parseTierVsTier(s string) map[diplomacy.Power]string {
	parts := strings.SplitN(s, "-vs-", 2)
	if len(parts) != 2 {
		// Treat as uniform difficulty
		return bot.ParsePowerConfig("*=" + s)
	}
	// First tier goes to a single power (france by default), rest get second tier
	return bot.ParsePowerConfig(fmt.Sprintf("france=%s,*=%s", parts[0], parts[1]))
}

func buildLabel(powers map[diplomacy.Power]string) string {
	diffs := make(map[string]int)
	for _, d := range powers {
		diffs[d]++
	}
	if len(diffs) == 1 {
		for d := range diffs {
			return fmt.Sprintf("botmatch: all-%s", d)
		}
	}

	// For 1-vs-many matchups, include the solo power's name
	if len(diffs) == 2 {
		for d, c := range diffs {
			if c == 1 {
				// Find which power has this difficulty
				for p, pd := range powers {
					if pd == d {
						otherDiff := ""
						otherCount := 0
						for od, oc := range diffs {
							if od != d {
								otherDiff = od
								otherCount = oc
							}
						}
						otherName := otherDiff
						if otherCount > 1 {
							otherName += "s"
						}
						return fmt.Sprintf("%s: %s vs %d %s", d, p, otherCount, otherName)
					}
				}
			}
		}
	}

	var parts []string
	for d, c := range diffs {
		name := d
		if c > 1 {
			name += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", c, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, " vs ")
}

func printSummary(results []*bot.ArenaResult, powers map[diplomacy.Power]string, maxYear, errCount int, label string, dryRun bool) {
	// Aggregate stats
	type stats struct {
		wins     int
		draws    int
		survived int
		totalSC  int
		games    int
	}

	byPower := make(map[string]*stats)
	for _, p := range diplomacy.AllPowers() {
		byPower[string(p)] = &stats{}
	}

	completed := 0
	for _, r := range results {
		if r == nil {
			continue
		}
		completed++
		for _, p := range diplomacy.AllPowers() {
			ps := string(p)
			s := byPower[ps]
			s.games++
			s.totalSC += r.SCCounts[ps]
			if r.Winner == ps {
				s.wins++
			} else if r.Winner == "" {
				s.draws++
			} else if r.SCCounts[ps] > 0 {
				s.survived++
			}
		}
	}

	fmt.Printf("\nResults (%d games, max year %d):\n", completed, maxYear)
	if errCount > 0 {
		fmt.Printf("  (%d games failed)\n", errCount)
	}

	for _, p := range diplomacy.AllPowers() {
		ps := string(p)
		s := byPower[ps]
		diff := powers[p]
		avgSC := 0.0
		if s.games > 0 {
			avgSC = float64(s.totalSC) / float64(s.games)
		}
		fmt.Printf("  %-10s (%s):  %d wins, %d draws, %d survived  -- avg SCs: %.1f\n",
			ps, diff, s.wins, s.draws, s.survived, avgSC)
	}

	if !dryRun && completed > 0 {
		fmt.Printf("\nGames saved to database -- review in UI under \"%s #1\" through \"#%d\"\n", label, completed)
	}
}

func printJSON(results []*bot.ArenaResult, total, errCount int) {
	out := struct {
		Total   int                `json:"total"`
		Errors  int                `json:"errors"`
		Results []*bot.ArenaResult `json:"results"`
	}{
		Total:   total,
		Errors:  errCount,
		Results: results,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
