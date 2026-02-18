//! Kruijswijk guess-and-check resolution algorithm.
//!
//! Faithfully ported from the Go implementation in `api/pkg/diplomacy/resolve.go`.
//! Uses an optimistic initial guess (all moves succeed) and iterates until
//! a consistent resolution is found.

use crate::board::adjacency::is_adjacent_fast as is_adjacent;
use crate::board::order::{Location, Order, OrderUnit};
use crate::board::province::{Coast, Power, Province, ProvinceType, PROVINCE_COUNT};
use crate::board::state::{BoardState, DislodgedUnit as StateDislodgedUnit};
use crate::board::unit::UnitType;

/// The outcome of resolving an order.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OrderResult {
    Succeeded,
    Failed,
    Dislodged,
    Bounced,
    Cut,
}

/// A resolved order paired with its result.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ResolvedOrder {
    pub order: Order,
    pub power: Power,
    pub result: OrderResult,
}

/// A unit that was dislodged during resolution.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DislodgedUnit {
    pub power: Power,
    pub unit_type: UnitType,
    pub province: Province,
    pub coast: Coast,
    pub attacker_from: Province,
}

/// Resolution state for the guess-and-check algorithm.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ResState {
    Unresolved,
    Guessing,
    Resolved,
}

/// Internal tracking for a single order during adjudication.
#[derive(Debug, Clone, Copy)]
struct AdjResult {
    order: Order,
    power: Power,
    state: ResState,
    resolution: bool,
    prov_idx: u8,
    target_idx: u8,
    /// For support: province of the supported unit.
    /// For convoy: province of the convoyed army.
    aux_loc_idx: u8,
    /// For support-move: destination of the supported move.
    /// For convoy: destination of the convoyed army.
    /// For support-hold: NONE_IDX (no target).
    aux_target_idx: u8,
}

const NONE_IDX: u8 = u8::MAX;

/// Reusable resolver that minimizes allocations across repeated calls.
///
/// Allocate once and call `resolve()` on each set of orders.
/// The returned `Vec`s are freshly allocated each call; the internal
/// lookup table and buffer are reused.
pub struct Resolver {
    lookup: [i16; PROVINCE_COUNT],
    adj_buf: Vec<AdjResult>,
}

impl Resolver {
    /// Creates a new resolver with the given initial capacity hint.
    pub fn new(capacity: usize) -> Self {
        Resolver {
            lookup: [-1; PROVINCE_COUNT],
            adj_buf: Vec::with_capacity(capacity),
        }
    }

    /// Resolves a set of movement-phase orders against the board state.
    ///
    /// Each `(Order, Power)` pair represents an order issued by the given power.
    /// Returns the resolved orders with outcomes, and any dislodged units.
    pub fn resolve(
        &mut self,
        orders: &[(Order, Power)],
        state: &BoardState,
    ) -> (Vec<ResolvedOrder>, Vec<DislodgedUnit>) {
        self.init(orders);
        self.adjudicate_all(state);
        self.build_results(orders, state)
    }

    fn init(&mut self, orders: &[(Order, Power)]) {
        self.adj_buf.clear();
        self.lookup.fill(-1);

        for (i, (order, power)) in orders.iter().enumerate() {
            let (prov_idx, target_idx, aux_loc_idx, aux_target_idx) = order_indices(order);

            self.adj_buf.push(AdjResult {
                order: *order,
                power: *power,
                state: ResState::Unresolved,
                resolution: false,
                prov_idx,
                target_idx,
                aux_loc_idx,
                aux_target_idx,
            });

            if prov_idx != NONE_IDX {
                self.lookup[prov_idx as usize] = i as i16;
            }
        }
    }

    fn order_at(&self, prov_idx: u8) -> Option<&AdjResult> {
        if prov_idx == NONE_IDX {
            return None;
        }
        let idx = self.lookup[prov_idx as usize];
        if idx < 0 {
            return None;
        }
        Some(&self.adj_buf[idx as usize])
    }

    fn adjudicate_all(&mut self, state: &BoardState) {
        let n = self.adj_buf.len();
        for i in 0..n {
            let prov_idx = self.adj_buf[i].prov_idx;
            self.adjudicate(prov_idx, state);
        }
    }

