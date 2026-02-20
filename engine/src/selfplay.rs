//! Self-play game generation for training data.
//!
//! Plays full Diplomacy games by cycling through all seven powers each phase,
//! using the engine's search to select orders. Records DFEN states, orders,
//! value estimates, and SC counts per phase for reinforcement learning.

use std::io::Write;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

use rand::rngs::SmallRng;
use rand::{Rng, SeedableRng};

use crate::board::province::{Power, ALL_POWERS};
use crate::board::state::{BoardState, Phase};
use crate::board::Order;
use crate::eval::evaluate_all;
use crate::movegen::random_orders;
use crate::protocol::dfen::{encode_dfen, parse_dfen};
use crate::protocol::dson::format_orders;
use crate::resolve::{
    advance_state, apply_builds, apply_resolution, apply_retreats, is_game_over, needs_build_phase,
    resolve_builds, resolve_retreats, Resolver,
};
use crate::search::{
    heuristic_build_orders, heuristic_retreat_orders, regret_matching_search, search,
};

/// Standard opening DFEN for a new game.
const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

/// Configuration for self-play game generation.
#[derive(Clone)]
pub struct SelfPlayConfig {
    /// Number of games to play.
    pub num_games: usize,
    /// Time budget per move search (milliseconds).
    pub movetime_ms: u64,
    /// Engine strength (1-100). Controls heuristic vs neural blend.
    pub strength: u64,
    /// Maximum game year before forced termination.
    pub max_year: u16,
    /// Temperature for move sampling (0.0 = argmax, higher = more exploration).
    pub temperature: f64,
    /// Temperature decay: multiply temperature by this factor each year.
    pub temperature_decay: f64,
    /// Dirichlet noise alpha for root policy exploration.
    pub dirichlet_alpha: f64,
    /// Fraction of Dirichlet noise to mix into root policy.
    pub dirichlet_epsilon: f64,
    /// Minimum year before declaring a stalemate (games ending before this are discarded).
    pub min_stalemate_year: u16,
    /// SC count threshold: flag games where a power reaches this many SCs before year 5.
    pub early_domination_scs: i32,
    /// Year threshold for early domination check.
    pub early_domination_year: u16,
    /// Number of parallel threads for concurrent games.
    pub threads: usize,
    /// Random seed (0 = use entropy).
    pub seed: u64,
    /// Suppress per-game progress output.
    pub quiet: bool,
}

impl Default for SelfPlayConfig {
    fn default() -> Self {
        SelfPlayConfig {
            num_games: 10,
            movetime_ms: 2000,
            strength: 100,
            max_year: 1920,
            temperature: 1.0,
            temperature_decay: 0.95,
            dirichlet_alpha: 0.3,
            dirichlet_epsilon: 0.25,
            min_stalemate_year: 1905,
            early_domination_scs: 14,
            early_domination_year: 1905,
            threads: 4,
            seed: 0,
            quiet: false,
        }
    }
}

/// A single recorded phase from a self-play game.
#[derive(Clone)]
pub struct PhaseRecord {
    /// DFEN of the board state at the start of this phase.
    pub dfen: String,
    /// Year of this phase.
    pub year: u16,
    /// Season abbreviation ('s' or 'f').
    pub season: char,
    /// Phase abbreviation ('m', 'r', or 'b').
    pub phase: char,
    /// Orders issued by each power, as DSON strings. Index by power ordinal.
    pub orders: Vec<(Power, String)>,
    /// Heuristic value estimates for all 7 powers at this state.
    pub values: [f32; 7],
    /// SC counts for each power at this state.
    pub sc_counts: [i32; 7],
}

/// Quality flags for a completed game.
#[derive(Clone, Default)]
pub struct GameQuality {
    /// Whether the game ended in stalemate before the minimum year.
    pub early_stalemate: bool,
    /// Whether a power dominated too quickly (hit SC threshold before year threshold).
    pub early_domination: bool,
    /// The dominating power, if any.
    pub domination_power: Option<Power>,
}

/// A complete self-play game record.
#[derive(Clone)]
pub struct GameRecord {
    /// Sequential game ID.
    pub game_id: usize,
    /// All phase records in order.
    pub phases: Vec<PhaseRecord>,
    /// The winning power (solo victory), if any.
    pub winner: Option<Power>,
    /// Final SC counts for each power.
    pub final_sc_counts: [i32; 7],
    /// Final year of the game.
    pub final_year: u16,
    /// Quality assessment.
    pub quality: GameQuality,
}

