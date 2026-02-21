//! Adjacency graph for the standard Diplomacy map.
//!
//! Each entry records a directed edge: (from, from_coast) -> (to, to_coast)
//! with flags for army and fleet passability. The table is symmetric: if A->B
//! exists then B->A also exists. All data is compile-time `static`.
//!
//! Split-coast provinces (bul, spa, stp) use coast-specific fleet adjacencies
//! and Coast::None for army adjacencies.

use super::province::{Coast, Province, PROVINCE_COUNT};

/// A single directed adjacency between two provinces.
#[derive(Debug, Clone, Copy)]
pub struct AdjacencyEntry {
    pub from: Province,
    pub from_coast: Coast,
    pub to: Province,
    pub to_coast: Coast,
    pub army_ok: bool,
    pub fleet_ok: bool,
}

/// Shorthand constructors for adjacency entries (used only in table construction).
const fn fleet(from: Province, fc: Coast, to: Province, tc: Coast) -> AdjacencyEntry {
    AdjacencyEntry {
        from,
        from_coast: fc,
        to,
        to_coast: tc,
        army_ok: false,
        fleet_ok: true,
    }
}
const fn army(from: Province, to: Province) -> AdjacencyEntry {
    AdjacencyEntry {
        from,
        from_coast: Coast::None,
        to,
        to_coast: Coast::None,
        army_ok: true,
        fleet_ok: false,
    }
}
const fn both(from: Province, to: Province) -> AdjacencyEntry {
    AdjacencyEntry {
        from,
        from_coast: Coast::None,
        to,
        to_coast: Coast::None,
        army_ok: true,
        fleet_ok: true,
    }
}

/// Shorthand coast aliases.
const N: Coast = Coast::None;
const NC: Coast = Coast::North;
const SC: Coast = Coast::South;
const EC: Coast = Coast::East;

/// Alias province names for readability.
use Province::*;

/// Total number of directed adjacency entries in the table.
///
/// Breakdown:
/// - Sea-to-sea (fleet): 21 pairs * 2 = 42
/// - Sea-to-coastal (fleet): 72 pairs * 2 = 144
/// - Inland-to-inland (army): 21 pairs * 2 = 42
/// - Inland-to-coastal (army): 35 pairs * 2 = 70
/// - Coastal-to-coastal both: 27 pairs * 2 = 54
/// - Coastal-to-coastal fleet only (split-coast): 11 pairs * 2 = 22
/// - Coastal-to-coastal/split army only: 9 pairs * 2 = 18
/// - Coastal-to-coastal army only (different seas): 6+5 pairs * 2 = 22
/// Total: 434
pub const ADJACENCY_COUNT: usize = 434;

