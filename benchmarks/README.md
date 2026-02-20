# Benchmark Results Summary

Arena benchmark results for Polite Betrayal Diplomacy bots. Each matchup pits one bot (the "tested" bot) as a single power against 6 opponents of a different tier. Win rate is from the perspective of the tested bot (the first one named in each matchup).

## Summary Table

| Matchup | Best Win Rate | Games | Date | Notes | Link |
|---------|--------------|-------|------|-------|------|
| Easy vs Random | 100% | 700 | 2026-02-18 | All 7 powers, 100 games each | [link](easy-vs-random/2026-02-18-all-powers-post-perf.md) |
| Medium vs Easy | 35% (best power: Turkey 65%) | 700 | 2026-02-18 | Exp F blend (best config); overall 35% | [link](experiments/2026-02-18-ply-experiment-f.md) |
| Rust vs Easy | 22.5% (partial, 4/7 powers) | 40 | 2026-02-18 | Post-phantom-fix; France/Germany 40% | [link](rust-vs-medium/2026-02-18-post-phantom-fix.md) |
| Rust vs Medium | 20% | 70 | 2026-02-19 | Pre-new-model baseline; France 60% | [link](rust-vs-medium/2026-02-19-pre-newmodel.md) |
| Hard vs Medium | 15.7% | 70 | 2026-02-19 | Turkey 40% best power | [link](hard-vs-medium/2026-02-18-baseline-s7.md) |
| Rust vs Hard | 20% vs 14.3% | 70 each | 2026-02-19 | Rust outperforms Hard vs Medium opponents | [link](rust-vs-hard/2026-02-19-comparison.md) |

---

## Easy vs Random

Easy bot (1) vs Random bots (6). Target: ~100% win rate.

| Run | Date | Games | Overall Win% | Best Power | Notes | Link |
|-----|------|-------|-------------|------------|-------|------|
| Initial baseline | 2026-02-17 | 140 | 100% | Russia (1905.8 avg victory) | 20 games/power, all powers 100% | [link](easy-vs-random/2026-02-17.md) |
| Post-perf optimization | 2026-02-18 | 700 | 100% | Russia (1905.3 avg victory) | 100 games/power, confirmed at scale | [link](easy-vs-random/2026-02-18-all-powers-post-perf.md) |

All 7 powers maintain 100% win rate across both runs. Continental powers finish by ~1906 on average; England is slowest (~1913) due to island start.

---

## Medium vs Easy

Medium bot (1) vs Easy bots (6). Target: ~100% win rate.

| Run | Date | Games | Overall Win% | Best Power | Notes | Link |
|-----|------|-------|-------------|------------|-------|------|
| France+Turkey only | 2026-02-17 | 40 | France 35%, Turkey 25% | France 35% | Initial 2-power probe | [link](medium-vs-easy/2026-02-17.md) |
| With opening book | 2026-02-17 | 40 | France 40%, Turkey 25% | France 40% | Marginal book improvement | [link](medium-vs-easy/2026-02-17-with-book.md) |
| All powers | 2026-02-17 | 700 | 13% | France 33%, Turkey 31% | First full all-powers run | [link](medium-vs-easy/2026-02-17-all-powers.md) |
| Pre-support baseline | 2026-02-18 | 700 | 23% | Turkey 51%, France 44% | Post-neural integration | [link](medium-vs-easy/2026-02-18-all-powers-pre-support.md) |
| Post-support | 2026-02-18 | 700 | 21.4% | Turkey 45%, France 40% | Support changes: Russia +7pp, Austria -5pp | [link](medium-vs-easy/2026-02-18-all-powers-post-support.md) |

