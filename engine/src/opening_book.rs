//! Opening book for the Diplomacy engine.
//!
//! Loads pre-computed opening moves from a JSON file and selects the best
//! matching entry for a given board state using a configurable scoring system.
//! Ported from the Go implementation in api/internal/bot/opening_book.go.

use std::collections::HashMap;
use std::fs;
use std::path::Path;

use rand::Rng;
use serde::Deserialize;

use crate::board::order::{Location, Order, OrderUnit};
use crate::board::province::{Coast, Power, Province, ALL_PROVINCES, PROVINCE_COUNT};
use crate::board::state::{BoardState, Phase, Season};
use crate::board::unit::UnitType;

/// The full opening book parsed from JSON.
#[derive(Debug, Clone, Deserialize)]
pub struct OpeningBook {
    pub entries: Vec<BookEntry>,
}

/// A single conditional entry in the opening book.
#[derive(Debug, Clone, Deserialize)]
pub struct BookEntry {
    pub power: String,
    pub year: u16,
    pub season: String,
    pub phase: String,
    pub condition: BookCondition,
    pub options: Vec<BookOption>,
}

/// Matching criteria for an entry. Fields are AND-ed in exact mode,
/// or contribute to a weighted score in hybrid mode.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct BookCondition {
    #[serde(default)]
    pub positions: HashMap<String, String>,
    #[serde(default)]
    pub owned_scs: Vec<String>,
    #[serde(default)]
    pub sc_count_min: u32,
    #[serde(default)]
    pub sc_count_max: u32,
    #[serde(default)]
    pub neighbor_stance: HashMap<String, String>,
    #[serde(default)]
    pub border_pressure: i32,
    #[serde(default)]
    pub theaters: HashMap<String, u32>,
    #[serde(default)]
    pub fleet_count: u32,
    #[serde(default)]
    pub army_count: u32,
}

/// A named, weighted set of orders to choose from.
#[derive(Debug, Clone, Deserialize)]
pub struct BookOption {
    pub name: String,
    pub weight: f64,
    pub orders: Vec<OrderInput>,
}

/// A single order as represented in the JSON opening book.
#[derive(Debug, Clone, Deserialize)]
pub struct OrderInput {
    pub unit_type: String,
    pub location: String,
    #[serde(default)]
    pub coast: String,
    pub order_type: String,
    #[serde(default)]
    pub target: String,
    #[serde(default)]
    pub target_coast: String,
    #[serde(default)]
    pub aux_loc: String,
    #[serde(default)]
    pub aux_target: String,
    #[serde(default)]
    pub aux_unit_type: String,
}

/// Match mode for book lookup.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MatchMode {
    Exact,
    Hybrid,
}

/// Configurable weights for the scoring system.
#[derive(Debug, Clone)]
pub struct BookMatchConfig {
    pub mode: MatchMode,
    pub min_score: f64,
    pub position_weight: f64,
    pub owned_sc_weight: f64,
    pub sc_count_weight: f64,
    pub neighbor_weight: f64,
    pub border_press_weight: f64,
    pub theater_weight: f64,
    pub fleet_army_weight: f64,
}

impl Default for BookMatchConfig {
    fn default() -> Self {
        BookMatchConfig {
            mode: MatchMode::Hybrid,
            min_score: 1.0,
            position_weight: 10.0,
            owned_sc_weight: 3.0,
            sc_count_weight: 1.0,
            neighbor_weight: 5.0,
            border_press_weight: 2.0,
            theater_weight: 2.0,
            fleet_army_weight: 1.5,
        }
    }
}

/// Loads an opening book from a JSON file at the given path.
pub fn load_book(path: &Path) -> Result<OpeningBook, String> {
    let data = fs::read_to_string(path)
        .map_err(|e| format!("failed to read {}: {}", path.display(), e))?;
    serde_json::from_str(&data).map_err(|e| format!("failed to parse opening book JSON: {}", e))
}

/// Loads an opening book from a JSON string.
pub fn load_book_from_str(json: &str) -> Result<OpeningBook, String> {
    serde_json::from_str(json).map_err(|e| format!("failed to parse opening book JSON: {}", e))
}

