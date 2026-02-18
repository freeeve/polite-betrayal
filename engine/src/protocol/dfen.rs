//! DFEN (Diplomacy FEN) encoding and decoding.
//!
//! DFEN is a compact string notation for representing a full Diplomacy board
//! position, inspired by chess FEN. It encodes unit positions, ownership,
//! supply-center control, phase, and season in a single line.
//!
//! Format: `<phase_info>/<units>/<supply_centers>/<dislodged>`
//!
//! See DUI_PROTOCOL.md section 2 for the full specification.

use crate::board::province::{Coast, Power, Province, ALL_POWERS, ALL_PROVINCES};
use crate::board::state::{BoardState, DislodgedUnit, Phase, Season};
use crate::board::unit::UnitType;

/// Errors that can occur during DFEN parsing.
#[derive(Debug, thiserror::Error)]
pub enum DfenError {
    #[error("expected 4 sections separated by '/', got {0}")]
    WrongSectionCount(usize),

    #[error("invalid year in phase info: '{0}'")]
    InvalidYear(String),

    #[error("invalid season character: '{0}'")]
    InvalidSeason(char),

    #[error("invalid phase character: '{0}'")]
    InvalidPhase(char),

    #[error("invalid power character: '{0}'")]
    InvalidPower(char),

    #[error("invalid unit type character: '{0}'")]
    InvalidUnitType(char),

    #[error("unknown province abbreviation: '{0}'")]
    UnknownProvince(String),

    #[error("invalid coast abbreviation: '{0}'")]
    InvalidCoast(String),

    #[error("duplicate unit at province '{0}'")]
    DuplicateUnit(String),

    #[error("duplicate SC entry for province '{0}'")]
    DuplicateSc(String),

    #[error("duplicate dislodged unit at province '{0}'")]
    DuplicateDislodged(String),

    #[error("invalid unit entry: '{0}'")]
    InvalidUnitEntry(String),

    #[error("invalid SC entry: '{0}'")]
    InvalidScEntry(String),

    #[error("invalid dislodged entry: '{0}'")]
    InvalidDislodgedEntry(String),

    #[error("phase info too short: '{0}'")]
    PhaseInfoTooShort(String),
}

/// Parses a power character, including 'N' for neutral (returns None).
fn parse_power_or_neutral(c: char) -> Result<Option<Power>, DfenError> {
    if c == 'N' {
        return Ok(None);
    }
    Power::from_dui_char(c)
        .map(Some)
        .ok_or(DfenError::InvalidPower(c))
}

/// Parses a power character (does not accept 'N').
fn parse_power(c: char) -> Result<Power, DfenError> {
    Power::from_dui_char(c).ok_or(DfenError::InvalidPower(c))
}

/// Parses a location string like "vie", "stp.sc", "bul.ec".
/// Returns (Province, Coast).
fn parse_location(s: &str) -> Result<(Province, Coast), DfenError> {
    let (prov_str, coast) = if let Some(dot_pos) = s.find('.') {
        let prov_part = &s[..dot_pos];
        let coast_part = &s[dot_pos + 1..];
        let coast = Coast::from_abbr(coast_part)
            .ok_or_else(|| DfenError::InvalidCoast(coast_part.to_string()))?;
        if coast == Coast::None {
            return Err(DfenError::InvalidCoast(coast_part.to_string()));
        }
        (prov_part, coast)
    } else {
        (s, Coast::None)
    };

    let province = Province::from_abbr(prov_str)
        .ok_or_else(|| DfenError::UnknownProvince(prov_str.to_string()))?;

    Ok((province, coast))
}

/// Parses the phase info section (e.g., "1901sm").
fn parse_phase_info(s: &str) -> Result<(u16, Season, Phase), DfenError> {
    if s.len() < 3 {
        return Err(DfenError::PhaseInfoTooShort(s.to_string()));
    }

    let phase_char = s.as_bytes()[s.len() - 1] as char;
    let season_char = s.as_bytes()[s.len() - 2] as char;
    let year_str = &s[..s.len() - 2];

    let year: u16 = year_str
        .parse()
        .map_err(|_| DfenError::InvalidYear(year_str.to_string()))?;

    let season = Season::from_dfen_char(season_char)
        .ok_or(DfenError::InvalidSeason(season_char))?;

    let phase = Phase::from_dfen_char(phase_char)
        .ok_or(DfenError::InvalidPhase(phase_char))?;

    Ok((year, season, phase))
}

