//! Board representation and game-state types.
//!
//! Contains the core data structures for provinces, units, adjacency,
//! orders, and the overall game state.

pub mod adjacency;
pub mod order;
pub mod province;
pub mod state;
pub mod unit;

pub use adjacency::{
    adj_from, fleet_coasts_to, is_adjacent, is_adjacent_fast, provinces_adjacent_to,
    AdjacencyEntry, ADJACENCIES, ADJACENCY_COUNT,
};
pub use order::{Location, Order, OrderUnit};
pub use province::{
    Coast, Power, Province, ProvinceInfo, ProvinceType, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT,
    PROVINCE_INFO, SUPPLY_CENTER_COUNT,
};
pub use state::{BoardState, DislodgedUnit, Phase, Season};
pub use unit::{Unit, UnitPosition, UnitType};
