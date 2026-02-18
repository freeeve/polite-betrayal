# Data Pipeline: Download and Parse Game Datasets

## Status: Pending

## Dependencies
None (can start in parallel with Phase 1/2)

## Description
Build the data ingestion pipeline to download, parse, and normalize Diplomacy game datasets for training. This is the first step of the Phase 3 ML pipeline.

1. **Data sources** (priority order):
   - Facebook Diplomacy Research dataset (~46K no-press games) — GitHub, most accessible
   - Kaggle Diplomacy Game Dataset (~5K games) — public download
   - webDiplomacy.net (~156K games) — requires admin contact, fallback if unavailable
   - Kaggle Betrayal Dataset (500 games with press) — for future press work

2. **Parser scripts** (`engine/data/scripts/`):
   - Python scripts to parse each dataset format into a unified internal format
   - Normalize territory IDs to 3-letter codes matching our province enum
   - Handle variant differences (some datasets use different naming conventions)
   - Deduplicate games that appear in multiple datasets

3. **Unified game format** (Parquet or JSON lines):
   - Game metadata: source, player count, outcome, year range
   - Per-phase records: DFEN state, orders for all powers, resolution results
   - Outcome labels: final SC distribution, win/draw/loss per power

4. **Validation**:
   - Verify all orders are legal for the given position
   - Flag and quarantine games with invalid/corrupt data
   - Statistics: total games, total phases, orders per phase, phase type distribution

## Acceptance Criteria
- At least 40K games parsed and validated (Facebook dataset alone)
- Unified format documented with schema
- Validation report: <5% of games quarantined for data issues
- Pipeline is idempotent (re-running produces identical output)
- Data stored in `engine/data/processed/` (gitignored, with download script)
- README in `engine/data/` explains how to reproduce the dataset

## Estimated Effort: L
