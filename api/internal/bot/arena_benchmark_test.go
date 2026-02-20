//go:build integration

package bot

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/internal/repository"
	"github.com/freeeve/polite-betrayal/api/internal/repository/postgres"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// benchNumGames returns BENCH_GAMES env var as int, or the provided default.
func benchNumGames(defaultN int) int {
	if s := os.Getenv("BENCH_GAMES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultN
}

// benchVerbose returns true when BENCH_VERBOSE=1, enabling per-game logging.
func benchVerbose() bool {
	return os.Getenv("BENCH_VERBOSE") == "1"
}

// benchSave returns true when BENCH_SAVE=1, saving games to DB instead of dry run.
func benchSave() bool {
	return os.Getenv("BENCH_SAVE") == "1"
}

// benchRepos returns DB repositories when BENCH_SAVE=1, or nil repos for dry run.
func benchRepos(t *testing.T) (repository.GameRepository, repository.PhaseRepository, repository.UserRepository) {
	t.Helper()
	if !benchSave() {
		return nil, nil, nil
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable"
	}
	db, err := postgres.Connect(dbURL)
	if err != nil {
		t.Fatalf("BENCH_SAVE=1 but cannot connect to DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return postgres.NewGameRepo(db), postgres.NewPhaseRepo(db), postgres.NewUserRepo(db)
}

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

// neuralEngineOptions returns ExternalOptions that enable neural-guided search.
// The model path is derived from the engine binary location.
func neuralEngineOptions(t *testing.T) []ExternalOption {
	t.Helper()
	modelsDir := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(enginePath(t)))), "models")
	if _, err := os.Stat(filepath.Join(modelsDir, "policy_v1.onnx")); err != nil {
		t.Logf("WARNING: policy model not found at %s; running without neural eval", modelsDir)
		return nil
	}
	return []ExternalOption{
		WithEngineOption("ModelPath", modelsDir),
		WithEngineOption("EvalMode", "neural"),
	}
}

// runBenchmarkSuite runs numGames arena games with the specified config.
func runBenchmarkSuite(t *testing.T, matchup string, numGames int, powerConfig string, maxYear int) *BenchmarkResult {
	t.Helper()

	bin := enginePath(t)
	origPath := ExternalEnginePath
	ExternalEnginePath = bin
	origOpts := ExternalEngineOptions
	ExternalEngineOptions = neuralEngineOptions(t)
	defer func() {
		ExternalEnginePath = origPath
		ExternalEngineOptions = origOpts
	}()

	gameRepo, phaseRepo, userRepo := benchRepos(t)

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
			DryRun:      !benchSave(),
		}

		start := time.Now()
		gameResult, err := RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
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

		if benchVerbose() {
			t.Logf("Game %d/%d: winner=%q year=%d phases=%d france_SCs=%d elapsed=%s",
				i+1, numGames, gameResult.Winner, gameResult.FinalYear, gameResult.TotalPhases, franceSCs, elapsed.Round(time.Millisecond))
		}
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

// TimelineBenchmarkResult holds aggregate metrics including per-year SC timeline stats.
type TimelineBenchmarkResult struct {
	Power        string
	NumGames     int
	Wins         int
	Draws        int
	Losses       int
	TotalSCs     int
	VictoryYears []int
	Durations    []time.Duration
	// SCByYear maps year -> slice of SC counts across all games for the test power
	SCByYear map[int][]int
}

// percentile returns the p-th percentile (0-100) from a sorted slice of ints.
func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	if lower >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	return sorted[lower]
}

