//! DUI protocol handling.
//!
//! This module implements parsing and serialization for the DUI (Diplomacy
//! Universal Interface) protocol, including DFEN position encoding, DSON
//! structured notation for orders, and the command parser for the main loop.

pub mod dfen;
pub mod dson;
pub mod parser;

pub use dfen::{encode_dfen, parse_dfen, DfenError};
pub use dson::{format_order, format_orders, parse_order, parse_orders, DsonError};
pub use parser::{parse_command, Command, GoParams};
