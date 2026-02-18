# Port Kruijswijk Resolver to Rust

## Status: Pending

## Dependencies
- 030 (Province/adjacency data)
- 036 (Move generator — needs Order types)

## Description
Port the Kruijswijk guess-and-check order resolution algorithm from `api/pkg/diplomacy/resolve.go` to Rust. This is the core adjudication engine that determines which orders succeed and which fail.

1. **Resolver struct** (`src/resolve/kruijswijk.rs`):
   - Port the `Resolver` struct with lookup table and adjacency buffer
   - Zero-allocation design: reuse buffers across calls
   - `resolve(&mut self, orders: &[Order], state: &BoardState) -> Vec<ResolvedOrder>`

2. **Key algorithm components**:
   - Guess-and-check loop with optimistic initial guess (true) — critical per MEMORY.md
   - Attack strength calculation
   - Defend strength calculation
   - Support counting (with support cuts)
   - Head-to-head battle detection and resolution
   - Convoy path resolution (convoy disruption)
   - Circular movement detection

3. **State application**:
   - `apply_resolution(state: &mut BoardState, resolved: &[ResolvedOrder])` — update unit positions
   - `apply_retreats(state: &mut BoardState, retreats: &[RetreatOrder])`
   - `apply_builds(state: &mut BoardState, builds: &[BuildOrder])`

4. **Cross-validation**: for a set of test positions, run both Go and Rust resolvers and verify identical outcomes

## Acceptance Criteria
- Passes all DATC (Diplomacy Adjudicator Test Cases) basic tests — see task 039
- Produces identical results to Go resolver on a corpus of 100+ random positions
- Handles all edge cases: circular movements, convoy paradoxes, head-to-head with support
- Uses optimistic initial guess (matching Go implementation)
- No heap allocation in the hot path (resolution loop)
- Benchmark: resolves a typical 7-player position in under 100 microseconds

## Estimated Effort: L
