//! Neural network feature encoding for ONNX model inference.
//!
//! Converts `BoardState` into the [81, 36] tensor format expected by the
//! trained policy and value networks. The 81 areas are 75 provinces plus
//! 6 bicoastal variants (bul/ec, bul/sc, spa/nc, spa/sc, stp/nc, stp/sc).

pub mod encoding;
