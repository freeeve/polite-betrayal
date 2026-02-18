//! DUI command parser.
//!
//! Parses incoming DUI protocol commands from raw text into structured
//! `Command` variants that the engine main loop can dispatch on.

use crate::board::province::Power;

/// Search constraints passed with the `go` command.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GoParams {
    pub movetime: Option<u64>,
    pub depth: Option<u32>,
    pub nodes: Option<u64>,
    pub infinite: bool,
}

impl Default for GoParams {
    fn default() -> Self {
        Self {
            movetime: None,
            depth: None,
            nodes: None,
            infinite: false,
        }
    }
}

/// A parsed server-to-engine DUI command.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Command {
    /// Initialize the DUI protocol handshake.
    Dui,

    /// Synchronization ping; engine must reply `readyok`.
    IsReady,

    /// Set an engine option: `setoption name <id> [value <x>]`.
    SetOption { name: String, value: Option<String> },

    /// Reset engine state for a new game.
    NewGame,

    /// Set the board position from a DFEN string.
    Position { dfen: String },

    /// Set the active power for the current position.
    SetPower { power: Power },

    /// Begin calculating orders with optional search constraints.
    Go(GoParams),

    /// Interrupt the current search immediately.
    Stop,

    /// Deliver a diplomatic press message (structured intent).
    Press { raw: String },

    /// Terminate the engine process.
    Quit,
}

/// Parses a single line of input into a `Command`.
///
/// Returns `None` for empty lines or unrecognized commands. Malformed
/// arguments for known commands also return `None` after logging to stderr.
pub fn parse_command(line: &str) -> Option<Command> {
    let trimmed = line.trim();
    if trimmed.is_empty() {
        return None;
    }

    let tokens: Vec<&str> = trimmed.split_whitespace().collect();
    if tokens.is_empty() {
        return None;
    }

    match tokens[0] {
        "dui" => Some(Command::Dui),
        "isready" => Some(Command::IsReady),
        "quit" => Some(Command::Quit),
        "newgame" => Some(Command::NewGame),
        "stop" => Some(Command::Stop),

        "setoption" => parse_setoption(&tokens),
        "position" => parse_position(&tokens),
        "setpower" => parse_setpower(&tokens),
        "go" => parse_go(&tokens),
        "press" => parse_press(&tokens, trimmed),

        other => {
            eprintln!("unknown command: {}", other);
            None
        }
    }
}

/// Parses `setoption name <id> [value <x>]`.
fn parse_setoption(tokens: &[&str]) -> Option<Command> {
    // Minimum: setoption name <id>
    if tokens.len() < 3 || tokens[1] != "name" {
        eprintln!("malformed setoption: expected 'setoption name <id> [value <x>]'");
        return None;
    }

    // Find the "value" keyword to split name from value.
    // The name can be multi-word in theory (UCI allows it), but we keep it simple.
    let value_idx = tokens.iter().position(|&t| t == "value");

    let (name, value) = match value_idx {
        Some(vi) => {
            let name_parts = &tokens[2..vi];
            let value_parts = &tokens[vi + 1..];
            if name_parts.is_empty() {
                eprintln!("malformed setoption: empty name");
                return None;
            }
            let name = name_parts.join(" ");
            let value = if value_parts.is_empty() {
                None
            } else {
                Some(value_parts.join(" "))
            };
            (name, value)
        }
        None => {
            let name = tokens[2..].join(" ");
            (name, None)
        }
    };

    Some(Command::SetOption { name, value })
}

/// Parses `position <dfen>`.
fn parse_position(tokens: &[&str]) -> Option<Command> {
    if tokens.len() < 2 {
        eprintln!("malformed position: expected 'position <dfen>'");
        return None;
    }
    // DFEN is a single token (no spaces) following "position"
    let dfen = tokens[1].to_string();
    Some(Command::Position { dfen })
}

/// Parses `setpower <power>`.
fn parse_setpower(tokens: &[&str]) -> Option<Command> {
    if tokens.len() < 2 {
        eprintln!("malformed setpower: expected 'setpower <power>'");
        return None;
    }
    match Power::from_name(tokens[1]) {
        Some(power) => Some(Command::SetPower { power }),
        None => {
            eprintln!("unknown power: '{}'", tokens[1]);
            None
        }
    }
}

/// Parses `go [movetime <ms>] [depth <n>] [nodes <n>] [infinite]`.
fn parse_go(tokens: &[&str]) -> Option<Command> {
    let mut params = GoParams::default();
    let mut i = 1;

    while i < tokens.len() {
        match tokens[i] {
            "movetime" => {
                i += 1;
                if i < tokens.len() {
                    match tokens[i].parse::<u64>() {
                        Ok(v) => params.movetime = Some(v),
                        Err(_) => {
                            eprintln!("invalid movetime value: '{}'", tokens[i]);
                        }
                    }
                }
            }
            "depth" => {
                i += 1;
                if i < tokens.len() {
                    match tokens[i].parse::<u32>() {
                        Ok(v) => params.depth = Some(v),
                        Err(_) => {
                            eprintln!("invalid depth value: '{}'", tokens[i]);
                        }
                    }
                }
            }
            "nodes" => {
                i += 1;
                if i < tokens.len() {
                    match tokens[i].parse::<u64>() {
                        Ok(v) => params.nodes = Some(v),
                        Err(_) => {
                            eprintln!("invalid nodes value: '{}'", tokens[i]);
                        }
                    }
                }
            }
            "infinite" => {
                params.infinite = true;
            }
            other => {
                eprintln!("unknown go parameter: '{}'", other);
            }
        }
        i += 1;
    }

    Some(Command::Go(params))
}

