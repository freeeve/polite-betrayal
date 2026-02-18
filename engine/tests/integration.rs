//! Integration tests for the realpolitik engine binary.
//!
//! Tests the full DUI protocol session flow by spawning the engine process,
//! sending commands via stdin, and verifying stdout responses.

use std::io::{BufRead, Write};
use std::process::{Command, Stdio};

/// Sends a sequence of commands to the engine and collects stdout lines.
fn run_engine(commands: &[&str]) -> Vec<String> {
    let exe = env!("CARGO_BIN_EXE_realpolitik");
    let mut child = Command::new(exe)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .expect("failed to start realpolitik");

    let mut stdin = child.stdin.take().unwrap();
    let stdout = child.stdout.take().unwrap();
    let reader = std::io::BufReader::new(stdout);

    for cmd in commands {
        writeln!(stdin, "{}", cmd).unwrap();
    }
    stdin.flush().unwrap();
    drop(stdin);

    let lines: Vec<String> = reader.lines().map(|l| l.unwrap()).collect();
    let status = child.wait().expect("failed to wait on child");
    assert!(status.success());
    lines
}

/// The standard initial-position DFEN.
const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

/// Retreat phase DFEN with Austrian dislodgment.
const RETREAT_DFEN: &str = "1902fr/Aabud,Aavie,Aftri,Aagre,Efnth,Efnwy,Eabel,Eflon,Ffmao,Fabur,Fapar,Ffbre,Gaden,Gamun,Gfkie,Gaber,Ifnap,Iaven,Iarom,Ramos,Rawar,Ragal,Rfstp.sc,Tabul,Tfbla,Tacon,Tasmy,Tfank/Abud,Agre,Atri,Avie,Ebel,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gden,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tbul,Tcon,Tsmy,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/Aaser<bul,Rfsev<bla";

/// Build phase DFEN where Austria has 5 SCs, 3 units.
const BUILD_DFEN: &str = "1901fb/Aatri,Aarum,Afgre/Abud,Agre,Arum,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Nhol,Nnwy,Npor,Nser,Nspa,Nswe,Ntun/-";

#[test]
fn dui_handshake_with_protocol_version() {
    let lines = run_engine(&["dui", "quit"]);

    assert!(lines.iter().any(|l| l == "id name realpolitik"));
    assert!(lines.iter().any(|l| l == "id author polite-betrayal"));
    assert!(lines.iter().any(|l| l == "protocol_version 1"));
    assert!(lines.iter().any(|l| l == "duiok"));

    // duiok must be the last line of the handshake
    let duiok_idx = lines.iter().position(|l| l == "duiok").unwrap();
    let proto_idx = lines.iter().position(|l| l == "protocol_version 1").unwrap();
    assert!(proto_idx < duiok_idx, "protocol_version must appear before duiok");
}

#[test]
fn dui_handshake_includes_options() {
    let lines = run_engine(&["dui", "quit"]);

    // At least one option line should be present
    let option_lines: Vec<&String> = lines.iter().filter(|l| l.starts_with("option ")).collect();
    assert!(!option_lines.is_empty(), "handshake should include option declarations");

    // Verify option format: "option name <id> type <type> ..."
    for opt in &option_lines {
        assert!(opt.contains("type "), "option line missing type: {}", opt);
    }
}

#[test]
fn isready_response() {
    let lines = run_engine(&["isready", "quit"]);
    assert!(lines.contains(&"readyok".to_string()));
}

#[test]
fn unknown_commands_are_ignored() {
    let lines = run_engine(&["foobar", "nonsense", "quit"]);
    assert!(lines.is_empty());
}

#[test]
fn empty_lines_are_ignored() {
    let lines = run_engine(&["", "  ", "isready", "quit"]);
    assert_eq!(lines.len(), 1);
    assert_eq!(lines[0], "readyok");
}

#[test]
fn full_handshake_then_isready() {
    let lines = run_engine(&["dui", "isready", "quit"]);

    // Should have handshake lines followed by readyok
    assert!(lines.iter().any(|l| l == "id name realpolitik"));
    assert!(lines.iter().any(|l| l == "duiok"));
    assert!(lines.last() == Some(&"readyok".to_string()));
}

#[test]
fn setoption_then_isready() {
    let lines = run_engine(&[
        "dui",
        "setoption name Threads value 8",
        "setoption name Strength value 80",
        "isready",
        "quit",
    ]);

    // setoption should not produce any output; isready should produce readyok
    assert!(lines.last() == Some(&"readyok".to_string()));
}

#[test]
fn position_setpower_go_produces_bestorders() {
    let lines = run_engine(&[
        "dui",
        "isready",
        "newgame",
        "setpower austria",
        &format!("position {}", INITIAL_DFEN),
        "go movetime 5000",
        "quit",
    ]);

    // Should contain a bestorders line
    let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
    assert_eq!(bestorders.len(), 1, "expected exactly one bestorders response");

    let orders_str = bestorders[0].strip_prefix("bestorders ").unwrap();
    // Austria has 3 units; should have 3 orders separated by " ; "
    let order_parts: Vec<&str> = orders_str.split(" ; ").collect();
    assert_eq!(order_parts.len(), 3, "Austria should have 3 orders, got: {}", orders_str);
}