/// Counts supply centers for each power.
fn sc_counts(state: &BoardState) -> [i32; 7] {
    let mut counts = [0i32; 7];
    for owner in state.sc_owner.iter() {
        if let Some(power) = owner {
            let idx = ALL_POWERS.iter().position(|p| p == power).unwrap();
            counts[idx] += 1;
        }
    }
    counts
}

/// Checks if the game is in a stalemate (no SC changes between two consecutive years).
fn is_stalemate(prev_scs: &[i32; 7], curr_scs: &[i32; 7]) -> bool {
    prev_scs == curr_scs
}

/// Generates Dirichlet noise for exploration.
#[allow(dead_code)]
fn dirichlet_noise(rng: &mut SmallRng, alpha: f64, n: usize) -> Vec<f64> {
    if n == 0 {
        return Vec::new();
    }
    // Approximate Dirichlet by sampling Gamma(alpha, 1) for each element.
    // We use the rejection method for small alpha.
    let mut samples: Vec<f64> = Vec::with_capacity(n);
    for _ in 0..n {
        let g = gamma_sample(rng, alpha);
        samples.push(g);
    }
    let total: f64 = samples.iter().sum();
    if total > 0.0 {
        for s in samples.iter_mut() {
            *s /= total;
        }
    } else {
        let uniform = 1.0 / n as f64;
        for s in samples.iter_mut() {
            *s = uniform;
        }
    }
    samples
}

/// Simple Gamma(alpha, 1) sampler using Marsaglia and Tsang's method.
#[allow(dead_code)]
fn gamma_sample(rng: &mut SmallRng, alpha: f64) -> f64 {
    if alpha < 1.0 {
        // Boost: Gamma(alpha) = Gamma(alpha+1) * U^(1/alpha)
        let u: f64 = rng.gen();
        return gamma_sample(rng, alpha + 1.0) * u.powf(1.0 / alpha);
    }
    let d = alpha - 1.0 / 3.0;
    let c = 1.0 / (9.0 * d).sqrt();
    loop {
        let _x: f64 = rng.gen::<f64>() * 2.0 - 1.0;
        // Use Box-Muller for normal sample
        let u1: f64 = rng.gen::<f64>().max(1e-30);
        let u2: f64 = rng.gen();
        let z = (-2.0 * u1.ln()).sqrt() * (2.0 * std::f64::consts::PI * u2).cos();

        let v = (1.0 + c * z).powi(3);
        if v <= 0.0 {
            continue;
        }
        let u: f64 = rng.gen();
        if u < 1.0 - 0.0331 * z.powi(4) {
            return d * v;
        }
        if u.ln() < 0.5 * z * z + d * (1.0 - v + v.ln()) {
            return d * v;
        }
    }
}

