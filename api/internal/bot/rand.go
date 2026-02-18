package bot

import "math/rand"

// botRng is the package-level random source used by all bot strategies.
// When nil, the functions below delegate to the global math/rand default.
// Use SeedBotRng to set a deterministic source for reproducible benchmarks.
var botRng *rand.Rand

// SeedBotRng sets a deterministic random source for reproducible bot behavior.
func SeedBotRng(seed int64) {
	botRng = rand.New(rand.NewSource(seed))
}

// ResetBotRng reverts to the default (non-deterministic) global random source.
func ResetBotRng() {
	botRng = nil
}

func botFloat64() float64 {
	if botRng != nil {
		return botRng.Float64()
	}
	return rand.Float64()
}

func botIntn(n int) int {
	if botRng != nil {
		return botRng.Intn(n)
	}
	return rand.Intn(n)
}

func botPerm(n int) []int {
	if botRng != nil {
		return botRng.Perm(n)
	}
	return rand.Perm(n)
}

func botShuffle(n int, swap func(i, j int)) {
	if botRng != nil {
		botRng.Shuffle(n, swap)
		return
	}
	rand.Shuffle(n, swap)
}

func botInt63() int64 {
	if botRng != nil {
		return botRng.Int63()
	}
	return rand.Int63()
}
