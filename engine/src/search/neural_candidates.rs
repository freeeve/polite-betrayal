//! Neural-guided candidate generation.
//!
//! Scores legal orders using policy network logits and blends neural
//! candidates with heuristic candidates for search diversity.

use crate::board::order::{Location, Order, OrderUnit};
use crate::board::province::{Coast, Power, Province, ALL_PROVINCES, PROVINCE_COUNT};
use crate::board::state::BoardState;
use crate::eval::NeuralEvaluator;
use crate::movegen::movement::legal_orders;
use crate::nn::encoding::NUM_AREAS;

/// Order type indices matching Python ORDER_TYPES:
/// ["hold", "move", "support", "convoy", "retreat", "build", "disband"]
const ORDER_TYPE_HOLD: usize = 0;
const ORDER_TYPE_MOVE: usize = 1;
const ORDER_TYPE_SUPPORT: usize = 2;
const ORDER_TYPE_CONVOY: usize = 3;
#[allow(dead_code)]
const ORDER_TYPE_RETREAT: usize = 4;
#[allow(dead_code)]
const ORDER_TYPE_BUILD: usize = 5;
#[allow(dead_code)]
const ORDER_TYPE_DISBAND: usize = 6;

const NUM_ORDER_TYPES: usize = 7;

/// Total feature vector length: 7 + 81 + 81 = 169.
const ORDER_VOCAB_SIZE: usize = NUM_ORDER_TYPES + NUM_AREAS + NUM_AREAS;

/// Offset for source area in the 169-dim vector.
const SRC_OFFSET: usize = NUM_ORDER_TYPES;

/// Offset for destination area in the 169-dim vector.
const DST_OFFSET: usize = NUM_ORDER_TYPES + NUM_AREAS;

/// Maps a province + coast to an area index (0..80) matching the Python AREA_INDEX.
///
/// Base provinces map to their enum ordinal (0..74). Bicoastal variants map to 75..80.
fn province_to_area(province: Province, coast: Coast) -> usize {
    match (province, coast) {
        (Province::Bul, Coast::East) => 75,
        (Province::Bul, Coast::South) => 76,
        (Province::Spa, Coast::North) => 77,
        (Province::Spa, Coast::South) => 78,
        (Province::Stp, Coast::North) => 79,
        (Province::Stp, Coast::South) => 80,
        _ => province as usize,
    }
}

/// Returns the area index for a Location, respecting coast specifiers.
fn location_to_area(loc: Location) -> usize {
    province_to_area(loc.province, loc.coast)
}

/// Returns the area index for a unit's source location.
fn unit_source_area(unit: &OrderUnit) -> usize {
    location_to_area(unit.location)
}

/// Computes the dot product of the policy logits with the order's feature encoding.
///
/// The encoding is multi-hot: order_type at [0:7], source at [7:88], dest at [88:169].
/// The score is the sum of logits at the active positions.
fn score_order_neural(order: &Order, logits: &[f32]) -> f32 {
    if logits.len() < ORDER_VOCAB_SIZE {
        return 0.0;
    }

    match *order {
        Order::Hold { ref unit } => {
            let type_score = logits[ORDER_TYPE_HOLD];
            let src_score = logits[SRC_OFFSET + unit_source_area(unit)];
            type_score + src_score
        }
        Order::Move { ref unit, dest } => {
            let type_score = logits[ORDER_TYPE_MOVE];
            let src_score = logits[SRC_OFFSET + unit_source_area(unit)];
            let dst_score = logits[DST_OFFSET + location_to_area(dest)];
            type_score + src_score + dst_score
        }
        Order::SupportHold { ref unit, .. } | Order::SupportMove { ref unit, .. } => {
            let type_score = logits[ORDER_TYPE_SUPPORT];
            let src_score = logits[SRC_OFFSET + unit_source_area(unit)];
            // For support-move, the destination is the supported move's target.
            let dst_score = match *order {
                Order::SupportMove { dest, .. } => logits[DST_OFFSET + location_to_area(dest)],
                Order::SupportHold { ref supported, .. } => {
                    logits[DST_OFFSET + unit_source_area(supported)]
                }
                _ => 0.0,
            };
            type_score + src_score + dst_score
        }
        Order::Convoy {
            ref unit,
            convoyed_to,
            ..
        } => {
            let type_score = logits[ORDER_TYPE_CONVOY];
            let src_score = logits[SRC_OFFSET + unit_source_area(unit)];
            let dst_score = logits[DST_OFFSET + location_to_area(convoyed_to)];
            type_score + src_score + dst_score
        }
        _ => 0.0,
    }
}