/// Complete adjacency table. Each bidirectional pair is stored as two directed entries.
///
/// This table is transcribed directly from the Go source in `map_data.go`.
/// The ordering follows the same grouping: sea-to-sea, sea-to-coastal,
/// inland-to-inland, inland-to-coastal, coastal-both, coastal-fleet-only,
/// coastal-army-only.
pub static ADJACENCIES: [AdjacencyEntry; ADJACENCY_COUNT] = [
    // ====================================================================
    // Sea-to-sea (fleet only) - 21 pairs, 42 entries
    // ====================================================================
    fleet(Adr, N, Ion, N),
    fleet(Ion, N, Adr, N),
    fleet(Aeg, N, Eas, N),
    fleet(Eas, N, Aeg, N),
    fleet(Aeg, N, Ion, N),
    fleet(Ion, N, Aeg, N),
    fleet(Bal, N, Bot, N),
    fleet(Bot, N, Bal, N),
    fleet(Eng, N, Iri, N),
    fleet(Iri, N, Eng, N),
    fleet(Eng, N, Mao, N),
    fleet(Mao, N, Eng, N),
    fleet(Eng, N, Nth, N),
    fleet(Nth, N, Eng, N),
    fleet(Gol, N, Tys, N),
    fleet(Tys, N, Gol, N),
    fleet(Gol, N, Wes, N),
    fleet(Wes, N, Gol, N),
    fleet(Hel, N, Nth, N),
    fleet(Nth, N, Hel, N),
    fleet(Ion, N, Eas, N),
    fleet(Eas, N, Ion, N),
    fleet(Ion, N, Tys, N),
    fleet(Tys, N, Ion, N),
    fleet(Iri, N, Mao, N),
    fleet(Mao, N, Iri, N),
    fleet(Iri, N, Nao, N),
    fleet(Nao, N, Iri, N),
    fleet(Mao, N, Nao, N),
    fleet(Nao, N, Mao, N),
    fleet(Mao, N, Wes, N),
    fleet(Wes, N, Mao, N),
    fleet(Nao, N, Nrg, N),
    fleet(Nrg, N, Nao, N),
    fleet(Nth, N, Nrg, N),
    fleet(Nrg, N, Nth, N),
    fleet(Nth, N, Ska, N),
    fleet(Ska, N, Nth, N),
    fleet(Nrg, N, Bar, N),
    fleet(Bar, N, Nrg, N),
    fleet(Tys, N, Wes, N),
    fleet(Wes, N, Tys, N),
    // ====================================================================
    // Sea-to-coastal (fleet only) - 72 pairs, 144 entries
    // ====================================================================

    // Adriatic Sea
    fleet(Adr, N, Alb, N),
    fleet(Alb, N, Adr, N),
    fleet(Adr, N, Apu, N),
    fleet(Apu, N, Adr, N),
    fleet(Adr, N, Tri, N),
    fleet(Tri, N, Adr, N),
    fleet(Adr, N, Ven, N),
    fleet(Ven, N, Adr, N),
    // Aegean Sea
    fleet(Aeg, N, Bul, SC),
    fleet(Bul, SC, Aeg, N),
    fleet(Aeg, N, Con, N),
    fleet(Con, N, Aeg, N),
    fleet(Aeg, N, Gre, N),
    fleet(Gre, N, Aeg, N),
    fleet(Aeg, N, Smy, N),
    fleet(Smy, N, Aeg, N),
    // Baltic Sea
    fleet(Bal, N, Ber, N),
    fleet(Ber, N, Bal, N),
    fleet(Bal, N, Den, N),
    fleet(Den, N, Bal, N),
    fleet(Bal, N, Kie, N),
    fleet(Kie, N, Bal, N),
    fleet(Bal, N, Lvn, N),
    fleet(Lvn, N, Bal, N),
    fleet(Bal, N, Pru, N),
    fleet(Pru, N, Bal, N),
    fleet(Bal, N, Swe, N),
    fleet(Swe, N, Bal, N),
    // Barents Sea
    fleet(Bar, N, Nwy, N),
    fleet(Nwy, N, Bar, N),
    fleet(Bar, N, Stp, NC),
    fleet(Stp, NC, Bar, N),
    // Black Sea
    fleet(Bla, N, Ank, N),
    fleet(Ank, N, Bla, N),
    fleet(Bla, N, Arm, N),
    fleet(Arm, N, Bla, N),
    fleet(Bla, N, Bul, EC),
    fleet(Bul, EC, Bla, N),
    fleet(Bla, N, Con, N),
    fleet(Con, N, Bla, N),
    fleet(Bla, N, Rum, N),
    fleet(Rum, N, Bla, N),
    fleet(Bla, N, Sev, N),
    fleet(Sev, N, Bla, N),
    // Gulf of Bothnia
    fleet(Bot, N, Fin, N),
    fleet(Fin, N, Bot, N),
    fleet(Bot, N, Lvn, N),
    fleet(Lvn, N, Bot, N),
    fleet(Bot, N, Stp, SC),
    fleet(Stp, SC, Bot, N),
    fleet(Bot, N, Swe, N),
    fleet(Swe, N, Bot, N),
    // Eastern Mediterranean
    fleet(Eas, N, Smy, N),
    fleet(Smy, N, Eas, N),
    fleet(Eas, N, Syr, N),
    fleet(Syr, N, Eas, N),
    // English Channel
    fleet(Eng, N, Bel, N),
    fleet(Bel, N, Eng, N),
    fleet(Eng, N, Bre, N),
    fleet(Bre, N, Eng, N),
    fleet(Eng, N, Lon, N),
    fleet(Lon, N, Eng, N),
    fleet(Eng, N, Pic, N),
    fleet(Pic, N, Eng, N),
    fleet(Eng, N, Wal, N),
    fleet(Wal, N, Eng, N),
    // Gulf of Lyon
    fleet(Gol, N, Mar, N),
    fleet(Mar, N, Gol, N),
    fleet(Gol, N, Pie, N),
    fleet(Pie, N, Gol, N),
    fleet(Gol, N, Spa, SC),
    fleet(Spa, SC, Gol, N),
    fleet(Gol, N, Tus, N),
    fleet(Tus, N, Gol, N),
    // Heligoland Bight
    fleet(Hel, N, Den, N),
    fleet(Den, N, Hel, N),
    fleet(Hel, N, Hol, N),
    fleet(Hol, N, Hel, N),
    fleet(Hel, N, Kie, N),
    fleet(Kie, N, Hel, N),
    // Ionian Sea
    fleet(Ion, N, Alb, N),
    fleet(Alb, N, Ion, N),
    fleet(Ion, N, Apu, N),
    fleet(Apu, N, Ion, N),
    fleet(Ion, N, Gre, N),
    fleet(Gre, N, Ion, N),
    fleet(Ion, N, Nap, N),
    fleet(Nap, N, Ion, N),
    fleet(Ion, N, Tun, N),
    fleet(Tun, N, Ion, N),
    // Irish Sea
    fleet(Iri, N, Lvp, N),
    fleet(Lvp, N, Iri, N),
    fleet(Iri, N, Wal, N),
    fleet(Wal, N, Iri, N),
    // Mid-Atlantic Ocean
    fleet(Mao, N, Bre, N),
    fleet(Bre, N, Mao, N),
    fleet(Mao, N, Gas, N),
    fleet(Gas, N, Mao, N),
    fleet(Mao, N, Naf, N),
    fleet(Naf, N, Mao, N),
    fleet(Mao, N, Por, N),
    fleet(Por, N, Mao, N),
    fleet(Mao, N, Spa, NC),
    fleet(Spa, NC, Mao, N),
    fleet(Mao, N, Spa, SC),
    fleet(Spa, SC, Mao, N),
    // North Atlantic Ocean
    fleet(Nao, N, Cly, N),
    fleet(Cly, N, Nao, N),
    fleet(Nao, N, Lvp, N),
    fleet(Lvp, N, Nao, N),
    // North Sea
    fleet(Nth, N, Bel, N),
    fleet(Bel, N, Nth, N),
    fleet(Nth, N, Den, N),
    fleet(Den, N, Nth, N),
    fleet(Nth, N, Edi, N),
    fleet(Edi, N, Nth, N),
    fleet(Nth, N, Hol, N),
    fleet(Hol, N, Nth, N),
    fleet(Nth, N, Lon, N),
    fleet(Lon, N, Nth, N),
    fleet(Nth, N, Nwy, N),
    fleet(Nwy, N, Nth, N),
    fleet(Nth, N, Yor, N),
    fleet(Yor, N, Nth, N),
    // Norwegian Sea
    fleet(Nrg, N, Cly, N),
    fleet(Cly, N, Nrg, N),
    fleet(Nrg, N, Edi, N),
    fleet(Edi, N, Nrg, N),
    fleet(Nrg, N, Nwy, N),
    fleet(Nwy, N, Nrg, N),
    // Skagerrak
    fleet(Ska, N, Den, N),
    fleet(Den, N, Ska, N),
    fleet(Ska, N, Nwy, N),
    fleet(Nwy, N, Ska, N),
    fleet(Ska, N, Swe, N),
    fleet(Swe, N, Ska, N),
    // Tyrrhenian Sea
    fleet(Tys, N, Nap, N),
    fleet(Nap, N, Tys, N),
    fleet(Tys, N, Rom, N),
    fleet(Rom, N, Tys, N),
    fleet(Tys, N, Tun, N),
    fleet(Tun, N, Tys, N),
    fleet(Tys, N, Tus, N),
    fleet(Tus, N, Tys, N),
    // Western Mediterranean
    fleet(Wes, N, Naf, N),
    fleet(Naf, N, Wes, N),
    fleet(Wes, N, Spa, SC),
    fleet(Spa, SC, Wes, N),
    fleet(Wes, N, Tun, N),
    fleet(Tun, N, Wes, N),
    // ====================================================================
    // Inland-to-inland (army only) - 21 pairs, 42 entries
    // ====================================================================
    army(Boh, Gal),
    army(Gal, Boh),
    army(Boh, Mun),
    army(Mun, Boh),
    army(Boh, Sil),
    army(Sil, Boh),
    army(Boh, Tyr),
    army(Tyr, Boh),
    army(Boh, Vie),
    army(Vie, Boh),
    army(Bud, Gal),
    army(Gal, Bud),
    army(Bud, Vie),
    army(Vie, Bud),
    army(Bur, Mun),
    army(Mun, Bur),
    army(Bur, Par),
    army(Par, Bur),
    army(Bur, Ruh),
    army(Ruh, Bur),
    army(Gal, Sil),
    army(Sil, Gal),
    army(Gal, Ukr),
    army(Ukr, Gal),
    army(Gal, Vie),
    army(Vie, Gal),
    army(Gal, War),
    army(War, Gal),
    army(Mos, Ukr),
    army(Ukr, Mos),
    army(Mos, War),
    army(War, Mos),
    army(Mun, Ruh),
    army(Ruh, Mun),
    army(Mun, Sil),
    army(Sil, Mun),
    army(Mun, Tyr),
    army(Tyr, Mun),
    army(Sil, War),
    army(War, Sil),
    army(Tyr, Vie),
    army(Vie, Tyr),
    army(Ukr, War),
    army(War, Ukr),
    // ====================================================================
    // Inland-to-coastal (army only) - 35 pairs, 70 entries
    // ====================================================================
    army(Bud, Rum),
    army(Rum, Bud),
    army(Bud, Ser),
    army(Ser, Bud),
    army(Bud, Tri),
    army(Tri, Bud),
    army(Bur, Bel),
    army(Bel, Bur),
    army(Bur, Gas),
    army(Gas, Bur),
    army(Bur, Mar),
    army(Mar, Bur),
    army(Bur, Pic),
    army(Pic, Bur),
    army(Gal, Rum),
    army(Rum, Gal),
    army(Gas, Mar),
    army(Mar, Gas),
    army(Mos, Lvn),
    army(Lvn, Mos),
    army(Mos, Sev),
    army(Sev, Mos),
    army(Mos, Stp),
    army(Stp, Mos),
    army(Mun, Ber),
    army(Ber, Mun),
    army(Mun, Kie),
    army(Kie, Mun),
    army(Par, Bre),
    army(Bre, Par),
    army(Par, Gas),
    army(Gas, Par),
    army(Par, Pic),
    army(Pic, Par),
    army(Ruh, Bel),
    army(Bel, Ruh),
    army(Ruh, Hol),
    army(Hol, Ruh),
    army(Ruh, Kie),
    army(Kie, Ruh),
    army(Ser, Alb),
    army(Alb, Ser),
    army(Ser, Bul),
    army(Bul, Ser),
    army(Ser, Gre),
    army(Gre, Ser),
    army(Ser, Rum),
    army(Rum, Ser),
    army(Ser, Tri),
    army(Tri, Ser),
    army(Sil, Ber),
    army(Ber, Sil),
    army(Sil, Pru),
    army(Pru, Sil),
    army(Tyr, Pie),
    army(Pie, Tyr),
    army(Tyr, Tri),
    army(Tri, Tyr),
    army(Tyr, Ven),
    army(Ven, Tyr),
    army(Ukr, Rum),
    army(Rum, Ukr),
    army(Ukr, Sev),
    army(Sev, Ukr),
    army(Vie, Tri),
    army(Tri, Vie),
    army(War, Lvn),
    army(Lvn, War),
    army(War, Pru),
    army(Pru, War),
    // ====================================================================
    // Coastal-to-coastal: both army and fleet - 27 pairs, 54 entries
    // (6 pairs moved to army-only: arm-smy, edi-lvp, fin-nwy, pie-ven, rom-ven, wal-yor)
    // ====================================================================
    both(Alb, Gre),
    both(Gre, Alb),
    both(Alb, Tri),
    both(Tri, Alb),
    both(Ank, Arm),
    both(Arm, Ank),
    both(Ank, Con),
    both(Con, Ank),
    both(Apu, Nap),
    both(Nap, Apu),
    both(Apu, Ven),
    both(Ven, Apu),
    both(Bel, Hol),
    both(Hol, Bel),
    both(Bel, Pic),
    both(Pic, Bel),
    both(Ber, Kie),
    both(Kie, Ber),
    both(Ber, Pru),
    both(Pru, Ber),
    both(Bre, Gas),
    both(Gas, Bre),
    both(Bre, Pic),
    both(Pic, Bre),
    both(Cly, Edi),
    both(Edi, Cly),
    both(Cly, Lvp),
    both(Lvp, Cly),
    both(Con, Smy),
    both(Smy, Con),
    both(Den, Kie),
    both(Kie, Den),
    both(Den, Swe),
    both(Swe, Den),
    army(Edi, Lvp),
    army(Lvp, Edi),
    both(Edi, Yor),
    both(Yor, Edi),
    army(Fin, Nwy),
    army(Nwy, Fin),
    both(Fin, Swe),
    both(Swe, Fin),
    both(Lon, Wal),
    both(Wal, Lon),
    both(Lon, Yor),
    both(Yor, Lon),
    both(Lvp, Wal),
    both(Wal, Lvp),
    both(Mar, Pie),
    both(Pie, Mar),
    both(Naf, Tun),
    both(Tun, Naf),
    both(Nwy, Swe),
    both(Swe, Nwy),
    both(Pie, Tus),
    both(Tus, Pie),
    army(Pie, Ven),
    army(Ven, Pie),
    both(Pru, Lvn),
    both(Lvn, Pru),
    both(Rom, Nap),
    both(Nap, Rom),
    both(Rom, Tus),
    both(Tus, Rom),
    army(Rom, Ven),
    army(Ven, Rom),
    both(Sev, Arm),
    both(Arm, Sev),
    both(Sev, Rum),
    both(Rum, Sev),
    army(Smy, Arm),
    army(Arm, Smy),
    both(Smy, Syr),
    both(Syr, Smy),
    both(Tri, Ven),
    both(Ven, Tri),
    army(Wal, Yor),
    army(Yor, Wal),
    // ====================================================================
    // Coastal-to-coastal: fleet only (split-coast) - 11 pairs, 22 entries
    // ====================================================================
    fleet(Con, N, Bul, EC),
    fleet(Bul, EC, Con, N),
    fleet(Con, N, Bul, SC),
    fleet(Bul, SC, Con, N),
    fleet(Gre, N, Bul, SC),
    fleet(Bul, SC, Gre, N),
    fleet(Rum, N, Bul, EC),
    fleet(Bul, EC, Rum, N),
    fleet(Gas, N, Spa, NC),
    fleet(Spa, NC, Gas, N),
    fleet(Mar, N, Spa, SC),
    fleet(Spa, SC, Mar, N),
    fleet(Por, N, Spa, NC),
    fleet(Spa, NC, Por, N),
    fleet(Por, N, Spa, SC),
    fleet(Spa, SC, Por, N),
    fleet(Fin, N, Stp, SC),
    fleet(Stp, SC, Fin, N),
    fleet(Lvn, N, Stp, SC),
    fleet(Stp, SC, Lvn, N),
    fleet(Nwy, N, Stp, NC),
    fleet(Stp, NC, Nwy, N),
    // ====================================================================
    // Coastal-to-coastal/split: army only - 9 pairs, 18 entries
    // ====================================================================
    army(Con, Bul),
    army(Bul, Con),
    army(Gre, Bul),
    army(Bul, Gre),
    army(Rum, Bul),
    army(Bul, Rum),
    army(Gas, Spa),
    army(Spa, Gas),
    army(Mar, Spa),
    army(Spa, Mar),
    army(Por, Spa),
    army(Spa, Por),
    army(Fin, Stp),
    army(Stp, Fin),
    army(Lvn, Stp),
    army(Stp, Lvn),
    army(Nwy, Stp),
    army(Stp, Nwy),
    // ====================================================================
    // Coastal-to-coastal: army only (different sea faces) - 5 pairs, 10 entries
    // ====================================================================
    army(Ank, Smy),
    army(Smy, Ank),
    army(Apu, Rom),
    army(Rom, Apu),
    army(Lvp, Yor),
    army(Yor, Lvp),
    army(Tus, Ven),
    army(Ven, Tus),
    army(Arm, Syr),
    army(Syr, Arm),
];

