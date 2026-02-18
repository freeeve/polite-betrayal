//! Realpolitik engine library.
//!
//! Exposes the board representation, resolver, move generation, and protocol
//! modules for use by integration tests and the binary entry point.

pub mod board;
pub mod engine;
pub mod eval;
pub mod movegen;
pub mod nn;
pub mod protocol;
pub mod resolve;
pub mod search;