/// Parses the units section (comma-separated entries or "-").
fn parse_units(s: &str, state: &mut BoardState) -> Result<(), DfenError> {
    if s == "-" {
        return Ok(());
    }

    for entry in s.split(',') {
        if entry.len() < 4 {
            return Err(DfenError::InvalidUnitEntry(entry.to_string()));
        }

        let mut chars = entry.chars();
        let power_char = chars.next().unwrap();
        let unit_char = chars.next().unwrap();
        let location_str: String = chars.collect();

        let power = parse_power(power_char)?;
        let unit_type = UnitType::from_dui_char(unit_char)
            .ok_or(DfenError::InvalidUnitType(unit_char))?;
        let (province, coast) = parse_location(&location_str)?;

        let idx = province as usize;
        if state.units[idx].is_some() {
            return Err(DfenError::DuplicateUnit(province.abbr().to_string()));
        }

        state.units[idx] = Some((power, unit_type));
        if coast != Coast::None {
            state.fleet_coast[idx] = Some(coast);
        }
    }

    Ok(())
}

/// Parses the supply centers section (comma-separated entries, all 34 listed).
fn parse_supply_centers(s: &str, state: &mut BoardState) -> Result<(), DfenError> {
    for entry in s.split(',') {
        if entry.len() < 4 {
            return Err(DfenError::InvalidScEntry(entry.to_string()));
        }

        let mut chars = entry.chars();
        let power_char = chars.next().unwrap();
        let prov_str: String = chars.collect();

        let owner = parse_power_or_neutral(power_char)?;
        let province = Province::from_abbr(&prov_str)
            .ok_or_else(|| DfenError::UnknownProvince(prov_str.to_string()))?;

        let idx = province as usize;
        if state.sc_owner[idx].is_some() {
            return Err(DfenError::DuplicateSc(province.abbr().to_string()));
        }

        // Only set owner for non-neutral; None means neutral SC
        state.sc_owner[idx] = owner;
    }

    Ok(())
}

/// Parses the dislodged units section (comma-separated entries or "-").
fn parse_dislodged(s: &str, state: &mut BoardState) -> Result<(), DfenError> {
    if s == "-" {
        return Ok(());
    }

    for entry in s.split(',') {
        let parts: Vec<&str> = entry.split('<').collect();
        if parts.len() != 2 {
            return Err(DfenError::InvalidDislodgedEntry(entry.to_string()));
        }

        let unit_part = parts[0];
        let attacker_prov_str = parts[1];

        if unit_part.len() < 4 {
            return Err(DfenError::InvalidDislodgedEntry(entry.to_string()));
        }

        let mut chars = unit_part.chars();
        let power_char = chars.next().unwrap();
        let unit_char = chars.next().unwrap();
        let location_str: String = chars.collect();

        let power = parse_power(power_char)?;
        let unit_type = UnitType::from_dui_char(unit_char)
            .ok_or(DfenError::InvalidUnitType(unit_char))?;
        let (province, coast) = parse_location(&location_str)?;
        let attacker_from = Province::from_abbr(attacker_prov_str)
            .ok_or_else(|| DfenError::UnknownProvince(attacker_prov_str.to_string()))?;

        let idx = province as usize;
        if state.dislodged[idx].is_some() {
            return Err(DfenError::DuplicateDislodged(province.abbr().to_string()));
        }

        state.dislodged[idx] = Some(DislodgedUnit {
            power,
            unit_type,
            coast,
            attacker_from,
        });
    }

    Ok(())
}

/// Parses a DFEN string into a BoardState.
///
/// Format: `<phase_info>/<units>/<supply_centers>/<dislodged>`
pub fn parse_dfen(s: &str) -> Result<BoardState, DfenError> {
    let sections: Vec<&str> = s.split('/').collect();
    if sections.len() != 4 {
        return Err(DfenError::WrongSectionCount(sections.len()));
    }

    let (year, season, phase) = parse_phase_info(sections[0])?;
    let mut state = BoardState::empty(year, season, phase);

    parse_units(sections[1], &mut state)?;
    parse_supply_centers(sections[2], &mut state)?;
    parse_dislodged(sections[3], &mut state)?;

    Ok(state)
}

/// Encodes a location (province + optional coast) for DFEN output.
fn encode_location(province: Province, coast: Coast) -> String {
    let abbr = province.abbr();
    if coast != Coast::None {
        format!("{}.{}", abbr, coast.abbr())
    } else {
        abbr.to_string()
    }
}

