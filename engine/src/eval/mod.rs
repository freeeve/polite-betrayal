//! Position evaluation.
//!
//! Scores a board position from a given power's perspective, considering
//! supply-center counts, unit positioning, and strategic factors.
//!
//! Ported from `api/internal/bot/search_util.go` (EvaluatePosition) and
//! `api/internal/bot/eval.go` (distance matrices, threat/defense helpers).

pub(crate) mod heuristic;

pub use heuristic::{evaluate, evaluate_all};
