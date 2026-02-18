# Session State — 2026-02-18 (Session 4)

## Session 4 Commits

| # | Hash | Description |
|---|------|-------------|
| 1 | `b60eaa6` | feat(engine): port opening book with JSON loading and match scoring |
| 2 | `045e2fc` | feat(engine): neural-guided candidate generation and search initialization |
| 3 | `8d4f5ed` | chore(tasks): mark rust retreat/build gen task as done |
| 4 | `38c7218` | fix(api/internal/bot): guard nil SupplyCenters in NearestUnownedSC |
| 5 | `25d4d6a` | feat(api/internal/bot): add BENCH_GAMES and BENCH_VERBOSE env vars |
| 6 | `c988bc3` | chore: add git stash prohibition to CLAUDE.md |
| 7 | `bec3c64` | chore: add BENCH_VERBOSE prohibition to CLAUDE.md |
| 8 | `79d6d49` | feat(engine): add performance profiling benchmarks |
| 9 | `ba151d6` | refactor(api/internal/bot): reset medium bot to clean easy baseline with opening book |
| 10 | `d7a07fc` | docs(benchmarks): Phase 2 Rust engine vs Go bots arena results |
| 11 | `7d325e9` | feat(api/internal/bot): add 1-ply lookahead to medium bot |
| 12 | `900b1e1` | perf(engine): cached lookahead orders and adaptive RM+ iterations |
| 13 | `3e4a951` | feat(engine): wire opening book into move selection pipeline |
| 14 | `20047e5` | feat(api/internal/bot): add front-aware build decisions to medium bot |
| 15 | `5986af4` | revert(api/internal/bot): restore pure 1-ply lookahead (blend was worse) |
| 16 | `b6462a2` | feat(engine): post-optimization profiling benchmarks |

## Key Results

### Rust Engine Profiling
- Bottleneck was movegen in lookahead simulation (67% of node cost)
- RM+ was capped at 48 iterations, wasting 90%+ of 5s time budget
- Fix: cached lookahead orders (P0) + adaptive iterations (P1)
- Result: 10x nodes/sec, 75x total nodes searched

### Rust Engine Post-Optimization Profiling
- 12x throughput improvement confirmed: ~65,000 nodes/sec (was ~5,500)
- Adaptive iterations: ~3,000 at 500ms budget (was fixed 48)
- Budget utilization: 75% (was 10%)
- New bottleneck: second-ply movegen (98% of per-node cost, ~14us of ~15us per node)
- Resolver, clone, apply, eval all negligible (<2% combined)
- Top optimization opportunities:
  - P0: Lightweight greedy movegen (skip support/convoy in lookahead) → 3-5x gain
  - P1: LRU cache for second-ply orders (similar states across iterations)
  - P2: Pre-allocate hot loop Vecs
  - P3: Rayon parallelism for counterfactual evals

### Rust Engine vs Go Bots (Pre-Optimization)
| Tier | Games | Win Rate | Target |
|------|-------|----------|--------|
| Easy | 10 | 30% | >80% |
| Medium | 10 | 10% | >40% |
| Hard | 3 | 0% | -- |

### Medium Bot Rebuild (Incremental from Easy)
| Version | Overall Win% | France | Turkey | Germany | England | Italy | Russia | Austria |
|---------|-------------|--------|--------|---------|---------|-------|--------|---------|
| Baseline (=easy) | 14% | 25% | 50% | 35% | 0% | 10% | 0% | 5% |
| +1-ply lookahead | 30% | 65% | 65% | 30% | 15% | 25% | 5% | 5% |
| +front-aware builds | 33% | 60% | 75% | 30% | 20% | 25% | 20% | 0% |
| +2-ply (reverted) | 25% | 50% | 40% | 40% | 25% | 15% | 0% | 5% |
| +blend 0.7/0.3 (reverted) | 27% | 55% | 55% | 20% | 20% | 20% | 15% | 5% |

Current best: 1-ply + front-aware builds = ~33% (20-game estimate, 700-game run in progress)

## Active Work (in progress)

| Agent | Task | Status |
|-------|------|--------|
| medium-reset | 700-game medium vs easy benchmark | Running |
| rust-bench-700 | 700-game Rust vs Easy (rebuilding with optimizations) | Running |
| rust-profiler2 | Post-optimization Rust profiling | Running |

## CRITICAL: Rust Engine Regression
- Post-optimization Rust engine dropped from 30% to 2.8% win rate vs Easy
- Commits between pre-opt (30%) and post-opt (2.8%): 900b1e1 (adaptive iterations), 3e4a951 (opening book wiring)
- One of these introduced a regression — need to bisect
- Possible causes: adaptive iterations changing search behavior, opening book returning bad moves, cached lookahead orders being stale/wrong
- **First action next session**: investigate and fix this regression

## Action Items After Compaction
- **Verify strategy_medium.go state**: medium-bot-dev may have overwritten medium-reset's clean 98-line file. Check `git log` and `wc -l` to confirm it's the right version (should be ~200 lines with 1-ply lookahead + front-aware builds, NOT the old 2000+ line version)
- Run 700-game medium vs easy benchmark (1-ply + builds baseline)
- Run 700-game Rust vs Easy benchmark (post-optimization with opening book)
- Start ply experiment matrix (task 078)

## Pending Tasks for Next Session

| Task | File | Description |
|------|------|-------------|
| 078 | `078_medium-bot-ply-experiments.md` | Test 1-ply, 3-ply, blend(1+3), blend(1+2+3) at 700 games each |
| 025 | (team task) | Investigate CoreML/Metal GPU acceleration for ONNX inference |
| 045 | `045_rust-perf-optimization.md` | Further Rust engine optimization (post-profiling) |
| 053 | `053_phase3-arena-evaluation.md` | Phase 3 neural engine vs Go hard bot |
| 054 | `054_self-play-pipeline.md` | Self-play game generation for RL training |
| 056 | `056_structured-press-dui.md` | Structured press intent in DUI protocol |

## Artifacts Produced

### Rust Engine Improvements
- Opening book: loaded from JSON, wired into move selection (book hit → skip search)
- Neural-guided search: policy network scores candidates, RM+ init from policy probs, strength parameter
- Performance: 10x throughput via cached lookahead + adaptive iterations
- Profiling benchmarks: criterion benches for movegen, RM+, resolve+eval

### Medium Bot Rebuild
- Reset to 98-line clean base (easy bot + opening book)
- 1-ply lookahead: 16 candidates, best-of-eval
- Front-aware builds: prioritize home SCs near active front, army/fleet preference by front type
- Old medium logic saved as `strategy_medium_old.go.bak`

### Benchmark Infrastructure
- BENCH_GAMES env var for configurable game count
- BENCH_VERBOSE env var (default quiet, opt-in verbose)
- TestBenchmark_RustVsEasyAllPowers test added