/// Returns true if a unit of the given type can move from `src` to `dst`,
/// optionally specifying coasts for fleet movement on split-coast provinces.
pub fn is_adjacent(
    src: Province,
    src_coast: Coast,
    dst: Province,
    dst_coast: Coast,
    is_fleet: bool,
) -> bool {
    is_adjacent_fast(src, src_coast, dst, dst_coast, is_fleet)
}

/// Returns all coasts at the destination reachable by fleet from the given source and coast.
pub fn fleet_coasts_to(src: Province, src_coast: Coast, dst: Province) -> Vec<Coast> {
    let mut coasts = Vec::new();
    for adj in adj_from(src) {
        if adj.to != dst || !adj.fleet_ok {
            continue;
        }
        if src_coast != Coast::None && adj.from_coast != Coast::None && adj.from_coast != src_coast
        {
            continue;
        }
        if !coasts.contains(&adj.to_coast) {
            coasts.push(adj.to_coast);
        }
    }
    coasts
}

/// Returns all provinces adjacent to the given province for the given unit type.
pub fn provinces_adjacent_to(prov: Province, coast: Coast, is_fleet: bool) -> Vec<Province> {
    let mut result = Vec::new();
    for adj in adj_from(prov) {
        if is_fleet && !adj.fleet_ok {
            continue;
        }
        if !is_fleet && !adj.army_ok {
            continue;
        }
        if coast != Coast::None && adj.from_coast != Coast::None && adj.from_coast != coast {
            continue;
        }
        if !result.contains(&adj.to) {
            result.push(adj.to);
        }
    }
    result
}

