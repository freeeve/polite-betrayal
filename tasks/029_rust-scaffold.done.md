# Rust Engine Scaffold (realpolitik)

## Status: Pending

## Dependencies
None

## Description
Create the Rust project scaffold for the realpolitik engine at `engine/` in the repo root. Set up the Cargo workspace, directory structure, and basic build pipeline.

1. Initialize `engine/Cargo.toml` with workspace configuration
2. Create the directory structure from section 4.1 of task 024:
   - `src/main.rs` — entry point with placeholder DUI loop
   - `src/protocol/` — mod.rs, parser.rs, dfen.rs, dson.rs (stubs)
   - `src/board/` — mod.rs, state.rs, province.rs, adjacency.rs, unit.rs, order.rs (stubs)
   - `src/movegen/` — mod.rs, movement.rs, retreat.rs, build.rs (stubs)
   - `src/resolve/` — mod.rs, kruijswijk.rs (stubs)
   - `src/search/` — mod.rs (stub)
   - `src/eval/` — mod.rs (stub)
   - `tests/` directory
3. Add initial dependencies: `thiserror` for error handling, dev deps for testing
4. Ensure `cargo build` and `cargo test` pass (with stub code)
5. Add engine metadata: name "realpolitik", version 0.1.0

## Acceptance Criteria
- `engine/` directory exists at repo root with complete directory structure
- `cargo build` succeeds with no errors
- `cargo test` succeeds (even if no real tests yet)
- `main.rs` has a basic stdin/stdout loop that responds to `dui` with `id name realpolitik` / `id author polite-betrayal` / `duiok` and `quit` to exit
- Code compiles in release mode: `cargo build --release`

## Estimated Effort: S
