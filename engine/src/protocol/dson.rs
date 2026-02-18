//! DSON (Diplomacy Standard Order Notation) encoding and decoding.
//!
//! DSON is a compact text notation for Diplomacy orders, used in the
//! `bestorders` response and `info` lines of the DUI protocol.
//! Coast separator is `/` (slash), province IDs are 3-letter lowercase,
//! and unit types are uppercase A/F.

use thiserror::Error;

use crate::board::order::{Location, Order, OrderUnit};
use crate::board::province::{Coast, Province};
use crate::board::unit::UnitType;

/// Errors that can occur when parsing DSON order strings.
#[derive(Debug, Error, PartialEq, Eq)]
pub enum DsonError {
    #[error("empty input")]
    EmptyInput,

    #[error("unknown unit type '{0}'")]
    UnknownUnitType(String),

    #[error("unknown province '{0}'")]
    UnknownProvince(String),

    #[error("unknown coast '{0}'")]
    UnknownCoast(String),

    #[error("unknown action '{0}'")]
    UnknownAction(String),

    #[error("unexpected end of input, expected {0}")]
    UnexpectedEnd(String),

    #[error("unexpected token '{found}', expected {expected}")]
    UnexpectedToken { expected: String, found: String },
}

/// Parses a single DSON order string into an `Order`.
///
/// Accepts canonical DSON forms like `A vie H`, `F nrg - stp/nc`, `W`, etc.
pub fn parse_order(s: &str) -> Result<Order, DsonError> {
    let s = s.trim();
    if s.is_empty() {
        return Err(DsonError::EmptyInput);
    }

    let tokens: Vec<&str> = s.split(' ').collect();
    if tokens.is_empty() {
        return Err(DsonError::EmptyInput);
    }

    // Waive is a special case: standalone "W"
    if tokens[0] == "W" {
        return Ok(Order::Waive);
    }

    // All other orders start with a unit: unit_char location
    let unit = parse_unit(&tokens, 0)?;
    let pos = 2; // consumed unit_char and location

    if pos >= tokens.len() {
        return Err(DsonError::UnexpectedEnd(
            "action (H, -, S, C, R, D, B)".to_string(),
        ));
    }

    match tokens[pos] {
        "H" => Ok(Order::Hold { unit }),

        "-" => {
            // Move: unit - location
            let dest = parse_location(&tokens, pos + 1)?;
            Ok(Order::Move { unit, dest })
        }

        "S" => {
            // Support: unit S supported_unit (H | - location)
            let supported = parse_unit(&tokens, pos + 1)?;
            let sup_pos = pos + 3; // past S, unit_char, location

            if sup_pos >= tokens.len() {
                return Err(DsonError::UnexpectedEnd(
                    "H or - after supported unit".to_string(),
                ));
            }

            match tokens[sup_pos] {
                "H" => Ok(Order::SupportHold { unit, supported }),
                "-" => {
                    let dest = parse_location(&tokens, sup_pos + 1)?;
                    Ok(Order::SupportMove {
                        unit,
                        supported,
                        dest,
                    })
                }
                other => Err(DsonError::UnexpectedToken {
                    expected: "H or -".to_string(),
                    found: other.to_string(),
                }),
            }
        }

        "C" => {
            // Convoy: unit C A from_location - to_location
            // Grammar says: convoy = "C" SP "A" SP location SP "-" SP location
            // The convoyed unit is always an Army
            if pos + 1 >= tokens.len() {
                return Err(DsonError::UnexpectedEnd("A (convoyed army)".to_string()));
            }
            if tokens[pos + 1] != "A" {
                return Err(DsonError::UnexpectedToken {
                    expected: "A (convoyed army)".to_string(),
                    found: tokens[pos + 1].to_string(),
                });
            }
            let from = parse_location(&tokens, pos + 2)?;

            let dash_pos = pos + 3;
            if dash_pos >= tokens.len() || tokens[dash_pos] != "-" {
                let found = if dash_pos >= tokens.len() {
                    return Err(DsonError::UnexpectedEnd("- (move arrow)".to_string()));
                } else {
                    tokens[dash_pos]
                };
                return Err(DsonError::UnexpectedToken {
                    expected: "-".to_string(),
                    found: found.to_string(),
                });
            }

            let to = parse_location(&tokens, dash_pos + 1)?;
            Ok(Order::Convoy {
                unit,
                convoyed_from: from,
                convoyed_to: to,
            })
        }

        "R" => {
            // Retreat: unit R location
            let dest = parse_location(&tokens, pos + 1)?;
            Ok(Order::Retreat { unit, dest })
        }

        "D" => Ok(Order::Disband { unit }),

        "B" => Ok(Order::Build { unit }),

        other => Err(DsonError::UnknownAction(other.to_string())),
    }
}

