# Experiment D — Pure 3-ply Evaluation

## Date: 2026-02-18
## Config: BENCH_GAMES=100 (700 total), Medium vs 6 Easy, max year 1930

## Description
Evaluate candidates at ply 3 instead of ply 1:
- Ply 1: resolve our candidate + opponent orders
- Ply 2: opponents respond to ply-1 (easy bot heuristic), we hold
- Ply 3: we respond to ply-2 (easy bot heuristic), opponents respond
- Evaluate position after ply 3

## Results

| Power   | Exp D (3-ply) | Baseline (1-ply) | Delta |
|---------|---------------|------------------|-------|
| Turkey  | 67%           | 51%              | +16pp |
| Germany | 32%           | 20%              | +12pp |
| England | 31%           | 15%              | +16pp |
| France  | 31%           | 44%              | -13pp |
| Italy   | 18%           | 18%              |  0pp  |
| Austria | 7%            | 9%               | -2pp  |
| Russia  | 3%            | 4%               | -1pp  |
| **Overall** | **27.0%** | **28.0%**        | **-1pp** |

## Verdict
Overall -1pp vs baseline. Big gains for England/Turkey/Germany, big loss for France.
3-ply alone is not better than 1-ply overall — the France regression is concerning.

## Runtime
3743s (~62 min), about 10x slower than 1-ply benchmarks.
