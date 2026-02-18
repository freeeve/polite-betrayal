# Implement DSON Order Notation in Rust

## Status: Pending

## Dependencies
- 028 (DUI protocol spec)
- 030 (Province/adjacency data in Rust)

## Description
Implement the DSON (Diplomacy Standard Order Notation) parser and formatter in Rust for encoding/decoding individual orders as compact text strings.

DSON covers all order types across all three phases:

1. **Movement phase orders**:
   - Hold: `A vie H`
   - Move: `A bud - rum`
   - Support hold: `A tyr S A vie H`
   - Support move: `A gal S A bud - rum`
   - Convoy: `F mid C A bre - spa`
   - Move to coast: `F nwg - stp/nc`

2. **Retreat phase orders**:
   - Retreat: `A vie R boh`
   - Disband: `F tri D`

3. **Build phase orders**:
   - Build: `A vie B`, `F stp/sc B`
   - Disband: `A war D`
   - Waive: `W`

4. **Multi-order parsing**: Parse `bestorders` lines with semicolon-separated orders

5. **Order struct** (`src/board/order.rs`):
   - Unified order type covering all phases
   - Includes: unit type, location, coast, order type, target, aux fields

## Acceptance Criteria
- Can parse and format every order type listed in DSON grammar (section 3.4)
- Round-trip: format(parse(s)) == s for canonical forms
- Multi-order parsing: correctly splits semicolon-separated order lists
- Coast handling: correctly parses `stp/nc`, `spa/sc`, `bul/ec`
- Error handling: descriptive errors for malformed orders (wrong province, missing target, etc.)
- Fuzz test: random valid orders parse/format correctly
- Unit tests for every order type variant

## Estimated Effort: S
