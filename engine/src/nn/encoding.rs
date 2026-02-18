//! Board state -> tensor encoding for neural network inference.
//!
//! Produces an [81, 36] f32 tensor matching the Python feature extraction
//! format in `data/scripts/features.py`. The 81 "areas" are the 75 map
//! provinces (sorted alphabetically by abbreviation, matching Province enum
//! ordinals 0..74) followed by 6 bicoastal variants in order:
//!   75: bul/ec, 76: bul/sc, 77: spa/nc, 78: spa/sc, 79: stp/nc, 80: stp/sc
//!
//! Feature layout per area (36 channels):
//!   [0:3]   unit present: [army, fleet, empty]
//!   [3:11]  unit owner:   [A, E, F, G, I, R, T, none]
//!   [11:20] SC owner:     [A, E, F, G, I, R, T, neutral, none]
//!   [20]    can build
//!   [21]    can disband
//!   [22:25] dislodged:    [army, fleet, none]
//!   [25:33] disl. owner:  [A, E, F, G, I, R, T, none]
//!   [33:36] province type: [land, sea, coast]

use crate::board::province::{
    Coast, Power, Province, ProvinceType, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT,
};
use crate::board::state::{BoardState, Phase};
use crate::board::unit::UnitType;

/// Total number of areas (75 provinces + 6 bicoastal variants).
pub const NUM_AREAS: usize = 81;

/// Number of features per area.
pub const NUM_FEATURES: usize = 36;

/// Number of powers.
const NUM_POWERS: usize = 7;

/// Feature offset constants.
const FEAT_UNIT_TYPE: usize = 0;
const FEAT_UNIT_OWNER: usize = 3;
const FEAT_SC_OWNER: usize = 11;
const FEAT_CAN_BUILD: usize = 20;
const FEAT_CAN_DISBAND: usize = 21;
const FEAT_DISLODGED_TYPE: usize = 22;
const FEAT_DISLODGED_OWNER: usize = 25;
const FEAT_PROVINCE_TYPE: usize = 33;

/// Bicoastal variant indices (appended after the 75 provinces).
/// These match the Python AREAS list sorted order:
///   sorted(PROVINCES) + sorted(["bul/ec", "bul/sc", "spa/nc", "spa/sc", "stp/nc", "stp/sc"])
const BUL_EC: usize = 75;
const BUL_SC: usize = 76;
const SPA_NC: usize = 77;
const SPA_SC: usize = 78;
const STP_NC: usize = 79;
const STP_SC: usize = 80;

/// Maps a Power to its feature index (0..6).
#[inline]
fn power_index(p: Power) -> usize {
    match p {
        Power::Austria => 0,
        Power::England => 1,
        Power::France => 2,
        Power::Germany => 3,
        Power::Italy => 4,
        Power::Russia => 5,
        Power::Turkey => 6,
    }
}

/// Returns the bicoastal variant index for a split-coast province, if applicable.
fn bicoastal_index(province: Province, coast: Coast) -> Option<usize> {
    match (province, coast) {
        (Province::Bul, Coast::East) => Some(BUL_EC),
        (Province::Bul, Coast::South) => Some(BUL_SC),
        (Province::Spa, Coast::North) => Some(SPA_NC),
        (Province::Spa, Coast::South) => Some(SPA_SC),
        (Province::Stp, Coast::North) => Some(STP_NC),
        (Province::Stp, Coast::South) => Some(STP_SC),
        _ => None,
    }
}

/// Returns the province type feature vector [land, sea, coast] for an area.
fn province_type_vec(area: usize) -> [f32; 3] {
    if area >= PROVINCE_COUNT {
        // Bicoastal variants inherit the type of their base province (coastal).
        return [0.0, 0.0, 1.0];
    }
    match ALL_PROVINCES[area].province_type() {
        ProvinceType::Land => [1.0, 0.0, 0.0],
        ProvinceType::Sea => [0.0, 1.0, 0.0],
        ProvinceType::Coastal => [0.0, 0.0, 1.0],
    }
}

