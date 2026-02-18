package bot

import (
	"context"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestRunGameDryRun(t *testing.T) {
	ctx := context.Background()
	cfg := ArenaConfig{
		GameName:    "test-dry-run",
		PowerConfig: ParsePowerConfig("*=easy"),
		MaxYear:     1910,
		Seed:        42,
		DryRun:      true,
	}

	result, err := RunGame(ctx, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunGame failed: %v", err)
	}

	if result.TotalPhases == 0 {
		t.Error("Expected at least one phase")
	}
	if result.FinalYear < 1901 {
		t.Errorf("Expected final year >= 1901, got %d", result.FinalYear)
	}
	if result.FinalYear > cfg.MaxYear+1 {
		t.Errorf("Expected final year <= %d, got %d", cfg.MaxYear+1, result.FinalYear)
	}

	// Verify SC counts are populated
	totalSC := 0
	for _, count := range result.SCCounts {
		totalSC += count
	}
	if totalSC == 0 {
		t.Error("Expected non-zero total SC count")
	}

	t.Logf("Result: winner=%q year=%d season=%s phases=%d", result.Winner, result.FinalYear, result.FinalSeason, result.TotalPhases)
	for power, sc := range result.SCCounts {
		t.Logf("  %s: %d SCs", power, sc)
	}
}

func TestRunGameCompletes(t *testing.T) {
	// Verify that a game with mixed difficulties completes without error.
	ctx := context.Background()
	cfg := ArenaConfig{
		GameName:    "test-mixed",
		PowerConfig: ParsePowerConfig("france=medium,*=easy"),
		MaxYear:     1905,
		Seed:        123,
		DryRun:      true,
	}

	result, err := RunGame(ctx, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunGame failed: %v", err)
	}

	if result.TotalPhases == 0 {
		t.Error("Expected at least one phase")
	}

	totalSC := 0
	for _, count := range result.SCCounts {
		totalSC += count
	}
	// Total owned SCs across 7 powers should be <= 34 (some may remain neutral)
	if totalSC == 0 || totalSC > 34 {
		t.Errorf("Expected 1-34 total owned SCs, got %d", totalSC)
	}

	t.Logf("Result: winner=%q year=%d phases=%d", result.Winner, result.FinalYear, result.TotalPhases)
}

func TestRunGameMaxYear(t *testing.T) {
	ctx := context.Background()
	cfg := ArenaConfig{
		GameName:    "test-max-year",
		PowerConfig: ParsePowerConfig("*=easy"),
		MaxYear:     1902,
		Seed:        99,
		DryRun:      true,
	}

	result, err := RunGame(ctx, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunGame failed: %v", err)
	}

	if result.FinalYear > cfg.MaxYear+1 {
		t.Errorf("Expected final year <= %d, got %d", cfg.MaxYear+1, result.FinalYear)
	}

	t.Logf("Result: winner=%q year=%d phases=%d", result.Winner, result.FinalYear, result.TotalPhases)
}

func TestRunGameAllDifficulties(t *testing.T) {
	difficulties := []struct {
		name    string
		maxYear int
	}{
		{"easy", 1905},
		{"medium", 1903},
		{"hard", 1902}, // hard is slow (2s/power/phase), keep short
	}
	for _, d := range difficulties {
		t.Run(d.name, func(t *testing.T) {
			if d.name == "hard" && testing.Short() {
				t.Skip("skipping hard bot test in short mode")
			}
			ctx := context.Background()
			cfg := ArenaConfig{
				GameName:    "test-" + d.name,
				PowerConfig: ParsePowerConfig("*=" + d.name),
				MaxYear:     d.maxYear,
				Seed:        42,
				DryRun:      true,
			}

			result, err := RunGame(ctx, cfg, nil, nil, nil)
			if err != nil {
				t.Fatalf("RunGame failed for %s: %v", d.name, err)
			}

			if result.TotalPhases == 0 {
				t.Error("Expected at least one phase")
			}
			t.Logf("%s: winner=%q year=%d phases=%d", d.name, result.Winner, result.FinalYear, result.TotalPhases)
		})
	}
}

func TestParsePowerConfig(t *testing.T) {
	tests := []struct {
		input    string
		expected map[diplomacy.Power]string
	}{
		{
			input: "*=easy",
			expected: map[diplomacy.Power]string{
				diplomacy.Austria: "easy", diplomacy.England: "easy", diplomacy.France: "easy",
				diplomacy.Germany: "easy", diplomacy.Italy: "easy", diplomacy.Russia: "easy",
				diplomacy.Turkey: "easy",
			},
		},
		{
			input: "france=hard,*=easy",
			expected: map[diplomacy.Power]string{
				diplomacy.Austria: "easy", diplomacy.England: "easy", diplomacy.France: "hard",
				diplomacy.Germany: "easy", diplomacy.Italy: "easy", diplomacy.Russia: "easy",
				diplomacy.Turkey: "easy",
			},
		},
		{
			input: "france=hard,germany=medium,*=easy",
			expected: map[diplomacy.Power]string{
				diplomacy.Austria: "easy", diplomacy.England: "easy", diplomacy.France: "hard",
				diplomacy.Germany: "medium", diplomacy.Italy: "easy", diplomacy.Russia: "easy",
				diplomacy.Turkey: "easy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParsePowerConfig(tt.input)
			for power, expectedDiff := range tt.expected {
				if got := result[power]; got != expectedDiff {
					t.Errorf("Power %s: expected %q, got %q", power, expectedDiff, got)
				}
			}
		})
	}
}

// TestMediumVsEasy runs 10 games of France (medium) vs 6 easy bots and reports
// win rate, draw rate, and SC counts. Used to benchmark medium bot improvements.
func TestMediumVsEasy(t *testing.T) {
	ctx := context.Background()
	numGames := 10

	wins := 0
	draws := 0
	losses := 0
	totalSCs := 0
	scCounts := make(map[string][]int) // power -> per-game SC counts

	for i := range numGames {
		cfg := ArenaConfig{
			GameName:    "medium-vs-easy",
			PowerConfig: ParsePowerConfig("france=medium,*=easy"),
			MaxYear:     1930,
			Seed:        int64(i + 1),
			DryRun:      true,
		}

		result, err := RunGame(ctx, cfg, nil, nil, nil)
		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		franceSCs := result.SCCounts["france"]
		totalSCs += franceSCs

		if result.Winner == "france" {
			wins++
		} else if result.Winner == "" {
			draws++
		} else {
			losses++
		}

		for power, sc := range result.SCCounts {
			scCounts[power] = append(scCounts[power], sc)
		}

		t.Logf("Game %d: winner=%q year=%d france_SCs=%d", i+1, result.Winner, result.FinalYear, franceSCs)
	}

	avgSCs := float64(totalSCs) / float64(numGames)
	winRate := float64(wins) / float64(numGames) * 100
	drawRate := float64(draws) / float64(numGames) * 100

	t.Logf("\n=== RESULTS: %d games ===", numGames)
	t.Logf("Wins: %d (%.0f%%), Draws: %d (%.0f%%), Losses: %d", wins, winRate, draws, drawRate, losses)
	t.Logf("Avg France SCs: %.1f", avgSCs)

	// Report per-power averages
	for _, power := range diplomacy.AllPowers() {
		counts := scCounts[string(power)]
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
		t.Logf("  %s: avg=%.1f survived=%d/%d", power, avg, survived, numGames)
	}
}

// TestEasyEnglandVsRandom runs 100 games of England (easy) vs 6 random bots.
// Used to benchmark easy bot fleet build improvements for England.
func TestEasyEnglandVsRandom(t *testing.T) {
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
			GameName:    "easy-england-vs-random",
			PowerConfig: ParsePowerConfig("england=easy,*=random"),
			MaxYear:     1930,
			Seed:        int64(i + 1),
			DryRun:      true,
		}

		result, err := RunGame(ctx, cfg, nil, nil, nil)
		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		englandSCs := result.SCCounts["england"]
		totalSCs += englandSCs

		if result.Winner == "england" {
			wins++
			victoryYears = append(victoryYears, result.FinalYear)
		} else if result.Winner == "" {
			draws++
		} else {
			losses++
		}

		for power, sc := range result.SCCounts {
			scCounts[power] = append(scCounts[power], sc)
		}

		t.Logf("Game %d: winner=%q year=%d england_SCs=%d", i+1, result.Winner, result.FinalYear, englandSCs)
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

	t.Logf("\n=== RESULTS: England (easy) vs 6 random — %d games ===", numGames)
	t.Logf("Wins: %d (%.0f%%), Draws: %d (%.0f%%), Losses: %d", wins, winRate, draws, drawRate, losses)
	t.Logf("Avg England SCs: %.1f", avgSCs)
	if len(victoryYears) > 0 {
		t.Logf("Avg Victory Year: %.1f", avgVictoryYear)
	}

	// Report per-power averages
	for _, power := range diplomacy.AllPowers() {
		counts := scCounts[string(power)]
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
		t.Logf("  %s: avg=%.1f survived=%d/%d", power, avg, survived, numGames)
	}
}