// runEasyVsRandomBenchmark runs numGames where testPower uses "easy" and all others use "random".
func runEasyVsRandomBenchmark(t *testing.T, testPower diplomacy.Power, numGames, maxYear int) *TimelineBenchmarkResult {
	t.Helper()

	gameRepo, phaseRepo, userRepo := benchRepos(t)

	powerStr := string(testPower)
	result := &TimelineBenchmarkResult{
		Power:    powerStr,
		NumGames: numGames,
		SCByYear: make(map[int][]int),
	}

	ctx := context.Background()

	for i := range numGames {
		// Build power config: testPower=easy, *=random
		pc := make(map[diplomacy.Power]string)
		for _, p := range diplomacy.AllPowers() {
			if p == testPower {
				pc[p] = "easy"
			} else {
				pc[p] = "random"
			}
		}

		cfg := ArenaConfig{
			GameName:    fmt.Sprintf("bench-easy-%s-vs-random", powerStr),
			PowerConfig: pc,
			MaxYear:     maxYear,
			Seed:        int64(i + 1),
			DryRun:      !benchSave(),
		}

		start := time.Now()
		gameResult, err := RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		result.Durations = append(result.Durations, elapsed)

		testSCs := gameResult.SCCounts[powerStr]
		result.TotalSCs += testSCs

		if gameResult.Winner == powerStr {
			result.Wins++
			result.VictoryYears = append(result.VictoryYears, gameResult.FinalYear)
		} else if gameResult.Winner == "" {
			result.Draws++
		} else {
			result.Losses++
		}

		// Collect SC timeline for the test power
		for idx, year := range gameResult.TimelineYears {
			scSlice := gameResult.SCTimeline[powerStr]
			if idx < len(scSlice) {
				result.SCByYear[year] = append(result.SCByYear[year], scSlice[idx])
			}
		}
	}

	return result
}

// logTimelineResults logs summary stats and per-year SC timeline table.
func logTimelineResults(t *testing.T, r *TimelineBenchmarkResult) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== %s (Easy) vs 6 Random — %d games ===\n", strings.Title(r.Power), r.NumGames))

	winRate := float64(r.Wins) / float64(r.NumGames) * 100
	avgSCs := float64(r.TotalSCs) / float64(r.NumGames)
	sb.WriteString(fmt.Sprintf("Win: %d/%d (%.0f%%), Draw: %d, Loss: %d\n", r.Wins, r.NumGames, winRate, r.Draws, r.Losses))
	sb.WriteString(fmt.Sprintf("Avg Final SCs: %.1f\n", avgSCs))

	if len(r.VictoryYears) > 0 {
		sum := 0
		for _, y := range r.VictoryYears {
			sum += y
		}
		sb.WriteString(fmt.Sprintf("Avg Victory Year: %.1f\n", float64(sum)/float64(len(r.VictoryYears))))
	}

	// Collect and sort years
	var years []int
	for y := range r.SCByYear {
		years = append(years, y)
	}
	sort.Ints(years)

	if len(years) > 0 {
		sb.WriteString("\nYear | Avg  | Min | P25 | P50 | P75 | P95 | Max | N\n")
		sb.WriteString("-----|------|-----|-----|-----|-----|-----|-----|---\n")
		for _, year := range years {
			counts := r.SCByYear[year]
			sorted := make([]int, len(counts))
			copy(sorted, counts)
			sort.Ints(sorted)

			sum := 0
			for _, c := range sorted {
				sum += c
			}
			avg := float64(sum) / float64(len(sorted))

			sb.WriteString(fmt.Sprintf("%d | %4.1f | %3d | %3d | %3d | %3d | %3d | %3d | %d\n",
				year, avg,
				sorted[0],
				percentile(sorted, 25),
				percentile(sorted, 50),
				percentile(sorted, 75),
				percentile(sorted, 95),
				sorted[len(sorted)-1],
				len(sorted),
			))
		}
	}

	t.Log(sb.String())
}

