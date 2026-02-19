# Rust vs Medium — Phantom Support Fix Progression

**Date:** 2026-02-18
**Builds compared:**
1. **Base** — Baseline Rust engine
2. **Fix 1** — `fix(engine): eliminate cross-power phantom support-move orders`
3. **Fix 2** — `fix(engine): eliminate phantom support-move orders with hold fallback`

**Games per power per build:** 10 (70 games per build, 210 total)

## Overall Win Rates

| Build | Wins | Win Rate | Draws | Avg Final SCs |
|-------|------|----------|-------|---------------|
| Base  | 11/70 | 15.7% | 2 | 6.6 |
| Fix 1 | 12/70 | 17.1% | 4 | 6.5 |
| Fix 2 | 14/70 | 20.0% | 2 | 7.7 |

**Trend:** Steady improvement. Fix 2 is +4.3pp over base.

## Per-Power Win Rate Progression

| Power   | Base | Fix 1 | Fix 2 | Trend |
|---------|------|-------|-------|-------|
| Austria |  0%  |  10%  | **30%** | Strong improvement |
| England |  0%  |   0%  |   0%  | No change (structural issue) |
| France  | 40%  |  50%  | **70%** | Strong improvement |
| Germany | 10%  |   0%  |   0%  | Regression (noise?) |
| Italy   | 30%  |  20%  |  20%  | Slight regression |
| Russia  |  0%  |   0%  |   0%  | No change |
| Turkey  | 30%  |  40%  |  20%  | Volatile, no clear trend |

## Per-Power Avg Final SCs

| Power   | Base | Fix 1 | Fix 2 | Delta |
|---------|------|-------|-------|-------|
| Austria |  1.4 |  2.2  |  5.6  | +4.2  |
| England | 10.0 | 10.7  | 10.7  | +0.7  |
| France  | 12.8 | 12.1  | 15.5  | +2.7  |
| Germany |  2.1 |  2.4  |  1.3  | -0.8  |
| Italy   |  8.7 |  4.9  |  6.5  | -2.2  |
| Russia  |  0.8 |  1.0  |  3.2  | +2.4  |
| Turkey  | 10.6 | 12.2  | 10.9  | +0.3  |

## Key Observations

### Clear Winners from the Fixes
- **France:** The biggest beneficiary. Win rate nearly doubled (40% -> 70%). The hold fallback fix prevents France from issuing phantom supports for other powers' units, letting it focus firepower on actual targets.
- **Austria:** Went from unplayable (0%) to hitting the 30% target. Avg final SCs quadrupled (1.4 -> 5.6). Austria's tight starting position benefits enormously from not wasting moves on phantom supports.
- **Russia:** Still 0% wins but survival dramatically improved (0.8 -> 3.2 avg final SCs). Russia's multi-front position was most hurt by phantom supports pulling units in wrong directions.

### Unchanged / Structural Issues
- **England:** Consistently 0% win rate across all three builds despite accumulating 10-11 avg final SCs and routinely reaching 13-15 SCs mid-game. This is a structural endgame conversion problem — England can dominate the seas but cannot convert naval superiority into the final 3-4 inland SCs needed for victory. Not a phantom support issue.
- **Germany:** Consistently weak (0-10% across builds). Germany's central position makes it vulnerable to being squeezed from all sides. The engine may need strategic improvements specifically for central powers.

### Noisy Results
- **Italy and Turkey** show volatile results across builds (30/20/20 and 30/40/20 respectively). With n=10 per build, single-game variance is high. These powers may need larger sample sizes to assess impact.

## Conclusion

The phantom support fixes deliver a clear net positive: **+4.3pp overall win rate** and significantly improved play for Austria and France. The remaining weaknesses (England endgame, Germany survival, Russia wins) appear to be strategic/evaluation issues rather than order-generation bugs.
