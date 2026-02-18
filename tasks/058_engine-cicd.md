# CI/CD for Engine Builds and Tests

## Status: Pending

## Dependencies
- 057 (Engine packaging)
- 039 (DATC tests — need tests to run in CI)

## Description
Set up continuous integration for the Rust engine, including automated builds, testing, and benchmarks.

1. **CI pipeline** (GitHub Actions or similar):
   - Trigger on push to engine/ directory
   - Build: `cargo build --release`
   - Test: `cargo test` (unit tests + DATC compliance)
   - Lint: `cargo clippy -- -D warnings`
   - Format: `cargo fmt -- --check`
   - Integration test: build Go server + Rust engine, run DUI protocol test

2. **Benchmark tracking**:
   - Run performance benchmarks on each PR
   - Compare against baseline, flag regressions > 10%
   - Track: resolution speed, search throughput, inference latency

3. **Artifact publishing**:
   - Build release binaries on tagged versions
   - Publish as GitHub release assets

4. **Go integration CI**:
   - Extend existing Go CI to optionally build Rust engine
   - Run ExternalStrategy integration tests when engine code changes

## Acceptance Criteria
- CI runs automatically on PRs touching `engine/`
- All tests pass in CI (unit, DATC, integration)
- Clippy and rustfmt enforced
- Build artifacts downloadable from CI
- CI completes in < 10 minutes

## Estimated Effort: M
