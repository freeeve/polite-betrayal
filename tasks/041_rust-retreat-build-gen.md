# Port Retreat and Build Order Generation to Rust

## Status: Pending

## Dependencies
- 038 (Resolver — needs resolution output types)
- 036 (Move generator — extends movegen module)

## Description
Complete the move generation for retreat and build phases in Rust, porting logic from Go's `retreat.go` and `build.go`.

1. **Retreat resolution** (`src/resolve/` or `src/movegen/retreat.rs`):
   - Port `ResolveRetreats` — handle conflicts when two dislodged units retreat to the same province
   - Port `ApplyRetreats` — update board state after retreat resolution
   - Civil disorder: auto-disband units with no valid retreat

2. **Build resolution** (`src/resolve/` or `src/movegen/build.rs`):
   - Port `ResolveBuildOrders` — validate and apply builds/disbands
   - Port civil disorder logic: auto-disband furthest units if player submits insufficient disbands
   - Waive handling: player can voluntarily skip builds

3. **Phase sequencing** (`src/board/`):
   - Port phase advancement logic from `phase.go`
   - Movement -> Retreat (if dislodged units exist) -> Fall Movement -> Build -> next year
   - SC ownership update after Fall resolution

## Acceptance Criteria
- Retreat generation produces correct legal retreats (no occupied provinces, no attacker origin)
- Retreat conflicts resolved correctly (both units disbanded)
- Build phase: correct number of builds/disbands based on SC count vs unit count
- Civil disorder works: missing orders result in automatic disbands
- Phase advancement correctly sequences through a full game year
- SC ownership updates only after Fall phase
- Tests cover: retreat conflicts, civil disorder, waive builds, disband selection

## Estimated Effort: M
