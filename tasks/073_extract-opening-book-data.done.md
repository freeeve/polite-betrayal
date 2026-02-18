# Extract Opening Book Data from Historical Games

## Status: In Progress

## Dependencies
- 046 (data pipeline — done)
- Game data must be downloaded (check if ml-train has completed)

## Description
Write a Python script to mine historical Diplomacy games for opening book entries through Fall 1902.

### Pipeline
1. Parse historical games using existing data/scripts infrastructure
2. Extract order sequences per power: S1901 → F1901 → W1901 builds → S1902 → F1902
3. Track game outcomes to weight by success
4. Cluster by position fingerprint (unit positions after each phase)
5. Filter to branches appearing in >5% of games
6. Output weighted entries as JSON for Go integration

### Output
- `data/scripts/extract_openings.py` — extraction script
- `benchmarks/opening-book-analysis.md` — coverage stats and analysis
- Generated opening data (JSON or Go code)

## Acceptance Criteria
- Script runs successfully on downloaded game data
- Covers all 7 powers through Fall 1902
- Each entry weighted by frequency and game outcome
- Sparse coverage (only common branches) with clear fallback points
- Stats show what % of games are covered by the book

## Estimated Effort: M