/// Plays a single self-play game and returns the game record.
pub fn play_game(config: &SelfPlayConfig, game_id: usize, rng: &mut SmallRng) -> GameRecord {
    let mut state = parse_dfen(INITIAL_DFEN).expect("failed to parse initial DFEN");
    let mut resolver = Resolver::new(64);
    let mut phases: Vec<PhaseRecord> = Vec::new();
    let mut prev_year_scs = sc_counts(&state);
    let mut stalemate_count = 0u32;
    let mut winner: Option<Power> = None;
    let mut quality = GameQuality::default();

    // Compute effective temperature per year (decays over time).
    let base_temp = config.temperature;
    let movetime = Duration::from_millis(config.movetime_ms);

    // Null writer for search output (discard info lines).
    let mut null_out = std::io::sink();

    loop {
        // Check termination conditions.
        if state.year > config.max_year {
            break;
        }
        if let Some(w) = is_game_over(&state) {
            winner = Some(w);
            break;
        }

        let dfen = encode_dfen(&state);
        let values = evaluate_all(&state);
        let counts = sc_counts(&state);

        // Check early domination.
        if state.year <= config.early_domination_year {
            for (i, &sc) in counts.iter().enumerate() {
                if sc >= config.early_domination_scs {
                    quality.early_domination = true;
                    quality.domination_power = Some(ALL_POWERS[i]);
                }
            }
        }

        // Effective temperature decays with year.
        let years_elapsed = (state.year as f64 - 1901.0).max(0.0);
        let eff_temp = base_temp * config.temperature_decay.powf(years_elapsed);

        // Collect orders for all alive powers.
        let mut phase_orders: Vec<(Power, String)> = Vec::new();
        let mut all_orders: Vec<(Order, Power)> = Vec::new();

        match state.phase {
            Phase::Movement => {
                for &power in ALL_POWERS.iter() {
                    if !power_has_units(&state, power) {
                        continue;
                    }

                    let result = if config.strength >= 80 {
                        regret_matching_search(
                            power,
                            &state,
                            movetime,
                            &mut null_out,
                            None,
                            config.strength,
                            None,
                        )
                    } else {
                        search(power, &state, movetime, &mut null_out)
                    };

                    let orders = if result.orders.is_empty() {
                        random_orders(power, &state, rng)
                    } else if eff_temp > 0.01 {
                        // Temperature sampling: with some probability, use random orders.
                        let p_random = (eff_temp * 0.1).min(0.5);
                        if rng.gen::<f64>() < p_random {
                            random_orders(power, &state, rng)
                        } else {
                            result.orders
                        }
                    } else {
                        result.orders
                    };

                    let dson = format_orders(&orders);
                    phase_orders.push((power, dson));
                    for o in orders {
                        all_orders.push((o, power));
                    }
                }

                // Resolve movement.
                let (results, dislodged) = resolver.resolve(&all_orders, &state);
                apply_resolution(&mut state, &results, &dislodged);
                let has_dislodged = state.dislodged.iter().any(|d| d.is_some());
                advance_state(&mut state, has_dislodged);
            }
            Phase::Retreat => {
                for &power in ALL_POWERS.iter() {
                    let retreat_orders = heuristic_retreat_orders(power, &state);
                    if retreat_orders.is_empty() {
                        continue;
                    }
                    let dson = format_orders(&retreat_orders);
                    phase_orders.push((power, dson));
                    let with_power: Vec<(Order, Power)> =
                        retreat_orders.into_iter().map(|o| (o, power)).collect();
                    let results = resolve_retreats(&with_power, &state);
                    apply_retreats(&mut state, &results);
                }
                advance_state(&mut state, false);
            }
            Phase::Build => {
                let mut build_orders_all: Vec<(Order, Power)> = Vec::new();
                for &power in ALL_POWERS.iter() {
                    let build_orders = heuristic_build_orders(power, &state);
                    if build_orders.is_empty() {
                        continue;
                    }
                    let dson = format_orders(&build_orders);
                    phase_orders.push((power, dson));
                    for o in build_orders {
                        build_orders_all.push((o, power));
                    }
                }
                let results = resolve_builds(&build_orders_all, &state);
                apply_builds(&mut state, &results);

                // Check stalemate after build phase (end of year).
                let new_scs = sc_counts(&state);
                if is_stalemate(&prev_year_scs, &new_scs) {
                    stalemate_count += 1;
                    if stalemate_count >= 3 {
                        // Three consecutive years with no SC changes = stalemate.
                        if state.year < config.min_stalemate_year {
                            quality.early_stalemate = true;
                        }
                        break;
                    }
                } else {
                    stalemate_count = 0;
                }
                prev_year_scs = new_scs;

                if !needs_build_phase(&state) {
                    advance_state(&mut state, false);
                } else {
                    advance_state(&mut state, false);
                }
            }
        }

        phases.push(PhaseRecord {
            dfen,
            year: state.year,
            season: state.season.dfen_char(),
            phase: state.phase.dfen_char(),
            orders: phase_orders,
            values,
            sc_counts: counts,
        });
    }

    let final_scs = sc_counts(&state);

    GameRecord {
        game_id,
        phases,
        winner,
        final_sc_counts: final_scs,
        final_year: state.year,
        quality,
    }
}

/// Returns true if the power has any units on the board.
fn power_has_units(state: &BoardState, power: Power) -> bool {
    state
        .units
        .iter()
        .any(|u| matches!(u, Some((p, _)) if *p == power))
}

/// Runs self-play generation, producing multiple game records.
///
/// When `config.threads > 1`, games are played concurrently using rayon.
pub fn run_self_play(config: &SelfPlayConfig) -> Vec<GameRecord> {
    let mut games = Vec::with_capacity(config.num_games);
    run_self_play_with_callback(config, |game| {
        games.push(game);
    });
    games
}