#[test]
fn go_for_all_seven_powers() {
    let expected_units: [(&str, usize); 7] = [
        ("austria", 3),
        ("england", 3),
        ("france", 3),
        ("germany", 3),
        ("italy", 3),
        ("russia", 4),
        ("turkey", 3),
    ];

    for (power_name, unit_count) in &expected_units {
        let lines = run_engine(&[
            "dui",
            "isready",
            "newgame",
            &format!("setpower {}", power_name),
            &format!("position {}", INITIAL_DFEN),
            "go",
            "quit",
        ]);

        let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
        assert_eq!(bestorders.len(), 1, "expected bestorders for {}", power_name);

        let orders_str = bestorders[0].strip_prefix("bestorders ").unwrap();
        let order_parts: Vec<&str> = orders_str.split(" ; ").collect();
        assert_eq!(
            order_parts.len(),
            *unit_count,
            "{} should have {} orders, got: {}",
            power_name,
            unit_count,
            orders_str,
        );
    }
}

#[test]
fn newgame_resets_state() {
    // First set position and get bestorders, then newgame and try go again
    // without setting position -- should produce no output for the second go
    let lines = run_engine(&[
        "dui",
        "isready",
        "setpower austria",
        &format!("position {}", INITIAL_DFEN),
        "go",
        "newgame",
        "go",
        "quit",
    ]);

    // Should have exactly one bestorders line (the second go has no position)
    let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
    assert_eq!(bestorders.len(), 1, "second go after newgame should produce no bestorders");
}

#[test]
fn multi_power_sequential_query() {
    // Query Austria then England sequentially without restarting
    let lines = run_engine(&[
        "dui",
        "isready",
        "newgame",
        "setpower austria",
        &format!("position {}", INITIAL_DFEN),
        "go movetime 5000",
        "setpower england",
        &format!("position {}", INITIAL_DFEN),
        "go movetime 5000",
        "quit",
    ]);

    let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
    assert_eq!(bestorders.len(), 2, "expected two bestorders responses for two powers");
}

#[test]
fn retreat_phase_produces_disband_orders() {
    let lines = run_engine(&[
        "dui",
        "isready",
        "setpower austria",
        &format!("position {}", RETREAT_DFEN),
        "go",
        "quit",
    ]);

    let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
    assert_eq!(bestorders.len(), 1);

    let orders_str = bestorders[0].strip_prefix("bestorders ").unwrap();
    // Austria has 1 dislodged unit at ser — should get 1 retreat order (retreat or disband)
    let order_parts: Vec<&str> = orders_str.split(" ; ").collect();
    assert_eq!(order_parts.len(), 1, "Austria should have 1 retreat order");
    assert!(
        order_parts[0].contains(" D") || order_parts[0].contains(" R "),
        "retreat order should be disband or retreat, got: {}",
        order_parts[0],
    );
}

#[test]
fn build_phase_produces_waive_orders() {
    let lines = run_engine(&[
        "dui",
        "isready",
        "setpower austria",
        &format!("position {}", BUILD_DFEN),
        "go",
        "quit",
    ]);

    let bestorders: Vec<&String> = lines.iter().filter(|l| l.starts_with("bestorders ")).collect();
    assert_eq!(bestorders.len(), 1);

    let orders_str = bestorders[0].strip_prefix("bestorders ").unwrap();
    // Austria has 5 SCs, 3 units -> 2 builds available
    let order_parts: Vec<&str> = orders_str.split(" ; ").collect();
    assert_eq!(order_parts.len(), 2, "Austria should have 2 build orders");
    for part in &order_parts {
        assert!(
            *part == "W" || part.ends_with(" B"),
            "build order should be waive or build, got: {}",
            part,
        );
    }
}

#[test]
fn malformed_position_does_not_crash() {
    let lines = run_engine(&[
        "dui",
        "isready",
        "position garbage_dfen",
        "isready",
        "quit",
    ]);

    // Engine should still respond to isready after malformed position
    let readyok_count = lines.iter().filter(|l| *l == "readyok").count();
    assert_eq!(readyok_count, 2, "engine should respond to both isready commands");
}

#[test]
fn eof_exits_cleanly() {
    // No quit command; just close stdin
    let lines = run_engine(&["dui", "isready"]);

    assert!(lines.iter().any(|l| l == "duiok"));
    assert!(lines.iter().any(|l| l == "readyok"));
}

#[test]
fn stop_does_not_crash() {
    let lines = run_engine(&["dui", "stop", "isready", "quit"]);
    assert!(lines.iter().any(|l| l == "readyok"));
}

#[test]
fn press_does_not_crash() {
    let lines = run_engine(&[
        "dui",
        "press france propose_alliance against germany",
        "isready",
        "quit",
    ]);
    assert!(lines.iter().any(|l| l == "readyok"));
}

#[test]
fn minimal_session_from_spec() {
    // Section 5.2 of the protocol spec
    let lines = run_engine(&[
        "dui",
        "isready",
        "newgame",
        "setpower austria",
        &format!("position {}", INITIAL_DFEN),
        "go movetime 5000",
        "quit",
    ]);

    // Verify the full flow produced expected outputs
    assert!(lines.iter().any(|l| l == "id name realpolitik"));
    assert!(lines.iter().any(|l| l == "id author polite-betrayal"));
    assert!(lines.iter().any(|l| l == "protocol_version 1"));
    assert!(lines.iter().any(|l| l == "duiok"));
    assert!(lines.iter().any(|l| l == "readyok"));
    assert!(lines.iter().any(|l| l.starts_with("bestorders ")));
}
