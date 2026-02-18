# Implement DUI Protocol Parser and Main Loop in Rust

## Status: Pending

## Dependencies
- 028 (DUI protocol spec)
- 031 (DFEN codec in Rust)
- 032 (DSON codec in Rust)

## Description
Implement the full DUI protocol command parser and main event loop in the Rust engine.

1. **Command parser** (`src/protocol/parser.rs`):
   - Parse all server-to-engine commands: `dui`, `isready`, `setoption`, `newgame`, `position`, `setpower`, `go`, `stop`, `press`, `quit`
   - Extract arguments: movetime, depth, nodes, infinite flag
   - Parse `setoption name <id> value <x>` with type validation

2. **Response formatter**:
   - Format all engine-to-server responses: `id`, `option`, `duiok`, `readyok`, `info`, `bestorders`, `press_out`
   - Info line formatting with optional fields (depth, nodes, nps, time, score, pv)

3. **Main loop** (`src/main.rs`):
   - Blocking stdin read loop (one command per line)
   - Dispatch commands to appropriate handlers
   - `dui` -> respond with id lines + declared options + `duiok`
   - `isready` -> respond with `readyok`
   - `position` -> decode DFEN and store current state
   - `setpower` -> store active power
   - `go` -> launch search (initially just call random move gen) and respond with `bestorders`
   - `stop` -> interrupt search and emit current best
   - `quit` -> clean exit
   - `newgame` -> reset engine state

4. **Engine state struct**: holds current position, active power, options, search handle

## Acceptance Criteria
- Engine correctly handles the full session flow from section 3.6 of the spec
- All commands parse without error for valid input
- Unknown commands are ignored gracefully (logged to stderr)
- Malformed commands produce error output to stderr but do not crash
- `go` currently returns random legal moves (placeholder until search is implemented)
- Protocol test: script sends a sequence of DUI commands via stdin and validates responses
- Engine exits cleanly on `quit` or EOF

## Estimated Effort: M
