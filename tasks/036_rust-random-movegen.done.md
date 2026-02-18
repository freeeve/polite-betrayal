# Implement Random Legal Move Generator in Rust

## Status: Pending

## Dependencies
- 030 (Province/adjacency data)
- 031 (DFEN codec — needs BoardState)

## Description
Implement legal move generation for all three phases in Rust, starting with a random selection strategy. This is the minimum viable "brain" for the engine to play legal Diplomacy.

1. **Movement phase** (`src/movegen/movement.rs`):
   - `legal_orders(unit: UnitEntry, prov: Province, state: &BoardState) -> Vec<Order>`
   - Generate all legal: holds, moves (army adjacency, fleet adjacency with coast handling), supports (hold and move), convoys
   - Port logic from Go's `LegalOrdersForUnit` in `search_util.go`

2. **Retreat phase** (`src/movegen/retreat.rs`):
   - `legal_retreats(unit: UnitEntry, prov: Province, dislodged_from: Province, state: &BoardState) -> Vec<Order>`
   - Can retreat to adjacent empty provinces not occupied and not the attacker's origin
   - Can always disband

3. **Build phase** (`src/movegen/build.rs`):
   - `legal_builds(power: Power, state: &BoardState) -> Vec<Order>`
   - If SC count > unit count: can build in unoccupied home SCs
   - If SC count < unit count: must disband units
   - Can waive builds

4. **Random selection**:
   - `random_orders(power: Power, state: &BoardState) -> Vec<Order>`
   - For each unit of the given power, pick one legal order uniformly at random
   - Wire this into the `go` command handler as the initial search implementation

## Acceptance Criteria
- Movement: generates correct legal orders for armies and fleets including coast-specific moves
- Supports: only generates supports for moves that are actually possible (target adjacency check)
- Convoys: generates convoy orders for fleets in sea provinces with valid army routes
- Retreats: excludes occupied provinces and attacker origin
- Builds: only allows builds in unoccupied home SCs of the correct power
- Random orders are always legal (no validation failures when resolved)
- Unit tests for tricky cases: split-coast moves, convoy chains, build eligibility
- The engine can play a full game of random moves without producing illegal orders

## Estimated Effort: M