/// Encodes a BoardState into a canonical DFEN string.
///
/// The output is deterministic: units and dislodged entries are grouped by power
/// (A, E, F, G, I, R, T) and sorted by province enum index within each group.
/// Supply centers follow the same power ordering plus neutral (N) at the end,
/// sorted alphabetically by province abbreviation within each group.
pub fn encode_dfen(state: &BoardState) -> String {
    let mut result = String::with_capacity(512);

    // Phase info
    result.push_str(&format!(
        "{}{}{}",
        state.year,
        state.season.dfen_char(),
        state.phase.dfen_char()
    ));

    result.push('/');

    // Units section
    let unit_str = encode_units(state);
    result.push_str(&unit_str);

    result.push('/');

    // Supply centers section
    let sc_str = encode_supply_centers(state);
    result.push_str(&sc_str);

    result.push('/');

    // Dislodged section
    let dis_str = encode_dislodged(state);
    result.push_str(&dis_str);

    result
}

/// Encodes the units section of the DFEN string.
///
/// Units are grouped by power in standard order (A, E, F, G, I, R, T),
/// and within each power, sorted by province enum index (which is alphabetical
/// by abbreviation).
fn encode_units(state: &BoardState) -> String {
    let mut entries: Vec<String> = Vec::new();

    for power in ALL_POWERS.iter() {
        // ALL_PROVINCES is already in alphabetical/index order
        for &prov in ALL_PROVINCES.iter() {
            let idx = prov as usize;
            if let Some((p, ut)) = state.units[idx] {
                if p == *power {
                    let coast = state.fleet_coast[idx].unwrap_or(Coast::None);
                    let loc = encode_location(prov, coast);
                    entries.push(format!("{}{}{}", power.dui_char(), ut.dui_char(), loc));
                }
            }
        }
    }

    if entries.is_empty() {
        "-".to_string()
    } else {
        entries.join(",")
    }
}

/// Encodes the supply centers section of the DFEN string.
///
/// SCs are grouped by power in standard order (A, E, F, G, I, R, T, N),
/// and within each group sorted alphabetically by province abbreviation.
fn encode_supply_centers(state: &BoardState) -> String {
    let mut entries: Vec<String> = Vec::new();

    // Owned SCs grouped by power in standard order
    for power in ALL_POWERS.iter() {
        // ALL_PROVINCES is already alphabetical
        for &prov in ALL_PROVINCES.iter() {
            if prov.is_supply_center() {
                if let Some(owner) = state.sc_owner[prov as usize] {
                    if owner == *power {
                        entries.push(format!("{}{}", power.dui_char(), prov.abbr()));
                    }
                }
            }
        }
    }

    // Neutral SCs (owner is None and province is a supply center)
    for &prov in ALL_PROVINCES.iter() {
        if prov.is_supply_center() && state.sc_owner[prov as usize].is_none() {
            entries.push(format!("N{}", prov.abbr()));
        }
    }

    entries.join(",")
}

