# DUI Protocol Specification

## Status: Pending

## Dependencies
None

## Description
Finalize the DUI (Diplomacy Universal Interface) protocol specification as documented in task 024, section 3. This includes:

1. **DFEN format** (Diplomacy FEN) — single-line board state encoding with phase info, units, supply centers, and dislodged units. Resolve any ambiguities (e.g., whether to list all SCs or only non-default).
2. **DSON format** (Diplomacy Standard Order Notation) — compact order notation for movement, retreat, and build phases. Formalize the grammar from section 3.4.
3. **Command set** — finalize all server-to-engine and engine-to-server commands (section 3.2/3.3).
4. **Session flow** — document the expected handshake, position setup, search, and termination sequence.
5. **Protocol versioning** — add a version field to the `dui` handshake for future compatibility.

Write the spec as a standalone document at `engine/docs/DUI_PROTOCOL.md` so it can serve as the reference for both Go and Rust implementations.

## Acceptance Criteria
- DFEN grammar is unambiguous and can encode any valid Diplomacy board state (including all split-coast provinces)
- DSON grammar can represent every legal order type (hold, move, support-hold, support-move, convoy, retreat, disband, build, waive)
- All commands are documented with expected arguments and responses
- Session flow example covers a full game turn (movement + retreat + build)
- Protocol version is specified (v1)
- At least 3 example DFEN strings with manually verified correctness (initial position, mid-game, retreat phase)

## Estimated Effort: M