// runTimelineBenchmark runs numGames where testPower uses testDiff and all others use opponentDiff.
func runTimelineBenchmark(t *testing.T, testPower diplomacy.Power, testDiff, opponentDiff string, numGames, maxYear int) *TimelineBenchmarkResult {
	t.Helper()

	gameRepo, phaseRepo, userRepo := benchRepos(t)

	powerStr := string(testPower)
	result := &TimelineBenchmarkResult{
		Power:    powerStr,
		NumGames: numGames,
		SCByYear: make(map[int][]int),
	}

	ctx := context.Background()

	for i := range numGames {
		pc := make(map[diplomacy.Power]string)
		for _, p := range diplomacy.AllPowers() {
			if p == testPower {
				pc[p] = testDiff
			} else {
				pc[p] = opponentDiff
			}
		}

		cfg := ArenaConfig{
			GameName:    fmt.Sprintf("bench-%s-%s-vs-%s", testDiff, powerStr, opponentDiff),
			PowerConfig: pc,
			MaxYear:     maxYear,
			Seed:        int64(i + 1),
			DryRun:      !benchSave(),
		}

		start := time.Now()
		gameResult, err := RunGame(ctx, cfg, gameRepo, phaseRepo, userRepo)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("game %d failed: %v", i+1, err)
		}

		result.Durations = append(result.Durations, elapsed)

		testSCs := gameResult.SCCounts[powerStr]
		result.TotalSCs += testSCs

		if gameResult.Winner == powerStr {
			result.Wins++
			result.VictoryYears = append(result.VictoryYears, gameResult.FinalYear)
		} else if gameResult.Winner == "" {
			result.Draws++
		} else {
			result.Losses++
		}

		// Collect SC timeline for the test power
		for idx, year := range gameResult.TimelineYears {
			scSlice := gameResult.SCTimeline[powerStr]
			if idx < len(scSlice) {
				result.SCByYear[year] = append(result.SCByYear[year], scSlice[idx])
			}
		}

		if benchVerbose() {
			t.Logf("Game %d/%d: winner=%q year=%d %s_SCs=%d elapsed=%s",
				i+1, numGames, gameResult.Winner, gameResult.FinalYear, powerStr, testSCs, elapsed.Round(time.Millisecond))
		}
	}

	return result
}

// logTimelineResultsLabeled logs summary stats with a custom label.
func logTimelineResultsLabeled(t *testing.T, r *TimelineBenchmarkResult, label string) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== %s — %d games ===\n", label, r.NumGames))

	winRate := float64(r.Wins) / float64(r.NumGames) * 100
	avgSCs := float64(r.TotalSCs) / float64(r.NumGames)
	sb.WriteString(fmt.Sprintf("Win: %d/%d (%.0f%%), Draw: %d, Loss: %d\n", r.Wins, r.NumGames, winRate, r.Draws, r.Losses))
	sb.WriteString(fmt.Sprintf("Avg Final SCs: %.1f\n", avgSCs))

	if len(r.VictoryYears) > 0 {
		sum := 0
		for _, y := range r.VictoryYears {
			sum += y
		}
		sb.WriteString(fmt.Sprintf("Avg Victory Year: %.1f\n", float64(sum)/float64(len(r.VictoryYears))))
	}

	var years []int
	for y := range r.SCByYear {
		years = append(years, y)
	}
	sort.Ints(years)

	if len(years) > 0 {
		sb.WriteString("\nYear | Avg  | Min | P25 | P50 | P75 | P95 | Max | N\n")
		sb.WriteString("-----|------|-----|-----|-----|-----|-----|-----|---\n")
		for _, year := range years {
			counts := r.SCByYear[year]
			sorted := make([]int, len(counts))
			copy(sorted, counts)
			sort.Ints(sorted)

			sum := 0
			for _, c := range sorted {
				sum += c
			}
			avg := float64(sum) / float64(len(sorted))

			sb.WriteString(fmt.Sprintf("%d | %4.1f | %3d | %3d | %3d | %3d | %3d | %3d | %d\n",
				year, avg,
				sorted[0],
				percentile(sorted, 25),
				percentile(sorted, 50),
				percentile(sorted, 75),
				percentile(sorted, 95),
				sorted[len(sorted)-1],
				len(sorted),
			))
		}
	}

	t.Log(sb.String())
}

// TestBenchmark_EasyVsRandom runs each of the 7 powers as Easy against 6 Random, 20 games each.
func TestBenchmark_EasyVsRandom(t *testing.T) {
	numGames := benchNumGames(20)
	maxYear := 1930

	for _, power := range diplomacy.AllPowers() {
		power := power // capture
		t.Run(string(power), func(t *testing.T) {
			r := runEasyVsRandomBenchmark(t, power, numGames, maxYear)
			logTimelineResults(t, r)
		})
	}
}

// TestBenchmark_MediumVsEasy runs France and Turkey as Medium against 6 Easy, 20 games each.
func TestBenchmark_MediumVsEasy(t *testing.T) {
	numGames := benchNumGames(20)
	maxYear := 1930

	for _, power := range []diplomacy.Power{diplomacy.France, diplomacy.Turkey} {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "medium", "easy", numGames, maxYear)
			label := fmt.Sprintf("%s (Medium) vs 6 Easy", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)
		})
	}
}