    /// Adjudicates the order at `prov_idx` using the Kruijswijk approach:
    /// when encountering a dependency cycle, guess a resolution, check
    /// consistency, and re-resolve if the guess was wrong.
    fn adjudicate(&mut self, prov_idx: u8, state: &BoardState) -> bool {
        if prov_idx == NONE_IDX {
            return false;
        }
        let lookup_idx = self.lookup[prov_idx as usize];
        if lookup_idx < 0 {
            return false;
        }
        let idx = lookup_idx as usize;

        match self.adj_buf[idx].state {
            ResState::Resolved => return self.adj_buf[idx].resolution,
            ResState::Guessing => return self.adj_buf[idx].resolution,
            ResState::Unresolved => {}
        }

        // Mark as guessing with optimistic initial guess (succeeds).
        self.adj_buf[idx].state = ResState::Guessing;
        self.adj_buf[idx].resolution = true;

        let result = self.resolve_order(prov_idx, state);

        // If still guessing and result differs from guess, re-resolve.
        if self.adj_buf[idx].state == ResState::Guessing && result != self.adj_buf[idx].resolution {
            self.adj_buf[idx].resolution = result;
            let result2 = self.resolve_order(prov_idx, state);
            self.adj_buf[idx].state = ResState::Resolved;
            self.adj_buf[idx].resolution = result2;
            return result2;
        }

        self.adj_buf[idx].state = ResState::Resolved;
        self.adj_buf[idx].resolution = result;
        result
    }

    fn resolve_order(&mut self, prov_idx: u8, state: &BoardState) -> bool {
        let idx = self.lookup[prov_idx as usize] as usize;
        match self.adj_buf[idx].order {
            Order::Hold { .. } => true,
            Order::Move { .. } => self.resolve_move(prov_idx, state),
            Order::SupportHold { .. } | Order::SupportMove { .. } => {
                self.resolve_support(prov_idx, state)
            }
            Order::Convoy { .. } => self.resolve_convoy(prov_idx, state),
            _ => false,
        }
    }