/// Set of all supply center province indices.
fn is_supply_center(prov_idx: usize) -> bool {
    if prov_idx >= PROVINCE_COUNT {
        // Bicoastal variants: check the base province.
        let base = match prov_idx {
            BUL_EC | BUL_SC => Province::Bul as usize,
            SPA_NC | SPA_SC => Province::Spa as usize,
            STP_NC | STP_SC => Province::Stp as usize,
            _ => return false,
        };
        ALL_PROVINCES[base].is_supply_center()
    } else {
        ALL_PROVINCES[prov_idx].is_supply_center()
    }
}

/// Encodes a `BoardState` into an [81 * 36] flat f32 array (row-major).
///
/// The tensor layout matches Python `features.encode_board_state()`.
pub fn encode_board_state(state: &BoardState) -> [f32; NUM_AREAS * NUM_FEATURES] {
    let mut tensor = [0.0f32; NUM_AREAS * NUM_FEATURES];

    // Static province type features.
    for area in 0..NUM_AREAS {
        let ptype = province_type_vec(area);
        let base = area * NUM_FEATURES + FEAT_PROVINCE_TYPE;
        tensor[base] = ptype[0];
        tensor[base + 1] = ptype[1];
        tensor[base + 2] = ptype[2];
    }

    // Unit positions.
    for i in 0..PROVINCE_COUNT {
        if let Some((power, unit_type)) = state.units[i] {
            let pi = power_index(power);
            set_unit_features(&mut tensor, i, unit_type, pi);

            // Also set on the bicoastal variant if the unit has a coast.
            if let Some(coast) = state.fleet_coast[i] {
                if let Some(var_idx) = bicoastal_index(ALL_PROVINCES[i], coast) {
                    set_unit_features(&mut tensor, var_idx, unit_type, pi);
                }
            }
        }
    }

    // Mark empty areas (no unit present).
    for area in 0..NUM_AREAS {
        let base = area * NUM_FEATURES;
        if tensor[base + FEAT_UNIT_TYPE] == 0.0 && tensor[base + FEAT_UNIT_TYPE + 1] == 0.0 {
            tensor[base + FEAT_UNIT_TYPE + 2] = 1.0; // empty
            tensor[base + FEAT_UNIT_OWNER + NUM_POWERS] = 1.0; // owner = none
        }
    }

    // Supply center ownership.
    let mut owned_sc = [false; PROVINCE_COUNT];
    for i in 0..PROVINCE_COUNT {
        if let Some(power) = state.sc_owner[i] {
            if !ALL_PROVINCES[i].is_supply_center() {
                continue;
            }
            owned_sc[i] = true;
            let pi = power_index(power);
            let base = i * NUM_FEATURES;
            tensor[base + FEAT_SC_OWNER + pi] = 1.0;

            // Also mark on bicoastal variants.
            let prov = ALL_PROVINCES[i];
            for &coast in prov.coasts() {
                if let Some(var_idx) = bicoastal_index(prov, coast) {
                    let vbase = var_idx * NUM_FEATURES;
                    tensor[vbase + FEAT_SC_OWNER + pi] = 1.0;
                }
            }
        }
    }

    // Mark neutral SCs and non-SC areas.
    for area in 0..NUM_AREAS {
        let base_prov = if area < PROVINCE_COUNT {
            area
        } else {
            match area {
                BUL_EC | BUL_SC => Province::Bul as usize,
                SPA_NC | SPA_SC => Province::Spa as usize,
                STP_NC | STP_SC => Province::Stp as usize,
                _ => continue,
            }
        };
        let abase = area * NUM_FEATURES;
        if is_supply_center(area) {
            if !owned_sc[base_prov] {
                tensor[abase + FEAT_SC_OWNER + NUM_POWERS] = 1.0; // neutral
            }
        } else {
            tensor[abase + FEAT_SC_OWNER + NUM_POWERS + 1] = 1.0; // none (not an SC)
        }
    }

    // Build/disband flags (adjustment phase).
    if state.phase == Phase::Build {
        encode_build_disband(&mut tensor, state);
    }

    // Dislodged units.
    for i in 0..PROVINCE_COUNT {
        if let Some(ref d) = state.dislodged[i] {
            let base = i * NUM_FEATURES;
            match d.unit_type {
                UnitType::Army => tensor[base + FEAT_DISLODGED_TYPE] = 1.0,
                UnitType::Fleet => tensor[base + FEAT_DISLODGED_TYPE + 1] = 1.0,
            }
            tensor[base + FEAT_DISLODGED_OWNER + power_index(d.power)] = 1.0;
        }
    }

    // Mark non-dislodged slots.
    for area in 0..NUM_AREAS {
        let base = area * NUM_FEATURES;
        if tensor[base + FEAT_DISLODGED_TYPE] == 0.0
            && tensor[base + FEAT_DISLODGED_TYPE + 1] == 0.0
        {
            tensor[base + FEAT_DISLODGED_TYPE + 2] = 1.0; // none
            tensor[base + FEAT_DISLODGED_OWNER + NUM_POWERS] = 1.0; // owner = none
        }
    }

    tensor
}

