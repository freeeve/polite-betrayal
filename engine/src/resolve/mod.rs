//! Order resolution.
//!
//! Resolves a set of simultaneous orders into outcomes (succeeds, fails,
//! dislodged) using the Kruijswijk algorithm.

pub mod kruijswijk;

pub use kruijswijk::{
    apply_resolution, resolve_orders, DislodgedUnit, OrderResult, ResolvedOrder, Resolver,
};