/// A candidate order scored by the neural network.
#[derive(Clone, Copy)]
pub struct NeuralScoredOrder {
    pub order: Order,
    pub neural_score: f32,
}

/// Generates top-K orders per unit using neural policy scores.
///
/// Returns one Vec per unit with candidates sorted descending by neural score.
/// Returns None if the policy network is unavailable or inference fails.
pub fn neural_top_k_per_unit(
    evaluator: &NeuralEvaluator,
    power: Power,
    state: &BoardState,
    k: usize,
) -> Option<Vec<Vec<NeuralScoredOrder>>> {
    if !evaluator.has_policy() {
        return None;
    }

    // Run policy inference: returns [max_units, 169] flattened logits.
    let logits = evaluator.policy(state, power)?;
    let per_unit_logit_size = ORDER_VOCAB_SIZE;

    // Collect units for this power (matching collect_unit_indices ordering).
    let mut unit_indices: Vec<usize> = Vec::new();
    for i in 0..PROVINCE_COUNT {
        if let Some((p, _)) = state.units[i] {
            if p == power {
                unit_indices.push(i);
            }
        }
    }

    if unit_indices.is_empty() {
        return Some(Vec::new());
    }

    let mut per_unit: Vec<Vec<NeuralScoredOrder>> = Vec::with_capacity(unit_indices.len());

    for (ui, &prov_idx) in unit_indices.iter().enumerate() {
        let prov = ALL_PROVINCES[prov_idx];
        let legal = legal_orders(prov, state);
        if legal.is_empty() {
            continue;
        }

        // Extract logits for this unit.
        let logit_start = ui * per_unit_logit_size;
        let logit_end = logit_start + per_unit_logit_size;
        if logit_end > logits.len() {
            // Logits shorter than expected: fall back to equal scores.
            let mut scored: Vec<NeuralScoredOrder> = legal
                .into_iter()
                .map(|o| NeuralScoredOrder {
                    order: o,
                    neural_score: 0.0,
                })
                .collect();
            scored.truncate(k);
            per_unit.push(scored);
            continue;
        }
        let unit_logits = &logits[logit_start..logit_end];

        // Score each legal order against the policy logits.
        let mut scored: Vec<NeuralScoredOrder> = legal
            .into_iter()
            .map(|o| NeuralScoredOrder {
                order: o,
                neural_score: score_order_neural(&o, unit_logits),
            })
            .collect();

        // Sort descending by neural score.
        scored.sort_by(|a, b| {
            b.neural_score
                .partial_cmp(&a.neural_score)
                .unwrap_or(std::cmp::Ordering::Equal)
        });
        scored.truncate(k);
        per_unit.push(scored);
    }

    Some(per_unit)
}

