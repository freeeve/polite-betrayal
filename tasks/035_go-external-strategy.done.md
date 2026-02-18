# Implement ExternalStrategy in Go

## Status: Pending

## Dependencies
- 034 (Go DFEN/DSON codecs)

## Description
Add a new `ExternalStrategy` to the Go bot system that launches and communicates with any DUI-compatible engine process via stdin/stdout.

1. **ExternalStrategy struct** (`api/internal/bot/strategy_external.go`):
   - Fields: engine path, process handle, stdin writer, stdout scanner, power, timeout, options
   - Implements the `Strategy` interface: `GenerateMovementOrders`, `GenerateRetreatOrders`, `GenerateBuildOrders`

2. **Process lifecycle**:
   - `NewExternalStrategy(enginePath string, opts ...Option)` — spawn process, perform DUI handshake
   - Send `dui`, wait for `duiok`
   - Send configured `setoption` commands
   - Send `isready`, wait for `readyok`
   - `Close()` — send `quit`, wait for process exit with timeout, force kill if needed

3. **Order generation flow**:
   - Send `newgame` at game start
   - For each phase: `position <dfen>` -> `setpower <power>` -> `go movetime <ms>`
   - Read lines until `bestorders` response
   - Parse DSON orders back to `[]OrderInput`
   - Handle timeout: if no response within deadline, send `stop` and read final `bestorders`

4. **Engine pool** (optional, stretch):
   - `EnginePool` for reusing engine processes across arena games
   - Acquire/release pattern with configurable pool size

5. **Difficulty mapping**:
   - Add "impossible" difficulty in `StrategyForDifficulty` that uses `ExternalStrategy`
   - Configurable engine path and options

## Acceptance Criteria
- ExternalStrategy implements the full Strategy interface
- Can launch a DUI engine subprocess and complete the handshake
- Timeout handling: correctly sends `stop` if engine exceeds time budget
- Process cleanup: no zombie processes on error or normal shutdown
- Graceful degradation: returns hold orders if engine crashes mid-game
- Unit tests with a mock engine (simple script that responds to DUI commands)
- Integration point: can be selected via difficulty="impossible" in game creation

## Estimated Effort: M
