# DATC Compliance Tests for Rust Resolver

## Status: Pending

## Dependencies
- 038 (Kruijswijk resolver in Rust)

## Description
Implement the Diplomacy Adjudicator Test Cases (DATC) as Rust tests to validate the resolver's correctness. The DATC is the standard test suite for Diplomacy adjudication engines.

1. **Test infrastructure** (`tests/datc_tests.rs`):
   - Helper functions to set up board positions from compact descriptions
   - Helper to submit orders and check resolution outcomes
   - Macro or builder pattern for concise test case definitions

2. **DATC sections to implement**:
   - **Section 6.A**: Basic movement (holds, moves, bounces)
   - **Section 6.B**: Coastal issues (split coasts, fleet movement)
   - **Section 6.C**: Circular movement
   - **Section 6.D**: Supports and cutting supports
   - **Section 6.E**: Head-to-head battles
   - **Section 6.F**: Convoys
   - **Section 6.G**: Convoy disruption and paradoxes
   - **Section 6.H**: Retreats
   - **Section 6.I**: Builds and disbands

3. **Cross-validation tests**:
   - Port test cases that already exist in Go (`api/pkg/diplomacy/*_test.go`)
   - Verify Rust produces identical results for every case

4. **Regression tests**:
   - Add test cases for known gotchas from MEMORY.md (Smyrna-Ankara non-adjacency, Vienna-Venice non-adjacency)

## Acceptance Criteria
- All DATC basic cases (6.A through 6.I) pass
- At least 60 individual test cases implemented
- Known edge cases from Go test suite are ported
- Convoy paradox handling matches Go implementation's behavior
- Tests run in under 5 seconds total
- Any failures produce clear output showing expected vs actual resolution

## Estimated Effort: M
