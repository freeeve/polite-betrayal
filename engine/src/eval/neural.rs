//! Neural network evaluation via ONNX Runtime.
//!
//! Loads policy and value ONNX models and runs inference using the `ort` crate.
//! Falls back to heuristic evaluation when no model is available.

#[cfg(feature = "neural")]
use ort::session::{builder::GraphOptimizationLevel, Session};
#[cfg(feature = "neural")]
use std::sync::Mutex;

use crate::board::province::Power;
use crate::board::state::BoardState;
use crate::nn::encoding::build_adjacency_matrix;
#[cfg(feature = "neural")]
use crate::nn::encoding::{collect_unit_indices, encode_board_state, NUM_AREAS, NUM_FEATURES};

/// Maximum number of units per power (used for policy network input padding).
#[cfg(feature = "neural")]
const MAX_UNITS: usize = 17;

/// Number of value outputs: [sc_share, win, draw, survival].
const VALUE_OUTPUT_SIZE: usize = 4;

/// Neural network evaluator. Holds ONNX sessions for policy and value models.
pub struct NeuralEvaluator {
    #[cfg(feature = "neural")]
    policy_session: Option<Mutex<Session>>,
    #[cfg(feature = "neural")]
    value_session: Option<Mutex<Session>>,
    #[allow(dead_code)]
    adjacency: Vec<f32>,
}

impl NeuralEvaluator {
    /// Creates a new NeuralEvaluator, loading ONNX models from the given paths.
    ///
    /// If a model file does not exist, that session is set to None and
    /// inference calls will fall back to heuristic evaluation.
    pub fn new(policy_path: Option<&str>, value_path: Option<&str>) -> Self {
        let adjacency = build_adjacency_matrix();

        #[cfg(feature = "neural")]
        {
            let policy_session = policy_path.and_then(|p| load_session(p)).map(Mutex::new);
            let value_session = value_path.and_then(|p| load_session(p)).map(Mutex::new);

            if policy_session.is_some() {
                eprintln!("info string Loaded policy ONNX model");
            }
            if value_session.is_some() {
                eprintln!("info string Loaded value ONNX model");
            }

            NeuralEvaluator {
                policy_session,
                value_session,
                adjacency,
            }
        }

        #[cfg(not(feature = "neural"))]
        {
            let _ = (policy_path, value_path);
            eprintln!("info string Neural eval disabled (compiled without 'neural' feature)");
            NeuralEvaluator { adjacency }
        }
    }

    /// Returns true if the policy model is loaded.
    pub fn has_policy(&self) -> bool {
        #[cfg(feature = "neural")]
        {
            self.policy_session.is_some()
        }
        #[cfg(not(feature = "neural"))]
        {
            false
        }
    }

    /// Returns true if the value model is loaded.
    pub fn has_value(&self) -> bool {
        #[cfg(feature = "neural")]
        {
            self.value_session.is_some()
        }
        #[cfg(not(feature = "neural"))]
        {
            false
        }
    }

    /// Runs the policy network on a single position.
    ///
    /// Returns order logits as a flat f32 vector. Returns None if no
    /// policy model is loaded or if inference fails.
    pub fn policy(&self, state: &BoardState, power: Power) -> Option<Vec<f32>> {
        #[cfg(feature = "neural")]
        {
            let mutex = self.policy_session.as_ref()?;
            let mut session = mutex.lock().ok()?;
            run_policy_inference(&mut session, &self.adjacency, state, power)
        }
        #[cfg(not(feature = "neural"))]
        {
            let _ = (state, power);
            None
        }
    }

    /// Runs the value network on a single position.
    ///
    /// Returns [sc_share, win_prob, draw_prob, survival_prob] for the given power.
    /// Returns None if no value model is loaded or if inference fails.
    pub fn value(&self, state: &BoardState, power: Power) -> Option<[f32; VALUE_OUTPUT_SIZE]> {
        #[cfg(feature = "neural")]
        {
            let mutex = self.value_session.as_ref()?;
            let mut session = mutex.lock().ok()?;
            run_value_inference(&mut session, &self.adjacency, state, power)
        }
        #[cfg(not(feature = "neural"))]
        {
            let _ = (state, power);
            None
        }
    }