/// Pre-computed per-province adjacency index for O(neighbors) lookup.
///
/// At first access, copies all adjacency entries into a vec sorted by
/// `from` province, and stores `(start, end)` offsets for each province.
/// Subsequent adjacency lookups use this index instead of scanning 424 entries.
use std::sync::LazyLock;

struct AdjIndex {
    entries: Vec<AdjacencyEntry>,
    offsets: [(u16, u16); PROVINCE_COUNT],
}

static ADJ_INDEX: LazyLock<AdjIndex> = LazyLock::new(|| {
    let mut sorted: Vec<AdjacencyEntry> = ADJACENCIES.to_vec();
    sorted.sort_by_key(|a| a.from as u8);

    let mut offsets = [(0u16, 0u16); PROVINCE_COUNT];
    let mut i = 0;
    for p in 0..PROVINCE_COUNT {
        let start = i;
        while i < sorted.len() && sorted[i].from as u8 == p as u8 {
            i += 1;
        }
        offsets[p] = (start as u16, i as u16);
    }

    AdjIndex {
        entries: sorted,
        offsets,
    }
});

/// Returns the adjacency entries originating from the given province.
#[inline]
pub fn adj_from(prov: Province) -> &'static [AdjacencyEntry] {
    let idx = &*ADJ_INDEX;
    let (start, end) = idx.offsets[prov as usize];
    &idx.entries[start as usize..end as usize]
}