/// Runs self-play generation, calling `on_game` with each completed game record.
///
/// This allows the caller to process games incrementally (e.g. write to disk)
/// rather than waiting for all games to finish.
pub fn run_self_play_with_callback<F>(config: &SelfPlayConfig, on_game: F)
where
    F: FnMut(GameRecord) + Send,
{
    if config.threads > 1 {
        run_self_play_parallel(config, on_game);
    } else {
        run_self_play_sequential(config, on_game);
    }
}

/// Sequential self-play: plays games one at a time.
fn run_self_play_sequential<F>(config: &SelfPlayConfig, mut on_game: F)
where
    F: FnMut(GameRecord),
{
    let mut rng = if config.seed != 0 {
        SmallRng::seed_from_u64(config.seed)
    } else {
        SmallRng::from_entropy()
    };

    for i in 0..config.num_games {
        let game_start = Instant::now();
        let game = play_game(config, i, &mut rng);
        if !config.quiet {
            let elapsed = game_start.elapsed().as_secs_f64();
            let outcome = match game.winner {
                Some(w) => format!("{} wins", power_name(w)),
                None => "draw".to_string(),
            };
            eprintln!(
                "Game {}/{}: {} in {} ({:.1}s)",
                i + 1,
                config.num_games,
                outcome,
                game.final_year,
                elapsed,
            );
        }
        on_game(game);
    }
}

/// Parallel self-play: plays games concurrently using rayon.
/// Uses a channel to deliver completed games to the callback from worker threads.
fn run_self_play_parallel<F>(config: &SelfPlayConfig, mut on_game: F)
where
    F: FnMut(GameRecord) + Send,
{
    use rayon::prelude::*;
    use std::sync::mpsc;

    let completed = AtomicUsize::new(0);
    let (tx, rx) = mpsc::channel::<GameRecord>();

    // Build thread pool with configured thread count.
    let pool = rayon::ThreadPoolBuilder::new()
        .num_threads(config.threads)
        .build()
        .expect("failed to build rayon thread pool");

    let config_clone = config.clone();
    let handle = std::thread::spawn(move || {
        pool.install(|| {
            (0..config_clone.num_games)
                .into_par_iter()
                .for_each_with(tx, |tx, i| {
                    let mut rng = if config_clone.seed != 0 {
                        SmallRng::seed_from_u64(config_clone.seed.wrapping_add(i as u64))
                    } else {
                        SmallRng::from_entropy()
                    };
                    let game_start = Instant::now();
                    let game = play_game(&config_clone, i, &mut rng);
                    if !config_clone.quiet {
                        let n = completed.fetch_add(1, Ordering::Relaxed) + 1;
                        let elapsed = game_start.elapsed().as_secs_f64();
                        let outcome = match game.winner {
                            Some(w) => format!("{} wins", power_name(w)),
                            None => "draw".to_string(),
                        };
                        eprintln!(
                            "Game {}/{}: {} in {} ({:.1}s)",
                            n, config_clone.num_games, outcome, game.final_year, elapsed,
                        );
                    }
                    let _ = tx.send(game);
                });
        });
    });

    // Receive completed games on the main thread and pass to callback.
    for game in rx {
        on_game(game);
    }

    handle.join().expect("selfplay worker thread panicked");
}

/// Writes game records as JSONL (one JSON object per game, one per line).
pub fn write_jsonl<W: Write>(games: &[GameRecord], out: &mut W) -> std::io::Result<()> {
    for game in games {
        write_game_json(game, out)?;
        writeln!(out)?;
    }
    out.flush()
}

/// Writes a single game record as a JSON object.
pub fn write_game_json<W: Write>(game: &GameRecord, out: &mut W) -> std::io::Result<()> {
    write!(out, "{{")?;
    write!(out, "\"game_id\":{}", game.game_id)?;
    write!(out, ",\"winner\":")?;
    match game.winner {
        Some(w) => write!(out, "\"{}\"", power_name(w))?,
        None => write!(out, "null")?,
    }
    write!(out, ",\"final_year\":{}", game.final_year)?;
    write!(out, ",\"final_sc_counts\":[")?;
    for (i, &sc) in game.final_sc_counts.iter().enumerate() {
        if i > 0 {
            write!(out, ",")?;
        }
        write!(out, "{}", sc)?;
    }
    write!(out, "]")?;
    write!(out, ",\"quality\":{{")?;
    write!(
        out,
        "\"early_stalemate\":{},\"early_domination\":{}",
        game.quality.early_stalemate, game.quality.early_domination
    )?;
    write!(out, "}}")?;

    write!(out, ",\"phases\":[")?;
    for (pi, phase) in game.phases.iter().enumerate() {
        if pi > 0 {
            write!(out, ",")?;
        }
        write_phase_json(phase, out)?;
    }
    write!(out, "]")?;
    write!(out, "}}")
}