    /// Runs the value network for all 7 powers.
    ///
    /// Returns per-power evaluation or None if the value model is unavailable.
    pub fn value_all(&self, state: &BoardState) -> Option<[[f32; VALUE_OUTPUT_SIZE]; 7]> {
        use crate::board::province::ALL_POWERS;
        let mut result = [[0.0f32; VALUE_OUTPUT_SIZE]; 7];
        for (i, &p) in ALL_POWERS.iter().enumerate() {
            result[i] = self.value(state, p)?;
        }
        Some(result)
    }

    /// Runs the policy network in batch mode. Returns one logit vector per (state, power) pair.
    pub fn policy_batch(&self, states: &[(&BoardState, Power)]) -> Option<Vec<Vec<f32>>> {
        #[cfg(feature = "neural")]
        {
            let mutex = self.policy_session.as_ref()?;
            let mut session = mutex.lock().ok()?;
            run_policy_batch(&mut session, &self.adjacency, states)
        }
        #[cfg(not(feature = "neural"))]
        {
            let _ = states;
            None
        }
    }

    /// Runs the value network in batch mode. Returns one value vector per (state, power) pair.
    pub fn value_batch(
        &self,
        states: &[(&BoardState, Power)],
    ) -> Option<Vec<[f32; VALUE_OUTPUT_SIZE]>> {
        #[cfg(feature = "neural")]
        {
            let mutex = self.value_session.as_ref()?;
            let mut session = mutex.lock().ok()?;
            run_value_batch(&mut session, &self.adjacency, states)
        }
        #[cfg(not(feature = "neural"))]
        {
            let _ = states;
            None
        }
    }
}

/// Loads an ONNX session from a file path. Returns None on failure.
#[cfg(feature = "neural")]
fn load_session(path: &str) -> Option<Session> {
    match Session::builder()
        .and_then(|b| b.with_optimization_level(GraphOptimizationLevel::Level3))
        .and_then(|b| b.with_intra_threads(4))
        .and_then(|b| b.commit_from_file(path))
    {
        Ok(session) => Some(session),
        Err(e) => {
            eprintln!("info string Failed to load ONNX model {}: {}", path, e);
            None
        }
    }
}

/// Maps a Power to its integer index matching the Python POWER_INDEX.
#[cfg(feature = "neural")]
fn power_to_index(p: Power) -> i64 {
    match p {
        Power::Austria => 0,
        Power::England => 1,
        Power::France => 2,
        Power::Germany => 3,
        Power::Italy => 4,
        Power::Russia => 5,
        Power::Turkey => 6,
    }
}

