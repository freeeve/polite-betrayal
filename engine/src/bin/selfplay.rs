//! Self-play game generation CLI.
//!
//! Plays Diplomacy games via self-play and outputs training data as JSONL.
//!
//! Usage:
//!   cargo run --release --bin selfplay -- [OPTIONS]
//!
//! Options:
//!   --games N       Number of games to play (default: 10)
//!   --movetime MS   Search time per move in ms (default: 2000)
//!   --strength N    Engine strength 1-100 (default: 100)
//!   --max-year Y    Maximum game year (default: 1920)
//!   --temperature T Exploration temperature (default: 1.0)
//!   --threads N     Number of parallel threads (default: 4)
//!   --seed N        Random seed, 0 for entropy (default: 0)
//!   --output FILE   Output file path (default: stdout)
//!   --quiet         Suppress summary output

use std::env;
use std::fs::File;
use std::io::{self, BufWriter, Write};
use std::sync::Mutex;
use std::time::Instant;

use realpolitik::selfplay::{self, GameRecord, SelfPlayConfig};

fn main() {
    let args: Vec<String> = env::args().collect();
    let mut config = SelfPlayConfig::default();
    let mut output_path: Option<String> = None;
    let mut quiet = false;

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--games" => {
                i += 1;
                config.num_games = args[i].parse().expect("invalid --games value");
            }
            "--movetime" => {
                i += 1;
                config.movetime_ms = args[i].parse().expect("invalid --movetime value");
            }
            "--strength" => {
                i += 1;
                config.strength = args[i].parse().expect("invalid --strength value");
            }
            "--max-year" => {
                i += 1;
                config.max_year = args[i].parse().expect("invalid --max-year value");
            }
            "--temperature" => {
                i += 1;
                config.temperature = args[i].parse().expect("invalid --temperature value");
            }
            "--threads" => {
                i += 1;
                config.threads = args[i].parse().expect("invalid --threads value");
            }
            "--seed" => {
                i += 1;
                config.seed = args[i].parse().expect("invalid --seed value");
            }
            "--output" => {
                i += 1;
                output_path = Some(args[i].clone());
            }
            "--quiet" => {
                quiet = true;
            }
            "--help" | "-h" => {
                print_usage();
                return;
            }
            other => {
                eprintln!("Unknown argument: {}", other);
                print_usage();
                std::process::exit(1);
            }
        }
        i += 1;
    }

    config.quiet = quiet;

    if !quiet {
        eprintln!(
            "Self-play: {} games, {}ms/move, strength {}, max year {}, temp {:.2}, {} threads",
            config.num_games,
            config.movetime_ms,
            config.strength,
            config.max_year,
            config.temperature,
            config.threads
        );
    }

    // Open output file (or stdout) before starting selfplay so games are written incrementally.
    // We flush after every game, so BufWriter is only needed to batch the small writes
    // within a single game's JSON serialization.
    let writer: Mutex<Box<dyn Write + Send>> = match &output_path {
        Some(path) => {
            let file = File::create(path).expect("failed to create output file");
            Mutex::new(Box::new(BufWriter::new(file)))
        }
        None => Mutex::new(Box::new(io::stdout())),
    };

    let start = Instant::now();
    let mut all_games: Vec<GameRecord> = Vec::with_capacity(config.num_games);
    let all_games_mu = Mutex::new(&mut all_games);
    let written = Mutex::new(0usize);
    let discarded = Mutex::new(0usize);

    selfplay::run_self_play_with_callback(&config, |game| {
        if game.quality.early_stalemate {
            *discarded.lock().unwrap() += 1;
        } else {
            // Write game to output immediately and flush so the follow-mode importer sees it.
            let mut w = writer.lock().unwrap();
            selfplay::write_game_json(&game, &mut *w).expect("failed to write game");
            writeln!(&mut *w).expect("failed to write newline");
            w.flush().expect("failed to flush output");
            *written.lock().unwrap() += 1;
        }
        all_games_mu.lock().unwrap().push(game);
    });

    let elapsed = start.elapsed();
    let written_count = *written.lock().unwrap();
    let discarded_count = *discarded.lock().unwrap();

    if !quiet {
        eprintln!(
            "Completed {} games in {:.1}s ({:.1} games/hour)",
            all_games.len(),
            elapsed.as_secs_f64(),
            all_games.len() as f64 / elapsed.as_secs_f64() * 3600.0
        );
        eprintln!(
            "Valid games written: {} (discarded {} early stalemates)",
            written_count, discarded_count
        );
        selfplay::print_summary(&all_games);
        if let Some(path) = &output_path {
            eprintln!("Wrote {} games to {}", written_count, path);
        }
    }
}

fn print_usage() {
    eprintln!("Usage: selfplay [OPTIONS]");
    eprintln!();
    eprintln!("Options:");
    eprintln!("  --games N        Number of games to play (default: 10)");
    eprintln!("  --movetime MS    Search time per move in ms (default: 2000)");
    eprintln!("  --strength N     Engine strength 1-100 (default: 100)");
    eprintln!("  --max-year Y     Maximum game year (default: 1920)");
    eprintln!("  --temperature T  Exploration temperature (default: 1.0)");
    eprintln!("  --threads N      Number of parallel threads (default: 4)");
    eprintln!("  --seed N         Random seed, 0 for entropy (default: 0)");
    eprintln!("  --output FILE    Output file path (default: stdout)");
    eprintln!("  --quiet          Suppress summary output");
    eprintln!("  --help           Show this help");
}