/// Sets unit type and owner features for an area.
fn set_unit_features(tensor: &mut [f32], area: usize, unit_type: UnitType, power_idx: usize) {
    let base = area * NUM_FEATURES;
    match unit_type {
        UnitType::Army => tensor[base + FEAT_UNIT_TYPE] = 1.0,
        UnitType::Fleet => tensor[base + FEAT_UNIT_TYPE + 1] = 1.0,
    }
    tensor[base + FEAT_UNIT_OWNER + power_idx] = 1.0;
}

/// Encodes build/disband flags during adjustment phases.
fn encode_build_disband(tensor: &mut [f32], state: &BoardState) {
    for &power in ALL_POWERS.iter() {
        let num_units = state
            .units
            .iter()
            .filter(|u| matches!(u, Some((p, _)) if *p == power))
            .count();
        let num_scs = state
            .sc_owner
            .iter()
            .enumerate()
            .filter(|(i, o)| **o == Some(power) && ALL_PROVINCES[*i].is_supply_center())
            .count();

        if num_scs > num_units {
            // Can build on owned home centers that are unoccupied.
            let occupied: Vec<usize> = state
                .units
                .iter()
                .enumerate()
                .filter_map(|(i, u)| u.map(|_| i))
                .collect();
            for i in 0..PROVINCE_COUNT {
                let prov = ALL_PROVINCES[i];
                if prov.home_power() == Some(power)
                    && prov.is_supply_center()
                    && state.sc_owner[i] == Some(power)
                    && !occupied.contains(&i)
                {
                    tensor[i * NUM_FEATURES + FEAT_CAN_BUILD] = 1.0;
                }
            }
        } else if num_units > num_scs {
            // Must disband: mark all of this power's units.
            for i in 0..PROVINCE_COUNT {
                if let Some((p, _)) = state.units[i] {
                    if p == power {
                        tensor[i * NUM_FEATURES + FEAT_CAN_DISBAND] = 1.0;
                    }
                }
            }
        }
    }
}