// TestBenchmark_MediumVsEasyAllPowers runs all 7 powers as Medium against 6 Easy, 100 games each.
func TestBenchmark_MediumVsEasyAllPowers(t *testing.T) {
	numGames := benchNumGames(100)
	maxYear := 1930

	for _, power := range diplomacy.AllPowers() {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "medium", "easy", numGames, maxYear)
			label := fmt.Sprintf("%s (Medium) vs 6 Easy", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)
		})
	}
}

// TestBenchmark_HardVsMediumAllPowers runs all 7 powers as Hard against 6 Medium, 10 games each.
func TestBenchmark_HardVsMediumAllPowers(t *testing.T) {
	numGames := benchNumGames(10)
	maxYear := 1930

	for _, power := range diplomacy.AllPowers() {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "hard", "medium", numGames, maxYear)
			label := fmt.Sprintf("%s (Hard) vs 6 Medium", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)
		})
	}
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
// Uses MaxYear 1905 and only 3 games because Go hard bots are very slow (~6min/game).
func TestBenchmark_RustVsHard(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	r := runBenchmarkSuite(t, "rust-france-vs-6-hard", 3, "france=external,*=hard", 1905)
	logBenchmarkResults(t, r)
}

// TestBenchmark_GonnxVsMedium runs each of the 7 powers as hard-gonnx against 6 medium bots.
func TestBenchmark_GonnxVsMedium(t *testing.T) {
	modelPath := filepath.Join("..", "..", "..", "engine", "models")
	if _, err := os.Stat(filepath.Join(modelPath, "policy_v2.onnx")); err != nil {
		t.Skip("policy_v2.onnx not found, skipping gonnx benchmark")
	}

	origPath := GonnxModelPath
	GonnxModelPath = modelPath
	defer func() { GonnxModelPath = origPath }()

	numGames := benchNumGames(10)
	maxYear := 1920

	for _, power := range diplomacy.AllPowers() {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "hard-gonnx", "medium", numGames, maxYear)
			label := fmt.Sprintf("%s (Gonnx) vs 6 Medium", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)
		})
	}
}

// TestBenchmark_RustVsEasyAllPowers runs the Rust engine as each of the 7 powers against 6 easy Go bots.
func TestBenchmark_RustVsEasyAllPowers(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	bin := enginePath(t)
	origPath := ExternalEnginePath
	ExternalEnginePath = bin
	origOpts := ExternalEngineOptions
	ExternalEngineOptions = neuralEngineOptions(t)
	defer func() {
		ExternalEnginePath = origPath
		ExternalEngineOptions = origOpts
	}()

	numGames := benchNumGames(100)
	maxYear := 1930

	for _, power := range diplomacy.AllPowers() {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "external", "easy", numGames, maxYear)
			label := fmt.Sprintf("%s (Rust) vs 6 Easy", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)

			winRate := float64(r.Wins) / float64(r.NumGames) * 100
			if winRate < 80 {
				t.Logf("WARNING: %s win rate %.0f%% below 80%% target vs easy bots", power, winRate)
			}
		})
	}
}

// TestBenchmark_RustVsMediumAllPowers runs the Rust engine as each of the 7 powers against 6 medium Go bots.
func TestBenchmark_RustVsMediumAllPowers(t *testing.T) {
	if os.Getenv("REALPOLITIK_PATH") == "" {
		t.Skip("REALPOLITIK_PATH not set")
	}

	bin := enginePath(t)
	origPath := ExternalEnginePath
	ExternalEnginePath = bin
	origOpts := ExternalEngineOptions
	ExternalEngineOptions = neuralEngineOptions(t)
	defer func() {
		ExternalEnginePath = origPath
		ExternalEngineOptions = origOpts
	}()

	numGames := benchNumGames(10)
	maxYear := 1930

	for _, power := range diplomacy.AllPowers() {
		power := power
		t.Run(string(power), func(t *testing.T) {
			r := runTimelineBenchmark(t, power, "external", "medium", numGames, maxYear)
			label := fmt.Sprintf("%s (Rust) vs 6 Medium", strings.Title(string(power)))
			logTimelineResultsLabeled(t, r, label)

			winRate := float64(r.Wins) / float64(r.NumGames) * 100
			if winRate < 30 {
				t.Logf("WARNING: %s win rate %.0f%% below 30%% target vs medium bots", power, winRate)
			}
		})
	}
}
