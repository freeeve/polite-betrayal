//go:build integration

package bot

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// BenchmarkResult holds aggregate metrics from a series of arena games.
type BenchmarkResult struct {
	Matchup      string
	NumGames     int
	Wins         int
	Draws        int
	Losses       int
	Survived     int
	TotalSCs     int
	VictoryYears []int
	GameLengths  []int // total phases per game
	SCCounts     map[string][]int
	Durations    []time.Duration // wall-clock time per game
}

// WinRate returns the win rate as a percentage.
func (b *BenchmarkResult) WinRate() float64 {
	return float64(b.Wins) / float64(b.NumGames) * 100
}

// DrawRate returns the draw rate as a percentage.
func (b *BenchmarkResult) DrawRate() float64 {
	return float64(b.Draws) / float64(b.NumGames) * 100
}

// SurvivalRate returns the percentage of games where the test power had >0 SCs.
func (b *BenchmarkResult) SurvivalRate() float64 {
	return float64(b.Survived) / float64(b.NumGames) * 100
}

// AvgSCs returns the average SC count for the test power.
func (b *BenchmarkResult) AvgSCs() float64 {
	return float64(b.TotalSCs) / float64(b.NumGames)
}

// AvgVictoryYear returns the average year of solo victories.
func (b *BenchmarkResult) AvgVictoryYear() float64 {
	if len(b.VictoryYears) == 0 {
		return 0
	}
	sum := 0
	for _, y := range b.VictoryYears {
		sum += y
	}
	return float64(sum) / float64(len(b.VictoryYears))
}

// AvgGameLength returns the average number of phases per game.
func (b *BenchmarkResult) AvgGameLength() float64 {
	if len(b.GameLengths) == 0 {
		return 0
	}
	sum := 0
	for _, l := range b.GameLengths {
		sum += l
	}
	return float64(sum) / float64(len(b.GameLengths))
}

// MedianDuration returns the median wall-clock time per game.
func (b *BenchmarkResult) MedianDuration() time.Duration {
	if len(b.Durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(b.Durations))
	copy(sorted, b.Durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// StdDevSCs returns the standard deviation of the test power's SC counts.
func (b *BenchmarkResult) StdDevSCs(power string) float64 {
	counts := b.SCCounts[power]
	if len(counts) < 2 {
		return 0
	}
	mean := b.AvgSCs()
	sumSq := 0.0
	for _, c := range counts {
		d := float64(c) - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(counts)-1))
}

// runBenchmarkSuite runs numGames arena games with the specified config.
func runBenchmarkSuite(t *testing.T, matchup string, numGames int, powerConfig string, maxYear int) *BenchmarkResult {
	t.Helper()

	bin := enginePath(t)
	origPath := ExternalEnginePath
	ExternalEnginePath = bin
	defer func() { ExternalEnginePath = origPath }()

	result := &BenchmarkResult{
		Matchup:  matchup,
		NumGames: numGames,
		SCCounts: make(map[string][]int),
	}

	ctx := context.Background()

	for i := range numGames {
		cfg := ArenaConfig{
			GameName:    matchup,
			PowerConfig: ParsePowerConfig(powerConfig),
			MaxYear:     maxYear,
			Seed:        int64(i + 1),
			DryRun:      true,
		}

		start := time.Now()
		gameResult, err := RunGame(ctx, cfg, nil, nil, nil)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		result.Durations = append(result.Durations, elapsed)
		result.GameLengths = append(result.GameLengths, gameResult.TotalPhases)

		franceSCs := gameResult.SCCounts["france"]
		result.TotalSCs += franceSCs

		if franceSCs > 0 {
			result.Survived++
		}

		if gameResult.Winner == "france" {
			result.Wins++
			result.VictoryYears = append(result.VictoryYears, gameResult.FinalYear)
		} else if gameResult.Winner == "" {
			result.Draws++
		} else {
			result.Losses++
		}

		for power, sc := range gameResult.SCCounts {
			result.SCCounts[power] = append(result.SCCounts[power], sc)
		}

		t.Logf("Game %d/%d: winner=%q year=%d phases=%d france_SCs=%d elapsed=%s",
			i+1, numGames, gameResult.Winner, gameResult.FinalYear, gameResult.TotalPhases, franceSCs, elapsed.Round(time.Millisecond))
	}

	return result
}

// logBenchmarkResults logs a comprehensive results summary.
func logBenchmarkResults(t *testing.T, r *BenchmarkResult) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== BENCHMARK: %s (%d games) ===\n", r.Matchup, r.NumGames))
	sb.WriteString(fmt.Sprintf("Win rate:     %d/%d (%.0f%%)\n", r.Wins, r.NumGames, r.WinRate()))
	sb.WriteString(fmt.Sprintf("Draw rate:    %d/%d (%.0f%%)\n", r.Draws, r.NumGames, r.DrawRate()))
	sb.WriteString(fmt.Sprintf("Loss rate:    %d/%d\n", r.Losses, r.NumGames))
	sb.WriteString(fmt.Sprintf("Survival:     %d/%d (%.0f%%)\n", r.Survived, r.NumGames, r.SurvivalRate()))
	sb.WriteString(fmt.Sprintf("Avg SCs:      %.1f (stddev=%.1f)\n", r.AvgSCs(), r.StdDevSCs("france")))
	if len(r.VictoryYears) > 0 {
		sb.WriteString(fmt.Sprintf("Avg Victory:  %.1f\n", r.AvgVictoryYear()))
	}
	sb.WriteString(fmt.Sprintf("Avg Phases:   %.1f\n", r.AvgGameLength()))
	sb.WriteString(fmt.Sprintf("Median Time:  %s\n", r.MedianDuration().Round(time.Millisecond)))

	sb.WriteString("\nPer-power SC averages:\n")
	for _, power := range diplomacy.AllPowers() {
		counts := r.SCCounts[string(power)]
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
		sb.WriteString(fmt.Sprintf("  %-8s avg=%.1f survived=%d/%d\n", power, avg, survived, r.NumGames))
	}

	t.Log(sb.String())
}

// TestBenchmark_RustVsEasy runs the Rust RM+ engine as France against 6 easy Go bots.
func TestBenchmark_RustVsEasy(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	r := runBenchmarkSuite(t, "rust-france-vs-6-easy", 10, "france=external,*=easy", 1930)
	logBenchmarkResults(t, r)

	// Acceptance: >80% win rate vs easy
	if r.WinRate() < 80 {
		t.Logf("WARNING: Win rate %.0f%% below 80%% target vs easy bots", r.WinRate())
	}
}

// TestBenchmark_RustVsMedium runs the Rust RM+ engine as France against 6 medium Go bots.
func TestBenchmark_RustVsMedium(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	r := runBenchmarkSuite(t, "rust-france-vs-6-medium", 10, "france=external,*=medium", 1930)
	logBenchmarkResults(t, r)

	// Acceptance: >40% win rate vs medium
	if r.WinRate() < 40 {
		t.Logf("WARNING: Win rate %.0f%% below 40%% target vs medium bots", r.WinRate())
	}
}

// TestBenchmark_RustVsHard runs the Rust RM+ engine as France against 6 hard Go bots.
// Uses MaxYear 1908 and only 5 games because Go hard bots are very slow (~2s/power/phase).
func TestBenchmark_RustVsHard(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	r := runBenchmarkSuite(t, "rust-france-vs-6-hard", 5, "france=external,*=hard", 1908)
	logBenchmarkResults(t, r)
}
