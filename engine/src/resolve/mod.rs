//! Order resolution.
//!
//! Resolves a set of simultaneous orders into outcomes (succeeds, fails,
//! dislodged) using the Kruijswijk algorithm. Also handles retreat-phase
//! and build-phase resolution, plus phase sequencing.

pub mod build;
pub mod kruijswijk;
pub mod phase;
pub mod retreat;

pub use kruijswijk::{
    apply_resolution, resolve_orders, DislodgedUnit, OrderResult, ResolvedOrder, Resolver,
};

pub use retreat::{apply_retreats, resolve_retreats, RetreatResult};

pub use build::{apply_builds, resolve_builds, BuildResult};

pub use phase::{advance_state, is_game_over, needs_build_phase, next_phase, update_sc_ownership};
