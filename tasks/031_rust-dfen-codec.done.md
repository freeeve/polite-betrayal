# Implement DFEN Encoder/Decoder in Rust

## Status: Pending

## Dependencies
- 028 (DUI protocol spec)
- 030 (Province/adjacency data in Rust)

## Description
Implement the DFEN (Diplomacy FEN) format in Rust for encoding and decoding complete board states as single-line strings.

DFEN format: `<phase_info>/<units>/<supply_centers>/<dislodged>`

1. **Parser** (`src/protocol/dfen.rs`):
   - Parse phase info: year, season (s/f), phase (m/r/b)
   - Parse unit entries: power char + unit type + location (with optional coast using dot separator)
   - Parse supply center entries: power char + province
   - Parse dislodged entries: power char + unit type + location + attacker origin, or `-` for none
   - Return a `BoardState` struct

2. **Encoder**:
   - Serialize a `BoardState` to DFEN string
   - Deterministic output (sorted units/SCs for reproducibility)

3. **BoardState struct** (`src/board/state.rs`):
   - Fixed-size arrays as described in section 4.2
   - `units: [Option<UnitEntry>; 75]`
   - `sc_owner: [Option<Power>; 75]` (None for non-SC or neutral)
   - `dislodged: [Option<DislodgedEntry>; 75]`
   - Phase info: year, season, phase

4. **Round-trip correctness**: encode(decode(s)) == s for canonical forms

## Acceptance Criteria
- Can parse the initial position DFEN from the protocol spec (section 3.5)
- Can parse mid-game positions with dislodged units
- Round-trip: decode then encode produces identical string
- Error handling: returns descriptive errors for malformed DFEN
- Fuzz test: random valid board states encode/decode correctly
- Unit tests cover: all 7 powers, both unit types, all 3 coast types, empty dislodged ("-"), multiple dislodged units

## Estimated Effort: M