/// Parses `press <structured_intent>` -- captures everything after "press" as raw text.
fn parse_press(tokens: &[&str], full_line: &str) -> Option<Command> {
    if tokens.len() < 2 {
        eprintln!("malformed press: expected 'press <structured_intent>'");
        return None;
    }
    // Capture everything after "press "
    let raw = full_line
        .trim()
        .strip_prefix("press")
        .unwrap_or("")
        .trim()
        .to_string();
    Some(Command::Press { raw })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_dui_command() {
        assert_eq!(parse_command("dui"), Some(Command::Dui));
    }

    #[test]
    fn parse_isready_command() {
        assert_eq!(parse_command("isready"), Some(Command::IsReady));
    }

    #[test]
    fn parse_quit_command() {
        assert_eq!(parse_command("quit"), Some(Command::Quit));
    }

    #[test]
    fn parse_newgame_command() {
        assert_eq!(parse_command("newgame"), Some(Command::NewGame));
    }

    #[test]
    fn parse_stop_command() {
        assert_eq!(parse_command("stop"), Some(Command::Stop));
    }

    #[test]
    fn parse_empty_line_returns_none() {
        assert_eq!(parse_command(""), None);
        assert_eq!(parse_command("  "), None);
        assert_eq!(parse_command("\t"), None);
    }

    #[test]
    fn parse_unknown_command_returns_none() {
        assert_eq!(parse_command("foobar"), None);
    }

    #[test]
    fn parse_setoption_with_value() {
        let cmd = parse_command("setoption name Threads value 8").unwrap();
        assert_eq!(
            cmd,
            Command::SetOption {
                name: "Threads".to_string(),
                value: Some("8".to_string()),
            }
        );
    }

    #[test]
    fn parse_setoption_string_value() {
        let cmd = parse_command("setoption name ModelPath value /opt/models/v2.onnx").unwrap();
        assert_eq!(
            cmd,
            Command::SetOption {
                name: "ModelPath".to_string(),
                value: Some("/opt/models/v2.onnx".to_string()),
            }
        );
    }

    #[test]
    fn parse_setoption_no_value() {
        let cmd = parse_command("setoption name ClearHash").unwrap();
        assert_eq!(
            cmd,
            Command::SetOption {
                name: "ClearHash".to_string(),
                value: None,
            }
        );
    }

    #[test]
    fn parse_setoption_malformed_returns_none() {
        assert_eq!(parse_command("setoption"), None);
        assert_eq!(parse_command("setoption foo"), None);
    }

    #[test]
    fn parse_position_dfen() {
        let dfen = "1901sm/Aavie,Aabud,Aftri/-/-";
        let cmd = parse_command(&format!("position {}", dfen)).unwrap();
        assert_eq!(
            cmd,
            Command::Position {
                dfen: dfen.to_string(),
            }
        );
    }

    #[test]
    fn parse_position_malformed_returns_none() {
        assert_eq!(parse_command("position"), None);
    }

    #[test]
    fn parse_setpower_all_powers() {
        for (name, power) in [
            ("austria", Power::Austria),
            ("england", Power::England),
            ("france", Power::France),
            ("germany", Power::Germany),
            ("italy", Power::Italy),
            ("russia", Power::Russia),
            ("turkey", Power::Turkey),
        ] {
            let cmd = parse_command(&format!("setpower {}", name)).unwrap();
            assert_eq!(cmd, Command::SetPower { power });
        }
    }

    #[test]
    fn parse_setpower_unknown_returns_none() {
        assert_eq!(parse_command("setpower narnia"), None);
        assert_eq!(parse_command("setpower"), None);
    }

    #[test]
    fn parse_go_no_params() {
        let cmd = parse_command("go").unwrap();
        assert_eq!(cmd, Command::Go(GoParams::default()));
    }

    #[test]
    fn parse_go_movetime() {
        let cmd = parse_command("go movetime 5000").unwrap();
        assert_eq!(
            cmd,
            Command::Go(GoParams {
                movetime: Some(5000),
                ..GoParams::default()
            })
        );
    }

    #[test]
    fn parse_go_depth() {
        let cmd = parse_command("go depth 3").unwrap();
        assert_eq!(
            cmd,
            Command::Go(GoParams {
                depth: Some(3),
                ..GoParams::default()
            })
        );
    }

    #[test]
    fn parse_go_nodes() {
        let cmd = parse_command("go nodes 100000").unwrap();
        assert_eq!(
            cmd,
            Command::Go(GoParams {
                nodes: Some(100000),
                ..GoParams::default()
            })
        );
    }

    #[test]
    fn parse_go_infinite() {
        let cmd = parse_command("go infinite").unwrap();
        assert_eq!(
            cmd,
            Command::Go(GoParams {
                infinite: true,
                ..GoParams::default()
            })
        );
    }

    #[test]
    fn parse_go_combined_params() {
        let cmd = parse_command("go movetime 5000 depth 3 nodes 100000").unwrap();
        assert_eq!(
            cmd,
            Command::Go(GoParams {
                movetime: Some(5000),
                depth: Some(3),
                nodes: Some(100000),
                infinite: false,
            })
        );
    }

    #[test]
    fn parse_press_command() {
        let cmd = parse_command("press france propose_alliance against germany").unwrap();
        assert_eq!(
            cmd,
            Command::Press {
                raw: "france propose_alliance against germany".to_string(),
            }
        );
    }

    #[test]
    fn parse_press_malformed_returns_none() {
        assert_eq!(parse_command("press"), None);
    }

    #[test]
    fn parse_with_leading_trailing_whitespace() {
        assert_eq!(parse_command("  dui  "), Some(Command::Dui));
        assert_eq!(parse_command("  isready  "), Some(Command::IsReady));
    }
}
