//go:build integration

package bot

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/efreeman/polite-betrayal/api/internal/repository/postgres"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TestMediumVsEasyByPowerDB runs 100 games per power: medium bot vs 6 easy bots,
// storing all games in the database for review.
// Run with: go test -tags integration -run TestMediumVsEasyByPowerDB -v -count=1 -timeout=0
// Or a single power: go test -tags integration -run TestMediumVsEasyByPowerDB/france -v -count=1 -timeout=0
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable"
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMediumVsEasyByPowerDB(t *testing.T) {
	db := openDB(t)
	gameRepo := postgres.NewGameRepo(db)
	phaseRepo := postgres.NewPhaseRepo(db)
	userRepo := postgres.NewUserRepo(db)

	powers := diplomacy.AllPowers()
	for _, power := range powers {
		t.Run(string(power), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			numGames := 100

			wins := 0
			draws := 0
			losses := 0
			totalSCs := 0
			var victoryYears []int
			scCounts := make(map[string][]int)

			for i := range numGames {
				cfg := ArenaConfig{
					GameName:    "bench-medium-" + string(power) + "-vs-easy",
					PowerConfig: ParsePowerConfig(string(power) + "=medium,*=easy"),
					MaxYear:     1930,
					Seed:        int64(i + 1),
					DryRun:      false,
				}

				result, err := RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
				if err != nil {
					t.Fatalf("game %d failed: %v", i+1, err)
				}

				pSCs := result.SCCounts[string(power)]
				totalSCs += pSCs

				if result.Winner == string(power) {
					wins++
					victoryYears = append(victoryYears, result.FinalYear)
				} else if result.Winner == "" {
					draws++
				} else {
					losses++
				}

				for p, sc := range result.SCCounts {
					scCounts[p] = append(scCounts[p], sc)
				}

				t.Logf("Game %d: winner=%q year=%d %s_SCs=%d", i+1, result.Winner, result.FinalYear, power, pSCs)
			}

			avgSCs := float64(totalSCs) / float64(numGames)
			winRate := float64(wins) / float64(numGames) * 100
			drawRate := float64(draws) / float64(numGames) * 100

			avgVictoryYear := 0.0
			if len(victoryYears) > 0 {
				sum := 0
				for _, y := range victoryYears {
					sum += y
				}
				avgVictoryYear = float64(sum) / float64(len(victoryYears))
			}

			t.Logf("\n=== RESULTS: %s (medium) vs 6 easy — %d games ===", power, numGames)
			t.Logf("Wins: %d (%.0f%%), Draws: %d (%.0f%%), Losses: %d", wins, winRate, draws, drawRate, losses)
			t.Logf("Avg %s SCs: %.1f", power, avgSCs)
			if len(victoryYears) > 0 {
				t.Logf("Avg Victory Year: %.1f", avgVictoryYear)
			}

			for _, p := range diplomacy.AllPowers() {
				counts := scCounts[string(p)]
				if len(counts) == 0 {
					continue
				}
				sum := 0
				survived := 0
				for _, c := range counts {
					sum += c
					if c > 0 {
						survived++
					}
				}
				avg := float64(sum) / float64(len(counts))
				t.Logf("  %s: avg=%.1f survived=%d/%d", p, avg, survived, numGames)
			}
		})
	}
}

// TestEasyVsRandomAllPowersDB runs 10 games per power: easy bot vs 6 random bots,
// storing all games in the database for review in the UI.
// Run with: go test -tags integration -run TestEasyVsRandomAllPowersDB -v -count=1 -timeout=600s
func TestEasyVsRandomAllPowersDB(t *testing.T) {
	db := openDB(t)
	gameRepo := postgres.NewGameRepo(db)
	phaseRepo := postgres.NewPhaseRepo(db)
	userRepo := postgres.NewUserRepo(db)

	powers := diplomacy.AllPowers()
	for _, power := range powers {
		t.Run(string(power), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			numGames := 10

			wins := 0
			draws := 0
			losses := 0
			totalSCs := 0
			var victoryYears []int
			scCounts := make(map[string][]int)

			for i := range numGames {
				cfg := ArenaConfig{
					GameName:    "bench-easy-" + string(power) + "-vs-random",
					PowerConfig: ParsePowerConfig(string(power) + "=easy,*=random"),
					MaxYear:     1930,
					Seed:        int64(i + 1),
					DryRun:      false,
				}

				result, err := RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
				if err != nil {
					t.Fatalf("game %d failed: %v", i+1, err)
				}

				pSCs := result.SCCounts[string(power)]
				totalSCs += pSCs

				if result.Winner == string(power) {
					wins++
					victoryYears = append(victoryYears, result.FinalYear)
				} else if result.Winner == "" {
					draws++
				} else {
					losses++
				}

				for p, sc := range result.SCCounts {
					scCounts[p] = append(scCounts[p], sc)
				}

				t.Logf("Game %d: winner=%q year=%d %s_SCs=%d", i+1, result.Winner, result.FinalYear, power, pSCs)
			}

			avgSCs := float64(totalSCs) / float64(numGames)
			winRate := float64(wins) / float64(numGames) * 100
			drawRate := float64(draws) / float64(numGames) * 100

			avgVictoryYear := 0.0
			if len(victoryYears) > 0 {
				sum := 0
				for _, y := range victoryYears {
					sum += y
				}
				avgVictoryYear = float64(sum) / float64(len(victoryYears))
			}

			t.Logf("\n=== RESULTS: %s (easy) vs 6 random — %d games ===", power, numGames)
			t.Logf("Wins: %d (%.0f%%), Draws: %d (%.0f%%), Losses: %d", wins, winRate, draws, drawRate, losses)
			t.Logf("Avg %s SCs: %.1f", power, avgSCs)
			if len(victoryYears) > 0 {
				t.Logf("Avg Victory Year: %.1f", avgVictoryYear)
			}

			for _, p := range diplomacy.AllPowers() {
				counts := scCounts[string(p)]
				if len(counts) == 0 {
					continue
				}
				sum := 0
				survived := 0
				for _, c := range counts {
					sum += c
					if c > 0 {
						survived++
					}
				}
				avg := float64(sum) / float64(len(counts))
				t.Logf("  %s: avg=%.1f survived=%d/%d", p, avg, survived, numGames)
			}
		})
	}
}
