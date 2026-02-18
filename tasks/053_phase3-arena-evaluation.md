# Phase 3 Arena Evaluation: Neural Engine vs Go Hard Bot

## Status: Pending

## Dependencies
- 052 (Neural-guided search)
- 044 (Phase 2 benchmark — baseline numbers)

## Description
Run the Phase 3 milestone evaluation: neural-guided Rust engine vs Go hard bot in arena games. Target: beat Go hard bot >60%.

1. **Evaluation matrix**:
   - realpolitik (neural + RM+) vs 6 Go hard bots — 20 games as France
   - realpolitik (neural + RM+) vs 6 Go hard bots — 20 games as Turkey
   - realpolitik (neural + RM+) vs 6 Go hard bots — 10 games as each other power
   - Compare against Phase 2 baseline (heuristic-only RM+)

2. **Ablation studies**:
   - Neural policy only (no value network) vs full neural
   - Heuristic eval only (no neural) vs full neural
   - Policy-guided candidates vs heuristic candidates (both with neural value)
   - Measure contribution of each component

3. **Metrics**:
   - Win rate (solo victory)
   - Average SC count at year 10
   - Survival rate
   - Per-phase decision time
   - Neural inference overhead (% of total search time)

4. **Regression check**: verify heuristic-only mode still matches Phase 2 performance

## Acceptance Criteria
- Neural engine beats Go hard bot > 60% (solo wins, playing France)
- Neural engine improvement over heuristic-only is statistically significant
- Ablation results documented
- Total evaluation runs complete within a reasonable timeframe (~2 hours for all games)
- Results written up with charts/tables for comparison

## Estimated Effort: M