    /// Determines if a move order succeeds.
    fn resolve_move(&mut self, prov_idx: u8, state: &BoardState) -> bool {
        let idx = self.lookup[prov_idx as usize] as usize;
        let ar = self.adj_buf[idx];

        // Check convoy requirement.
        if self.needs_convoy(&ar) && !self.has_convoy_path(&ar, state) {
            return false;
        }

        let attack_str = self.attack_strength(prov_idx, state);
        let hold_str = self.hold_strength(ar.target_idx, state);

        if attack_str <= hold_str {
            return false;
        }

        // Head-to-head battle check.
        if let Some(defender) = self.order_at(ar.target_idx) {
            let defender_target = defender.target_idx;
            let is_move = matches!(defender.order, Order::Move { .. });
            if is_move && defender_target == prov_idx {
                let defend_attack = self.attack_strength(ar.target_idx, state);
                if attack_str <= defend_attack {
                    return false;
                }
            }
        }

        // Check prevent strength of all other units moving to the same target.
        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];
            if other.prov_idx == prov_idx {
                continue;
            }
            if matches!(other.order, Order::Move { .. }) && other.target_idx == ar.target_idx {
                let prevent_str = self.prevent_strength(other.prov_idx, state);
                if attack_str <= prevent_str {
                    return false;
                }
            }
        }

        true
    }

    /// Determines if support is successfully given (not cut).
    fn resolve_support(&mut self, prov_idx: u8, state: &BoardState) -> bool {
        let idx = self.lookup[prov_idx as usize] as usize;
        let ar_power = self.adj_buf[idx].power;
        let ar_aux_target = self.adj_buf[idx].aux_target_idx;

        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];

            if !matches!(other.order, Order::Move { .. }) {
                continue;
            }
            if other.target_idx != prov_idx {
                continue;
            }

            // Support cannot be cut by the unit being supported against.
            if ar_aux_target != NONE_IDX && other.prov_idx == ar_aux_target {
                continue;
            }

            // Support cannot be cut by a unit of the same power.
            if other.power == ar_power {
                continue;
            }

            // For a convoyed attack, the convoy must succeed for the cut.
            if self.needs_convoy(&other) && !self.adjudicate(other.prov_idx, state) {
                continue;
            }

            return false;
        }

        true
    }

    /// Determines if a convoy order succeeds (fleet is not dislodged).
    fn resolve_convoy(&mut self, prov_idx: u8, state: &BoardState) -> bool {
        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];
            if matches!(other.order, Order::Move { .. }) && other.target_idx == prov_idx {
                if self.adjudicate(other.prov_idx, state) {
                    return false;
                }
            }
        }
        true
    }

    /// Computes the attack strength of a move order.
    fn attack_strength(&mut self, prov_idx: u8, state: &BoardState) -> i32 {
        let idx = self.lookup[prov_idx as usize] as usize;
        let ar = self.adj_buf[idx];

        if !matches!(ar.order, Order::Move { .. }) {
            return 0;
        }

        let target_prov = Province::from_u8(ar.target_idx);
        let attacker_power = ar.power;

        // A unit cannot attack a province occupied by a unit of the same power
        // UNLESS the occupying unit is moving away.
        if let Some(target_prov) = target_prov {
            if let Some((occ_power, _)) = state.units[target_prov as usize] {
                if occ_power == attacker_power {
                    if let Some(occ_ar) = self.order_at(ar.target_idx) {
                        let is_move = matches!(occ_ar.order, Order::Move { .. });
                        let occ_target = occ_ar.target_idx;
                        if !is_move {
                            return 0;
                        }
                        // If occupier is moving back to our province, strength is 0.
                        if occ_target == prov_idx {
                            return 0;
                        }
                    } else {
                        return 0;
                    }
                }
            }
        }

        let mut strength: i32 = 1;

        // Count successful support for this move.
        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];
            if !matches!(other.order, Order::SupportMove { .. }) {
                continue;
            }
            if other.aux_loc_idx != prov_idx {
                continue;
            }
            if other.aux_target_idx != ar.target_idx {
                continue;
            }
            if self.adjudicate(other.prov_idx, state) {
                strength += 1;
            }
        }

        strength
    }

    /// Computes the hold strength of a province.
    fn hold_strength(&mut self, prov_idx: u8, state: &BoardState) -> i32 {
        if prov_idx == NONE_IDX {
            return 0;
        }
        let lookup_idx = self.lookup[prov_idx as usize];
        if lookup_idx < 0 {
            return 0;
        }
        let idx = lookup_idx as usize;
        let ar = self.adj_buf[idx];

        // If the unit is moving, hold strength depends on whether it succeeds.
        if matches!(ar.order, Order::Move { .. }) {
            if self.adjudicate(prov_idx, state) {
                return 0;
            }
            return 1;
        }

        let mut strength: i32 = 1;

        // Count successful support-hold orders.
        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];
            if !matches!(other.order, Order::SupportHold { .. }) {
                continue;
            }
            if other.aux_loc_idx != prov_idx || other.aux_target_idx != NONE_IDX {
                continue;
            }
            if self.adjudicate(other.prov_idx, state) {
                strength += 1;
            }
        }
        strength
    }

    /// Computes the prevent strength of a move order.
    fn prevent_strength(&mut self, prov_idx: u8, state: &BoardState) -> i32 {
        let idx = self.lookup[prov_idx as usize] as usize;
        let ar = self.adj_buf[idx];

        if !matches!(ar.order, Order::Move { .. }) {
            return 0;
        }

        // Head-to-head: if defender is moving toward us, our prevent strength
        // depends on whether our move succeeds.
        if let Some(defender) = self.order_at(ar.target_idx) {
            let is_move = matches!(defender.order, Order::Move { .. });
            let def_target = defender.target_idx;
            if is_move && def_target == prov_idx {
                if !self.adjudicate(prov_idx, state) {
                    return 0;
                }
            }
        }

        let mut strength: i32 = 1;

        let n = self.adj_buf.len();
        for i in 0..n {
            let other = self.adj_buf[i];
            if !matches!(other.order, Order::SupportMove { .. }) {
                continue;
            }
            if other.aux_loc_idx != prov_idx || other.aux_target_idx != ar.target_idx {
                continue;
            }
            if self.adjudicate(other.prov_idx, state) {
                strength += 1;
            }
        }
        strength
    }

    /// Returns true if the move requires a convoy chain (army moving to non-adjacent province).
    fn needs_convoy(&self, ar: &AdjResult) -> bool {
        let unit = match ar.order {
            Order::Move { unit, dest } => {
                if unit.unit_type != UnitType::Army {
                    return false;
                }
                (unit, dest)
            }
            _ => return false,
        };

        let (unit_ou, dest) = unit;
        !is_adjacent(
            unit_ou.location.province,
            unit_ou.location.coast,
            dest.province,
            dest.coast,
            false,
        )
    }

    /// Checks if there's a successful convoy chain for the given move.
    fn has_convoy_path(&mut self, ar: &AdjResult, state: &BoardState) -> bool {
        let (src_prov, dst_prov) = match ar.order {
            Order::Move { unit, dest } => (unit.location.province, dest.province),
            _ => return false,
        };

        let src_idx = src_prov as u8;
        let tgt_idx = dst_prov as u8;

        let mut visited = [false; PROVINCE_COUNT];
        // Use a fixed-size queue (max 19 sea provinces can be convoy waypoints).
        let mut queue = [0u8; 19];
        let mut queue_head = 0usize;
        let mut queue_tail = 0usize;

        let n = self.adj_buf.len();

        // Find convoy orders matching this move that are adjacent to source.
        for i in 0..n {
            let convoy = self.adj_buf[i];
            if !matches!(convoy.order, Order::Convoy { .. }) {
                continue;
            }
            if convoy.aux_loc_idx != src_idx || convoy.aux_target_idx != tgt_idx {
                continue;
            }
            let convoy_prov = Province::from_u8(convoy.prov_idx);
            if convoy_prov.is_none() {
                continue;
            }
            let cp = convoy_prov.unwrap();
            if cp.province_type() != ProvinceType::Sea {
                continue;
            }
            if is_adjacent(src_prov, Coast::None, cp, Coast::None, true) {
                if self.adjudicate(convoy.prov_idx, state) {
                    visited[convoy.prov_idx as usize] = true;
                    queue[queue_tail] = convoy.prov_idx;
                    queue_tail += 1;
                }
            }
        }

        // BFS through convoy chain.
        while queue_head < queue_tail {
            let current = queue[queue_head];
            queue_head += 1;

            let current_prov = Province::from_u8(current).unwrap();

            // Check if current convoy province is adjacent to destination.
            if is_adjacent(current_prov, Coast::None, dst_prov, Coast::None, true) {
                return true;
            }

            for i in 0..n {
                let convoy = self.adj_buf[i];
                if visited[convoy.prov_idx as usize] {
                    continue;
                }
                if !matches!(convoy.order, Order::Convoy { .. }) {
                    continue;
                }
                if convoy.aux_loc_idx != src_idx || convoy.aux_target_idx != tgt_idx {
                    continue;
                }
                let convoy_prov = Province::from_u8(convoy.prov_idx);
                if convoy_prov.is_none() {
                    continue;
                }
                let cp = convoy_prov.unwrap();
                if cp.province_type() != ProvinceType::Sea {
                    continue;
                }
                if is_adjacent(current_prov, Coast::None, cp, Coast::None, true) {
                    if self.adjudicate(convoy.prov_idx, state) {
                        visited[convoy.prov_idx as usize] = true;
                        if queue_tail < queue.len() {
                            queue[queue_tail] = convoy.prov_idx;
                            queue_tail += 1;
                        }
                    }
                }
            }
        }

        false
    }

    /// Converts internal adjudication state to the external result format.
    fn build_results(
        &self,
        orders: &[(Order, Power)],
        _state: &BoardState,
    ) -> (Vec<ResolvedOrder>, Vec<DislodgedUnit>) {
        let mut results = Vec::with_capacity(orders.len());
        let mut dislodged = Vec::new();

        // Build map of successful moves: target -> source province index.
        let mut successful_move_from = [NONE_IDX; PROVINCE_COUNT];
        for ar in &self.adj_buf {
            if matches!(ar.order, Order::Move { .. }) && ar.resolution {
                if (ar.target_idx as usize) < PROVINCE_COUNT {
                    successful_move_from[ar.target_idx as usize] = ar.prov_idx;
                }
            }
        }

        for (i, (order, power)) in orders.iter().enumerate() {
            let ar = &self.adj_buf[i];

            let mut result = match ar.order {
                Order::Move { .. } => {
                    if ar.resolution {
                        OrderResult::Succeeded
                    } else {
                        OrderResult::Bounced
                    }
                }
                Order::SupportHold { .. } | Order::SupportMove { .. } => {
                    if ar.resolution {
                        OrderResult::Succeeded
                    } else {
                        OrderResult::Cut
                    }
                }
                Order::Convoy { .. } => {
                    if ar.resolution {
                        OrderResult::Succeeded
                    } else {
                        OrderResult::Failed
                    }
                }
                Order::Hold { .. } => OrderResult::Succeeded,
                _ => OrderResult::Succeeded,
            };

            // Check if this unit was dislodged by a successful move.
            let attacker = successful_move_from[ar.prov_idx as usize];
            if attacker != NONE_IDX {
                let was_successful_move = matches!(ar.order, Order::Move { .. }) && ar.resolution;
                if !was_successful_move {
                    result = OrderResult::Dislodged;
                    let (unit_type, coast) = order_unit_info(order);
                    dislodged.push(DislodgedUnit {
                        power: *power,
                        unit_type,
                        province: Province::from_u8(ar.prov_idx).unwrap(),
                        coast,
                        attacker_from: Province::from_u8(attacker).unwrap(),
                    });
                }
            }

            results.push(ResolvedOrder {
                order: *order,
                power: *power,
                result,
            });
        }

        (results, dislodged)
    }
}

