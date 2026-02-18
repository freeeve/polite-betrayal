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
use crate::movegen::random_orders;
use crate::protocol::dfen::parse_dfen;
use crate::protocol::dson::format_orders;
use crate::search::{
    heuristic_build_orders, heuristic_retreat_orders, regret_matching_search, search,
};

/// Default search time in milliseconds.
const DEFAULT_MOVETIME_MS: u64 = 5000;

/// Holds the mutable state of the engine between commands.
pub struct Engine {
    pub position: Option<BoardState>,
    pub active_power: Option<Power>,
    pub options: HashMap<String, String>,
    rng: SmallRng,
}

impl Engine {
    /// Creates a new engine with no position or active power.
    pub fn new() -> Self {
        Engine {
            position: None,
            active_power: None,
            options: HashMap::new(),
            rng: SmallRng::from_entropy(),
        }
    }

    /// Resets all engine state for a new game.
    pub fn new_game(&mut self) {
        self.position = None;
        self.active_power = None;
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
        match value {
            Some(v) => {
                self.options.insert(name, v);
            }
            None => {
                self.options.insert(name, String::new());
            }
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
        let state = match &self.position {
            Some(s) => s,
            None => {
                eprintln!("go: no position set");
                return;
            }
        };

        let power = match self.active_power {
            Some(p) => p,
            None => {
                eprintln!("go: no active power set");
                return;
            }
        };

        let orders = match state.phase {
            Phase::Movement => {
                let movetime = self.movetime();
                let strength = self.strength();
                let result = if strength >= 80 {
                    regret_matching_search(power, state, movetime, out)
                } else {
                    search(power, state, movetime, out)
                };
                if result.orders.is_empty() {
                    random_orders(power, state, &mut self.rng)
                } else {
                    result.orders
                }
            }
            Phase::Retreat => {
                let orders = heuristic_retreat_orders(power, state);
                if orders.is_empty() {
                    random_orders(power, state, &mut self.rng)
                } else {
                    orders
                }
            }
            Phase::Build => {
                let orders = heuristic_build_orders(power, state);
                if orders.is_empty() {
                    random_orders(power, state, &mut self.rng)
                } else {
                    orders
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
}