/// Writes a single phase record as a JSON object.
fn write_phase_json<W: Write>(phase: &PhaseRecord, out: &mut W) -> std::io::Result<()> {
    write!(out, "{{")?;
    write!(out, "\"dfen\":\"{}\",", escape_json(&phase.dfen))?;
    write!(
        out,
        "\"year\":{},\"season\":\"{}\",\"phase\":\"{}\"",
        phase.year, phase.season, phase.phase
    )?;

    write!(out, ",\"orders\":{{")?;
    for (i, (power, dson)) in phase.orders.iter().enumerate() {
        if i > 0 {
            write!(out, ",")?;
        }
        write!(out, "\"{}\":\"{}\"", power_name(*power), escape_json(dson))?;
    }
    write!(out, "}}")?;

    write!(out, ",\"values\":[")?;
    for (i, &v) in phase.values.iter().enumerate() {
        if i > 0 {
            write!(out, ",")?;
        }
        write!(out, "{:.4}", v)?;
    }
    write!(out, "]")?;

    write!(out, ",\"sc_counts\":[")?;
    for (i, &sc) in phase.sc_counts.iter().enumerate() {
        if i > 0 {
            write!(out, ",")?;
        }
        write!(out, "{}", sc)?;
    }
    write!(out, "]")?;

    write!(out, "}}")
}

/// Returns the lowercase power name for JSON output.
fn power_name(power: Power) -> &'static str {
    match power {
        Power::Austria => "austria",
        Power::England => "england",
        Power::France => "france",
        Power::Germany => "germany",
        Power::Italy => "italy",
        Power::Russia => "russia",
        Power::Turkey => "turkey",
    }
}

/// Escapes special characters for JSON string values.
fn escape_json(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            _ => out.push(c),
        }
    }
    out
}