/// Builds the 81x81 adjacency matrix matching the Python `build_adjacency_matrix()`.
///
/// Returns a flat row-major [81*81] f32 array with self-loops and bicoastal
/// variant inheritance.
pub fn build_adjacency_matrix() -> Vec<f32> {
    use crate::board::adjacency::ADJACENCIES;

    let mut adj = vec![0.0f32; NUM_AREAS * NUM_AREAS];

    // Add edges from the adjacency table (over base provinces only).
    for entry in ADJACENCIES.iter() {
        let i = entry.from as usize;
        let j = entry.to as usize;
        if i < PROVINCE_COUNT && j < PROVINCE_COUNT {
            adj[i * NUM_AREAS + j] = 1.0;
            adj[j * NUM_AREAS + i] = 1.0;
        }
    }

    // Connect bicoastal variants to their base and propagate base adjacencies.
    let split_coasts: [(Province, &[(Coast, usize)]); 3] = [
        (
            Province::Bul,
            &[(Coast::East, BUL_EC), (Coast::South, BUL_SC)],
        ),
        (
            Province::Spa,
            &[(Coast::North, SPA_NC), (Coast::South, SPA_SC)],
        ),
        (
            Province::Stp,
            &[(Coast::North, STP_NC), (Coast::South, STP_SC)],
        ),
    ];

    for (base_prov, coasts) in &split_coasts {
        let base_idx = *base_prov as usize;
        for &(_coast, var_idx) in *coasts {
            // Variant <-> base.
            adj[base_idx * NUM_AREAS + var_idx] = 1.0;
            adj[var_idx * NUM_AREAS + base_idx] = 1.0;
            // Variant inherits all base adjacencies.
            for k in 0..NUM_AREAS {
                if adj[base_idx * NUM_AREAS + k] == 1.0 {
                    adj[var_idx * NUM_AREAS + k] = 1.0;
                    adj[k * NUM_AREAS + var_idx] = 1.0;
                }
            }
        }
    }

    // Self-loops.
    for i in 0..NUM_AREAS {
        adj[i * NUM_AREAS + i] = 1.0;
    }

    adj
}

