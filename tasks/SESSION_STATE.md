# Session State — 2026-02-17 (Sessions 2-3)

## Session 3 Commits (continuation)

| # | Hash | Description |
|---|------|-------------|
| 23 | `94ffc35` | chore(engine): .gitignore for ONNX model binaries |
| 24 | `5709cad` | chore: gitignore data/processed and data/checkpoints |
| 25 | `8087a1c` | feat(data): add neighbor behavior features to opening book |

## Session 2 Commits (22 commits)

| # | Hash | Description |
|---|------|-------------|
| 1 | `b23db41` | perf: cache adjacency lookups and singleton StandardMap |
| 2 | `ee3f470` | perf: reduce allocations in hard bot regret matching |
| 3 | `2e90f21` | chore: mark polygon fix and opening book tasks as done |
| 4 | `bf7c801` | feat: integrate ONNX neural network evaluation (Rust) |
| 5 | `ce13945` | chore: mark resolver and ONNX integration as done |
| 6 | `f9359a1` | fix: revert SC defense penalty scaling |
| 7 | `9f86f0c` | feat: wire up ArenaConfig.Seed for reproducibility |
| 8 | `21b8915` | feat: medium bot mid-game improvements (16%→47%) |
| 9 | `7146a7c` | feat: SC timeline stats + easy vs random benchmark |
| 10 | `4f9fff7` | docs: medium vs easy benchmark with timeline |
| 11 | `b6a98d2` | feat: opening book extraction script |
| 12 | `8f1beef` | feat: theater classification for opening book |
| 13 | `be3bdb7` | feat: neighbor stance classifier |
| 14 | `d351aa3` | feat: convert 1901 opening book to JSON |
| 15 | `c889e5c` | chore: update task file statuses |
| 16 | `56a6180` | feat: refactor opening book to embedded JSON with scoring engine |
| 17 | `c5cdf1d` | feat: add new unit and destroyed unit symbols to map legend |
| 18 | `7aee020` | feat: extend opening book extraction through F1904 |
| 19 | `1ab2f60` | feat: feature-based opening book extraction through 1907 |
| 20 | `26f21ee` | docs: medium vs easy with book benchmark |
| 21 | `a18bbcc` | refactor: remove unused opening book functions |
| 22 | `fb083bd` | docs: full 7-power medium vs easy benchmark |

## Key Benchmark Results

### Medium vs 6 Easy — All Powers (100 games each, MaxYear 1930)
| Power | Win Rate | Avg SCs |
|-------|----------|---------|
| France | 33% | 9.0 |
| Turkey | 31% | 9.9 |
| Germany | 9% | 4.4 |
| England | 8% | 7.3 |
| Austria | 3% | 1.4 |
| Italy | 3% | 2.9 |
| Russia | 3% | 1.6 |

### Easy vs 6 Random — All Powers (20 games each)
- All 7 powers: 100% win rate
- Fastest: Austria (1905.7), Slowest: England (1913.2)

## Artifacts Produced

### Neural Network
- `data/checkpoints/best_policy.pt` (15MB) — trained PyTorch checkpoint
- `engine/models/policy_v1.onnx` (5.3MB) — exported ONNX model, validated
- No value network checkpoint yet (only policy head trained)

### Opening Book
- `data/processed/opening_book.json` (1.4MB, gitignored) — 1,222 variants / 615 clusters
- Covers S1901 through F1906 with feature-based matching
- Condition features: positions, owned_scs, neighbor_stance, border_pressure, border_bounces, neighbor_sc_counts, theaters, fleet/army counts
- Go scoring engine with 4 match modes committed in `api/internal/bot/opening_book.go`
- Extraction script: `python3 data/scripts/extract_openings.py`

## Known Bugs

### eval.go:366 — Nil Pointer Dereference (task 077)
- `NearestUnownedSCByUnit` panics when `gs.SupplyCenters` is nil
- Called from `predictEnemyTargets` in strategy_medium.go:1002
- Occurs during hard bot multi-phase simulation
- Failing tests: TestHardVsMedium, TestTacticalStrategy_BetterThanHeuristic, TestTacticalStrategy_DefendsOwnSCWhenEnemyAdjacent

## Pending Tasks for Next Session

| Task | File | Description |
|------|------|-------------|
| 076 | `076_rust-opening-book.md` | Port opening book to Rust engine |
| 077 | `077_fix-eval-nil-panic.md` | Fix nil pointer dereference in eval.go |
| 052 | `052_neural-guided-search.md` | Wire policy_v1.onnx into Rust engine search |
| — | — | Integrate 1,222-variant opening book and benchmark all match modes |
| — | — | Benchmark extended book vs 1901-only book across all powers |
| — | — | Medium bot: multi-front defense for central powers (Austria/Russia) |
| — | — | Train value network (only policy head exists) |

## Blocked Tasks

| Task | Blocked By | Notes |
|------|-----------|-------|
| 053 (phase3 arena eval) | 044 + 052 | Needs phase2 bench + neural search |
| 076 (Rust opening book) | — | Ready to start |
| 052 (neural-guided search) | — | policy_v1.onnx ready, can start |
