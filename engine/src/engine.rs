//! Engine state management.
//!
//! Holds the current board position, active power, engine options, and
//! runs search for the `go` command. Uses RM+ search at high strength
//! (>= 80) and Cartesian search otherwise.

use std::collections::HashMap;
use std::io::Write;
use std::time::Duration;

use rand::rngs::SmallRng;
use rand::SeedableRng;

use crate::board::province::Power;
use crate::board::state::{BoardState, Phase};
use crate::eval::NeuralEvaluator;
use crate::movegen::random_orders;
use crate::opening_book::{self, BookMatchConfig, OpeningBook};
use crate::protocol::dfen::parse_dfen;
use crate::protocol::dson::format_orders;
use crate::search::{
    heuristic_build_orders, heuristic_retreat_orders, regret_matching_search, search,
};

/// Default search time in milliseconds.
const DEFAULT_MOVETIME_MS: u64 = 5000;

/// Default path for the opening book JSON file.
const DEFAULT_BOOK_PATH: &str = "data/processed/opening_book.json";

/// Holds the mutable state of the engine between commands.
pub struct Engine {
    pub position: Option<BoardState>,
    pub active_power: Option<Power>,
    pub options: HashMap<String, String>,
    pub neural: Option<NeuralEvaluator>,
    book: Option<OpeningBook>,
    book_loaded: bool,
    rng: SmallRng,
}

impl Engine {
    /// Creates a new engine with no position or active power.
    pub fn new() -> Self {
        Engine {
            position: None,
            active_power: None,
            options: HashMap::new(),
            neural: None,
            book: None,
            book_loaded: false,
            rng: SmallRng::from_entropy(),
        }
    }

    /// Resets all engine state for a new game.
    pub fn new_game(&mut self) {
        self.position = None;
        self.active_power = None;
    }

    /// Lazily loads the opening book from the configured BookPath (or default).
    fn ensure_book(&mut self) {
        if self.book_loaded {
            return;
        }
        self.book_loaded = true;
        let path_str = self
            .options
            .get("BookPath")
            .cloned()
            .unwrap_or_else(|| DEFAULT_BOOK_PATH.to_string());
        if path_str.is_empty() {
            return;
        }
        let path = std::path::Path::new(&path_str);
        match opening_book::load_book(path) {
            Ok(b) => {
                eprintln!(
                    "info string loaded opening book ({} entries)",
                    b.entries.len()
                );
                self.book = Some(b);
            }
            Err(e) => {
                eprintln!("info string opening book not loaded: {}", e);
            }
        }
    }

    /// Lazily initializes the neural evaluator when ModelPath is first set.
    fn ensure_neural(&mut self) {
        if self.neural.is_some() {
            return;
        }
        let model_dir = match self.options.get("ModelPath") {
            Some(p) if !p.is_empty() => p.clone(),
            _ => return,
        };
        let policy_path = format!("{}/policy_v1.onnx", model_dir);
        let value_path = format!("{}/value_v1.onnx", model_dir);
        self.neural = Some(NeuralEvaluator::new(Some(&policy_path), Some(&value_path)));
    }

    /// Sets the current board position from a DFEN string.
    /// Returns an error message on failure.
    pub fn set_position(&mut self, dfen: &str) -> Result<(), String> {
        match parse_dfen(dfen) {
            Ok(state) => {
                self.position = Some(state);
                Ok(())
            }
            Err(e) => Err(format!("failed to parse DFEN: {}", e)),
        }
    }

    /// Sets the active power.
    pub fn set_power(&mut self, power: Power) {
        self.active_power = Some(power);
    }

    /// Sets an engine option.
    pub fn set_option(&mut self, name: String, value: Option<String>) {
        let reload_neural = name == "ModelPath";
        let reload_book = name == "BookPath";
        match value {
            Some(v) => {
                self.options.insert(name, v);
            }
            None => {
                self.options.insert(name, String::new());
            }
        }
        if reload_neural {
            self.neural = None; // force re-initialization
            self.ensure_neural();
        }
        if reload_book {
            self.book = None;
            self.book_loaded = false;
            self.ensure_book();
        }
    }

    /// Runs the movement phase search (RM+ or Cartesian based on strength).
    fn run_movement_search<W: Write>(
        &mut self,
        power: Power,
        out: &mut W,
    ) -> Vec<crate::board::Order> {
        let movetime = self.movetime();
        let strength = self.strength();
        let state = self.position.as_ref().unwrap();
        let result = if strength >= 80 {
            regret_matching_search(power, state, movetime, out, self.neural.as_ref(), strength)
        } else {
            search(power, state, movetime, out)
        };
        if result.orders.is_empty() {
            let state = self.position.as_ref().unwrap();
            random_orders(power, state, &mut self.rng)
        } else {
            result.orders
        }
    }