/// Applies resolved movement orders to the board state.
///
/// Moves successful units to their destinations and removes dislodged units.
pub fn apply_resolution(
    state: &mut BoardState,
    results: &[ResolvedOrder],
    dislodged: &[DislodgedUnit],
) {
    // First, remove dislodged units from the board so they don't block incoming moves.
    for d in dislodged {
        state.units[d.province as usize] = None;
        state.fleet_coast[d.province as usize] = None;
        state.dislodged[d.province as usize] = Some(StateDislodgedUnit {
            power: d.power,
            unit_type: d.unit_type,
            coast: d.coast,
            attacker_from: d.attacker_from,
        });
    }

    // Then apply successful moves.
    for ro in results {
        if ro.result != OrderResult::Succeeded {
            continue;
        }
        if let Order::Move { unit, dest } = ro.order {
            let src = unit.location.province;
            let dst = dest.province;

            // Move the unit.
            if let Some(unit_data) = state.units[src as usize].take() {
                state.units[dst as usize] = Some(unit_data);
            }

            // Update fleet coast.
            state.fleet_coast[src as usize] = None;
            if dest.coast != Coast::None {
                state.fleet_coast[dst as usize] = Some(dest.coast);
            } else if !dst.has_coasts() {
                state.fleet_coast[dst as usize] = None;
            }
        }
    }
}

