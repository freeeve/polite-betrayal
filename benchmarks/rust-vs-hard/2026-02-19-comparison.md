# Rust Engine vs Hard Bot — Comparison Against Medium Opponents

**Date:** 2026-02-19
**Benchmark:** Both engines play each power (10 games each) vs 6 medium opponents
**Rust engine:** Old smaller neural model (before 15.4M retrained model)
**Hard bot:** 5s time budget per move

## Win Rate Comparison

| Power | Rust vs Medium | Hard vs Medium |
|-------|---------------|----------------|
| France | 60% | 10% |
| Italy | 30% | 20% |
| England | 20% | 10% |
| Turkey | 20% | 40% |
| Austria | 10% | 20% |
| Germany | 0% | 0% |
| Russia | 0% | 0% |
| **Overall** | **20%** | **14.3%** |

## Analysis

**Rust outperforms Hard overall (20% vs 14.3%) with approximately 25,000x less compute.**
The Rust engine runs a single forward pass through a small neural network (~0.2ms per move),
while the Hard bot performs tree search with a 5s budget per move.

**France at 60% is the standout for Rust.** The neural network has clearly learned strong
French opening and midgame patterns. France's concentrated home centers and strong
defensive geography likely make it the easiest power for a value-net-guided policy to exploit.
Hard bot only manages 10% as France, suggesting tree search with a 5s budget struggles to
find the right long-term plans for France.

**Hard only beats Rust on Turkey (40% vs 20%) and Austria (20% vs 10%).** Turkey benefits from
deep lookahead in the late game — it has a slow start but can snowball once it breaks through
the Balkans, which tree search handles better. Austria's surrounded position similarly rewards
the ability to search multiple defensive lines simultaneously.

**Germany and Russia are 0% for both — structural weakness.** Germany is attacked from all
sides early and collapses under medium-level pressure from France, England, and Russia.
Russia starts with the most units but the most borders to defend, and gets carved up by
coordinated neighbors. Neither engine can overcome these positional disadvantages.

**England has endgame stall in both.** Both engines get England to high SC counts (avg ~10 SCs)
but struggle to close out victories. England's island position makes it easy to expand to
Scandinavia and Iberia but hard to push into the continent's core for the final 18 SCs.
Rust: 20% win but avg 10.2 final SCs. Hard: 10% win but avg 9.7 final SCs.

**Hard bot's 5s time budget limits its effectiveness.** With more compute budget (e.g., 30s or 60s),
the hard bot would likely improve, particularly on Turkey where its tree search advantage
is already evident. However, even with generous time, the diminishing returns of minimax in
a 7-player game with imperfect information suggest the gap would not close dramatically.

**These Rust results are with the OLD smaller neural model, before the 15.4M retrained model.**
The new model with 4x the parameters and autoregressive decoding is expected to improve
across the board, particularly on weaker powers where the old model's policy was too noisy.