// TestHardVsMedium runs 5 games of France (hard) vs 6 medium bots.
// Used to benchmark hard bot improvements. Kept small due to speed (~2min/game).
func TestHardVsMedium(t *testing.T) {
	ctx := context.Background()
	numGames := 10

	wins := 0
	draws := 0
	losses := 0
	totalSCs := 0
	scCounts := make(map[string][]int)

	for i := range numGames {
		cfg := ArenaConfig{
			GameName:    "hard-vs-medium",
			PowerConfig: ParsePowerConfig("france=hard,*=medium"),
			MaxYear:     1930,
			Seed:        int64(i + 1),
			DryRun:      true,
		}

		result, err := RunGame(ctx, cfg, nil, nil, nil)
		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		franceSCs := result.SCCounts["france"]
		totalSCs += franceSCs

		if result.Winner == "france" {
			wins++
		} else if result.Winner == "" {
			draws++
		} else {
			losses++
		}

		for power, sc := range result.SCCounts {
			scCounts[power] = append(scCounts[power], sc)
		}

		t.Logf("Game %d: winner=%q year=%d france_SCs=%d", i+1, result.Winner, result.FinalYear, franceSCs)
	}

	avgSCs := float64(totalSCs) / float64(numGames)
	winRate := float64(wins) / float64(numGames) * 100
	drawRate := float64(draws) / float64(numGames) * 100

	t.Logf("\n=== RESULTS: Hard France vs 6 Medium — %d games ===", numGames)
	t.Logf("Wins: %d (%.0f%%), Draws: %d (%.0f%%), Losses: %d", wins, winRate, draws, drawRate, losses)
	t.Logf("Avg France SCs: %.1f", avgSCs)

	for _, power := range diplomacy.AllPowers() {
		counts := scCounts[string(power)]
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
		t.Logf("  %s: avg=%.1f survived=%d/%d", power, avg, survived, numGames)
	}
}