/// Runs single-position policy inference.
#[cfg(feature = "neural")]
fn run_policy_inference(
    session: &mut Session,
    adjacency: &[f32],
    state: &BoardState,
    power: Power,
) -> Option<Vec<f32>> {
    use ort::value::Value;

    let board_data = encode_board_state(state);
    let unit_indices = collect_unit_indices(state, power, MAX_UNITS);
    let power_idx = power_to_index(power);

    let board_tensor =
        Value::from_array(([1, NUM_AREAS, NUM_FEATURES], board_data.to_vec())).ok()?;
    let adj_tensor = Value::from_array(([NUM_AREAS, NUM_AREAS], adjacency.to_vec())).ok()?;
    let unit_tensor = Value::from_array(([1, MAX_UNITS], unit_indices)).ok()?;
    let power_tensor = Value::from_array(([1_usize], vec![power_idx])).ok()?;

    let outputs = session
        .run(ort::inputs![
            board_tensor,
            adj_tensor,
            unit_tensor,
            power_tensor
        ])
        .ok()?;

    let (_shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
    Some(data.to_vec())
}

/// Runs single-position value inference.
#[cfg(feature = "neural")]
fn run_value_inference(
    session: &mut Session,
    adjacency: &[f32],
    state: &BoardState,
    power: Power,
) -> Option<[f32; VALUE_OUTPUT_SIZE]> {
    use ort::value::Value;

    let board_data = encode_board_state(state);
    let power_idx = power_to_index(power);

    let board_tensor =
        Value::from_array(([1, NUM_AREAS, NUM_FEATURES], board_data.to_vec())).ok()?;
    let adj_tensor = Value::from_array(([NUM_AREAS, NUM_AREAS], adjacency.to_vec())).ok()?;
    let power_tensor = Value::from_array(([1_usize], vec![power_idx])).ok()?;

    let outputs = session
        .run(ort::inputs![board_tensor, adj_tensor, power_tensor])
        .ok()?;

    let (_shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
    if data.len() >= VALUE_OUTPUT_SIZE {
        let mut result = [0.0f32; VALUE_OUTPUT_SIZE];
        result.copy_from_slice(&data[..VALUE_OUTPUT_SIZE]);
        Some(result)
    } else {
        None
    }
}

/// Runs batched policy inference.
#[cfg(feature = "neural")]
fn run_policy_batch(
    session: &mut Session,
    adjacency: &[f32],
    states: &[(&BoardState, Power)],
) -> Option<Vec<Vec<f32>>> {
    use ort::value::Value;

    let batch_size = states.len();
    if batch_size == 0 {
        return Some(Vec::new());
    }

    let mut board_data = Vec::with_capacity(batch_size * NUM_AREAS * NUM_FEATURES);
    let mut unit_data = Vec::with_capacity(batch_size * MAX_UNITS);
    let mut power_data = Vec::with_capacity(batch_size);

    for &(state, power) in states {
        board_data.extend_from_slice(&encode_board_state(state));
        unit_data.extend_from_slice(&collect_unit_indices(state, power, MAX_UNITS));
        power_data.push(power_to_index(power));
    }

    let board_tensor =
        Value::from_array(([batch_size, NUM_AREAS, NUM_FEATURES], board_data)).ok()?;
    let adj_tensor = Value::from_array(([NUM_AREAS, NUM_AREAS], adjacency.to_vec())).ok()?;
    let unit_tensor = Value::from_array(([batch_size, MAX_UNITS], unit_data)).ok()?;
    let power_tensor = Value::from_array(([batch_size], power_data)).ok()?;

    let outputs = session
        .run(ort::inputs![
            board_tensor,
            adj_tensor,
            unit_tensor,
            power_tensor
        ])
        .ok()?;

    let (shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;
    let cols = if shape.len() >= 2 {
        shape[1] as usize
    } else {
        data.len()
    };

    let mut results = Vec::with_capacity(batch_size);
    for i in 0..batch_size {
        let start = i * cols;
        let end = start + cols;
        if end <= data.len() {
            results.push(data[start..end].to_vec());
        }
    }
    Some(results)
}

/// Runs batched value inference.
#[cfg(feature = "neural")]
fn run_value_batch(
    session: &mut Session,
    adjacency: &[f32],
    states: &[(&BoardState, Power)],
) -> Option<Vec<[f32; VALUE_OUTPUT_SIZE]>> {
    use ort::value::Value;

    let batch_size = states.len();
    if batch_size == 0 {
        return Some(Vec::new());
    }

    let mut board_data = Vec::with_capacity(batch_size * NUM_AREAS * NUM_FEATURES);
    let mut power_data = Vec::with_capacity(batch_size);

    for &(state, power) in states {
        board_data.extend_from_slice(&encode_board_state(state));
        power_data.push(power_to_index(power));
    }

    let board_tensor =
        Value::from_array(([batch_size, NUM_AREAS, NUM_FEATURES], board_data)).ok()?;
    let adj_tensor = Value::from_array(([NUM_AREAS, NUM_AREAS], adjacency.to_vec())).ok()?;
    let power_tensor = Value::from_array(([batch_size], power_data)).ok()?;

    let outputs = session
        .run(ort::inputs![board_tensor, adj_tensor, power_tensor])
        .ok()?;

    let (_shape, data) = outputs[0].try_extract_tensor::<f32>().ok()?;

    let mut results = Vec::with_capacity(batch_size);
    for i in 0..batch_size {
        let start = i * VALUE_OUTPUT_SIZE;
        let end = start + VALUE_OUTPUT_SIZE;
        if end <= data.len() {
            let mut arr = [0.0f32; VALUE_OUTPUT_SIZE];
            arr.copy_from_slice(&data[start..end]);
            results.push(arr);
        }
    }
    Some(results)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn neural_evaluator_no_models() {
        let eval = NeuralEvaluator::new(None, None);
        assert!(!eval.has_policy());
        assert!(!eval.has_value());
    }

    #[test]
    fn neural_evaluator_missing_path() {
        let eval = NeuralEvaluator::new(
            Some("/nonexistent/policy.onnx"),
            Some("/nonexistent/value.onnx"),
        );
        assert!(!eval.has_policy());
        assert!(!eval.has_value());
    }

    #[test]
    fn fallback_returns_none() {
        use crate::board::state::{Phase, Season};
        let eval = NeuralEvaluator::new(None, None);
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(eval.policy(&state, Power::Austria).is_none());
        assert!(eval.value(&state, Power::Austria).is_none());
        assert!(eval.value_all(&state).is_none());
    }
}