/// Prints a summary of self-play results to stderr.
pub fn print_summary(games: &[GameRecord]) {
    let total = games.len();
    let mut win_counts = [0usize; 7];
    let mut draw_count = 0usize;
    let mut stalemate_count = 0usize;
    let mut domination_count = 0usize;
    let mut total_phases = 0usize;
    let mut total_years = 0u32;

    for game in games {
        total_phases += game.phases.len();
        total_years += (game.final_year - 1901) as u32;

        if let Some(w) = game.winner {
            let idx = ALL_POWERS.iter().position(|p| *p == w).unwrap();
            win_counts[idx] += 1;
        } else {
            draw_count += 1;
        }

        if game.quality.early_stalemate {
            stalemate_count += 1;
        }
        if game.quality.early_domination {
            domination_count += 1;
        }
    }

    eprintln!("=== Self-Play Summary ===");
    eprintln!("Games: {}", total);
    eprintln!(
        "Avg phases/game: {:.1}",
        total_phases as f64 / total.max(1) as f64
    );
    eprintln!(
        "Avg years/game: {:.1}",
        total_years as f64 / total.max(1) as f64
    );
    eprintln!("Draws: {}", draw_count);
    eprintln!("Early stalemates (filtered): {}", stalemate_count);
    eprintln!("Early dominations (flagged): {}", domination_count);
    eprintln!("Win distribution:");
    for (i, &power) in ALL_POWERS.iter().enumerate() {
        let pct = 100.0 * win_counts[i] as f64 / total.max(1) as f64;
        eprintln!(
            "  {:>8}: {} ({:.1}%)",
            power_name(power),
            win_counts[i],
            pct
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn play_single_game_completes() {
        let config = SelfPlayConfig {
            num_games: 1,
            movetime_ms: 200,
            strength: 50,
            max_year: 1905,
            temperature: 0.5,
            seed: 42,
            ..Default::default()
        };
        let mut rng = SmallRng::seed_from_u64(42);
        let game = play_game(&config, 0, &mut rng);

        assert!(
            !game.phases.is_empty(),
            "Game should have at least one phase"
        );
        assert!(
            game.final_year <= config.max_year + 1,
            "Game should end by max year, ended at {}",
            game.final_year
        );
    }

    #[test]
    fn game_record_has_valid_dfen() {
        let config = SelfPlayConfig {
            num_games: 1,
            movetime_ms: 200,
            strength: 50,
            max_year: 1903,
            temperature: 0.0,
            seed: 123,
            ..Default::default()
        };
        let mut rng = SmallRng::seed_from_u64(123);
        let game = play_game(&config, 0, &mut rng);

        // Every phase should have a parseable DFEN.
        for phase in &game.phases {
            let result = parse_dfen(&phase.dfen);
            assert!(result.is_ok(), "Phase DFEN should be valid: {}", phase.dfen);
        }
    }

    #[test]
    fn sequential_run_produces_correct_count() {
        let config = SelfPlayConfig {
            num_games: 3,
            movetime_ms: 100,
            strength: 50,
            max_year: 1903,
            temperature: 0.5,
            threads: 1,
            seed: 99,
            ..Default::default()
        };
        let games = run_self_play(&config);
        assert_eq!(games.len(), 3);
    }

    #[test]
    fn parallel_run_produces_correct_count() {
        let config = SelfPlayConfig {
            num_games: 4,
            movetime_ms: 100,
            strength: 50,
            max_year: 1903,
            temperature: 0.5,
            threads: 2,
            seed: 77,
            ..Default::default()
        };
        let games = run_self_play(&config);
        assert_eq!(games.len(), 4);
    }

    #[test]
    fn jsonl_output_is_valid() {
        let config = SelfPlayConfig {
            num_games: 1,
            movetime_ms: 100,
            strength: 50,
            max_year: 1903,
            temperature: 0.0,
            threads: 1,
            seed: 55,
            ..Default::default()
        };
        let games = run_self_play(&config);
        let mut buf = Vec::new();
        write_jsonl(&games, &mut buf).unwrap();
        let output = String::from_utf8(buf).unwrap();

        // Should be valid JSON lines.
        for line in output.lines() {
            assert!(
                line.starts_with('{'),
                "Each line should start with '{{': {}",
                line
            );
            assert!(
                line.ends_with('}'),
                "Each line should end with '}}': {}",
                line
            );
            // Verify it contains expected fields.
            assert!(line.contains("\"game_id\""), "Should contain game_id");
            assert!(line.contains("\"phases\""), "Should contain phases");
            assert!(line.contains("\"dfen\""), "Should contain dfen");
        }
    }

    #[test]
    fn sc_counts_initial_position() {
        let state = parse_dfen(INITIAL_DFEN).unwrap();
        let counts = sc_counts(&state);
        // Initial: A=3, E=3, F=3, G=3, I=3, R=4, T=3
        assert_eq!(counts[0], 3); // Austria
        assert_eq!(counts[1], 3); // England
        assert_eq!(counts[5], 4); // Russia
    }

    #[test]
    fn dirichlet_noise_sums_to_one() {
        let mut rng = SmallRng::seed_from_u64(42);
        let noise = dirichlet_noise(&mut rng, 0.3, 10);
        assert_eq!(noise.len(), 10);
        let sum: f64 = noise.iter().sum();
        assert!(
            (sum - 1.0).abs() < 1e-6,
            "Dirichlet noise should sum to 1.0, got {}",
            sum
        );
        assert!(
            noise.iter().all(|&v| v >= 0.0),
            "All values should be non-negative"
        );
    }

    #[test]
    fn stalemate_detection() {
        let a = [3, 3, 3, 3, 3, 4, 3];
        let b = [3, 3, 3, 3, 3, 4, 3];
        assert!(is_stalemate(&a, &b));

        let c = [4, 3, 3, 3, 3, 3, 3];
        assert!(!is_stalemate(&a, &c));
    }

    #[test]
    fn power_name_all_powers() {
        assert_eq!(power_name(Power::Austria), "austria");
        assert_eq!(power_name(Power::England), "england");
        assert_eq!(power_name(Power::France), "france");
        assert_eq!(power_name(Power::Germany), "germany");
        assert_eq!(power_name(Power::Italy), "italy");
        assert_eq!(power_name(Power::Russia), "russia");
        assert_eq!(power_name(Power::Turkey), "turkey");
    }

    #[test]
    fn escape_json_special_chars() {
        assert_eq!(escape_json("hello"), "hello");
        assert_eq!(escape_json("he\"llo"), "he\\\"llo");
        assert_eq!(escape_json("a\\b"), "a\\\\b");
        assert_eq!(escape_json("a\nb"), "a\\nb");
    }
}