/// Collects unit indices for a given power. Returns province indices (area indices)
/// of units belonging to the specified power, suitable for the policy network's
/// `unit_indices` input. Padded to `max_units` with zeros.
pub fn collect_unit_indices(state: &BoardState, power: Power, max_units: usize) -> Vec<i64> {
    let mut indices = Vec::with_capacity(max_units);
    for i in 0..PROVINCE_COUNT {
        if let Some((p, _)) = state.units[i] {
            if p == power && indices.len() < max_units {
                indices.push(i as i64);
            }
        }
    }
    // Pad with zeros.
    while indices.len() < max_units {
        indices.push(0);
    }
    indices
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::state::{Phase, Season};
    use crate::protocol::dfen::parse_dfen;

    const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

    fn initial_state() -> BoardState {
        parse_dfen(INITIAL_DFEN).expect("failed to parse initial DFEN")
    }

    #[test]
    fn tensor_shape_and_values() {
        let state = initial_state();
        let tensor = encode_board_state(&state);
        assert_eq!(tensor.len(), NUM_AREAS * NUM_FEATURES);

        // All values should be 0.0 or 1.0 (one-hot encoding).
        for &v in tensor.iter() {
            assert!(v == 0.0 || v == 1.0, "Unexpected value: {}", v);
        }
    }

    #[test]
    fn vienna_has_austrian_army() {
        let state = initial_state();
        let tensor = encode_board_state(&state);
        let vie_idx = Province::Vie as usize;
        let base = vie_idx * NUM_FEATURES;

        // Unit type: army.
        assert_eq!(tensor[base + FEAT_UNIT_TYPE], 1.0, "Vie should have army");
        assert_eq!(tensor[base + FEAT_UNIT_TYPE + 1], 0.0, "Vie has no fleet");
        assert_eq!(tensor[base + FEAT_UNIT_TYPE + 2], 0.0, "Vie not empty");

        // Unit owner: Austria (index 0).
        assert_eq!(tensor[base + FEAT_UNIT_OWNER], 1.0, "Austria owns Vie unit");
    }

    #[test]
    fn london_has_english_fleet() {
        let state = initial_state();
        let tensor = encode_board_state(&state);
        let lon_idx = Province::Lon as usize;
        let base = lon_idx * NUM_FEATURES;

        assert_eq!(tensor[base + FEAT_UNIT_TYPE], 0.0);
        assert_eq!(
            tensor[base + FEAT_UNIT_TYPE + 1],
            1.0,
            "Lon should have fleet"
        );
        assert_eq!(
            tensor[base + FEAT_UNIT_OWNER + 1],
            1.0,
            "England owns Lon unit"
        );
    }

    #[test]
    fn stp_south_coast_fleet() {
        let state = initial_state();
        let tensor = encode_board_state(&state);

        // Base Stp province should have the fleet.
        let stp_idx = Province::Stp as usize;
        let base = stp_idx * NUM_FEATURES;
        assert_eq!(tensor[base + FEAT_UNIT_TYPE + 1], 1.0, "Stp has fleet");
        assert_eq!(
            tensor[base + FEAT_UNIT_OWNER + 5],
            1.0,
            "Russia owns Stp unit"
        );

        // Stp/sc variant should also show the fleet.
        let var_base = STP_SC * NUM_FEATURES;
        assert_eq!(
            tensor[var_base + FEAT_UNIT_TYPE + 1],
            1.0,
            "Stp/sc has fleet"
        );
        assert_eq!(
            tensor[var_base + FEAT_UNIT_OWNER + 5],
            1.0,
            "Russia owns Stp/sc unit"
        );

        // Stp/nc should be empty.
        let nc_base = STP_NC * NUM_FEATURES;
        assert_eq!(tensor[nc_base + FEAT_UNIT_TYPE + 2], 1.0, "Stp/nc is empty");
    }

    #[test]
    fn sc_ownership_initial() {
        let state = initial_state();
        let tensor = encode_board_state(&state);

        // Vienna is Austrian SC.
        let vie_base = Province::Vie as usize * NUM_FEATURES;
        assert_eq!(
            tensor[vie_base + FEAT_SC_OWNER],
            1.0,
            "Vie SC owned by Austria"
        );

        // Serbia is neutral SC.
        let ser_base = Province::Ser as usize * NUM_FEATURES;
        assert_eq!(
            tensor[ser_base + FEAT_SC_OWNER + NUM_POWERS],
            1.0,
            "Ser is neutral SC"
        );

        // Bohemia is not an SC.
        let boh_base = Province::Boh as usize * NUM_FEATURES;
        assert_eq!(
            tensor[boh_base + FEAT_SC_OWNER + NUM_POWERS + 1],
            1.0,
            "Boh is not an SC"
        );
    }

    #[test]
    fn province_types_correct() {
        let tensor = encode_board_state(&initial_state());

        // Bohemia is inland.
        let boh_base = Province::Boh as usize * NUM_FEATURES;
        assert_eq!(tensor[boh_base + FEAT_PROVINCE_TYPE], 1.0, "Boh is land");

        // North Sea is sea.
        let nth_base = Province::Nth as usize * NUM_FEATURES;
        assert_eq!(tensor[nth_base + FEAT_PROVINCE_TYPE + 1], 1.0, "Nth is sea");

        // London is coastal.
        let lon_base = Province::Lon as usize * NUM_FEATURES;
        assert_eq!(
            tensor[lon_base + FEAT_PROVINCE_TYPE + 2],
            1.0,
            "Lon is coast"
        );

        // Bicoastal variant is coastal.
        let bul_ec_base = BUL_EC * NUM_FEATURES;
        assert_eq!(
            tensor[bul_ec_base + FEAT_PROVINCE_TYPE + 2],
            1.0,
            "Bul/ec is coast"
        );
    }

    #[test]
    fn empty_provinces_marked_correctly() {
        let tensor = encode_board_state(&initial_state());

        // Galicia has no unit.
        let gal_base = Province::Gal as usize * NUM_FEATURES;
        assert_eq!(tensor[gal_base + FEAT_UNIT_TYPE + 2], 1.0, "Gal is empty");
        assert_eq!(
            tensor[gal_base + FEAT_UNIT_OWNER + NUM_POWERS],
            1.0,
            "Gal owner = none"
        );
    }

    #[test]
    fn no_dislodged_in_initial() {
        let tensor = encode_board_state(&initial_state());
        for area in 0..NUM_AREAS {
            let base = area * NUM_FEATURES;
            assert_eq!(
                tensor[base + FEAT_DISLODGED_TYPE + 2],
                1.0,
                "Area {} should have no dislodged unit",
                area
            );
        }
    }

    #[test]
    fn adjacency_matrix_shape() {
        let adj = build_adjacency_matrix();
        assert_eq!(adj.len(), NUM_AREAS * NUM_AREAS);
    }

    #[test]
    fn adjacency_has_self_loops() {
        let adj = build_adjacency_matrix();
        for i in 0..NUM_AREAS {
            assert_eq!(
                adj[i * NUM_AREAS + i],
                1.0,
                "Self-loop missing for area {}",
                i
            );
        }
    }

    #[test]
    fn adjacency_is_symmetric() {
        let adj = build_adjacency_matrix();
        for i in 0..NUM_AREAS {
            for j in 0..NUM_AREAS {
                assert_eq!(
                    adj[i * NUM_AREAS + j],
                    adj[j * NUM_AREAS + i],
                    "Asymmetric at ({}, {})",
                    i,
                    j
                );
            }
        }
    }

    #[test]
    fn adjacency_known_edges() {
        let adj = build_adjacency_matrix();
        // Vienna <-> Bohemia should be connected.
        let vie = Province::Vie as usize;
        let boh = Province::Boh as usize;
        assert_eq!(
            adj[vie * NUM_AREAS + boh],
            1.0,
            "Vie-Boh should be adjacent"
        );

        // Vienna <-> Venice should NOT be directly connected.
        let ven = Province::Ven as usize;
        assert_eq!(
            adj[vie * NUM_AREAS + ven],
            0.0,
            "Vie-Ven should not be adjacent"
        );
    }

    #[test]
    fn bicoastal_variants_connected_to_base() {
        let adj = build_adjacency_matrix();
        let bul = Province::Bul as usize;
        assert_eq!(adj[bul * NUM_AREAS + BUL_EC], 1.0);
        assert_eq!(adj[bul * NUM_AREAS + BUL_SC], 1.0);
    }

    #[test]
    fn collect_unit_indices_austria() {
        let state = initial_state();
        let indices = collect_unit_indices(&state, Power::Austria, 17);
        assert_eq!(indices.len(), 17);

        // Austria has 3 units: Vie, Bud, Tri.
        let expected: Vec<i64> = vec![
            Province::Bud as i64,
            Province::Tri as i64,
            Province::Vie as i64,
        ];
        let mut actual: Vec<i64> = indices[..3].to_vec();
        actual.sort();
        let mut exp_sorted = expected.clone();
        exp_sorted.sort();
        assert_eq!(actual, exp_sorted);

        // Remaining slots should be zero-padded.
        for &idx in &indices[3..] {
            assert_eq!(idx, 0);
        }
    }

    #[test]
    fn build_phase_marks_can_build() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.set_sc_owner(Province::Tri, Some(Power::Austria));
        state.set_sc_owner(Province::Ser, Some(Power::Austria));
        // 4 SCs, 1 unit -> needs 3 builds.
        state.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);

        let tensor = encode_board_state(&state);

        // Vie, Bud, Tri are home centers and unoccupied -> can build.
        assert_eq!(
            tensor[Province::Vie as usize * NUM_FEATURES + FEAT_CAN_BUILD],
            1.0,
            "Vie should be buildable"
        );
        assert_eq!(
            tensor[Province::Bud as usize * NUM_FEATURES + FEAT_CAN_BUILD],
            1.0,
            "Bud should be buildable"
        );
        assert_eq!(
            tensor[Province::Tri as usize * NUM_FEATURES + FEAT_CAN_BUILD],
            1.0,
            "Tri should be buildable"
        );
    }
}