/// Converts neural scores to probability weights via softmax.
pub fn softmax_weights(scores: &[f32]) -> Vec<f64> {
    if scores.is_empty() {
        return Vec::new();
    }
    let max = scores.iter().cloned().fold(f32::NEG_INFINITY, f32::max);
    let exps: Vec<f64> = scores.iter().map(|s| ((*s - max) as f64).exp()).collect();
    let sum: f64 = exps.iter().sum();
    if sum > 0.0 {
        exps.iter().map(|e| e / sum).collect()
    } else {
        vec![1.0 / scores.len() as f64; scores.len()]
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::order::{Location, OrderUnit};
    use crate::board::province::{Coast, Province};
    use crate::board::state::{Phase, Season};
    use crate::board::unit::UnitType;

    #[test]
    fn province_to_area_base_provinces() {
        assert_eq!(province_to_area(Province::Adr, Coast::None), 0);
        assert_eq!(province_to_area(Province::Vie, Coast::None), 70);
        assert_eq!(province_to_area(Province::Yor, Coast::None), 74);
    }

    #[test]
    fn province_to_area_bicoastal() {
        assert_eq!(province_to_area(Province::Bul, Coast::East), 75);
        assert_eq!(province_to_area(Province::Bul, Coast::South), 76);
        assert_eq!(province_to_area(Province::Spa, Coast::North), 77);
        assert_eq!(province_to_area(Province::Spa, Coast::South), 78);
        assert_eq!(province_to_area(Province::Stp, Coast::North), 79);
        assert_eq!(province_to_area(Province::Stp, Coast::South), 80);
    }

    #[test]
    fn score_hold_order() {
        let unit = OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(Province::Vie),
        };
        let order = Order::Hold { unit };

        // Logits with high values for hold type and Vie source.
        let mut logits = vec![0.0f32; ORDER_VOCAB_SIZE];
        logits[ORDER_TYPE_HOLD] = 5.0;
        logits[SRC_OFFSET + Province::Vie as usize] = 3.0;

        let score = score_order_neural(&order, &logits);
        assert!((score - 8.0).abs() < 0.001, "Expected 8.0, got {}", score);
    }

    #[test]
    fn score_move_order() {
        let unit = OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(Province::Bud),
        };
        let order = Order::Move {
            unit,
            dest: Location::new(Province::Ser),
        };

        let mut logits = vec![0.0f32; ORDER_VOCAB_SIZE];
        logits[ORDER_TYPE_MOVE] = 4.0;
        logits[SRC_OFFSET + Province::Bud as usize] = 2.0;
        logits[DST_OFFSET + Province::Ser as usize] = 6.0;

        let score = score_order_neural(&order, &logits);
        assert!((score - 12.0).abs() < 0.001, "Expected 12.0, got {}", score);
    }

    #[test]
    fn score_support_move_order() {
        let unit = OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(Province::Gal),
        };
        let supported = OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(Province::Bud),
        };
        let order = Order::SupportMove {
            unit,
            supported,
            dest: Location::new(Province::Rum),
        };

        let mut logits = vec![0.0f32; ORDER_VOCAB_SIZE];
        logits[ORDER_TYPE_SUPPORT] = 3.0;
        logits[SRC_OFFSET + Province::Gal as usize] = 1.0;
        logits[DST_OFFSET + Province::Rum as usize] = 5.0;

        let score = score_order_neural(&order, &logits);
        assert!((score - 9.0).abs() < 0.001, "Expected 9.0, got {}", score);
    }

    #[test]
    fn softmax_basic() {
        let weights = softmax_weights(&[1.0, 2.0, 3.0]);
        assert_eq!(weights.len(), 3);
        // Should sum to ~1.0.
        let sum: f64 = weights.iter().sum();
        assert!((sum - 1.0).abs() < 0.001);
        // Largest should be last.
        assert!(weights[2] > weights[1]);
        assert!(weights[1] > weights[0]);
    }

    #[test]
    fn softmax_empty() {
        let weights = softmax_weights(&[]);
        assert!(weights.is_empty());
    }

    #[test]
    fn neural_top_k_returns_none_without_model() {
        let evaluator = NeuralEvaluator::new(None, None);
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let result = neural_top_k_per_unit(&evaluator, Power::Austria, &state, 5);
        assert!(result.is_none());
    }
}