/// Parses a semicolon-separated list of DSON orders.
///
/// Orders are separated by ` ; ` (space-semicolon-space). A single order
/// without separators is valid.
pub fn parse_orders(s: &str) -> Result<Vec<Order>, DsonError> {
    let s = s.trim();
    if s.is_empty() {
        return Err(DsonError::EmptyInput);
    }

    s.split(" ; ")
        .map(|part| parse_order(part.trim()))
        .collect()
}

/// Formats a single `Order` as a canonical DSON string.
pub fn format_order(order: &Order) -> String {
    match order {
        Order::Hold { unit } => {
            format!("{} H", format_unit(unit))
        }
        Order::Move { unit, dest } => {
            format!("{} - {}", format_unit(unit), format_location(dest))
        }
        Order::SupportHold { unit, supported } => {
            format!("{} S {} H", format_unit(unit), format_unit(supported))
        }
        Order::SupportMove {
            unit,
            supported,
            dest,
        } => {
            format!(
                "{} S {} - {}",
                format_unit(unit),
                format_unit(supported),
                format_location(dest)
            )
        }
        Order::Convoy {
            unit,
            convoyed_from,
            convoyed_to,
        } => {
            format!(
                "{} C A {} - {}",
                format_unit(unit),
                format_location(convoyed_from),
                format_location(convoyed_to)
            )
        }
        Order::Retreat { unit, dest } => {
            format!("{} R {}", format_unit(unit), format_location(dest))
        }
        Order::Disband { unit } => {
            format!("{} D", format_unit(unit))
        }
        Order::Build { unit } => {
            format!("{} B", format_unit(unit))
        }
        Order::Waive => "W".to_string(),
    }
}

/// Formats a slice of orders as a ` ; `-separated DSON string.
pub fn format_orders(orders: &[Order]) -> String {
    orders
        .iter()
        .map(|o| format_order(o))
        .collect::<Vec<_>>()
        .join(" ; ")
}

/// Parses a unit (unit_char + location) from token slice at given index.
fn parse_unit(tokens: &[&str], idx: usize) -> Result<OrderUnit, DsonError> {
    if idx >= tokens.len() {
        return Err(DsonError::UnexpectedEnd("unit type (A or F)".to_string()));
    }

    let unit_type = match tokens[idx] {
        "A" => UnitType::Army,
        "F" => UnitType::Fleet,
        other => return Err(DsonError::UnknownUnitType(other.to_string())),
    };

    let location = parse_location(tokens, idx + 1)?;

    Ok(OrderUnit {
        unit_type,
        location,
    })
}

/// Parses a location (prov_id or prov_id/coast) from token slice at given index.
fn parse_location(tokens: &[&str], idx: usize) -> Result<Location, DsonError> {
    if idx >= tokens.len() {
        return Err(DsonError::UnexpectedEnd("province location".to_string()));
    }

    let token = tokens[idx];

    // Check for coast separator
    if let Some(slash_pos) = token.find('/') {
        let prov_str = &token[..slash_pos];
        let coast_str = &token[slash_pos + 1..];

        let province = Province::from_abbr(prov_str)
            .ok_or_else(|| DsonError::UnknownProvince(prov_str.to_string()))?;
        let coast = Coast::from_abbr(coast_str)
            .ok_or_else(|| DsonError::UnknownCoast(coast_str.to_string()))?;

        Ok(Location::with_coast(province, coast))
    } else {
        let province = Province::from_abbr(token)
            .ok_or_else(|| DsonError::UnknownProvince(token.to_string()))?;
        Ok(Location::new(province))
    }
}

/// Formats a unit reference as "A prov" or "F prov/coast".
fn format_unit(unit: &OrderUnit) -> String {
    format!(
        "{} {}",
        unit.unit_type.dson_char(),
        format_location(&unit.location)
    )
}

