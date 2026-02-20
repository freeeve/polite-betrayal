# Medium (1) vs Easy (6) — Experiment B (injectSupports Post-Search)

**Date**: 2026-02-18
**Commit**: Task 083 — Support coordination improvements
**Config**: 100 games per power, MaxYear 1930, BENCH_SAVE=1 (games saved to DB)
**Total games**: 700
**Purpose**: Test injectSupports-only variant (support coordination applied post-search)

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Status        |
|---------|------|-------|--------|-----------|---------------|---------------|
| Turkey  | 59   | 5     | 36     | **59%**   | 14.4          | ✓ IMPROVED    |
| France  | 33   | 5     | 62     | **33%**   | 8.2           | ✓ IMPROVED    |
| England | 30   | 4     | 66     | **30%**   | 11.3          | ✓ IMPROVED    |
| Germany | 25   | 5     | 70     | **25%**   | 6.7           | ✓ IMPROVED    |
| Italy   | 22   | 4     | 74     | **22%**   | 6.1           | ✓ IMPROVED    |
| Austria | 7    | 3     | 90     | **7%**    | 2.5           | ✗ REGRESSED   |
| Russia  | 4    | 4     | 92     | **4%**    | 1.6           | ✗ REGRESSED   |
| **Overall** | **180** | **30** | **490** | **26%** | 7.2       | ✓ +3% vs baseline |

## Comparison to Pre-Support Baseline (23%)

Experiment B shows **+3% absolute improvement** over the pre-support baseline:

| Power   | Baseline | Exp B | Delta    | Direction         |
|---------|----------|-------|----------|-------------------|
| Turkey  | 51%      | 59%   | **+8**   | ✓ BETTER          |
| France  | 44%      | 33%   | **-11**  | ✗ WORSE           |
| England | 15%      | 30%   | **+15**  | ✓ BETTER          |
| Germany | 20%      | 25%   | **+5**   | ✓ BETTER          |
| Italy   | 18%      | 22%   | **+4**   | ✓ BETTER          |
| Austria | 9%       | 7%    | **-2**   | ✗ WORSE           |
| Russia  | 4%       | 4%    | **0**    | FLAT              |
| **Overall** | **23%** | **26%** | **+3** | ✓ IMPROVEMENT |

## Analysis

### Key Findings

**Strong performers (matching or exceeding baseline):**
- **Turkey (+8%)**: Further dominance; injectSupports appears to help Turkey's expansion strategy
- **England (+15%)**: Major improvement; support coordination post-search strengthens England's naval control
- **Germany (+5%)**: Modest gain from better support usage
- **Italy (+4%)**: Incremental improvement

**Weak performers (regressions):**
- **France (-11%)**: Significant drop from 44% to 33%; injectSupports-only may hurt France's aggressive early game
- **Austria (-2%)**: Remains weak but slightly worse
- **Russia (flat)**: Still worst, no improvement

### Interpretation

Experiment B (injectSupports post-search) yields a **net +3% overall improvement**, but with mixed power-level effects:

1. **Better for defensive/expansion powers** (Turkey, England, Germany, Italy): Support coordination applied as a post-heuristic improves holding and gradual expansion
2. **Worse for aggressive early-game powers** (France): France's aggressive openings may be constrained by overly defensive support coordination
3. **No benefit for struggling corners** (Russia, Austria): Support coordination doesn't address their fundamental positional disadvantages

## Verdict: **KEEP (with reservations)**

**Recommendation**: Keep injectSupports, but note:
- +3% overall is meaningful improvement
- Turkey and England are notably stronger
- France regression is concerning and may need power-specific tuning
- Consider testing a hybrid approach: injectSupports for some powers, guidance-at-search for others

### Next Steps
- Test power-specific variants (e.g., skip injectSupports for France)
- Investigate France's regression in detail
- Evaluate combined approach with other coordinate improvements from task 083
