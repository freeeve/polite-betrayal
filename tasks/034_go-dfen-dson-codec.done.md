# Implement DFEN/DSON Codecs in Go

## Status: Pending

## Dependencies
- 028 (DUI protocol spec)

## Description
Implement DFEN encoding and DSON parsing/formatting on the Go side so the server can communicate board state and orders with any DUI engine.

1. **DFEN encoder** (`api/pkg/diplomacy/dfen.go` or `api/internal/bot/dfen.go`):
   - `EncodeDFEN(gs *GameState) string` — serialize a GameState to DFEN
   - Handle all phase types (movement, retreat, build)
   - Handle split-coast provinces with dot notation
   - Handle dislodged units with attacker origin
   - Deterministic output for reproducibility

2. **DFEN decoder** (for testing/validation):
   - `DecodeDFEN(s string) (*GameState, error)` — parse DFEN back to GameState
   - Round-trip tests: encode(gs) -> decode -> compare

3. **DSON formatter**:
   - `FormatDSON(orders []OrderInput) string` — serialize orders to DSON for `bestorders` parsing
   - `ParseDSON(s string) ([]OrderInput, error)` — parse engine's `bestorders` response

4. **Cross-validation**: verify Go DFEN output matches expected Rust DFEN parsing by testing identical board states

## Acceptance Criteria
- DFEN encodes the initial game state correctly (matches spec example)
- DFEN round-trip: decode(encode(gs)) produces equivalent GameState
- DSON can format and parse all order types from existing Go Order struct
- Cross-validation test: encode a GameState in Go, decode in Rust, verify identical
- Unit tests cover edge cases: empty dislodged, split coasts, all 7 powers, neutral SCs

## Estimated Effort: M