/// Fast is_adjacent using per-province index.
pub fn is_adjacent_fast(
    src: Province,
    src_coast: Coast,
    dst: Province,
    dst_coast: Coast,
    is_fleet: bool,
) -> bool {
    for adj in adj_from(src) {
        if adj.to != dst {
            continue;
        }
        if is_fleet && !adj.fleet_ok {
            continue;
        }
        if !is_fleet && !adj.army_ok {
            continue;
        }
        if src_coast != Coast::None && adj.from_coast != Coast::None && adj.from_coast != src_coast
        {
            continue;
        }
        if dst_coast != Coast::None && adj.to_coast != Coast::None && adj.to_coast != dst_coast {
            continue;
        }
        return true;
    }
    false
}

#[cfg(test)]
mod tests {
    use super::super::province::{ProvinceType, ALL_PROVINCES};
    use super::*;
    use std::collections::HashSet;

    #[test]
    fn adjacency_count() {
        assert_eq!(ADJACENCIES.len(), ADJACENCY_COUNT);
    }

    #[test]
    fn adjacency_symmetry() {
        for adj in ADJACENCIES.iter() {
            let reverse_exists = ADJACENCIES.iter().any(|r| {
                r.from == adj.to
                    && r.to == adj.from
                    && r.from_coast == adj.to_coast
                    && r.to_coast == adj.from_coast
                    && r.army_ok == adj.army_ok
                    && r.fleet_ok == adj.fleet_ok
            });
            assert!(
                reverse_exists,
                "Missing reverse adjacency: {:?}({:?}) -> {:?}({:?}) army={} fleet={}",
                adj.from, adj.from_coast, adj.to, adj.to_coast, adj.army_ok, adj.fleet_ok
            );
        }
    }

