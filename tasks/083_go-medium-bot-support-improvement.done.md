# Task 083: Go Medium Bot Support Improvement

## Status: Needs Rework

## Background
Original changes (cohesion bonus, injectSupports, buildOrdersFromScored, numSamples 24->12)
caused a regression: medium bot win rate dropped from 9% to 4% as Austria vs 6 Easy bots.

## What was reverted
- Territorial cohesion bonus in EvaluatePosition (search_util.go)
- Post-search injectSupports method (strategy_medium.go)
- buildOrdersFromScored candidates from hard bot (strategy_medium.go)

## What was kept
- numSamples increase from 12 to 24 (safe change, more diversity)

## Next steps
- Investigate each reverted change individually to determine which caused the regression
- Consider re-adding changes one at a time with benchmark validation
