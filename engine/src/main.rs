//! Realpolitik -- a Diplomacy engine implementing the DUI protocol.
//!
//! This binary reads commands from stdin and writes responses to stdout,
//! following the DUI (Diplomacy Universal Interface) convention.

use std::io::{self, BufRead};

use realpolitik::engine::Engine;
use realpolitik::protocol::parser::{parse_command, Command};

/// Runs the main DUI protocol loop, reading commands from stdin
/// and writing responses to stdout.
fn main() {
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut out = io::BufWriter::new(stdout.lock());
    let mut engine = Engine::new();

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };

        let cmd = match parse_command(&line) {
            Some(c) => c,
            None => continue,
        };

        match cmd {
            Command::Dui => {
                engine.handle_dui(&mut out);
            }
            Command::IsReady => {
                engine.handle_isready(&mut out);
            }
            Command::SetOption { name, value } => {
                engine.set_option(name, value);
            }
            Command::NewGame => {
                engine.new_game();
            }
            Command::Position { dfen } => {
                if let Err(e) = engine.set_position(&dfen) {
                    eprintln!("{}", e);
                }
            }
            Command::SetPower { power } => {
                engine.set_power(power);
            }
            Command::Go(_params) => {
                engine.handle_go(&mut out);
            }
            Command::Stop => {
                // No async search to interrupt yet; no-op
            }
            Command::Press { raw: _ } => {
                // Press handling is stubbed for now
            }
            Command::Quit => {
                break;
            }
        }
    }
}
