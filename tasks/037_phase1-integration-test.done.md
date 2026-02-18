# Phase 1 Integration Test: Go Server to Rust Engine

## Status: Pending

## Dependencies
- 033 (Rust DUI protocol loop)
- 035 (Go ExternalStrategy)
- 036 (Rust random move generator)

## Description
End-to-end integration test proving the Go server can launch the Rust engine, send it positions via DUI, and receive legal orders back. This is the Phase 1 milestone deliverable.

1. **Build pipeline**:
   - Test script that builds the Rust engine (`cargo build --release`)
   - Then runs Go integration tests that reference the built binary

2. **Integration test** (`api/internal/bot/strategy_external_test.go`):
   - Spawn the realpolitik engine binary
   - Complete DUI handshake (dui -> duiok)
   - Send initial position via DFEN
   - Set power to each of the 7 powers in sequence
   - Issue `go movetime 1000` and read `bestorders`
   - Verify returned orders are legal by passing them through Go's order validation
   - Repeat for a few turns to simulate a short game

3. **Cross-validation test**:
   - Encode a known GameState as DFEN in Go
   - Send to Rust engine, have it echo back the parsed state (via a debug command or by verifying orders reference valid provinces)
   - Verify no data loss in the DFEN encoding/decoding

4. **Arena smoke test**:
   - Run a single arena game with the Rust engine (random moves) against Go easy bots
   - Verify the game completes without crashes or protocol errors
   - Rust engine will lose (random moves), but the game must finish cleanly

## Acceptance Criteria
- Integration test passes: Go launches Rust engine, exchanges DUI messages, receives legal orders
- No zombie processes after test completion
- Arena smoke test completes a full game (up to year limit) without errors
- All 7 powers can be played by the Rust engine in a single test
- Test runs in CI (once CI is set up) or locally via `make test-integration`

## Estimated Effort: M
