//! Heuristic position evaluation.
//!
//! Placeholder — full implementation is in progress.

use crate::board::{BoardState, Power, ALL_POWERS, PROVINCE_COUNT};

/// Evaluates the board position from the perspective of a single power.
/// Returns a score where higher is better for the given power.
pub fn evaluate(power: Power, state: &BoardState) -> f64 {
    // Simple SC count heuristic as placeholder.
    let sc = state.sc_owner.iter().filter(|o| **o == Some(power)).count();
    sc as f64
}

/// Evaluates the board position for all powers, returning an array of scores
/// indexed by power order in ALL_POWERS.
pub fn evaluate_all(state: &BoardState) -> [f64; 7] {
    let mut scores = [0.0; 7];
    for (i, &power) in ALL_POWERS.iter().enumerate() {
        scores[i] = evaluate(power, state);
    }
    scores
}