**Best configuration from experiments**: Experiment F (3-ply blend 0.5/0.2/0.3) achieved **35.3%** overall win rate with France 63%, Turkey 67%, England 41%. See [Experiments](#experiments) section.

**Key findings**:
- France and Turkey are the strongest medium bot powers (corner/edge positions)
- Russia is consistently the weakest (0-11%) due to sprawling multi-front borders
- Win rate roughly doubled (13% to 23%) after neural-guided search integration
- Bimodal outcomes persist: medium either snowballs or collapses

---

## Rust vs Easy

Rust engine (1) vs Easy bots (6). Target: >80% win rate.

| Run | Date | Games | Overall Win% | Best Power | Notes | Link |
|-----|------|-------|-------------|------------|-------|------|
| Phase 2 (France only) | 2026-02-18 | 10 | 30% | France 30% | Initial RM+ baseline, France only | [link](rust-vs-medium/2026-02-18-phase2-vs-go.md) |
| Pre-improvement baseline | 2026-02-18 | 21 | 10% | France/Turkey 33% | All powers, 3 games each | [link](rust-vs-easy/2026-02-18-all-powers.md) |
| Post-improvements | 2026-02-18 | 21 | 10% | France 67% | All engine improvements stacked | [link](rust-vs-easy/2026-02-18-all-powers-post-improvements.md) |
| Post-phantom-fix | 2026-02-18 | 40 | 22.5% | France/Germany 40% | 10 games/power, 4 of 7 powers complete | [link](rust-vs-medium/2026-02-18-post-phantom-fix.md) |

**Status**: Still far below 80% target. France is the only consistently competitive power. England builds large empires (11+ SCs) but cannot close out victories.

---

## Rust vs Medium

Rust engine (1) vs Medium bots (6).

| Run | Date | Games | Overall Win% | Best Power | Notes | Link |
|-----|------|-------|-------------|------------|-------|------|
| Phase 2 (France only) | 2026-02-18 | 10 | 10% | France 10% | Initial RM+ baseline | [link](rust-vs-medium/2026-02-18-phase2-vs-go.md) |
| Base (pre-fix) | 2026-02-18 | 70 | 15.7% | France 40% | All powers, 10 games each | [link](rust-vs-medium/2026-02-18-base.md) |
| First support fix | 2026-02-18 | 70 | 17.1% | France 50% | Cross-power phantom fix | [link](rust-vs-medium/2026-02-18-first-support-fix.md) |
| Second support fix | 2026-02-18 | 70 | 20.0% | France 70% | Hold fallback phantom fix | [link](rust-vs-medium/2026-02-18-second-support-fix.md) |
| All powers (neural) | 2026-02-18 | 70 | 17% | France 40% | Full neural eval mode | [link](rust-vs-medium/2026-02-18-all-powers.md) |
| Value net blend | 2026-02-18 | 21 | 14% | France/Italy/Turkey 33% | Marginal improvement, shifted win distribution | [link](rust-vs-medium/2026-02-18-value-net-blend.md) |
| Progression summary | 2026-02-18 | 210 | 15.7% to 20.0% | France 40% to 70% | Tracks 3 builds | [link](rust-vs-medium/2026-02-18-progression.md) |
| Pre-new-model baseline | 2026-02-19 | 70 | 20% | France 60% | Old smaller neural model | [link](rust-vs-medium/2026-02-19-pre-newmodel.md) |

**Key findings**:
- France is consistently the strongest power for the Rust engine (40-70%)
- Phantom support fixes were the biggest single improvement (+4.3pp overall, France 40% to 70%)
- England accumulates high SCs (10+) but 0% win rate -- endgame conversion problem
- Germany and Russia remain near 0% across all builds

---

## Hard vs Medium

Hard bot (1) vs Medium bots (6).

| Run | Date | Games | Overall Win% | Best Power | Notes | Link |
|-----|------|-------|-------------|------------|-------|------|
| Session 7 (run 1) | 2026-02-19 | 70 | 15.7% | Turkey 40% | 10 games/power, DryRun | [link](hard-vs-medium/2026-02-18-baseline-s7.md) |
| Session 7 (run 2) | 2026-02-19 | 70 | 14.3% | Turkey 40% | 5s time budget | [link](hard-vs-medium/2026-02-19-baseline-s7.md) |

**Key findings**:
- Hard bot wins ~15% overall, roughly 1 in 6 games
- Turkey is the dominant power for the hard bot (40% in both runs)
- Russia is 0% in both runs; Germany and England struggle (0-10%)
- England has high avg SCs (10+) but cannot close out victories, similar to Rust engine

---

## Rust vs Hard

Comparison of Rust engine and Hard bot, both playing against Medium opponents.

| Run | Date | Notes | Link |
|-----|------|-------|------|
| Side-by-side comparison | 2026-02-19 | Rust 20% vs Hard 14.3% overall | [link](rust-vs-hard/2026-02-19-comparison.md) |

**Key findings**:
- Rust outperforms Hard overall (20% vs 14.3%) with ~25,000x less compute
- Rust dominates as France (60% vs 10%), Italy (30% vs 20%), England (20% vs 10%)
- Hard only beats Rust on Turkey (40% vs 20%) and Austria (20% vs 10%)
- Germany and Russia are 0% for both engines

---

## Experiments

Medium bot configuration experiments, all testing 100 games per power (700 total) against Easy bots.

| Experiment | Date | Overall Win% | Key Result | Link |
|------------|------|-------------|------------|------|
| Exp A: Cohesion bonus | 2026-02-18 | 21% | Slight regression vs 23% baseline; France -8pp | [link](experiments/2026-02-18-experiment-a-cohesion-bonus.md) |
| Exp B: Inject supports | 2026-02-18 | 26% | +3pp vs baseline; England +15pp, Turkey +8pp | [link](experiments/2026-02-18-experiment-b-inject-supports.md) |
| Exp C: Build orders | 2026-02-18 | 28% | +5pp vs baseline; best single-change improvement | [link](experiments/2026-02-18-experiment-c-buildorders.md) |
| Exp D: Pure 3-ply | 2026-02-18 | 27% | Turkey/England +16pp each, France -13pp | [link](experiments/2026-02-18-ply-experiment-d.md) |
| Exp E: Blend ply-1+3 | 2026-02-18 | 32.6% | All powers improved; France +13pp, England +20pp | [link](experiments/2026-02-18-ply-experiment-e.md) |
| **Exp F: Blend all 3 plies** | **2026-02-18** | **35.3%** | **WINNER: +7.3pp vs baseline; all powers improved** | [link](experiments/2026-02-18-ply-experiment-f.md) |
| Exp F (all powers detail) | 2026-02-18 | 34.7% | Detailed SC timelines for Exp F variant | [link](experiments/2026-02-18-ply-experiment-f-blend-all.md) |
| Extended opening book | 2026-02-18 | 23.4% | Book extension hurt performance vs Exp F (-12pp) | [link](experiments/2026-02-18-opening-book-extended.md) |
| Austria regression (task 083) | 2026-02-18 | — | Austria dropped 9% to 4% after support changes | [link](experiments/2026-02-18-austria-regression-task083.md) |

### Rust Engine Performance Experiments

| Experiment | Date | Key Result | Link |
|------------|------|------------|------|
| Engine profile (pre-opt) | 2026-02-18 | ~5K-6K nodes/sec, 2-ply lookahead is bottleneck | [link](experiments/2026-02-18-engine-profile.md) |
| Engine profile (post-opt) | 2026-02-18 | ~65K nodes/sec, 12x improvement via caching | [link](experiments/2026-02-18-engine-profile-post-opt.md) |
| Optimization round 2 | 2026-02-18 | K=16 candidates, budget rebalance, parallel warm-start | [link](experiments/2026-02-18-engine-opt-round2.md) |

---

## Analysis

In-depth analysis documents covering bot architecture, comparisons, and strategic patterns.

| Document | Description | Link |
|----------|------------|------|
| Baseline results | Historical win rates for Easy/Medium/Hard bots across development iterations | [link](analysis/baseline-results.md) |
| Opening book analysis | Statistics on 127K-game opening book: 2,587 clusters, 5,081 order variants | [link](analysis/opening-book.md) |
| Cicero comparison | Side-by-side comparison of our Rust engine architecture vs Meta's Cicero system | [link](analysis/cicero-comparison.md) |
