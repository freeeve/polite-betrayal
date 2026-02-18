# Third-Party DUI Engine Support

## Status: Pending

## Dependencies
- 028 (DUI protocol spec — finalized)
- 035 (Go ExternalStrategy — working engine launcher)

## Description
Enable third-party developers to write their own DUI-compatible Diplomacy engines that plug into the game server.

1. **Protocol documentation**:
   - Publish the DUI protocol spec as a standalone document
   - Include example implementations in Python and/or JavaScript (minimal working engines)
   - Document the exact binary path and argument conventions

2. **Engine registration**:
   - Configuration file listing available engines (path, name, default options)
   - Server discovers engines at startup
   - API endpoint to list available engines

3. **Example engine** (`engine/examples/random_engine.py`):
   - Minimal Python DUI engine that plays random legal moves
   - Demonstrates the protocol handshake, DFEN parsing, and DSON output
   - Serves as a template for third-party developers

4. **Validation tool**:
   - Script that sends a suite of DUI commands to an engine and validates responses
   - Tests: handshake, position parsing, legal order generation, timeout handling
   - Pass/fail report for protocol compliance

## Acceptance Criteria
- Example Python engine works end-to-end with the Go server
- Protocol validation tool catches common implementation errors
- Documentation is sufficient for a developer to write a DUI engine without reading Go/Rust source
- At least one non-Rust engine (Python example) passes all validation tests

## Estimated Effort: M