/// Looks up opening book orders for the given power and board state.
/// Returns None if no matching entry is found.
pub fn lookup_opening(
    book: &OpeningBook,
    state: &BoardState,
    power: Power,
    cfg: &BookMatchConfig,
) -> Option<Vec<Order>> {
    let target_season = parse_season_str_to_enum(state.season);
    let target_phase = parse_phase_str_to_enum(state.phase);

    // Filter entries matching (year, season, phase, power).
    let candidates: Vec<&BookEntry> = book
        .entries
        .iter()
        .filter(|e| {
            e.year == state.year
                && e.season == target_season
                && e.phase == target_phase
                && parse_power_str(&e.power) == Some(power)
        })
        .collect();

    if candidates.is_empty() {
        return None;
    }

    // Score each candidate.
    let mut matches: Vec<(&BookEntry, f64)> = Vec::new();
    for entry in &candidates {
        let score = score_condition(&entry.condition, state, power, cfg);
        if score < 0.0 {
            continue;
        }
        if score < cfg.min_score {
            continue;
        }
        matches.push((entry, score));
    }

    if matches.is_empty() {
        return None;
    }

    // Sort by score descending.
    matches.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));

    // Collect options from all entries at the top score (within epsilon).
    let top_score = matches[0].1;
    let mut top_options: Vec<&BookOption> = Vec::new();
    for (entry, score) in &matches {
        if top_score - score > 0.01 {
            break;
        }
        for opt in &entry.options {
            top_options.push(opt);
        }
    }

    // Weighted random selection.
    let selected = weighted_select(&top_options)?;

    // Convert OrderInput to engine Order.
    convert_orders(&selected.orders, power)
}

/// Picks an option from a weighted list using random selection.
fn weighted_select<'a>(options: &[&'a BookOption]) -> Option<&'a BookOption> {
    if options.is_empty() {
        return None;
    }
    let total: f64 = options.iter().map(|o| o.weight).sum();
    if total <= 0.0 {
        return Some(options[0]);
    }
    let mut rng = rand::thread_rng();
    let r = rng.gen::<f64>() * total;
    let mut cum = 0.0;
    for opt in options {
        cum += opt.weight;
        if r < cum {
            return Some(opt);
        }
    }
    Some(options[options.len() - 1])
}

/// Computes a match score for a condition against the board state.
/// Returns negative if there is a hard mismatch in exact mode.
fn score_condition(
    cond: &BookCondition,
    state: &BoardState,
    power: Power,
    cfg: &BookMatchConfig,
) -> f64 {
    let mut score = 0.0;

    // Tier 1: exact positions
    if !cond.positions.is_empty() {
        let actual = unit_key(state, power);
        let mut matched = 0;
        for (prov, utype) in &cond.positions {
            if actual.get(prov.as_str()) == Some(&utype.as_str()) {
                matched += 1;
            }
        }
        let pos_max = cond.positions.len() as f64 * cfg.position_weight;

        if cfg.mode == MatchMode::Exact {
            if matched != cond.positions.len() {
                return -1.0;
            }
            score += pos_max;
        } else {
            score += matched as f64 * cfg.position_weight;
        }
    }

    // Tier 2: SC ownership
    if !cond.owned_scs.is_empty() {
        let mut matched = 0;
        for sc_name in &cond.owned_scs {
            if let Some(prov) = Province::from_abbr(sc_name) {
                if state.sc_owner[prov as usize] == Some(power) {
                    matched += 1;
                }
            }
        }
        let sc_max = cond.owned_scs.len() as f64 * cfg.owned_sc_weight;

        if cfg.mode == MatchMode::Exact {
            if matched != cond.owned_scs.len() {
                return -1.0;
            }
            score += sc_max;
        } else {
            score += matched as f64 * cfg.owned_sc_weight;
        }
    }

    // Tier 2: SC count range
    if cond.sc_count_min > 0 || cond.sc_count_max > 0 {
        let count = sc_count(state, power);
        let in_range = (cond.sc_count_min == 0 || count >= cond.sc_count_min)
            && (cond.sc_count_max == 0 || count <= cond.sc_count_max);

        if cfg.mode == MatchMode::Exact {
            if !in_range {
                return -1.0;
            }
            score += cfg.sc_count_weight;
        } else if in_range {
            score += cfg.sc_count_weight;
        }
    }

    // Tier 3: neighbor stances (simplified -- match on border pressure only in Rust port)
    // Note: full neighbor stance classification requires adjacency BFS which is
    // available in the heuristic evaluator. For the opening book, we skip stance
    // matching and rely on position/SC matching which is the primary discriminator.

    // Tier 3: border pressure
    if cond.border_pressure > 0 {
        let actual = border_pressure(state, power);
        let diff = (actual - cond.border_pressure).abs();
        if diff <= 1 {
            score += cfg.border_press_weight;
        } else if cfg.mode == MatchMode::Exact {
            return -1.0;
        }
    }

    // Tier 4: fleet/army counts
    let mut fa_fields = 0u32;
    if cond.fleet_count > 0 {
        fa_fields += 1;
    }
    if cond.army_count > 0 {
        fa_fields += 1;
    }
    if fa_fields > 0 {
        let (fleets, armies) = fleet_army_count(state, power);
        let mut matched = 0u32;
        if cond.fleet_count > 0 && fleets == cond.fleet_count {
            matched += 1;
        }
        if cond.army_count > 0 && armies == cond.army_count {
            matched += 1;
        }

        if cfg.mode == MatchMode::Exact {
            if matched != fa_fields {
                return -1.0;
            }
            score += fa_fields as f64 * cfg.fleet_army_weight;
        } else {
            score += matched as f64 * cfg.fleet_army_weight;
        }
    }

    score
}