/// Extracts province indices from an Order enum for the internal lookup table.
fn order_indices(order: &Order) -> (u8, u8, u8, u8) {
    match *order {
        Order::Hold { unit } => (unit.location.province as u8, NONE_IDX, NONE_IDX, NONE_IDX),
        Order::Move { unit, dest } => (
            unit.location.province as u8,
            dest.province as u8,
            NONE_IDX,
            NONE_IDX,
        ),
        Order::SupportHold { unit, supported } => (
            unit.location.province as u8,
            NONE_IDX,
            supported.location.province as u8,
            NONE_IDX,
        ),
        Order::SupportMove {
            unit,
            supported,
            dest,
        } => (
            unit.location.province as u8,
            NONE_IDX,
            supported.location.province as u8,
            dest.province as u8,
        ),
        Order::Convoy {
            unit,
            convoyed_from,
            convoyed_to,
        } => (
            unit.location.province as u8,
            NONE_IDX,
            convoyed_from.province as u8,
            convoyed_to.province as u8,
        ),
        _ => (NONE_IDX, NONE_IDX, NONE_IDX, NONE_IDX),
    }
}

/// Extracts unit type and coast from an Order.
fn order_unit_info(order: &Order) -> (UnitType, Coast) {
    match *order {
        Order::Hold { unit }
        | Order::Move { unit, .. }
        | Order::SupportHold { unit, .. }
        | Order::SupportMove { unit, .. }
        | Order::Convoy { unit, .. }
        | Order::Retreat { unit, .. }
        | Order::Disband { unit }
        | Order::Build { unit } => (unit.unit_type, unit.location.coast),
        Order::Waive => (UnitType::Army, Coast::None),
    }
}

impl Province {
    /// Converts a u8 index back to a Province, returning None if out of range.
    pub fn from_u8(idx: u8) -> Option<Province> {
        if (idx as usize) < PROVINCE_COUNT {
            // Safety: Province is repr(u8) and we checked bounds.
            Some(unsafe { std::mem::transmute(idx) })
        } else {
            None
        }
    }
}

