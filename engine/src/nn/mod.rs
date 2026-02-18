//! Neural network feature encoding for ONNX model inference.
//!
//! Converts `BoardState` into the [81, 47] tensor format expected by the
//! trained policy and value networks. The 81 areas are 75 provinces plus
//! 6 bicoastal variants (bul/ec, bul/sc, spa/nc, spa/sc, stp/nc, stp/sc).
//! Features 0..36 encode the current state; features 36..47 encode the
//! previous turn's unit positions (type + owner) for temporal context.

pub mod encoding;
