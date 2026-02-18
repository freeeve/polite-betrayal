# Revert SC Defense Penalty Scaling

## Status: In Progress

## Dependencies
- None

## Description
The SC defense penalty scaling from commit 5a76f70 made the medium bot worse. The scaling formula `scScale = max(0.1, (ownSCs-3)/10.0)` makes defense penalty nearly zero at 3-5 SCs, causing France to abandon home SCs in the opening.

Revert to original values:
- Restore base penalty to `16.0 * threat`
- Remove empire-size scaling
- Remove Phase 2b fallback for negative-score moves (if added in that commit)
- Fix unused "math" import in strategy_medium.go

## Acceptance Criteria
- Medium France avg SCs improve from ~4.6 back toward ~9.2 (20-game benchmark)
- No compilation errors or test regressions

## Estimated Effort: S