// TestHardVsMediumByPower runs 100 games for a single hard power vs 6 medium bots.
// Run with: go test -run TestHardVsMediumByPower/france -v -count=1 -timeout=0
func TestHardVsMediumByPower(t *testing.T) {
	powers := diplomacy.AllPowers()
	for _, power := range powers {
		t.Run(string(power), func(t *testing.T) {
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
					GameName:    "hard-" + string(power) + "-vs-medium",
					PowerConfig: ParsePowerConfig(string(power) + "=hard,*=medium"),
					MaxYear:     1930,
					Seed:        int64(i + 1),
					DryRun:      true,
				}

				result, err := RunGame(ctx, cfg, nil, nil, nil)
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

			t.Logf("\n=== RESULTS: %s (hard) vs 6 medium — %d games ===", power, numGames)
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

// TestEasyVsRandomAllPowers runs 100 games per power, each with 1 easy bot vs 6 random bots.
// Reports win rate, draw rate, avg SCs, and avg victory year per power.
// Run with: go test -run TestEasyVsRandomAllPowers -v -count=1 -timeout=600s
func TestEasyVsRandomAllPowers(t *testing.T) {
	powers := diplomacy.AllPowers()
	for _, power := range powers {
		t.Run(string(power), func(t *testing.T) {
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
					GameName:    "easy-" + string(power) + "-vs-random",
					PowerConfig: ParsePowerConfig(string(power) + "=easy,*=random"),
					MaxYear:     1930,
					Seed:        int64(i + 1),
					DryRun:      true,
				}

				result, err := RunGame(ctx, cfg, nil, nil, nil)
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

func TestRunGameContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := ArenaConfig{
		GameName:    "test-cancel",
		PowerConfig: ParsePowerConfig("*=easy"),
		MaxYear:     1930,
		Seed:        1,
		DryRun:      true,
	}

	_, err := RunGame(ctx, cfg, nil, nil, nil)
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
}