    /// Returns the configured search time from options, or the default.
    fn movetime(&self) -> Duration {
        let ms = self
            .options
            .get("SearchTime")
            .and_then(|v| v.parse::<u64>().ok())
            .unwrap_or(DEFAULT_MOVETIME_MS);
        Duration::from_millis(ms)
    }

    /// Returns true if the engine is configured for neural evaluation.
    #[allow(dead_code)]
    fn use_neural(&self) -> bool {
        let mode = self
            .options
            .get("EvalMode")
            .map(|s| s.as_str())
            .unwrap_or("heuristic");
        mode == "neural" || mode == "auto"
    }

    /// Handles the DUI handshake: writes id, options, protocol_version, and duiok.
    pub fn handle_dui<W: Write>(&self, out: &mut W) {
        writeln!(out, "id name realpolitik").unwrap();
        writeln!(out, "id author polite-betrayal").unwrap();
        writeln!(out, "option name Threads type spin default 4 min 1 max 64").unwrap();
        writeln!(
            out,
            "option name SearchTime type spin default 5000 min 100 max 60000"
        )
        .unwrap();
        writeln!(
            out,
            "option name Strength type spin default 100 min 1 max 100"
        )
        .unwrap();
        writeln!(out, "option name ModelPath type string default models").unwrap();
        writeln!(
            out,
            "option name EvalMode type combo default heuristic var heuristic var neural var auto"
        )
        .unwrap();
        writeln!(
            out,
            "option name BookPath type string default {}",
            DEFAULT_BOOK_PATH
        )
        .unwrap();
        writeln!(out, "protocol_version 1").unwrap();
        writeln!(out, "duiok").unwrap();
        out.flush().unwrap();
    }

    /// Handles the `isready` command.
    pub fn handle_isready<W: Write>(&self, out: &mut W) {
        writeln!(out, "readyok").unwrap();
        out.flush().unwrap();
    }

    /// Returns the configured strength from options (default 100).
    fn strength(&self) -> u64 {
        self.options
            .get("Strength")
            .and_then(|v| v.parse::<u64>().ok())
            .unwrap_or(100)
    }

    /// Handles the `go` command. Uses RM+ search at high strength (>= 80)
    /// and Cartesian search otherwise. Retreat/build phases use heuristics.
    pub fn handle_go<W: Write>(&mut self, out: &mut W) {
        if self.position.is_none() {
            eprintln!("go: no position set");
            return;
        }

        let power = match self.active_power {
            Some(p) => p,
            None => {
                eprintln!("go: no active power set");
                return;
            }
        };

        self.ensure_neural();
        self.ensure_book();

        // Try opening book lookup first (before borrowing self mutably for search).
        let book_hit = {
            let state = self.position.as_ref().unwrap();
            if state.phase == Phase::Movement {
                if let Some(ref book) = self.book {
                    let cfg = BookMatchConfig::default();
                    opening_book::lookup_opening(book, state, power, &cfg)
                } else {
                    None
                }
            } else {
                None
            }
        };

        let orders = if let Some(book_orders) = book_hit {
            let _ = writeln!(out, "info string opening book hit for {:?}", power);
            book_orders
        } else {
            let phase = self.position.as_ref().unwrap().phase;
            match phase {
                Phase::Movement => self.run_movement_search(power, out),
                Phase::Retreat => {
                    let state = self.position.as_ref().unwrap();
                    let orders = heuristic_retreat_orders(power, state);
                    if orders.is_empty() {
                        random_orders(power, state, &mut self.rng)
                    } else {
                        orders
                    }
                }
                Phase::Build => {
                    let state = self.position.as_ref().unwrap();
                    let orders = heuristic_build_orders(power, state);
                    if orders.is_empty() {
                        random_orders(power, state, &mut self.rng)
                    } else {
                        orders
                    }
                }
            }
        };

        let dson = format_orders(&orders);
        writeln!(out, "bestorders {}", dson).unwrap();
        out.flush().unwrap();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::state::{Phase, Season};

    const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

    #[test]
    fn new_engine_has_no_state() {
        let engine = Engine::new();
        assert!(engine.position.is_none());
        assert!(engine.active_power.is_none());
        assert!(engine.options.is_empty());
    }

    #[test]
    fn new_game_resets_state() {
        let mut engine = Engine::new();
        engine.set_position(INITIAL_DFEN).unwrap();
        engine.set_power(Power::Austria);
        engine.new_game();
        assert!(engine.position.is_none());
        assert!(engine.active_power.is_none());
    }

    #[test]
    fn set_position_valid_dfen() {
        let mut engine = Engine::new();
        assert!(engine.set_position(INITIAL_DFEN).is_ok());
        assert!(engine.position.is_some());
        let state = engine.position.as_ref().unwrap();
        assert_eq!(state.year, 1901);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Movement);
    }

