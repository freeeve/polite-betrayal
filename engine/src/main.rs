//! Realpolitik -- a Diplomacy engine implementing the DUI protocol.
//!
//! This binary reads commands from stdin and writes responses to stdout,
//! following the DUI (Diplomacy Universal Interface) convention.
//!
//! Stdin is read on a dedicated thread and commands are forwarded via
//! an mpsc channel so that `go` search runs asynchronously and `stop`
//! can interrupt it.

use std::io::{self, BufRead};
use std::sync::mpsc;
use std::time::Duration;

use realpolitik::engine::Engine;
use realpolitik::protocol::parser::{parse_command, Command};

/// Poll interval while a search is in flight (10 ms).
const SEARCH_POLL_MS: u64 = 10;

/// Runs the main DUI protocol loop with async go/stop support.
fn main() {
    let stdout = io::stdout();
    let mut out = io::BufWriter::new(stdout.lock());
    let mut engine = Engine::new();

    // Spawn a dedicated stdin reader thread.
    let (tx, rx) = mpsc::channel::<String>();
    std::thread::spawn(move || {
        let stdin = io::stdin();
        for line in stdin.lock().lines() {
            match line {
                Ok(l) => {
                    if tx.send(l).is_err() {
                        break;
                    }
                }
                Err(_) => break,
            }
        }
    });

    loop {
        // Decide whether to block or poll based on search state.
        let line = if engine.is_searching() {
            match rx.recv_timeout(Duration::from_millis(SEARCH_POLL_MS)) {
                Ok(l) => Some(l),
                Err(mpsc::RecvTimeoutError::Timeout) => {
                    // Check if the search finished naturally.
                    engine.poll_search_done(&mut out);
                    continue;
                }
                Err(mpsc::RecvTimeoutError::Disconnected) => break,
            }
        } else {
            match rx.recv() {
                Ok(l) => Some(l),
                Err(_) => break,
            }
        };

        let line = match line {
            Some(l) => l,
            None => break,
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
                // If searching, wait for it to finish before responding.
                if engine.is_searching() {
                    engine.handle_stop(&mut out);
                }
                engine.handle_isready(&mut out);
            }
            Command::SetOption { name, value } => {
                engine.set_option(name, value);
            }
            Command::NewGame => {
                if engine.is_searching() {
                    engine.handle_stop(&mut out);
                }
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
            Command::Go(params) => {
                engine.handle_go(&mut out, Some(&params));
            }
            Command::Stop => {
                if engine.is_searching() {
                    engine.handle_stop(&mut out);
                }
            }
            Command::Press { raw } => {
                engine.handle_press(&raw);
            }
            Command::Quit => {
                // Flush any in-flight search results before exiting.
                if engine.is_searching() {
                    engine.handle_stop(&mut out);
                }
                break;
            }
        }
    }
}