/// Encodes the dislodged units section of the DFEN string.
///
/// Dislodged units are grouped by power in standard order (A, E, F, G, I, R, T),
/// and within each power, sorted by province enum index.
fn encode_dislodged(state: &BoardState) -> String {
    let mut entries: Vec<String> = Vec::new();

    for power in ALL_POWERS.iter() {
        for &prov in ALL_PROVINCES.iter() {
            if let Some(ref d) = state.dislodged[prov as usize] {
                if d.power == *power {
                    let loc = encode_location(prov, d.coast);
                    entries.push(format!(
                        "{}{}{}<{}",
                        d.power.dui_char(),
                        d.unit_type.dui_char(),
                        loc,
                        d.attacker_from.abbr()
                    ));
                }
            }
        }
    }

    if entries.is_empty() {
        "-".to_string()
    } else {
        entries.join(",")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::province::PROVINCE_COUNT;

    /// The initial position DFEN from the spec (section 7.1).
    const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

    /// Mid-game position DFEN from the spec (section 7.2).
    const MID_GAME_DFEN: &str = "1903fm/Aabud,Aarum,Afgre,Aavie,Efnth,Efnwy,Eayor,Eflon,Ffmao,Fabur,Famar,Ffpor,Gaden,Gahol,Gamun,Gfkie,Gfska,Iftys,Iaven,Iarom,Rfsev,Ramos,Rawar,Tfank,Tabul,Tacon,Tasmy/Abud,Agre,Arum,Atri,Avie,Eedi,Elon,Elvp,Enwy,Fbre,Fmar,Fpar,Fspa,Gber,Gden,Ghol,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rwar,Tank,Tbul,Tcon,Tsmy,Nbel,Npor,Nser,Nstp,Nswe,Ntun/-";

    /// Retreat phase DFEN from the spec (section 7.3).
    const RETREAT_DFEN: &str = "1902fr/Aabud,Aavie,Aftri,Aagre,Efnth,Efnwy,Eabel,Eflon,Ffmao,Fabur,Fapar,Ffbre,Gaden,Gamun,Gfkie,Gaber,Ifnap,Iaven,Iarom,Ramos,Rawar,Ragal,Rfstp.sc,Tabul,Tfbla,Tacon,Tasmy,Tfank/Abud,Agre,Atri,Avie,Ebel,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gden,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tbul,Tcon,Tsmy,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/Aaser<bul,Rfsev<bla";

    #[test]
    fn parse_initial_position() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse initial DFEN");

        assert_eq!(state.year, 1901);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Movement);

        // Austrian units
        assert_eq!(state.units[Province::Vie as usize], Some((Power::Austria, UnitType::Army)));
        assert_eq!(state.units[Province::Bud as usize], Some((Power::Austria, UnitType::Army)));
        assert_eq!(state.units[Province::Tri as usize], Some((Power::Austria, UnitType::Fleet)));

        // English units
        assert_eq!(state.units[Province::Lon as usize], Some((Power::England, UnitType::Fleet)));
        assert_eq!(state.units[Province::Edi as usize], Some((Power::England, UnitType::Fleet)));
        assert_eq!(state.units[Province::Lvp as usize], Some((Power::England, UnitType::Army)));

        // Russian fleet on south coast of StP
        assert_eq!(state.units[Province::Stp as usize], Some((Power::Russia, UnitType::Fleet)));
        assert_eq!(state.fleet_coast[Province::Stp as usize], Some(Coast::South));

        // SC ownership
        assert_eq!(state.sc_owner[Province::Vie as usize], Some(Power::Austria));
        assert_eq!(state.sc_owner[Province::Lon as usize], Some(Power::England));
        assert_eq!(state.sc_owner[Province::Mos as usize], Some(Power::Russia));

        // Neutral SCs
        assert_eq!(state.sc_owner[Province::Bel as usize], None);
        assert_eq!(state.sc_owner[Province::Bul as usize], None);

        // No dislodged units
        assert!(state.dislodged.iter().all(|d| d.is_none()));

        // 22 units total
        let unit_count = state.units.iter().filter(|u| u.is_some()).count();
        assert_eq!(unit_count, 22);
    }

    #[test]
    fn parse_mid_game_position() {
        let state = parse_dfen(MID_GAME_DFEN).expect("failed to parse mid-game DFEN");

        assert_eq!(state.year, 1903);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Movement);

        // Austria expanded: A bud, A rum, F gre, A vie
        assert_eq!(state.units[Province::Bud as usize], Some((Power::Austria, UnitType::Army)));
        assert_eq!(state.units[Province::Rum as usize], Some((Power::Austria, UnitType::Army)));
        assert_eq!(state.units[Province::Gre as usize], Some((Power::Austria, UnitType::Fleet)));
        assert_eq!(state.units[Province::Vie as usize], Some((Power::Austria, UnitType::Army)));

        // Germany has 5 units
        let german_count = state.units.iter()
            .filter(|u| matches!(u, Some((Power::Germany, _))))
            .count();
        assert_eq!(german_count, 5);

        // Total: 27
        let unit_count = state.units.iter().filter(|u| u.is_some()).count();
        assert_eq!(unit_count, 27);

        // Austria has 5 SCs
        let austria_scs = (0..PROVINCE_COUNT)
            .filter(|&i| state.sc_owner[i] == Some(Power::Austria))
            .count();
        assert_eq!(austria_scs, 5);

        assert!(state.dislodged.iter().all(|d| d.is_none()));
    }

    #[test]
    fn parse_retreat_phase_position() {
        let state = parse_dfen(RETREAT_DFEN).expect("failed to parse retreat DFEN");

        assert_eq!(state.year, 1902);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Retreat);

        // 28 units in main section
        let unit_count = state.units.iter().filter(|u| u.is_some()).count();
        assert_eq!(unit_count, 28);

        // 2 dislodged units
        let dislodged_count = state.dislodged.iter().filter(|d| d.is_some()).count();
        assert_eq!(dislodged_count, 2);

        // Austrian army at Serbia dislodged from Bulgaria
        let d_ser = state.dislodged[Province::Ser as usize].unwrap();
        assert_eq!(d_ser.power, Power::Austria);
        assert_eq!(d_ser.unit_type, UnitType::Army);
        assert_eq!(d_ser.attacker_from, Province::Bul);

        // Russian fleet at Sevastopol dislodged from Black Sea
        let d_sev = state.dislodged[Province::Sev as usize].unwrap();
        assert_eq!(d_sev.power, Power::Russia);
        assert_eq!(d_sev.unit_type, UnitType::Fleet);
        assert_eq!(d_sev.attacker_from, Province::Bla);
    }

    #[test]
    fn roundtrip_canonical_form() {
        // Parse from spec, encode to canonical form, then verify re-parsing
        // produces identical state. The encoder uses its own canonical ordering
        // which may differ from the spec examples' ordering.
        for dfen in [INITIAL_DFEN, MID_GAME_DFEN, RETREAT_DFEN] {
            let state1 = parse_dfen(dfen).expect("failed to parse");
            let encoded = encode_dfen(&state1);
            let state2 = parse_dfen(&encoded).expect("failed to reparse");
            assert_eq!(state1, state2, "State mismatch after roundtrip for: {}", dfen);

            // Encoding the re-parsed state should produce identical output
            let re_encoded = encode_dfen(&state2);
            assert_eq!(encoded, re_encoded, "Canonical form not stable");
        }
    }

    #[test]
    fn encode_initial_position_structure() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        let encoded = encode_dfen(&state);

        // Verify structure: 4 sections separated by /
        let sections: Vec<&str> = encoded.split('/').collect();
        assert_eq!(sections.len(), 4);

        // Phase info
        assert_eq!(sections[0], "1901sm");

        // Units section has 22 entries
        let unit_entries: Vec<&str> = sections[1].split(',').collect();
        assert_eq!(unit_entries.len(), 22);

        // SC section has 34 entries
        let sc_entries: Vec<&str> = sections[2].split(',').collect();
        assert_eq!(sc_entries.len(), 34);

        // Dislodged section is "-"
        assert_eq!(sections[3], "-");
    }

    #[test]
    fn encode_retreat_position_structure() {
        let state = parse_dfen(RETREAT_DFEN).expect("failed to parse");
        let encoded = encode_dfen(&state);

        let sections: Vec<&str> = encoded.split('/').collect();
        assert_eq!(sections.len(), 4);
        assert_eq!(sections[0], "1902fr");

        // Units section has 28 entries
        let unit_entries: Vec<&str> = sections[1].split(',').collect();
        assert_eq!(unit_entries.len(), 28);

        // Dislodged has 2 entries
        let dis_entries: Vec<&str> = sections[3].split(',').collect();
        assert_eq!(dis_entries.len(), 2);

        // Dislodged entries should contain the expected units
        assert!(encoded.contains("Aaser<bul"));
        assert!(encoded.contains("Rfsev<bla"));
    }

    #[test]
    fn all_seven_powers_present_in_initial() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");

        for power in ALL_POWERS.iter() {
            let has_unit = state.units.iter().any(|u| matches!(u, Some((p, _)) if *p == *power));
            assert!(has_unit, "Power {:?} should have units in initial position", power);
        }
    }

    #[test]
    fn both_unit_types_present() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");

        let has_army = state.units.iter().any(|u| matches!(u, Some((_, UnitType::Army))));
        let has_fleet = state.units.iter().any(|u| matches!(u, Some((_, UnitType::Fleet))));
        assert!(has_army);
        assert!(has_fleet);
    }

    #[test]
    fn all_coast_types_covered() {
        // South coast: stp.sc in initial position
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.fleet_coast[Province::Stp as usize], Some(Coast::South));

        // North coast
        let nc_dfen = "1901sm/Rfstp.nc/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";
        let state_nc = parse_dfen(nc_dfen).expect("failed to parse NC");
        assert_eq!(state_nc.fleet_coast[Province::Stp as usize], Some(Coast::North));

        // East coast
        let ec_dfen = "1901sm/Tfbul.ec/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";
        let state_ec = parse_dfen(ec_dfen).expect("failed to parse EC");
        assert_eq!(state_ec.fleet_coast[Province::Bul as usize], Some(Coast::East));
    }

    #[test]
    fn empty_units_dash() {
        let dfen = "1901sm/-/Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";
        let state = parse_dfen(dfen).expect("failed to parse");
        let unit_count = state.units.iter().filter(|u| u.is_some()).count();
        assert_eq!(unit_count, 0);
    }

    #[test]
    fn empty_dislodged_dash() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        let dislodged_count = state.dislodged.iter().filter(|d| d.is_none()).count();
        assert_eq!(dislodged_count, PROVINCE_COUNT);
    }

    #[test]
    fn multiple_dislodged_units() {
        let state = parse_dfen(RETREAT_DFEN).expect("failed to parse");
        let dislodged_count = state.dislodged.iter().filter(|d| d.is_some()).count();
        assert_eq!(dislodged_count, 2);
    }

    #[test]
    fn error_wrong_section_count_too_few() {
        let err = parse_dfen("1901sm/units/scs").unwrap_err();
        assert!(matches!(err, DfenError::WrongSectionCount(3)));
    }

    #[test]
    fn error_wrong_section_count_too_many() {
        let err = parse_dfen("1901sm/a/b/c/d").unwrap_err();
        assert!(matches!(err, DfenError::WrongSectionCount(5)));
    }

    #[test]
    fn error_invalid_season() {
        let err = parse_dfen("1901xm/-/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidSeason('x')));
    }

    #[test]
    fn error_invalid_phase() {
        let err = parse_dfen("1901sx/-/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidPhase('x')));
    }

    #[test]
    fn error_invalid_year() {
        let err = parse_dfen("abcdsm/-/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidYear(_)));
    }

    #[test]
    fn error_invalid_power() {
        let err = parse_dfen("1901sm/Xavie/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidPower('X')));
    }

    #[test]
    fn error_invalid_unit_type() {
        let err = parse_dfen("1901sm/Axvie/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidUnitType('x')));
    }

    #[test]
    fn error_unknown_province() {
        let err = parse_dfen("1901sm/Aaxyz/-/-").unwrap_err();
        assert!(matches!(err, DfenError::UnknownProvince(_)));
    }

    #[test]
    fn error_invalid_coast() {
        let err = parse_dfen("1901sm/Afstp.xx/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidCoast(_)));
    }

    #[test]
    fn error_duplicate_unit() {
        let err = parse_dfen("1901sm/Aavie,Gavie/-/-").unwrap_err();
        assert!(matches!(err, DfenError::DuplicateUnit(_)));
    }

    #[test]
    fn error_invalid_dislodged_no_attacker() {
        // Dislodged entry missing the '<' separator
        let err = parse_dfen("1901sr/-/Avie/Aavie").unwrap_err();
        assert!(matches!(err, DfenError::InvalidDislodgedEntry(_)));
    }

    #[test]
    fn error_phase_info_too_short() {
        let err = parse_dfen("sm/-/-/-").unwrap_err();
        assert!(matches!(err, DfenError::PhaseInfoTooShort(_)));
    }

    #[test]
    fn encode_empty_board() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let encoded = encode_dfen(&state);
        // No SCs assigned: all 34 SCs listed as neutral
        assert!(encoded.starts_with("1901sm/-/"));

        // Verify re-parsing produces same state
        let reparsed = parse_dfen(&encoded).expect("failed to reparse empty board");
        assert_eq!(reparsed.year, 1901);
        assert!(reparsed.units.iter().all(|u| u.is_none()));
    }

    #[test]
    fn supply_center_count_in_initial() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");

        let count_for = |power: Power| -> usize {
            (0..PROVINCE_COUNT)
                .filter(|&i| state.sc_owner[i] == Some(power))
                .count()
        };
        assert_eq!(count_for(Power::Austria), 3);
        assert_eq!(count_for(Power::England), 3);
        assert_eq!(count_for(Power::France), 3);
        assert_eq!(count_for(Power::Germany), 3);
        assert_eq!(count_for(Power::Italy), 3);
        assert_eq!(count_for(Power::Russia), 4);
        assert_eq!(count_for(Power::Turkey), 3);

        let neutral_count = ALL_PROVINCES.iter()
            .filter(|p| p.is_supply_center() && state.sc_owner[**p as usize].is_none())
            .count();
        assert_eq!(neutral_count, 12);
    }

    #[test]
    fn build_phase_parses() {
        let build_dfen = "1901fb/Aatri,Aarum,Afgre,Eflon,Efnth,Ealvp,Ffbre,Fapar,Faspa,Gfkie,Gaden,Gasil,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Raukr,Rawar,Rfsev,Tfank,Tacon,Tabul/Abud,Atri,Avie,Arum,Agre,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Gden,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nhol,Nnwy,Npor,Nser,Nspa,Nswe,Ntun/-";
        let state = parse_dfen(build_dfen).expect("failed to parse build phase");
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Build);
    }

    #[test]
    fn dislodged_fleet_with_coast() {
        // A fleet at stp.sc dislodged from bot
        let dfen = "1902fr/Ramos/Rmos,Rsev,Rstp,Rwar,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/Rfstp.sc<bot";
        let state = parse_dfen(dfen).expect("failed to parse");
        let d = state.dislodged[Province::Stp as usize].unwrap();
        assert_eq!(d.power, Power::Russia);
        assert_eq!(d.unit_type, UnitType::Fleet);
        assert_eq!(d.coast, Coast::South);
        assert_eq!(d.attacker_from, Province::Bot);

        // Round-trip the dislodged section
        let encoded = encode_dfen(&state);
        let reparsed = parse_dfen(&encoded).expect("failed to reparse");
        assert_eq!(reparsed.dislodged[Province::Stp as usize], state.dislodged[Province::Stp as usize]);
    }

    #[test]
    fn encode_deterministic_ordering() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        let e1 = encode_dfen(&state);
        let e2 = encode_dfen(&state);
        assert_eq!(e1, e2);
    }

    #[test]
    fn sc_entry_with_partial_list() {
        // Only a few SCs listed (non-standard but parseable)
        let dfen = "1901sm/-/Avie,Nbel/-";
        let state = parse_dfen(dfen).expect("failed to parse");
        assert_eq!(state.sc_owner[Province::Vie as usize], Some(Power::Austria));
        assert_eq!(state.sc_owner[Province::Bel as usize], None); // N = neutral = None
    }

    #[test]
    fn error_empty_string() {
        let err = parse_dfen("").unwrap_err();
        assert!(matches!(err, DfenError::WrongSectionCount(_)));
    }

    #[test]
    fn turkey_units_parse_correctly() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.units[Province::Ank as usize], Some((Power::Turkey, UnitType::Fleet)));
        assert_eq!(state.units[Province::Con as usize], Some((Power::Turkey, UnitType::Army)));
        assert_eq!(state.units[Province::Smy as usize], Some((Power::Turkey, UnitType::Army)));
    }

    #[test]
    fn france_units_parse_correctly() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.units[Province::Bre as usize], Some((Power::France, UnitType::Fleet)));
        assert_eq!(state.units[Province::Par as usize], Some((Power::France, UnitType::Army)));
        assert_eq!(state.units[Province::Mar as usize], Some((Power::France, UnitType::Army)));
    }

    #[test]
    fn germany_units_parse_correctly() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.units[Province::Kie as usize], Some((Power::Germany, UnitType::Fleet)));
        assert_eq!(state.units[Province::Ber as usize], Some((Power::Germany, UnitType::Army)));
        assert_eq!(state.units[Province::Mun as usize], Some((Power::Germany, UnitType::Army)));
    }

    #[test]
    fn italy_units_parse_correctly() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.units[Province::Nap as usize], Some((Power::Italy, UnitType::Fleet)));
        assert_eq!(state.units[Province::Rom as usize], Some((Power::Italy, UnitType::Army)));
        assert_eq!(state.units[Province::Ven as usize], Some((Power::Italy, UnitType::Army)));
    }

    #[test]
    fn russia_units_parse_correctly() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        assert_eq!(state.units[Province::Stp as usize], Some((Power::Russia, UnitType::Fleet)));
        assert_eq!(state.units[Province::Mos as usize], Some((Power::Russia, UnitType::Army)));
        assert_eq!(state.units[Province::War as usize], Some((Power::Russia, UnitType::Army)));
        assert_eq!(state.units[Province::Sev as usize], Some((Power::Russia, UnitType::Fleet)));
    }

    #[test]
    fn encode_units_grouped_by_power() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        let encoded = encode_dfen(&state);
        let sections: Vec<&str> = encoded.split('/').collect();
        let unit_str = sections[1];

        // Austrian units should come first, then English, etc.
        let first_a = unit_str.find('A').unwrap();
        let first_e = unit_str.find('E').unwrap();
        let first_f = unit_str.find('F').unwrap();
        let first_g = unit_str.find('G').unwrap();
        let first_i = unit_str.find('I').unwrap();
        let first_r = unit_str.find('R').unwrap();
        let first_t = unit_str.find('T').unwrap();

        assert!(first_a < first_e);
        assert!(first_e < first_f);
        assert!(first_f < first_g);
        assert!(first_g < first_i);
        assert!(first_i < first_r);
        assert!(first_r < first_t);
    }

    #[test]
    fn encode_scs_grouped_by_power() {
        let state = parse_dfen(INITIAL_DFEN).expect("failed to parse");
        let encoded = encode_dfen(&state);
        let sections: Vec<&str> = encoded.split('/').collect();
        let sc_str = sections[2];

        // Verify SC ordering matches the spec
        assert_eq!(
            sc_str,
            "Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun"
        );
    }

    #[test]
    fn error_invalid_sc_entry_too_short() {
        let err = parse_dfen("1901sm/-/AB/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidScEntry(_)));
    }

    #[test]
    fn error_invalid_unit_entry_too_short() {
        let err = parse_dfen("1901sm/Aa/-/-").unwrap_err();
        assert!(matches!(err, DfenError::InvalidUnitEntry(_)));
    }

    #[test]
    fn error_duplicate_sc() {
        let err = parse_dfen("1901sm/-/Avie,Gvie/-").unwrap_err();
        assert!(matches!(err, DfenError::DuplicateSc(_)));
    }

    #[test]
    fn error_duplicate_dislodged() {
        let err = parse_dfen("1901sr/-/Avie/Aavie<boh,Gavie<mun").unwrap_err();
        assert!(matches!(err, DfenError::DuplicateDislodged(_)));
    }

    #[test]
    fn fuzz_roundtrip_with_variations() {
        // Manually constructed DFEN strings with various features
        let cases = [
            // Minimal valid DFEN
            "1901sm/-/Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-",
            // Single army
            "2025sm/Aavie/Avie/-",
            // Single fleet with coast
            "1901sm/Rfstp.sc/Rstp/-",
            // Fleet on BUL east coast
            "1905fm/Tfbul.ec/Tbul/-",
            // Spain north coast
            "1903fm/Ffspa.nc/Fspa/-",
            // Multiple dislodged
            "1902fr/Tabul,Tacon/Tank,Tbul,Tcon,Tsmy/Aaser<bul,Rfsev<bla",
        ];

        for dfen in cases {
            let state = parse_dfen(dfen).expect(&format!("failed to parse: {}", dfen));
            let encoded = encode_dfen(&state);
            let state2 = parse_dfen(&encoded).expect(&format!("failed to reparse: {}", encoded));
            assert_eq!(state, state2, "Roundtrip mismatch for: {}", dfen);

            let re_encoded = encode_dfen(&state2);
            assert_eq!(encoded, re_encoded, "Canonical form unstable for: {}", dfen);
        }
    }

    #[test]
    fn large_year_values() {
        let dfen = "9999fm/-/Nbel/-";
        let state = parse_dfen(dfen).expect("failed to parse large year");
        assert_eq!(state.year, 9999);
        assert_eq!(state.season, Season::Fall);
    }

    #[test]
    fn all_powers_can_own_scs() {
        let dfen = "1901sm/-/Avie,Elon,Fpar,Gber,Irom,Rmos,Tank,Nbel/-";
        let state = parse_dfen(dfen).expect("failed to parse");
        assert_eq!(state.sc_owner[Province::Vie as usize], Some(Power::Austria));
        assert_eq!(state.sc_owner[Province::Lon as usize], Some(Power::England));
        assert_eq!(state.sc_owner[Province::Par as usize], Some(Power::France));
        assert_eq!(state.sc_owner[Province::Ber as usize], Some(Power::Germany));
        assert_eq!(state.sc_owner[Province::Rom as usize], Some(Power::Italy));
        assert_eq!(state.sc_owner[Province::Mos as usize], Some(Power::Russia));
        assert_eq!(state.sc_owner[Province::Ank as usize], Some(Power::Turkey));
        assert_eq!(state.sc_owner[Province::Bel as usize], None); // Neutral
    }
}
