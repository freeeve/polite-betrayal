# Structured Press Intent Support in DUI Protocol

## Status: Pending

## Dependencies
- 033 (DUI protocol loop — extend protocol)
- 043 (RM+ search — needs to factor in press)

## Description
Implement the structured press (diplomacy) commands in the DUI protocol, matching the existing Go `DiplomaticIntent` types from `diplomacy_msg.go`.

1. **DUI press commands** (already specified in section 3.2):
   - Inbound: `press <from_power> <message_type> [args...]`
   - Outbound: `press_out <to_power> <message_type> [args...]`
   - Message types: request_support, propose_nonaggression, propose_alliance, threaten, offer_deal, accept, reject

2. **Rust press handling**:
   - Parse inbound press commands and store in engine state
   - Factor press into search: trust model adjusts opponent modeling weights
   - Simple trust heuristic: increase cooperation weight for allies, decrease for threats
   - Generate outbound press based on search results (e.g., request support for planned moves)

3. **Go integration**:
   - ExternalStrategy sends press messages from other players to the engine
   - Reads press_out and forwards to the game's messaging system
   - Map between Go DiplomaticIntent types and DUI press format

4. **Trust model** (simple v1):
   - Track per-power trust score based on whether they followed through on press commitments
   - Trust decays over time, increases with fulfilled agreements
   - Used to weight opponent modeling in RM+ search

## Acceptance Criteria
- Engine receives and parses all press message types without error
- Engine factors press into move selection (measurable difference with/without press)
- Engine generates contextually appropriate outbound press (e.g., requests support for actual planned moves)
- Trust model updates correctly based on follow-through
- Integration test: full game with press exchange between Go server and Rust engine

## Estimated Effort: M