    #[test]
    fn set_position_invalid_dfen() {
        let mut engine = Engine::new();
        let result = engine.set_position("garbage");
        assert!(result.is_err());
        assert!(engine.position.is_none());
    }

    #[test]
    fn set_option_stores_value() {
        let mut engine = Engine::new();
        engine.set_option("Threads".to_string(), Some("8".to_string()));
        assert_eq!(engine.options.get("Threads"), Some(&"8".to_string()));
    }

    #[test]
    fn handle_go_outputs_bestorders() {
        let mut engine = Engine::new();
        engine.set_position(INITIAL_DFEN).unwrap();
        engine.set_power(Power::Austria);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        // Output may contain info lines before bestorders
        assert!(
            output_str.contains("bestorders "),
            "Output should contain bestorders: {}",
            output_str
        );
        let bestorders_line = output_str
            .lines()
            .find(|l| l.starts_with("bestorders "))
            .unwrap();
        let orders_part = bestorders_line.strip_prefix("bestorders ").unwrap();
        let order_count = orders_part.split(" ; ").count();
        assert_eq!(order_count, 3);
    }

    #[test]
    fn handle_go_russia_has_four_orders() {
        let mut engine = Engine::new();
        engine.set_position(INITIAL_DFEN).unwrap();
        engine.set_power(Power::Russia);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        let bestorders_line = output_str
            .lines()
            .find(|l| l.starts_with("bestorders "))
            .unwrap();
        let orders_part = bestorders_line.strip_prefix("bestorders ").unwrap();
        let order_count = orders_part.split(" ; ").count();
        assert_eq!(order_count, 4);
    }