/// Convenience function that creates a resolver, resolves, and returns results.
pub fn resolve_orders(
    orders: &[(Order, Power)],
    state: &BoardState,
) -> (Vec<ResolvedOrder>, Vec<DislodgedUnit>) {
    let mut resolver = Resolver::new(orders.len());
    resolver.resolve(orders, state)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::order::{Location, OrderUnit};
    use crate::board::province::{Coast, Power, Province};
    use crate::board::state::{BoardState, Phase, Season};
    use crate::board::unit::UnitType;

    fn empty_state() -> BoardState {
        BoardState::empty(1901, Season::Spring, Phase::Movement)
    }

    fn army(province: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(province),
        }
    }

    fn fleet(province: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::new(province),
        }
    }

    fn fleet_coast(province: Province, coast: Coast) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::with_coast(province, coast),
        }
    }

    fn result_for(results: &[ResolvedOrder], province: Province) -> OrderResult {
        for r in results {
            let prov = match r.order {
                Order::Hold { unit } => unit.location.province,
                Order::Move { unit, .. } => unit.location.province,
                Order::SupportHold { unit, .. } => unit.location.province,
                Order::SupportMove { unit, .. } => unit.location.province,
                Order::Convoy { unit, .. } => unit.location.province,
                _ => continue,
            };
            if prov == province {
                return r.result;
            }
        }
        panic!("No result found for {:?}", province);
    }

    // === Basic hold ===

    #[test]
    fn hold_succeeds() {
        let mut state = empty_state();
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = vec![(
            Order::Hold {
                unit: army(Province::Vie),
            },
            Power::Austria,
        )];

        let (results, dislodged) = resolve_orders(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
        assert!(dislodged.is_empty());
    }

    // === Basic move ===

    #[test]
    fn simple_move_succeeds() {
        let mut state = empty_state();
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = vec![(
            Order::Move {
                unit: army(Province::Vie),
                dest: Location::new(Province::Bud),
            },
            Power::Austria,
        )];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Vie), OrderResult::Succeeded);
    }

    #[test]
    fn move_bounces_against_hold() {
        let mut state = empty_state();
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Russia, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Vie),
                    dest: Location::new(Province::Bud),
                },
                Power::Austria,
            ),
            (
                Order::Hold {
                    unit: army(Province::Bud),
                },
                Power::Russia,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Vie), OrderResult::Bounced);
        assert_eq!(result_for(&results, Province::Bud), OrderResult::Succeeded);
    }

    // === Supported attack ===

    #[test]
    fn supported_attack_dislodges() {
        let mut state = empty_state();
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportMove {
                    unit: army(Province::Tri),
                    supported: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Hold {
                    unit: army(Province::Ven),
                },
                Power::Italy,
            ),
        ];

        let (results, dislodged) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Tyr), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Ven), OrderResult::Dislodged);
        assert_eq!(dislodged.len(), 1);
        assert_eq!(dislodged[0].province, Province::Ven);
        assert_eq!(dislodged[0].attacker_from, Province::Tyr);
    }

    // === DATC 6.A.5: Self-support hold not possible ===

    #[test]
    fn datc_6a5_self_support_hold() {
        let mut state = empty_state();
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Hold {
                    unit: army(Province::Ven),
                },
                Power::Italy,
            ),
            (
                Order::SupportMove {
                    unit: army(Province::Tyr),
                    supported: army(Province::Tri),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Tri),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
        ];

        let (results, dislodged) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Tri), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Ven), OrderResult::Dislodged);
        assert_eq!(dislodged.len(), 1);
    }

    // === DATC 6.C.1: Three army circular movement ===

    #[test]
    fn datc_6c1_three_army_circular() {
        let mut state = empty_state();
        state.place_unit(Province::Boh, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Mun, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Sil, Power::Germany, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Boh),
                    dest: Location::new(Province::Mun),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Mun),
                    dest: Location::new(Province::Sil),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Sil),
                    dest: Location::new(Province::Boh),
                },
                Power::Germany,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Boh), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Mun), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Sil), OrderResult::Succeeded);
    }

    // === DATC 6.C.2: Three army circular with support ===

    #[test]
    fn datc_6c2_circular_with_support() {
        let mut state = empty_state();
        state.place_unit(Province::Boh, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Mun, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Sil, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Tyr, Power::Germany, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Boh),
                    dest: Location::new(Province::Mun),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Mun),
                    dest: Location::new(Province::Sil),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Sil),
                    dest: Location::new(Province::Boh),
                },
                Power::Germany,
            ),
            (
                Order::SupportMove {
                    unit: army(Province::Tyr),
                    supported: army(Province::Boh),
                    dest: Location::new(Province::Mun),
                },
                Power::Germany,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Boh), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Mun), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Sil), OrderResult::Succeeded);
    }

    // === DATC 6.D.1: Supported hold prevents dislodgement ===

    #[test]
    fn datc_6d1_supported_hold() {
        let mut state = empty_state();
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Rum, Power::Russia, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Hold {
                    unit: army(Province::Bud),
                },
                Power::Austria,
            ),
            (
                Order::SupportHold {
                    unit: army(Province::Ser),
                    supported: army(Province::Bud),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Rum),
                    dest: Location::new(Province::Bud),
                },
                Power::Russia,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Rum), OrderResult::Bounced);
        assert_eq!(result_for(&results, Province::Bud), OrderResult::Succeeded);
    }

    // === DATC 6.D.2: Move cuts support on hold ===

    #[test]
    fn datc_6d2_move_cuts_support_on_hold() {
        let mut state = empty_state();
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Rum, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Bul, Power::Russia, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Hold {
                    unit: army(Province::Bud),
                },
                Power::Austria,
            ),
            (
                Order::SupportHold {
                    unit: army(Province::Ser),
                    supported: army(Province::Bud),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Rum),
                    dest: Location::new(Province::Bud),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: army(Province::Bul),
                    dest: Location::new(Province::Ser),
                },
                Power::Russia,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Ser), OrderResult::Cut);
        // After support is cut: Rum -> Bud is 1 vs 1, bounces.
        assert_eq!(result_for(&results, Province::Rum), OrderResult::Bounced);
    }

    // === DATC 6.D.3: Move cuts support on move ===

    #[test]
    fn datc_6d3_move_cuts_support_on_move() {
        let mut state = empty_state();
        state.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Rum, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Bul, Power::Turkey, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportMove {
                    unit: army(Province::Ser),
                    supported: army(Province::Bud),
                    dest: Location::new(Province::Rum),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Bud),
                    dest: Location::new(Province::Rum),
                },
                Power::Austria,
            ),
            (
                Order::Hold {
                    unit: army(Province::Rum),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: army(Province::Bul),
                    dest: Location::new(Province::Ser),
                },
                Power::Turkey,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Ser), OrderResult::Cut);
        assert_eq!(result_for(&results, Province::Bud), OrderResult::Bounced);
    }

    // === DATC 6.D.4: Support to hold on unit supporting a hold ===

    #[test]
    fn datc_6d4_mutual_support_hold() {
        let mut state = empty_state();
        state.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Kie, Power::Germany, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Pru, Power::Russia, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportHold {
                    unit: army(Province::Ber),
                    supported: fleet(Province::Kie),
                },
                Power::Germany,
            ),
            (
                Order::SupportHold {
                    unit: fleet(Province::Kie),
                    supported: army(Province::Ber),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Pru),
                    dest: Location::new(Province::Ber),
                },
                Power::Russia,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Pru), OrderResult::Bounced);
    }

    // === DATC 6.D.7: Support can't be cut by target ===

    #[test]
    fn datc_6d7_support_cant_be_cut_by_target() {
        let mut state = empty_state();
        state.place_unit(Province::Mun, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Sil, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::War, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Boh, Power::Austria, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportMove {
                    unit: army(Province::Mun),
                    supported: army(Province::Sil),
                    dest: Location::new(Province::Boh),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Sil),
                    dest: Location::new(Province::Boh),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::War),
                    dest: Location::new(Province::Sil),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: army(Province::Boh),
                    dest: Location::new(Province::Mun),
                },
                Power::Austria,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        // Bohemia's move to Munich cannot cut Munich's support (target exception).
        assert_eq!(result_for(&results, Province::Sil), OrderResult::Succeeded);
    }

    // === DATC 6.E.1: No swap without convoy ===

    #[test]
    fn datc_6e1_no_swap_without_convoy() {
        let mut state = empty_state();
        state.place_unit(Province::Rom, Power::Italy, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Rom),
                    dest: Location::new(Province::Ven),
                },
                Power::Italy,
            ),
            (
                Order::Move {
                    unit: army(Province::Ven),
                    dest: Location::new(Province::Rom),
                },
                Power::Italy,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Rom), OrderResult::Bounced);
        assert_eq!(result_for(&results, Province::Ven), OrderResult::Bounced);
    }

    // === DATC 6.E.2: Supported head-to-head wins ===

    #[test]
    fn datc_6e2_supported_head_to_head() {
        let mut state = empty_state();
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportMove {
                    unit: army(Province::Tri),
                    supported: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Ven),
                    dest: Location::new(Province::Tyr),
                },
                Power::Italy,
            ),
        ];

        let (results, dislodged) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Tyr), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Ven), OrderResult::Dislodged);
        assert_eq!(dislodged.len(), 1);
    }

    // === DATC 6.E.6: Beleaguered garrison ===

    #[test]
    fn datc_6e6_beleaguered_garrison() {
        let mut state = empty_state();
        state.place_unit(Province::Mun, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Bur, Power::France, UnitType::Army, Coast::None);
        state.place_unit(Province::Tyr, Power::Italy, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::Hold {
                    unit: army(Province::Mun),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Bur),
                    dest: Location::new(Province::Mun),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: army(Province::Tyr),
                    dest: Location::new(Province::Mun),
                },
                Power::Italy,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Mun), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Bur), OrderResult::Bounced);
        assert_eq!(result_for(&results, Province::Tyr), OrderResult::Bounced);
    }

    // === DATC 6.F.1: Simple convoy ===

    #[test]
    fn datc_6f1_simple_convoy() {
        let mut state = empty_state();
        state.place_unit(Province::Lon, Power::England, UnitType::Army, Coast::None);
        state.place_unit(Province::Nth, Power::England, UnitType::Fleet, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Lon),
                    dest: Location::new(Province::Nwy),
                },
                Power::England,
            ),
            (
                Order::Convoy {
                    unit: fleet(Province::Nth),
                    convoyed_from: Location::new(Province::Lon),
                    convoyed_to: Location::new(Province::Nwy),
                },
                Power::England,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Lon), OrderResult::Succeeded);
    }

    // === DATC 6.F.2: Disrupted convoy ===

    #[test]
    fn datc_6f2_disrupted_convoy() {
        let mut state = empty_state();
        state.place_unit(Province::Lon, Power::England, UnitType::Army, Coast::None);
        state.place_unit(Province::Nth, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Eng, Power::France, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Bel, Power::France, UnitType::Fleet, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Lon),
                    dest: Location::new(Province::Nwy),
                },
                Power::England,
            ),
            (
                Order::Convoy {
                    unit: fleet(Province::Nth),
                    convoyed_from: Location::new(Province::Lon),
                    convoyed_to: Location::new(Province::Nwy),
                },
                Power::England,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Eng),
                    dest: Location::new(Province::Nth),
                },
                Power::France,
            ),
            (
                Order::SupportMove {
                    unit: fleet(Province::Bel),
                    supported: fleet(Province::Eng),
                    dest: Location::new(Province::Nth),
                },
                Power::France,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Nth), OrderResult::Dislodged);
        assert_eq!(result_for(&results, Province::Lon), OrderResult::Bounced);
    }

    // === Chained moves (regression from Go tests) ===

    #[test]
    fn chained_moves() {
        let mut state = empty_state();
        state.place_unit(Province::Par, Power::France, UnitType::Army, Coast::None);
        state.place_unit(Province::Bre, Power::England, UnitType::Fleet, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Par),
                    dest: Location::new(Province::Bre),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Bre),
                    dest: Location::new(Province::Gas),
                },
                Power::England,
            ),
        ];

        let (results, dislodged) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Par), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Bre), OrderResult::Succeeded);
        assert!(dislodged.is_empty());
    }

    // === Three-way rotation ===

    #[test]
    fn three_way_rotation() {
        let mut state = empty_state();
        state.place_unit(Province::Bre, Power::France, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Eng, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Mao, Power::Germany, UnitType::Fleet, Coast::None);

        let orders = vec![
            (
                Order::Move {
                    unit: fleet(Province::Bre),
                    dest: Location::new(Province::Eng),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Eng),
                    dest: Location::new(Province::Mao),
                },
                Power::England,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Mao),
                    dest: Location::new(Province::Bre),
                },
                Power::Germany,
            ),
        ];

        let (results, _) = resolve_orders(&orders, &state);
        assert_eq!(result_for(&results, Province::Bre), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Eng), OrderResult::Succeeded);
        assert_eq!(result_for(&results, Province::Mao), OrderResult::Succeeded);
    }

    // === Apply resolution ===

    #[test]
    fn apply_resolution_moves_units() {
        let mut state = empty_state();
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = vec![(
            Order::Move {
                unit: army(Province::Vie),
                dest: Location::new(Province::Bud),
            },
            Power::Austria,
        )];

        let (results, dislodged) = resolve_orders(&orders, &state);
        apply_resolution(&mut state, &results, &dislodged);

        assert!(state.units[Province::Vie as usize].is_none());
        assert_eq!(
            state.units[Province::Bud as usize],
            Some((Power::Austria, UnitType::Army))
        );
    }

    #[test]
    fn apply_resolution_dislodges_unit() {
        let mut state = empty_state();
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        let orders = vec![
            (
                Order::SupportMove {
                    unit: army(Province::Tri),
                    supported: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Tyr),
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
            (
                Order::Hold {
                    unit: army(Province::Ven),
                },
                Power::Italy,
            ),
        ];

        let (results, dislodged) = resolve_orders(&orders, &state);
        apply_resolution(&mut state, &results, &dislodged);

        // Austria should be in Venice now.
        assert_eq!(
            state.units[Province::Ven as usize],
            Some((Power::Austria, UnitType::Army))
        );
        // Italy should be dislodged.
        assert!(state.dislodged[Province::Ven as usize].is_some());
        let d = state.dislodged[Province::Ven as usize].unwrap();
        assert_eq!(d.power, Power::Italy);
        assert_eq!(d.attacker_from, Province::Tyr);
    }

    // === Reusable resolver ===

    #[test]
    fn resolver_can_be_reused() {
        let mut resolver = Resolver::new(8);

        // First resolution
        let mut state = empty_state();
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        let orders1 = vec![(
            Order::Move {
                unit: army(Province::Vie),
                dest: Location::new(Province::Bud),
            },
            Power::Austria,
        )];
        let (results1, _) = resolver.resolve(&orders1, &state);
        assert_eq!(result_for(&results1, Province::Vie), OrderResult::Succeeded);

        // Second resolution (different orders, same resolver)
        let mut state2 = empty_state();
        state2.place_unit(Province::Lon, Power::England, UnitType::Fleet, Coast::None);
        let orders2 = vec![(
            Order::Hold {
                unit: fleet(Province::Lon),
            },
            Power::England,
        )];
        let (results2, _) = resolver.resolve(&orders2, &state2);
        assert_eq!(result_for(&results2, Province::Lon), OrderResult::Succeeded);
    }
}