/// Builds a position fingerprint: province abbreviation -> unit type string.
fn unit_key<'a>(state: &'a BoardState, power: Power) -> HashMap<&'a str, &'a str> {
    let mut map = HashMap::new();
    for prov in ALL_PROVINCES {
        if let Some((p, ut)) = state.units[prov as usize] {
            if p == power {
                let ut_str = match ut {
                    UnitType::Army => "army",
                    UnitType::Fleet => "fleet",
                };
                map.insert(prov.abbr(), ut_str);
            }
        }
    }
    map
}

/// Counts supply centers owned by the given power.
fn sc_count(state: &BoardState, power: Power) -> u32 {
    state.sc_owner.iter().filter(|o| **o == Some(power)).count() as u32
}

/// Counts enemy units adjacent to the given power's supply centers.
fn border_pressure(state: &BoardState, power: Power) -> i32 {
    use crate::board::adjacency::adj_from;

    // Collect our SCs.
    let mut our_scs = [false; PROVINCE_COUNT];
    for prov in ALL_PROVINCES {
        if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
            our_scs[prov as usize] = true;
        }
    }

    // Build border zone: provinces adjacent to our SCs that are not our SCs.
    let mut border_zone = [false; PROVINCE_COUNT];
    for prov in ALL_PROVINCES {
        if our_scs[prov as usize] {
            for adj in adj_from(prov) {
                if !our_scs[adj.to as usize] {
                    border_zone[adj.to as usize] = true;
                }
            }
        }
    }

    // Count enemy units in the border zone.
    let mut count = 0;
    for prov in ALL_PROVINCES {
        if border_zone[prov as usize] {
            if let Some((p, _)) = state.units[prov as usize] {
                if p != power {
                    count += 1;
                }
            }
        }
    }
    count
}

/// Counts fleet and army units for a power.
fn fleet_army_count(state: &BoardState, power: Power) -> (u32, u32) {
    let mut fleets = 0u32;
    let mut armies = 0u32;
    for prov in ALL_PROVINCES {
        if let Some((p, ut)) = state.units[prov as usize] {
            if p == power {
                match ut {
                    UnitType::Fleet => fleets += 1,
                    UnitType::Army => armies += 1,
                }
            }
        }
    }
    (fleets, armies)
}

/// Converts season enum to the JSON string representation.
fn parse_season_str_to_enum(season: Season) -> &'static str {
    match season {
        Season::Spring => "spring",
        Season::Fall => "fall",
    }
}

/// Converts phase enum to the JSON string representation.
fn parse_phase_str_to_enum(phase: Phase) -> &'static str {
    match phase {
        Phase::Movement => "movement",
        Phase::Retreat => "retreat",
        Phase::Build => "build",
    }
}

/// Parses a power name string to the Power enum.
fn parse_power_str(s: &str) -> Option<Power> {
    Power::from_name(s)
}

/// Parses a unit type string from JSON to the UnitType enum.
fn parse_unit_type_str(s: &str) -> Option<UnitType> {
    match s {
        "army" => Some(UnitType::Army),
        "fleet" => Some(UnitType::Fleet),
        _ => None,
    }
}

