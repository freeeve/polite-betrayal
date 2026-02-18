# Filter Quick Message Dropdowns by Adjacency and Context

## Goal
Make the quick message (canned message) dropdowns in chat contextually smart — filter province and power selections based on game state, adjacency, and cascading field selections.

## Current Problems
1. **Power dropdown** shows all 7 powers, including yourself and the recipient you're already talking to
2. **"Request support" `to` field** shows all provinces adjacent to ANY of your units — should cascade based on `from` selection
3. **No recipient range check** — you can ask a power for support even if they have no units in range
4. **Province fields** don't account for who you're talking to in DMs

## Requirements

### 1. Cascade `to` based on `from` selection (Request support)
- When user picks a `from` province, `to` dropdown should only show provinces adjacent to that specific province
- Currently `_relevantProvinces` for field `to` uses the full `myAdjacentSet` (union of all unit adjacencies)
- Need to pass `_province1` (the `from` selection) into the province filtering for `to`

### 2. Filter `from` by recipient's support range (Request support)
- Only show your unit locations where the **recipient** actually has a unit that could support
- i.e., for each of your units at province X, check if the recipient has any unit adjacent to X or adjacent to any neighbor of X
- If recipient is null (public chat), show all your units as before

### 3. Power dropdown filtering (Propose alliance)
- Exclude `myPower` (yourself) from the list
- Exclude `recipientPower` (the person you're talking to — you wouldn't propose alliance against them)
- Optionally prioritize powers that are geographic neighbors (have units adjacent to both you and recipient)

### 4. Province context in DMs
- When in a DM tab (`recipientPower` is set), prioritize provinces near both players' territories
- For "Threaten", prioritize the recipient's SCs that are adjacent to your units
- For "Non-aggression pact", prioritize border provinces between you and recipient
- For "Offer deal", `mine` should be near your units, `yours` near their units (already partially done)

## Key Files
- `ui/lib/features/messages/messages_screen.dart` — `_CannedMessagePicker`, `_buildFieldInputs()`, `_relevantProvinces()`
- `ui/lib/features/messages/province_autocomplete.dart` — autocomplete widget
- `ui/lib/core/map/adjacency_data.dart` — `allAdjacent()`, `armyTargets()`, `fleetTargets()`
- `ui/lib/core/models/game_state.dart` — `unitsOf(power)`, `supplyCenters`

## Implementation Notes
- `_CannedMessagePicker` is a `StatefulWidget` with `_province1`, `_province2`, `_power` state vars
- `_buildFieldInputs()` at line 387 builds the form — needs to pass current selections into filtering
- `_relevantProvinces()` at line 462 needs a new optional param for the selected `from` province
- When `_province1` changes via `setState`, the `to` field will automatically rebuild with new filtered suggestions
- `ProvinceAutocomplete` takes a `suggestions` list — just change what's passed in

## Acceptance Criteria
- [ ] Power dropdown excludes self and current chat recipient
- [ ] "Request support" `to` cascades from `from` selection
- [ ] "Request support" `from` only shows units the recipient can actually support (in DMs)
- [ ] Province suggestions are contextually relevant to the conversation (DM vs public)
- [ ] All templates still work correctly (no regressions)
