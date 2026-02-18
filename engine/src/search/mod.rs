//! Search and planning.
//!
//! Explores the space of possible order sets to find strong moves,
//! using evaluation heuristics to guide the search.

pub mod cartesian;
pub mod regret_matching;

pub use cartesian::{
    heuristic_build_orders, heuristic_retreat_orders, search, SearchInfo, SearchResult,
};
pub use regret_matching::regret_matching_search;