    #[test]
    fn handle_dui_outputs_handshake() {
        let engine = Engine::new();
        let mut output = Vec::new();
        engine.handle_dui(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(output_str.contains("id name realpolitik"));
        assert!(output_str.contains("id author polite-betrayal"));
        assert!(output_str.contains("option name ModelPath"));
        assert!(output_str.contains("option name EvalMode"));
        assert!(output_str.contains("protocol_version 1"));
        assert!(output_str.contains("duiok"));
    }

    #[test]
    fn handle_isready_outputs_readyok() {
        let engine = Engine::new();
        let mut output = Vec::new();
        engine.handle_isready(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert_eq!(output_str.trim(), "readyok");
    }

    #[test]
    fn handle_dui_includes_book_path_option() {
        let engine = Engine::new();
        let mut output = Vec::new();
        engine.handle_dui(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(
            output_str.contains("option name BookPath"),
            "DUI handshake should advertise BookPath option"
        );
    }

    #[test]
    fn book_loaded_from_inline_json() {
        let mut engine = Engine::new();
        // Directly inject a book to test the lookup path.
        let json = r#"{
          "entries": [{
            "power": "austria",
            "year": 1901,
            "season": "spring",
            "phase": "movement",
            "condition": {
              "positions": {"bud": "army", "vie": "army", "tri": "fleet"},
              "owned_scs": ["bud", "tri", "vie"],
              "sc_count_min": 3, "sc_count_max": 3,
              "fleet_count": 1, "army_count": 2
            },
            "options": [{
              "name": "test_opening",
              "weight": 1.0,
              "orders": [
                {"unit_type":"army","location":"vie","order_type":"move","target":"gal"},
                {"unit_type":"fleet","location":"tri","order_type":"move","target":"alb"},
                {"unit_type":"army","location":"bud","order_type":"move","target":"ser"}
              ]
            }]
          }]
        }"#;
        engine.book = Some(opening_book::load_book_from_str(json).unwrap());
        engine.book_loaded = true;
        engine.set_position(INITIAL_DFEN).unwrap();
        engine.set_power(Power::Austria);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(
            output_str.contains("opening book hit"),
            "Should report book hit: {}",
            output_str
        );
        assert!(
            output_str.contains("bestorders "),
            "Should still output bestorders: {}",
            output_str
        );
        let bestorders_line = output_str
            .lines()
            .find(|l| l.starts_with("bestorders "))
            .unwrap();
        let orders_part = bestorders_line.strip_prefix("bestorders ").unwrap();
        let order_count = orders_part.split(" ; ").count();
        assert_eq!(order_count, 3, "Austria has 3 units");
    }

    #[test]
    fn book_miss_falls_through_to_search() {
        let mut engine = Engine::new();
        // Load a book that only has entries for 1901 -- set year to 1902 to miss.
        let json = r#"{
          "entries": [{
            "power": "austria",
            "year": 1901,
            "season": "spring",
            "phase": "movement",
            "condition": {},
            "options": [{
              "name": "test",
              "weight": 1.0,
              "orders": [
                {"unit_type":"army","location":"vie","order_type":"hold"},
                {"unit_type":"fleet","location":"tri","order_type":"hold"},
                {"unit_type":"army","location":"bud","order_type":"hold"}
              ]
            }]
          }]
        }"#;
        engine.book = Some(opening_book::load_book_from_str(json).unwrap());
        engine.book_loaded = true;
        // Use a 1902 position so the book doesn't match.
        let dfen_1902 = "1902sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-";
        engine.set_position(dfen_1902).unwrap();
        engine.set_power(Power::Austria);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(
            !output_str.contains("opening book hit"),
            "Should not report book hit for wrong year"
        );
        assert!(
            output_str.contains("bestorders "),
            "Should still output bestorders via search: {}",
            output_str
        );
    }

    #[test]
    fn no_book_falls_through_to_search() {
        let mut engine = Engine::new();
        // Set BookPath to empty string to disable book loading.
        engine.set_option("BookPath".to_string(), Some(String::new()));
        engine.set_position(INITIAL_DFEN).unwrap();
        engine.set_power(Power::Austria);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(
            !output_str.contains("opening book hit"),
            "No book should mean no book hit"
        );
        assert!(
            output_str.contains("bestorders "),
            "Should still output bestorders: {}",
            output_str
        );
    }

    #[test]
    fn book_not_used_for_build_phase() {
        let mut engine = Engine::new();
        let json = r#"{
          "entries": [{
            "power": "austria",
            "year": 1901,
            "season": "fall",
            "phase": "build",
            "condition": {},
            "options": [{
              "name": "test",
              "weight": 1.0,
              "orders": [
                {"unit_type":"army","location":"vie","order_type":"build"}
              ]
            }]
          }]
        }"#;
        engine.book = Some(opening_book::load_book_from_str(json).unwrap());
        engine.book_loaded = true;
        // Use a build phase DFEN. Austria has 4 SCs but only 3 units => build.
        let build_dfen = "1901fb/Aavie,Aabud,Aftri/Abud,Atri,Avie,Aser/-";
        engine.set_position(build_dfen).unwrap();
        engine.set_power(Power::Austria);

        let mut output = Vec::new();
        engine.handle_go(&mut output);

        let output_str = String::from_utf8(output).unwrap();
        assert!(
            !output_str.contains("opening book hit"),
            "Book should not be used in build phase"
        );
    }

    #[test]
    fn book_with_actual_file() {
        let path = std::path::Path::new(
            "/Users/efreeman/polite-betrayal/data/processed/opening_book.json",
        );
        if !path.exists() {
            return;
        }
        let mut engine = Engine::new();
        engine.set_option(
            "BookPath".to_string(),
            Some(path.to_string_lossy().to_string()),
        );
        engine.set_position(INITIAL_DFEN).unwrap();

        // Test all 7 powers get a book hit on spring 1901.
        for &p in &[
            Power::Austria,
            Power::England,
            Power::France,
            Power::Germany,
            Power::Italy,
            Power::Russia,
            Power::Turkey,
        ] {
            engine.set_power(p);
            let mut output = Vec::new();
            engine.handle_go(&mut output);

            let output_str = String::from_utf8(output).unwrap();
            assert!(
                output_str.contains("opening book hit"),
                "{:?} should get a book hit in spring 1901: {}",
                p,
                output_str
            );
            let bestorders_line = output_str
                .lines()
                .find(|l| l.starts_with("bestorders "))
                .unwrap();
            let orders_part = bestorders_line.strip_prefix("bestorders ").unwrap();
            assert!(
                !orders_part.is_empty(),
                "{:?} should have non-empty orders",
                p
            );
        }
    }
}
