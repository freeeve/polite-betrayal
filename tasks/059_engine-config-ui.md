# Engine Configuration in Flutter UI

## Status: Pending

## Dependencies
- 035 (Go ExternalStrategy — needs backend support for engine selection)
- 057 (Engine packaging — needs a deployable engine)

## Description
Add UI controls in the Flutter frontend to configure the Rust engine when creating games with "impossible" difficulty bots.

1. **Game creation screen**:
   - Add "impossible" difficulty option that uses the external Rust engine
   - Show engine options when impossible difficulty is selected:
     - Search time (1s, 5s, 10s, 30s)
     - Strength (1-100 slider)
     - Model selection (if multiple models available)

2. **Backend API**:
   - Extend game creation endpoint to accept engine configuration
   - Pass configuration through to ExternalStrategy via DUI setoption

3. **Engine status**:
   - Show whether the Rust engine binary is available on the server
   - Graceful fallback message if engine is not installed

## Acceptance Criteria
- UI shows "impossible" difficulty option
- Engine options are configurable from the UI
- Game creation with impossible difficulty successfully launches the Rust engine
- Graceful error handling if engine binary is missing

## Estimated Effort: M