/// Parses a coast string from JSON.
fn parse_coast_str(s: &str) -> Coast {
    match s {
        "nc" => Coast::North,
        "sc" => Coast::South,
        "ec" => Coast::East,
        _ => Coast::None,
    }
}

/// Converts a list of OrderInputs to engine Orders.
/// Returns None if any order cannot be converted.
fn convert_orders(inputs: &[OrderInput], power: Power) -> Option<Vec<Order>> {
    let mut orders = Vec::with_capacity(inputs.len());
    for input in inputs {
        let order = convert_single_order(input, power)?;
        orders.push(order);
    }
    Some(orders)
}

/// Converts a single OrderInput to an engine Order.
fn convert_single_order(input: &OrderInput, _power: Power) -> Option<Order> {
    let unit_type = parse_unit_type_str(&input.unit_type)?;
    let province = Province::from_abbr(&input.location)?;
    let coast = parse_coast_str(&input.coast);

    let unit = OrderUnit {
        unit_type,
        location: Location { province, coast },
    };

    match input.order_type.as_str() {
        "hold" => Some(Order::Hold { unit }),
        "move" => {
            let target_prov = Province::from_abbr(&input.target)?;
            let target_coast = parse_coast_str(&input.target_coast);
            Some(Order::Move {
                unit,
                dest: Location {
                    province: target_prov,
                    coast: target_coast,
                },
            })
        }
        "support" => {
            let aux_unit_type = parse_unit_type_str(&input.aux_unit_type)?;
            let aux_prov = Province::from_abbr(&input.aux_loc)?;
            let supported = OrderUnit {
                unit_type: aux_unit_type,
                location: Location::new(aux_prov),
            };
            if input.aux_target.is_empty() {
                Some(Order::SupportHold { unit, supported })
            } else {
                let dest_prov = Province::from_abbr(&input.aux_target)?;
                Some(Order::SupportMove {
                    unit,
                    supported,
                    dest: Location::new(dest_prov),
                })
            }
        }
        "convoy" => {
            let from_prov = Province::from_abbr(&input.aux_loc)?;
            let to_prov = Province::from_abbr(&input.aux_target)?;
            Some(Order::Convoy {
                unit,
                convoyed_from: Location::new(from_prov),
                convoyed_to: Location::new(to_prov),
            })
        }
        "build" => Some(Order::Build { unit }),
        "disband" => Some(Order::Disband { unit }),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::province::ALL_POWERS;

    /// Minimal JSON for a Spring 1901 Austria entry.
    fn test_json() -> &'static str {
        r#"{
  "entries": [
    {
      "power": "austria",
      "year": 1901,
      "season": "spring",
      "phase": "movement",
      "condition": {
        "positions": {
          "bud": "army",
          "vie": "army",
          "tri": "fleet"
        },
        "owned_scs": ["bud", "tri", "vie"],
        "sc_count_min": 3,
        "sc_count_max": 3,
        "fleet_count": 1,
        "army_count": 2
      },
      "options": [
        {
          "name": "southern_opening",
          "weight": 0.5,
          "orders": [
            { "unit_type": "army", "location": "vie", "order_type": "move", "target": "gal" },
            { "unit_type": "fleet", "location": "tri", "order_type": "move", "target": "alb" },
            { "unit_type": "army", "location": "bud", "order_type": "move", "target": "ser" }
          ]
        },
        {
          "name": "balkan_opening",
          "weight": 0.5,
          "orders": [
            { "unit_type": "army", "location": "bud", "order_type": "move", "target": "ser" },
            { "unit_type": "fleet", "location": "tri", "order_type": "move", "target": "alb" },
            { "unit_type": "army", "location": "vie", "order_type": "move", "target": "tri" }
          ]
        }
      ]
    },
    {
      "power": "england",
      "year": 1901,
      "season": "spring",
      "phase": "movement",
      "condition": {
        "positions": {
          "lon": "fleet",
          "edi": "fleet",
          "lvp": "army"
        },
        "owned_scs": ["lon", "edi", "lvp"],
        "sc_count_min": 3,
        "sc_count_max": 3,
        "fleet_count": 2,
        "army_count": 1
      },
      "options": [
        {
          "name": "north_sea_opening",
          "weight": 1.0,
          "orders": [
            { "unit_type": "fleet", "location": "lon", "order_type": "move", "target": "nth" },
            { "unit_type": "fleet", "location": "edi", "order_type": "move", "target": "nrg" },
            { "unit_type": "army", "location": "lvp", "order_type": "move", "target": "yor" }
          ]
        }
      ]
    }
  ]
}"#
    }

    /// Creates a standard 1901 Spring initial board state.
    fn initial_state() -> BoardState {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);

        // Austria
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Fleet, Coast::None);

        // England
        state.place_unit(Province::Lon, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Edi, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Lvp, Power::England, UnitType::Army, Coast::None);

        // France
        state.place_unit(Province::Bre, Power::France, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Par, Power::France, UnitType::Army, Coast::None);
        state.place_unit(Province::Mar, Power::France, UnitType::Army, Coast::None);

        // Germany
        state.place_unit(Province::Kie, Power::Germany, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Mun, Power::Germany, UnitType::Army, Coast::None);

        // Italy
        state.place_unit(Province::Nap, Power::Italy, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Rom, Power::Italy, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        // Russia
        state.place_unit(Province::Stp, Power::Russia, UnitType::Fleet, Coast::South);
        state.place_unit(Province::Mos, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::War, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Sev, Power::Russia, UnitType::Fleet, Coast::None);

        // Turkey
        state.place_unit(Province::Ank, Power::Turkey, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Con, Power::Turkey, UnitType::Army, Coast::None);
        state.place_unit(Province::Smy, Power::Turkey, UnitType::Army, Coast::None);

        // Set initial SC ownership
        for prov in ALL_PROVINCES {
            if prov.is_supply_center() {
                if let Some(home) = prov.home_power() {
                    state.set_sc_owner(prov, Some(home));
                }
            }
        }

        state
    }

    #[test]
    fn load_book_from_json_string() {
        let book = load_book_from_str(test_json()).unwrap();
        assert_eq!(book.entries.len(), 2);
        assert_eq!(book.entries[0].power, "austria");
        assert_eq!(book.entries[0].options.len(), 2);
        assert_eq!(book.entries[1].power, "england");
    }

    #[test]
    fn lookup_austria_spring_1901() {
        let book = load_book_from_str(test_json()).unwrap();
        let state = initial_state();
        let cfg = BookMatchConfig::default();

        let orders = lookup_opening(&book, &state, Power::Austria, &cfg);
        assert!(orders.is_some(), "Austria should match spring 1901");
        let orders = orders.unwrap();
        assert_eq!(orders.len(), 3, "Austria has 3 units");
    }

    #[test]
    fn lookup_england_spring_1901() {
        let book = load_book_from_str(test_json()).unwrap();
        let state = initial_state();
        let cfg = BookMatchConfig::default();

        let orders = lookup_opening(&book, &state, Power::England, &cfg);
        assert!(orders.is_some(), "England should match spring 1901");
        let orders = orders.unwrap();
        assert_eq!(orders.len(), 3);
    }

    #[test]
    fn no_match_for_wrong_year() {
        let book = load_book_from_str(test_json()).unwrap();
        let mut state = initial_state();
        state.year = 1950;
        let cfg = BookMatchConfig::default();

        assert!(lookup_opening(&book, &state, Power::Austria, &cfg).is_none());
    }

    #[test]
    fn no_match_for_retreat_phase() {
        let book = load_book_from_str(test_json()).unwrap();
        let mut state = initial_state();
        state.phase = Phase::Retreat;
        let cfg = BookMatchConfig::default();

        assert!(lookup_opening(&book, &state, Power::Austria, &cfg).is_none());
    }

    #[test]
    fn no_match_for_displaced_units() {
        let book = load_book_from_str(test_json()).unwrap();
        let mut state = initial_state();
        // Move England's army from lvp to yor (position mismatch).
        state.units[Province::Lvp as usize] = None;
        state.place_unit(Province::Yor, Power::England, UnitType::Army, Coast::None);
        let cfg = BookMatchConfig {
            mode: MatchMode::Exact,
            ..BookMatchConfig::default()
        };

        assert!(
            lookup_opening(&book, &state, Power::England, &cfg).is_none(),
            "Displaced units should not match in exact mode"
        );
    }

    #[test]
    fn score_condition_positions_full_match() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            positions: [
                ("lon".into(), "fleet".into()),
                ("edi".into(), "fleet".into()),
                ("lvp".into(), "army".into()),
            ]
            .into_iter()
            .collect(),
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(score > 0.0, "Full position match should be positive");
        assert!(
            (score - 3.0 * cfg.position_weight).abs() < 0.001,
            "Score should be 3 * position_weight"
        );
    }

    #[test]
    fn score_condition_positions_partial_match() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            positions: [
                ("lon".into(), "fleet".into()),
                ("edi".into(), "fleet".into()),
                ("lvp".into(), "fleet".into()), // wrong: lvp has army
            ]
            .into_iter()
            .collect(),
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(
            (score - 2.0 * cfg.position_weight).abs() < 0.001,
            "Score should be 2 * position_weight for 2/3 match"
        );
    }

    #[test]
    fn score_condition_exact_mode_rejects_partial() {
        let state = initial_state();
        let cfg = BookMatchConfig {
            mode: MatchMode::Exact,
            ..BookMatchConfig::default()
        };
        let cond = BookCondition {
            positions: [
                ("lon".into(), "fleet".into()),
                ("edi".into(), "fleet".into()),
                ("lvp".into(), "fleet".into()), // mismatch
            ]
            .into_iter()
            .collect(),
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert_eq!(score, -1.0, "Exact mode should return -1 for partial match");
    }

    #[test]
    fn score_condition_sc_ownership() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            owned_scs: vec!["lon".into(), "edi".into(), "lvp".into()],
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(
            (score - 3.0 * cfg.owned_sc_weight).abs() < 0.001,
            "All 3 England SCs should match"
        );
    }

    #[test]
    fn score_condition_sc_count_in_range() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            sc_count_min: 3,
            sc_count_max: 5,
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(
            (score - cfg.sc_count_weight).abs() < 0.001,
            "England has 3 SCs which is in [3,5]"
        );
    }

    #[test]
    fn score_condition_sc_count_out_of_range() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            sc_count_min: 10,
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(score < 0.001, "England has 3 SCs, min 10 should not match");
    }

    #[test]
    fn score_condition_fleet_army_counts() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        // England: 2 fleets, 1 army
        let cond = BookCondition {
            fleet_count: 2,
            army_count: 1,
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(
            (score - 2.0 * cfg.fleet_army_weight).abs() < 0.001,
            "Both fleet and army counts should match"
        );
    }

    #[test]
    fn score_condition_fleet_army_mismatch() {
        let state = initial_state();
        let cfg = BookMatchConfig::default();
        let cond = BookCondition {
            fleet_count: 1,
            army_count: 2,
            ..Default::default()
        };

        let score = score_condition(&cond, &state, Power::England, &cfg);
        assert!(score < 0.001, "Wrong fleet/army counts should score 0");
    }

    #[test]
    fn weighted_selection_coverage() {
        let book = load_book_from_str(test_json()).unwrap();
        let state = initial_state();
        let cfg = BookMatchConfig::default();

        let mut seen = HashMap::new();
        for _ in 0..500 {
            let orders = lookup_opening(&book, &state, Power::Austria, &cfg).unwrap();
            // Use first order's destination as key.
            let key = format!("{:?}", orders[0]);
            *seen.entry(key).or_insert(0) += 1;
        }

        assert!(
            seen.len() >= 2,
            "With 2 equal-weight options, both should appear. Got {} distinct.",
            seen.len()
        );
    }

    #[test]
    fn convert_move_order() {
        let input = OrderInput {
            unit_type: "army".into(),
            location: "vie".into(),
            coast: String::new(),
            order_type: "move".into(),
            target: "gal".into(),
            target_coast: String::new(),
            aux_loc: String::new(),
            aux_target: String::new(),
            aux_unit_type: String::new(),
        };

        let order = convert_single_order(&input, Power::Austria).unwrap();
        assert_eq!(
            order,
            Order::Move {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Vie),
                },
                dest: Location::new(Province::Gal),
            }
        );
    }

    #[test]
    fn convert_hold_order() {
        let input = OrderInput {
            unit_type: "fleet".into(),
            location: "lon".into(),
            coast: String::new(),
            order_type: "hold".into(),
            target: String::new(),
            target_coast: String::new(),
            aux_loc: String::new(),
            aux_target: String::new(),
            aux_unit_type: String::new(),
        };

        let order = convert_single_order(&input, Power::England).unwrap();
        assert_eq!(
            order,
            Order::Hold {
                unit: OrderUnit {
                    unit_type: UnitType::Fleet,
                    location: Location::new(Province::Lon),
                },
            }
        );
    }

    #[test]
    fn convert_build_order() {
        let input = OrderInput {
            unit_type: "fleet".into(),
            location: "stp".into(),
            coast: "sc".into(),
            order_type: "build".into(),
            target: String::new(),
            target_coast: String::new(),
            aux_loc: String::new(),
            aux_target: String::new(),
            aux_unit_type: String::new(),
        };

        let order = convert_single_order(&input, Power::Russia).unwrap();
        assert_eq!(
            order,
            Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Fleet,
                    location: Location::with_coast(Province::Stp, Coast::South),
                },
            }
        );
    }

    #[test]
    fn convert_disband_order() {
        let input = OrderInput {
            unit_type: "army".into(),
            location: "war".into(),
            coast: String::new(),
            order_type: "disband".into(),
            target: String::new(),
            target_coast: String::new(),
            aux_loc: String::new(),
            aux_target: String::new(),
            aux_unit_type: String::new(),
        };

        let order = convert_single_order(&input, Power::Russia).unwrap();
        assert_eq!(
            order,
            Order::Disband {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::War),
                },
            }
        );
    }

    #[test]
    fn border_pressure_no_enemies() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Par, Power::France, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Par, Some(Power::France));

        let bp = border_pressure(&state, Power::France);
        assert_eq!(bp, 0, "No enemy units means zero border pressure");
    }

    #[test]
    fn border_pressure_with_enemies() {
        let mut state = BoardState::empty(1902, Season::Spring, Phase::Movement);
        state.place_unit(Province::Par, Power::France, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Par, Some(Power::France));
        state.set_sc_owner(Province::Mar, Some(Power::France));
        state.set_sc_owner(Province::Bre, Some(Power::France));

        // German units adjacent to French SCs
        state.place_unit(Province::Bur, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Pic, Power::Germany, UnitType::Army, Coast::None);

        let bp = border_pressure(&state, Power::France);
        assert!(
            bp >= 2,
            "Two enemy units adjacent to French SCs: got {}",
            bp
        );
    }

    #[test]
    fn load_actual_book_file() {
        let path = Path::new("/Users/efreeman/polite-betrayal/data/processed/opening_book.json");
        if !path.exists() {
            // Skip if file doesn't exist in CI.
            return;
        }
        let book = load_book(path).unwrap();
        assert!(
            !book.entries.is_empty(),
            "Actual opening book should have entries"
        );

        // Verify all powers have spring 1901 entries.
        for power in ALL_POWERS {
            let has_entry = book.entries.iter().any(|e| {
                e.year == 1901
                    && e.season == "spring"
                    && e.phase == "movement"
                    && parse_power_str(&e.power) == Some(power)
            });
            assert!(has_entry, "{:?} should have a spring 1901 entry", power);
        }
    }

    #[test]
    fn lookup_all_powers_actual_book() {
        let path = Path::new("/Users/efreeman/polite-betrayal/data/processed/opening_book.json");
        if !path.exists() {
            return;
        }
        let book = load_book(path).unwrap();
        let state = initial_state();
        let cfg = BookMatchConfig::default();

        for power in ALL_POWERS {
            let orders = lookup_opening(&book, &state, power, &cfg);
            assert!(
                orders.is_some(),
                "{:?} should have opening orders in spring 1901",
                power
            );
            let orders = orders.unwrap();
            // Count how many units this power has.
            let unit_count = ALL_PROVINCES
                .iter()
                .filter(|p| {
                    state.units[**p as usize]
                        .map(|(pw, _)| pw == power)
                        .unwrap_or(false)
                })
                .count();
            assert_eq!(
                orders.len(),
                unit_count,
                "{:?}: order count {} != unit count {}",
                power,
                orders.len(),
                unit_count
            );
        }
    }

    #[test]
    fn default_config_values() {
        let cfg = BookMatchConfig::default();
        assert_eq!(cfg.mode, MatchMode::Hybrid);
        assert!(cfg.min_score > 0.0);
        assert!(cfg.position_weight > cfg.neighbor_weight);
        assert!(cfg.neighbor_weight > cfg.sc_count_weight);
    }
}
