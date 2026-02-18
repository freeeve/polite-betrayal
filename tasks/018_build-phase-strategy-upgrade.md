# Build Phase Strategy Upgrade

## Goal
Improve build/disband decisions for Medium, Hard, and Extreme bots which all delegate builds to Easy (HeuristicStrategy).

## Investigation
Current build logic (strategy_easy.go:516-533, generateBuilds/generateDisbands):
- **Builds**: Pick home SCs closest to nearest unowned SC. Fleet if ratio < 25%, else 20% random chance.
- **Disbands**: Units farthest from any unowned SC are disbanded first.

This is reasonable but has blind spots:
- No awareness of which theater needs reinforcements (e.g. building fleet in St. Petersburg when the fight is in the Balkans)
- Fleet ratio heuristic doesn't consider geography (e.g. England/Italy need more fleets than Austria/Russia)
- Disband logic doesn't consider defensive value (might disband a unit defending a home SC)

## Delegation Chain
- Easy: own build logic (described above)
- Medium: delegates to Easy
- Hard: delegates to Easy
- Extreme: delegates to Easy

## Proposed Changes
1. **Power-aware fleet ratio targets**: England/Italy/Turkey want ~50% fleets, Austria/Germany want ~20%, France/Russia ~30%
2. **Theater-aware builds**: Score home SCs based on proximity to the "active front" (contested SCs) rather than just nearest unowned SC
3. **Defensive disband logic**: Never disband the last unit adjacent to an unowned home SC
4. **Optional: search-based builds for Hard/Extreme**: Evaluate position after each possible build combo (build space is small enough)

## Acceptance Criteria
- Build decisions visibly improve for powers with specific fleet needs (England, Italy, Turkey)
- Disband decisions preserve defensive coverage
- No regression in build-heavy endgame scenarios

## Key Files
- `api/internal/bot/strategy_easy.go` — generateBuilds, generateDisbands, GenerateBuildOrders
- `api/internal/bot/strategy_hard.go:283` — delegates to HeuristicStrategy
- `api/internal/bot/strategy_medium.go:199` — delegates to HeuristicStrategy
- `api/internal/bot/strategy_extreme.go:64` — delegates to HeuristicStrategy
