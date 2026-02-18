# Port Province and Adjacency Data to Rust

## Status: Pending

## Dependencies
- 029 (Rust scaffold)

## Description
Port the province definitions and adjacency graph from `api/pkg/diplomacy/map_data.go` to Rust as compile-time constants.

1. Define `Province` enum with all 75 provinces as `#[repr(u8)]` variants (0..74)
2. Define `Coast` enum (None, North, South, East)
3. Define `UnitType` enum (Army, Fleet)
4. Define `Power` enum (Austria, England, France, Germany, Italy, Russia, Turkey)
5. Create `ProvinceInfo` struct with: name, abbreviation, is_supply_center, province_type (Land, Sea, Coastal), home_power
6. Build compile-time const adjacency table: for each province, list of adjacent provinces with flags for army/fleet passability and coast information
7. Handle split-coast provinces correctly: spa (nc/sc), stp (nc/sc), bul (ec/sc)
8. Cross-validate against Go `map_data.go` — every adjacency in Go must exist in Rust and vice versa

Key gotchas from MEMORY.md:
- Smyrna and Ankara are NOT adjacent
- Vienna and Venice are NOT adjacent
- Split-coast handling must match Go exactly

## Acceptance Criteria
- All 75 provinces defined with correct metadata
- Adjacency graph matches `map_data.go` exactly (write a cross-validation test)
- Split-coast provinces have correct coast-specific adjacencies
- Province lookup by abbreviation (3-letter string -> Province enum) works
- All data is `const` or `static` (no runtime allocation)
- Unit tests verify: province count, SC count (34 total), adjacency symmetry (if A adj B then B adj A)

## Estimated Effort: M