/// Formats a location as "prov" or "prov/coast".
fn format_location(loc: &Location) -> String {
    if loc.coast == Coast::None {
        loc.province.abbr().to_string()
    } else {
        format!("{}/{}", loc.province.abbr(), loc.coast.abbr())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // -- Helper constructors --

    fn loc(prov: Province) -> Location {
        Location::new(prov)
    }

    fn loc_coast(prov: Province, coast: Coast) -> Location {
        Location::with_coast(prov, coast)
    }

    fn army(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Army,
            location: loc(prov),
        }
    }

    fn fleet(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: loc(prov),
        }
    }

    fn fleet_coast(prov: Province, coast: Coast) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: loc_coast(prov, coast),
        }
    }

    // -- Movement phase parse tests --

    #[test]
    fn parse_hold() {
        let order = parse_order("A vie H").unwrap();
        assert_eq!(
            order,
            Order::Hold {
                unit: army(Province::Vie)
            }
        );
    }

    #[test]
    fn parse_move_army() {
        let order = parse_order("A bud - rum").unwrap();
        assert_eq!(
            order,
            Order::Move {
                unit: army(Province::Bud),
                dest: loc(Province::Rum)
            }
        );
    }

    #[test]
    fn parse_move_fleet() {
        let order = parse_order("F tri - adr").unwrap();
        assert_eq!(
            order,
            Order::Move {
                unit: fleet(Province::Tri),
                dest: loc(Province::Adr)
            }
        );
    }

    #[test]
    fn parse_move_to_coast() {
        let order = parse_order("F nrg - stp/nc").unwrap();
        assert_eq!(
            order,
            Order::Move {
                unit: fleet(Province::Nrg),
                dest: loc_coast(Province::Stp, Coast::North),
            }
        );
    }

    #[test]
    fn parse_support_hold() {
        let order = parse_order("A tyr S A vie H").unwrap();
        assert_eq!(
            order,
            Order::SupportHold {
                unit: army(Province::Tyr),
                supported: army(Province::Vie),
            }
        );
    }

    #[test]
    fn parse_support_move() {
        let order = parse_order("A gal S A bud - rum").unwrap();
        assert_eq!(
            order,
            Order::SupportMove {
                unit: army(Province::Gal),
                supported: army(Province::Bud),
                dest: loc(Province::Rum),
            }
        );
    }

    #[test]
    fn parse_support_move_fleet() {
        let order = parse_order("F adr S F tri - ven").unwrap();
        assert_eq!(
            order,
            Order::SupportMove {
                unit: fleet(Province::Adr),
                supported: fleet(Province::Tri),
                dest: loc(Province::Ven),
            }
        );
    }

    #[test]
    fn parse_convoy() {
        let order = parse_order("F mao C A bre - spa").unwrap();
        assert_eq!(
            order,
            Order::Convoy {
                unit: fleet(Province::Mao),
                convoyed_from: loc(Province::Bre),
                convoyed_to: loc(Province::Spa),
            }
        );
    }

    #[test]
    fn parse_convoy_multi_sea() {
        let order = parse_order("F nth C A lon - nwy").unwrap();
        assert_eq!(
            order,
            Order::Convoy {
                unit: fleet(Province::Nth),
                convoyed_from: loc(Province::Lon),
                convoyed_to: loc(Province::Nwy),
            }
        );
    }

    // -- Retreat phase parse tests --

    #[test]
    fn parse_retreat() {
        let order = parse_order("A vie R boh").unwrap();
        assert_eq!(
            order,
            Order::Retreat {
                unit: army(Province::Vie),
                dest: loc(Province::Boh)
            }
        );
    }

    #[test]
    fn parse_retreat_from_coast() {
        let order = parse_order("F stp/nc R nwy").unwrap();
        assert_eq!(
            order,
            Order::Retreat {
                unit: fleet_coast(Province::Stp, Coast::North),
                dest: loc(Province::Nwy),
            }
        );
    }

    #[test]
    fn parse_disband_retreat_phase() {
        let order = parse_order("F tri D").unwrap();
        assert_eq!(
            order,
            Order::Disband {
                unit: fleet(Province::Tri)
            }
        );
    }

    // -- Build phase parse tests --

    #[test]
    fn parse_build_army() {
        let order = parse_order("A vie B").unwrap();
        assert_eq!(
            order,
            Order::Build {
                unit: army(Province::Vie)
            }
        );
    }

    #[test]
    fn parse_build_fleet_coast() {
        let order = parse_order("F stp/sc B").unwrap();
        assert_eq!(
            order,
            Order::Build {
                unit: fleet_coast(Province::Stp, Coast::South),
            }
        );
    }

    #[test]
    fn parse_disband_build_phase() {
        let order = parse_order("A war D").unwrap();
        assert_eq!(
            order,
            Order::Disband {
                unit: army(Province::War)
            }
        );
    }

    #[test]
    fn parse_waive() {
        let order = parse_order("W").unwrap();
        assert_eq!(order, Order::Waive);
    }

    // -- Multi-order parse tests --

    #[test]
    fn parse_multi_orders() {
        let orders = parse_orders("A vie - tri ; A bud - ser ; F tri - alb").unwrap();
        assert_eq!(orders.len(), 3);
        assert_eq!(
            orders[0],
            Order::Move {
                unit: army(Province::Vie),
                dest: loc(Province::Tri)
            }
        );
        assert_eq!(
            orders[1],
            Order::Move {
                unit: army(Province::Bud),
                dest: loc(Province::Ser)
            }
        );
        assert_eq!(
            orders[2],
            Order::Move {
                unit: fleet(Province::Tri),
                dest: loc(Province::Alb)
            }
        );
    }

    #[test]
    fn parse_single_order_via_parse_orders() {
        let orders = parse_orders("A vie H").unwrap();
        assert_eq!(orders.len(), 1);
        assert_eq!(
            orders[0],
            Order::Hold {
                unit: army(Province::Vie)
            }
        );
    }

    #[test]
    fn parse_multi_retreat_orders() {
        let orders = parse_orders("A vie R boh ; F tri D").unwrap();
        assert_eq!(orders.len(), 2);
        assert_eq!(
            orders[0],
            Order::Retreat {
                unit: army(Province::Vie),
                dest: loc(Province::Boh)
            }
        );
        assert_eq!(
            orders[1],
            Order::Disband {
                unit: fleet(Province::Tri)
            }
        );
    }

    #[test]
    fn parse_multi_build_orders() {
        let orders = parse_orders("A vie B ; F stp/sc B").unwrap();
        assert_eq!(orders.len(), 2);
        assert_eq!(
            orders[0],
            Order::Build {
                unit: army(Province::Vie)
            }
        );
        assert_eq!(
            orders[1],
            Order::Build {
                unit: fleet_coast(Province::Stp, Coast::South),
            }
        );
    }

    #[test]
    fn parse_waive_in_multi() {
        let orders = parse_orders("W").unwrap();
        assert_eq!(orders.len(), 1);
        assert_eq!(orders[0], Order::Waive);
    }

    // -- Format tests --

    #[test]
    fn format_hold() {
        let s = format_order(&Order::Hold {
            unit: army(Province::Vie),
        });
        assert_eq!(s, "A vie H");
    }

    #[test]
    fn format_move() {
        let s = format_order(&Order::Move {
            unit: army(Province::Bud),
            dest: loc(Province::Rum),
        });
        assert_eq!(s, "A bud - rum");
    }

    #[test]
    fn format_move_to_coast() {
        let s = format_order(&Order::Move {
            unit: fleet(Province::Nrg),
            dest: loc_coast(Province::Stp, Coast::North),
        });
        assert_eq!(s, "F nrg - stp/nc");
    }

    #[test]
    fn format_support_hold() {
        let s = format_order(&Order::SupportHold {
            unit: army(Province::Tyr),
            supported: army(Province::Vie),
        });
        assert_eq!(s, "A tyr S A vie H");
    }

    #[test]
    fn format_support_move() {
        let s = format_order(&Order::SupportMove {
            unit: army(Province::Gal),
            supported: army(Province::Bud),
            dest: loc(Province::Rum),
        });
        assert_eq!(s, "A gal S A bud - rum");
    }

    #[test]
    fn format_convoy() {
        let s = format_order(&Order::Convoy {
            unit: fleet(Province::Mao),
            convoyed_from: loc(Province::Bre),
            convoyed_to: loc(Province::Spa),
        });
        assert_eq!(s, "F mao C A bre - spa");
    }

    #[test]
    fn format_retreat() {
        let s = format_order(&Order::Retreat {
            unit: army(Province::Vie),
            dest: loc(Province::Boh),
        });
        assert_eq!(s, "A vie R boh");
    }

    #[test]
    fn format_retreat_from_coast() {
        let s = format_order(&Order::Retreat {
            unit: fleet_coast(Province::Stp, Coast::North),
            dest: loc(Province::Nwy),
        });
        assert_eq!(s, "F stp/nc R nwy");
    }

    #[test]
    fn format_disband() {
        let s = format_order(&Order::Disband {
            unit: fleet(Province::Tri),
        });
        assert_eq!(s, "F tri D");
    }

    #[test]
    fn format_build_army() {
        let s = format_order(&Order::Build {
            unit: army(Province::Vie),
        });
        assert_eq!(s, "A vie B");
    }

    #[test]
    fn format_build_fleet_coast() {
        let s = format_order(&Order::Build {
            unit: fleet_coast(Province::Stp, Coast::South),
        });
        assert_eq!(s, "F stp/sc B");
    }

    #[test]
    fn format_waive() {
        let s = format_order(&Order::Waive);
        assert_eq!(s, "W");
    }

    #[test]
    fn format_multi_orders() {
        let orders = vec![
            Order::Move {
                unit: army(Province::Vie),
                dest: loc(Province::Tri),
            },
            Order::Move {
                unit: army(Province::Bud),
                dest: loc(Province::Ser),
            },
            Order::Move {
                unit: fleet(Province::Tri),
                dest: loc(Province::Alb),
            },
        ];
        let s = format_orders(&orders);
        assert_eq!(s, "A vie - tri ; A bud - ser ; F tri - alb");
    }

    // -- Round-trip tests --

    #[test]
    fn roundtrip_hold() {
        let input = "A vie H";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_move() {
        let input = "A bud - rum";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_move_coast() {
        let input = "F nrg - stp/nc";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_support_hold() {
        let input = "A tyr S A vie H";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_support_move() {
        let input = "A gal S A bud - rum";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_convoy() {
        let input = "F mao C A bre - spa";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_retreat() {
        let input = "A vie R boh";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_retreat_coast() {
        let input = "F stp/nc R nwy";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_disband() {
        let input = "F tri D";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_build_army() {
        let input = "A vie B";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_build_fleet_coast() {
        let input = "F stp/sc B";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_waive() {
        let input = "W";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_multi_orders() {
        let input = "A vie - tri ; A bud - ser ; F tri - alb";
        let orders = parse_orders(input).unwrap();
        assert_eq!(format_orders(&orders), input);
    }

    #[test]
    fn roundtrip_multi_build() {
        let input = "A vie B ; A bud B";
        let orders = parse_orders(input).unwrap();
        assert_eq!(format_orders(&orders), input);
    }

    #[test]
    fn roundtrip_mixed_retreat() {
        let input = "A vie R boh ; F tri D";
        let orders = parse_orders(input).unwrap();
        assert_eq!(format_orders(&orders), input);
    }

    // -- All three split-coast provinces --

    #[test]
    fn roundtrip_bul_ec() {
        let input = "F bul/ec - con";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_bul_sc() {
        let input = "F bul/sc - gre";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_spa_nc() {
        let input = "F spa/nc - mao";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_spa_sc() {
        let input = "F spa/sc - gol";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_stp_nc() {
        let input = "F stp/nc - bar";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    #[test]
    fn roundtrip_stp_sc() {
        let input = "F stp/sc - bot";
        assert_eq!(format_order(&parse_order(input).unwrap()), input);
    }

    // -- Error handling tests --

    #[test]
    fn error_empty_input() {
        assert_eq!(parse_order(""), Err(DsonError::EmptyInput));
        assert_eq!(parse_order("  "), Err(DsonError::EmptyInput));
    }

    #[test]
    fn error_empty_multi_input() {
        assert_eq!(parse_orders(""), Err(DsonError::EmptyInput));
    }

    #[test]
    fn error_unknown_unit_type() {
        let err = parse_order("X vie H").unwrap_err();
        assert_eq!(err, DsonError::UnknownUnitType("X".to_string()));
    }

    #[test]
    fn error_unknown_province() {
        let err = parse_order("A xyz H").unwrap_err();
        assert_eq!(err, DsonError::UnknownProvince("xyz".to_string()));
    }

    #[test]
    fn error_unknown_coast() {
        let err = parse_order("F stp/xx - bar").unwrap_err();
        assert_eq!(err, DsonError::UnknownCoast("xx".to_string()));
    }

    #[test]
    fn error_unknown_action() {
        let err = parse_order("A vie X").unwrap_err();
        assert_eq!(err, DsonError::UnknownAction("X".to_string()));
    }

    #[test]
    fn error_missing_action() {
        let err = parse_order("A vie").unwrap_err();
        assert!(matches!(err, DsonError::UnexpectedEnd(_)));
    }

    #[test]
    fn error_missing_move_dest() {
        let err = parse_order("A vie -").unwrap_err();
        assert!(matches!(err, DsonError::UnexpectedEnd(_)));
    }

    #[test]
    fn error_missing_support_action() {
        let err = parse_order("A gal S A bud").unwrap_err();
        assert!(matches!(err, DsonError::UnexpectedEnd(_)));
    }

    #[test]
    fn error_convoy_not_army() {
        let err = parse_order("F nth C F lon - nwy").unwrap_err();
        assert!(matches!(err, DsonError::UnexpectedToken { .. }));
    }

    #[test]
    fn error_convoy_missing_dash() {
        let err = parse_order("F nth C A lon").unwrap_err();
        assert!(matches!(err, DsonError::UnexpectedEnd(_)));
    }

    #[test]
    fn error_in_multi_order() {
        let err = parse_orders("A vie H ; X bud H").unwrap_err();
        assert_eq!(err, DsonError::UnknownUnitType("X".to_string()));
    }

    // -- Protocol spec examples --

    #[test]
    fn spec_movement_examples() {
        // From DUI_PROTOCOL.md section 3.1
        assert!(parse_order("A vie H").is_ok());
        assert!(parse_order("A bud - rum").is_ok());
        assert!(parse_order("F tri - adr").is_ok());
        assert!(parse_order("A gal S A bud - rum").is_ok());
        assert!(parse_order("A tyr S A vie H").is_ok());
        assert!(parse_order("F mao C A bre - spa").is_ok());
        assert!(parse_order("F nrg - stp/nc").is_ok());
    }

    #[test]
    fn spec_retreat_examples() {
        // From DUI_PROTOCOL.md section 3.2
        assert!(parse_order("A vie R boh").is_ok());
        assert!(parse_order("F tri D").is_ok());
        assert!(parse_order("F stp/nc R nwy").is_ok());
    }

    #[test]
    fn spec_build_examples() {
        // From DUI_PROTOCOL.md section 3.3
        assert!(parse_order("A vie B").is_ok());
        assert!(parse_order("F stp/sc B").is_ok());
        assert!(parse_order("A war D").is_ok());
        assert!(parse_order("W").is_ok());
    }

    #[test]
    fn spec_bestorders_example() {
        // From DUI_PROTOCOL.md section 5.1 session flow
        let orders = parse_orders("A vie - tri ; A bud - ser ; F tri - alb").unwrap();
        assert_eq!(orders.len(), 3);
        assert_eq!(
            format_orders(&orders),
            "A vie - tri ; A bud - ser ; F tri - alb"
        );
    }

    #[test]
    fn spec_bestorders_retreat_example() {
        let orders = parse_orders("A gal R ukr").unwrap();
        assert_eq!(orders.len(), 1);
        assert_eq!(format_orders(&orders), "A gal R ukr");
    }

    #[test]
    fn spec_bestorders_build_example() {
        let orders = parse_orders("A vie B ; A bud B").unwrap();
        assert_eq!(orders.len(), 2);
        assert_eq!(format_orders(&orders), "A vie B ; A bud B");
    }

    // -- Edge cases --

    #[test]
    fn leading_trailing_whitespace_ignored() {
        let order = parse_order("  A vie H  ").unwrap();
        assert_eq!(
            order,
            Order::Hold {
                unit: army(Province::Vie)
            }
        );
    }

    #[test]
    fn support_fleet_hold() {
        let input = "F adr S F tri H";
        let order = parse_order(input).unwrap();
        assert_eq!(
            order,
            Order::SupportHold {
                unit: fleet(Province::Adr),
                supported: fleet(Province::Tri),
            }
        );
        assert_eq!(format_order(&order), input);
    }

    #[test]
    fn format_empty_orders_slice() {
        assert_eq!(format_orders(&[]), "");
    }

    #[test]
    fn format_single_order_no_separator() {
        let orders = vec![Order::Waive];
        assert_eq!(format_orders(&orders), "W");
    }

    // -- France example from section 5.3 --
    #[test]
    fn spec_france_example() {
        let orders = parse_orders("F bre - mao ; A par - bur ; A mar S A par - bur").unwrap();
        assert_eq!(orders.len(), 3);
        assert_eq!(
            format_orders(&orders),
            "F bre - mao ; A par - bur ; A mar S A par - bur"
        );
    }
}
