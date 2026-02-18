# Self-Play Game Generation Pipeline

## Status: Complete

## Dependencies
- 052 (Neural-guided search -- needs a working neural engine)
- 038 (Resolver -- for fast game simulation)

## Implementation

### Files Created
- `engine/src/selfplay.rs` -- Core self-play library module
- `engine/src/bin/selfplay.rs` -- CLI binary entry point

### Files Modified
- `engine/src/lib.rs` -- Added `pub mod selfplay;`

### Architecture
- Single-engine `setpower` cycling (one engine instance, cycles through all 7 powers per phase)
- Full game loop: Movement (RM+ or Cartesian search) -> Retreat (heuristic) -> Build (heuristic)
- Phase sequencing via existing `advance_state()` / `resolve` infrastructure

### Features
1. **Self-play orchestrator** (Rust binary):
   - Configurable: games, movetime, strength, max year, temperature, threads, seed
   - Records every phase: DFEN state, orders (DSON), heuristic value estimates, SC counts
   - Outputs JSONL format (one JSON object per game per line)

2. **Exploration**:
   - Temperature-based sampling: decays over game years via `temp * decay^(year-1901)`
   - Probability of random orders proportional to temperature
   - Dirichlet noise generation (Marsaglia/Tsang gamma sampler) ready for neural integration

3. **Data format** (JSONL):
   - Per game: game_id, winner, final_year, final_sc_counts, quality flags
   - Per phase: dfen, year, season, phase, orders by power (DSON), value estimates, sc_counts
   - Compatible with Python JSON loading for training scripts

4. **Parallelism**:
   - Rayon-based parallel game generation with configurable thread count
   - Deterministic with seed (per-thread seed = base_seed + game_id)

5. **Game quality filtering**:
   - Stalemate detection: 3 consecutive years with no SC changes
   - Early stalemate flag: games stalling before min_stalemate_year (default: 1905)
   - Early domination flag: power reaching threshold SCs before year threshold
   - Filtered games excluded from output, summary printed to stderr

### CLI Usage
```
cargo run --release --bin selfplay -- --games 10 --movetime 2000 --strength 100 \
  --max-year 1920 --temperature 1.0 --threads 4 --output data/selfplay.jsonl
```

### Tests
- 10 unit tests covering: game completion, DFEN validity, sequential/parallel runs,
  JSONL output, SC counting, Dirichlet noise, stalemate detection, power names, JSON escaping
- All 503 existing engine tests continue to pass

## Estimated Effort: L