    #[test]
    fn no_self_adjacency() {
        for adj in ADJACENCIES.iter() {
            assert_ne!(adj.from, adj.to, "Self-adjacency found for {:?}", adj.from);
        }
    }

    #[test]
    fn smyrna_ankara_army_only() {
        // Army can move between Smy and Ank (they share a land border)
        assert!(is_adjacent(
            Province::Smy,
            Coast::None,
            Province::Ank,
            Coast::None,
            false
        ));
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Smy,
            Coast::None,
            false
        ));
        // Fleet cannot (Ankara faces Black Sea, Smyrna faces Aegean)
        assert!(!is_adjacent(
            Province::Smy,
            Coast::None,
            Province::Ank,
            Coast::None,
            true
        ));
        assert!(!is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Smy,
            Coast::None,
            true
        ));
    }

    #[test]
    fn vienna_venice_not_adjacent() {
        assert!(!is_adjacent(
            Province::Vie,
            Coast::None,
            Province::Ven,
            Coast::None,
            false
        ));
        assert!(!is_adjacent(
            Province::Vie,
            Coast::None,
            Province::Ven,
            Coast::None,
            true
        ));
    }

    #[test]
    fn vienna_neighbors() {
        let army_neighbors = provinces_adjacent_to(Province::Vie, Coast::None, false);
        let expected: HashSet<Province> = [
            Province::Boh,
            Province::Bud,
            Province::Gal,
            Province::Tyr,
            Province::Tri,
        ]
        .into_iter()
        .collect();
        let actual: HashSet<Province> = army_neighbors.into_iter().collect();
        assert_eq!(actual, expected, "Vienna army neighbors mismatch");
    }

    #[test]
    fn split_coast_bulgaria() {
        // Army can move to Bulgaria from Con, Gre, Rum, Ser
        let army_adj = provinces_adjacent_to(Province::Bul, Coast::None, false);
        let expected_army: HashSet<Province> =
            [Province::Con, Province::Gre, Province::Rum, Province::Ser]
                .into_iter()
                .collect();
        let actual_army: HashSet<Province> = army_adj.into_iter().collect();
        assert_eq!(actual_army, expected_army);

        // Fleet on EC can reach: Bla, Con, Rum
        let fleet_ec = provinces_adjacent_to(Province::Bul, Coast::East, true);
        let expected_ec: HashSet<Province> = [Province::Bla, Province::Con, Province::Rum]
            .into_iter()
            .collect();
        let actual_ec: HashSet<Province> = fleet_ec.into_iter().collect();
        assert_eq!(actual_ec, expected_ec);

        // Fleet on SC can reach: Aeg, Con, Gre
        let fleet_sc = provinces_adjacent_to(Province::Bul, Coast::South, true);
        let expected_sc: HashSet<Province> = [Province::Aeg, Province::Con, Province::Gre]
            .into_iter()
            .collect();
        let actual_sc: HashSet<Province> = fleet_sc.into_iter().collect();
        assert_eq!(actual_sc, expected_sc);
    }

    #[test]
    fn split_coast_spain() {
        // Fleet on NC can reach: Mao, Gas, Por
        let fleet_nc = provinces_adjacent_to(Province::Spa, Coast::North, true);
        let expected_nc: HashSet<Province> = [Province::Mao, Province::Gas, Province::Por]
            .into_iter()
            .collect();
        let actual_nc: HashSet<Province> = fleet_nc.into_iter().collect();
        assert_eq!(actual_nc, expected_nc);

        // Fleet on SC can reach: Gol, Mao, Mar, Por, Wes
        let fleet_sc = provinces_adjacent_to(Province::Spa, Coast::South, true);
        let expected_sc: HashSet<Province> = [
            Province::Gol,
            Province::Mao,
            Province::Mar,
            Province::Por,
            Province::Wes,
        ]
        .into_iter()
        .collect();
        let actual_sc: HashSet<Province> = fleet_sc.into_iter().collect();
        assert_eq!(actual_sc, expected_sc);
    }

    #[test]
    fn split_coast_st_petersburg() {
        // Fleet on NC can reach: Bar, Nwy
        let fleet_nc = provinces_adjacent_to(Province::Stp, Coast::North, true);
        let expected_nc: HashSet<Province> = [Province::Bar, Province::Nwy].into_iter().collect();
        let actual_nc: HashSet<Province> = fleet_nc.into_iter().collect();
        assert_eq!(actual_nc, expected_nc);

        // Fleet on SC can reach: Bot, Fin, Lvn
        let fleet_sc = provinces_adjacent_to(Province::Stp, Coast::South, true);
        let expected_sc: HashSet<Province> = [Province::Bot, Province::Fin, Province::Lvn]
            .into_iter()
            .collect();
        let actual_sc: HashSet<Province> = fleet_sc.into_iter().collect();
        assert_eq!(actual_sc, expected_sc);
    }

    #[test]
    fn sea_provinces_have_no_army_adjacencies() {
        for p in ALL_PROVINCES.iter() {
            if p.province_type() == ProvinceType::Sea {
                let army_adj = provinces_adjacent_to(*p, Coast::None, false);
                assert!(
                    army_adj.is_empty(),
                    "Sea province {:?} should have no army adjacencies, got {:?}",
                    p,
                    army_adj
                );
            }
        }
    }

    #[test]
    fn inland_provinces_have_no_fleet_adjacencies() {
        for p in ALL_PROVINCES.iter() {
            if p.province_type() == ProvinceType::Land {
                let fleet_adj = provinces_adjacent_to(*p, Coast::None, true);
                assert!(
                    fleet_adj.is_empty(),
                    "Inland province {:?} should have no fleet adjacencies, got {:?}",
                    p,
                    fleet_adj
                );
            }
        }
    }

    #[test]
    fn every_province_has_at_least_one_adjacency() {
        for p in ALL_PROVINCES.iter() {
            let has_any = ADJACENCIES.iter().any(|adj| adj.from == *p);
            assert!(has_any, "Province {:?} has no adjacencies", p);
        }
    }

    #[test]
    fn known_adjacencies_sample() {
        // A selection of known adjacencies to spot-check
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Arm,
            Coast::None,
            true
        ));
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Con,
            Coast::None,
            true
        ));
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Arm,
            Coast::None,
            false
        ));
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Con,
            Coast::None,
            false
        ));
        assert!(is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Bla,
            Coast::None,
            true
        ));
        assert!(!is_adjacent(
            Province::Ank,
            Coast::None,
            Province::Bla,
            Coast::None,
            false
        ));

        // England -> France connections
        assert!(is_adjacent(
            Province::Eng,
            Coast::None,
            Province::Bre,
            Coast::None,
            true
        ));
        assert!(is_adjacent(
            Province::Eng,
            Coast::None,
            Province::Lon,
            Coast::None,
            true
        ));

        // Italy: Rom-Ven is army-only (Rome faces Tyrrhenian, Venice faces Adriatic)
        assert!(!is_adjacent(
            Province::Rom,
            Coast::None,
            Province::Ven,
            Coast::None,
            true
        ));
        assert!(is_adjacent(
            Province::Rom,
            Coast::None,
            Province::Ven,
            Coast::None,
            false
        ));
    }

    #[test]
    fn adjacency_entry_counts_per_category() {
        let army_only = ADJACENCIES
            .iter()
            .filter(|a| a.army_ok && !a.fleet_ok)
            .count();
        let fleet_only = ADJACENCIES
            .iter()
            .filter(|a| !a.army_ok && a.fleet_ok)
            .count();
        let both_count = ADJACENCIES
            .iter()
            .filter(|a| a.army_ok && a.fleet_ok)
            .count();

        // army only: inland-inland(42) + inland-coastal(70) + split-coast-army(18)
        //   + coastal army-only changed(12) + coastal army-only added(10) + gas-mar(2) = 154
        assert_eq!(army_only, 154, "army-only entry count");
        // fleet only: sea-sea(42) + sea-coastal(144) + split-coast-fleet(22) + sea-coastal-extra(6) = 214
        assert_eq!(fleet_only, 214, "fleet-only entry count");
        // both: 27 pairs * 2 = 54 (was 33 pairs, 6 moved to army-only)
        assert_eq!(both_count, 66, "both-army-and-fleet entry count");
        assert_eq!(army_only + fleet_only + both_count, ADJACENCY_COUNT);
    }

    #[test]
    fn gas_mar_is_army_only_between_them() {
        // Gas-Mar: Gas is coastal, Mar is coastal, but their adjacency comes from
        // the inland-to-coastal section (via Bur connection pattern), but actually
        // gas and mar share a land border. In the Go source, gas-mar is addArmyAdj
        // because it's listed under "inland-to-coastal" (gas is coastal but bur
        // connects them). Actually, gas and mar: both are coastal. gas-mar appears
        // under "Inland-to-coastal adjacencies" in the Go source - addArmyAdj("gas", "mar").
        // This means gas-mar is army-only (no fleet passage between them directly).
        assert!(is_adjacent(
            Province::Gas,
            Coast::None,
            Province::Mar,
            Coast::None,
            false
        ));
        // Gas and Mar are not directly fleet-adjacent (no shared sea border).
        // Fleets go Gas<->MAO and Mar<->GoL instead.
        assert!(!is_adjacent(
            Province::Gas,
            Coast::None,
            Province::Mar,
            Coast::None,
            true
        ));
    }
}
