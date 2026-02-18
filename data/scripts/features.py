#!/usr/bin/env python3
"""Feature extraction for Diplomacy neural network training.

Converts parsed game data (from parse.py output) into training-ready
numpy arrays. Produces board state tensors, order labels, value labels,
and a GNN adjacency matrix.

Board state tensor layout: [81, 47] per position
  81 areas = 75 provinces + 6 bicoastal variants (spa/nc, spa/sc, stp/nc, stp/sc, bul/ec, bul/sc)
  47 features per area (see FEATURE_SPEC below)

Usage:
    python3 scripts/features.py [--input processed/games.jsonl] [--output-dir processed/features]
"""

import argparse
import hashlib
import json
import logging
import re
import sys
from collections import defaultdict
from pathlib import Path

import numpy as np

from province_map import POWER_NAMES, PROVINCES, PROVINCE_SET, SPLIT_COASTS

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

PROCESSED_DIR = Path(__file__).resolve().parent.parent / "processed"

# ---- Area index: 75 provinces + 6 bicoastal variants = 81 ----
# Sorted alphabetically for deterministic ordering.
BICOASTAL_VARIANTS = [
    "bul/ec", "bul/sc", "spa/nc", "spa/sc", "stp/nc", "stp/sc",
]
AREAS = sorted(PROVINCES) + sorted(BICOASTAL_VARIANTS)
AREA_INDEX = {area: i for i, area in enumerate(AREAS)}
NUM_AREAS = len(AREAS)  # 81

# ---- Power index ----
POWER_INDEX = {p: i for i, p in enumerate(POWER_NAMES)}
NUM_POWERS = len(POWER_NAMES)  # 7

# ---- Province type classification ----
INLAND_PROVINCES = {
    "boh", "bud", "bur", "gal", "mos", "mun", "par", "ruh",
    "ser", "sil", "tyr", "ukr", "vie", "war",
}
SEA_PROVINCES = {
    "adr", "aeg", "bal", "bar", "bla", "bot", "eas", "eng",
    "gol", "hel", "ion", "iri", "mao", "nao", "nrg", "nth",
    "ska", "tys", "wes",
}

# ---- Feature layout (47 features per area) ----
# [0:3]   Unit present: [army, fleet, empty]
# [3:11]  Unit owner: [A, E, F, G, I, R, T, none]
# [11:20] SC owner: [A, E, F, G, I, R, T, neutral, none]
# [20]    Can build (bool)
# [21]    Can disband (bool)
# [22:25] Dislodged unit: [army, fleet, none]
# [25:33] Dislodged owner: [A, E, F, G, I, R, T, none]
# [33:36] Province type: [land, sea, coast]
# [36:39] Prev unit present: [army, fleet, empty]
# [39:47] Prev unit owner: [A, E, F, G, I, R, T, none]
NUM_FEATURES = 47

FEAT_UNIT_TYPE = 0        # 3 slots
FEAT_UNIT_OWNER = 3       # 8 slots
FEAT_SC_OWNER = 11        # 9 slots
FEAT_CAN_BUILD = 20       # 1 slot
FEAT_CAN_DISBAND = 21     # 1 slot
FEAT_DISLODGED_TYPE = 22  # 3 slots
FEAT_DISLODGED_OWNER = 25 # 8 slots
FEAT_PROVINCE_TYPE = 33   # 3 slots
FEAT_PREV_UNIT_TYPE = 36  # 3 slots
FEAT_PREV_UNIT_OWNER = 39 # 8 slots

# ---- Adjacency data (ported from map_data.go) ----
# Each tuple is (from, to) indicating bidirectional adjacency.
# For the GNN, we build a binary adjacency matrix over the 81 areas.
# An edge exists between two areas if any unit type can move between them.
ADJACENCY_PAIRS = [
    # Sea-to-sea
    ("adr", "ion"), ("aeg", "eas"), ("aeg", "ion"), ("bal", "bot"),
    ("eng", "iri"), ("eng", "mao"), ("eng", "nth"), ("gol", "tys"),
    ("gol", "wes"), ("hel", "nth"), ("ion", "eas"), ("ion", "tys"),
    ("iri", "mao"), ("iri", "nao"), ("mao", "nao"), ("mao", "wes"),
    ("nao", "nrg"), ("nth", "nrg"), ("nth", "ska"), ("nrg", "bar"),
    ("tys", "wes"),
    # Sea-to-coastal
    ("adr", "alb"), ("adr", "apu"), ("adr", "tri"), ("adr", "ven"),
    ("aeg", "con"), ("aeg", "gre"), ("aeg", "smy"),
    ("bal", "ber"), ("bal", "den"), ("bal", "kie"), ("bal", "lvn"),
    ("bal", "pru"), ("bal", "swe"),
    ("bar", "nwy"),
    ("bla", "ank"), ("bla", "arm"), ("bla", "con"), ("bla", "rum"), ("bla", "sev"),
    ("bot", "fin"), ("bot", "lvn"), ("bot", "swe"),
    ("eas", "smy"), ("eas", "syr"),
    ("eng", "bel"), ("eng", "bre"), ("eng", "lon"), ("eng", "pic"), ("eng", "wal"),
    ("gol", "mar"), ("gol", "pie"), ("gol", "tus"),
    ("hel", "den"), ("hel", "hol"), ("hel", "kie"),
    ("ion", "alb"), ("ion", "apu"), ("ion", "gre"), ("ion", "nap"), ("ion", "tun"),
    ("iri", "lvp"), ("iri", "wal"),
    ("mao", "bre"), ("mao", "gas"), ("mao", "naf"), ("mao", "por"),
    ("nao", "cly"), ("nao", "lvp"),
    ("nth", "bel"), ("nth", "den"), ("nth", "edi"), ("nth", "hol"),
    ("nth", "lon"), ("nth", "nwy"), ("nth", "yor"),
    ("nrg", "cly"), ("nrg", "edi"), ("nrg", "nwy"),
    ("ska", "den"), ("ska", "nwy"), ("ska", "swe"),
    ("tys", "nap"), ("tys", "rom"), ("tys", "tun"), ("tys", "tus"),
    ("wes", "naf"), ("wes", "tun"),
    # Inland-to-inland (army only)
    ("boh", "gal"), ("boh", "mun"), ("boh", "sil"), ("boh", "tyr"), ("boh", "vie"),
    ("bud", "gal"), ("bud", "vie"),
    ("bur", "mun"), ("bur", "par"), ("bur", "ruh"),
    ("gal", "sil"), ("gal", "ukr"), ("gal", "vie"), ("gal", "war"),
    ("mos", "ukr"), ("mos", "war"),
    ("mun", "ruh"), ("mun", "sil"), ("mun", "tyr"),
    ("sil", "war"),
    ("tyr", "vie"),
    ("ukr", "war"),
    # Inland-to-coastal (army only)
    ("bud", "rum"), ("bud", "ser"), ("bud", "tri"),
    ("bur", "bel"), ("bur", "gas"), ("bur", "mar"), ("bur", "pic"),
    ("gal", "rum"),
    ("gas", "mar"),
    ("mos", "lvn"), ("mos", "sev"), ("mos", "stp"),
    ("mun", "ber"), ("mun", "kie"),
    ("par", "bre"), ("par", "gas"), ("par", "pic"),
    ("ruh", "bel"), ("ruh", "hol"), ("ruh", "kie"),
    ("ser", "alb"), ("ser", "bul"), ("ser", "gre"), ("ser", "rum"), ("ser", "tri"),
    ("sil", "ber"), ("sil", "pru"),
    ("tyr", "pie"), ("tyr", "tri"), ("tyr", "ven"),
    ("ukr", "rum"), ("ukr", "sev"),
    ("vie", "tri"),
    ("war", "lvn"), ("war", "pru"),
    # Coastal-to-coastal (both army and fleet)
    ("alb", "gre"), ("alb", "tri"),
    ("ank", "arm"), ("ank", "con"),
    ("apu", "nap"), ("apu", "ven"),
    ("bel", "hol"), ("bel", "pic"),
    ("ber", "kie"), ("ber", "pru"),
    ("bre", "gas"), ("bre", "pic"),
    ("cly", "edi"), ("cly", "lvp"),
    ("con", "smy"),
    ("den", "kie"), ("den", "swe"),
    ("hol", "kie"),
    ("edi", "lvp"), ("edi", "yor"),
    ("fin", "nwy"), ("fin", "swe"),
    ("lon", "wal"), ("lon", "yor"),
    ("lvp", "wal"), ("lvp", "yor"),
    ("mar", "pie"),
    ("naf", "tun"),
    ("nwy", "swe"),
    ("pie", "tus"), ("pie", "ven"),
    ("pru", "lvn"),
    ("apu", "rom"), ("rom", "nap"), ("rom", "tus"), ("rom", "ven"),
    ("sev", "arm"), ("sev", "rum"),
    ("ank", "smy"), ("smy", "arm"), ("smy", "syr"),
    ("tri", "ven"),
    ("wal", "yor"),
    # Coastal-to-split-coast (fleet only - but still adjacent)
    ("con", "bul"), ("gre", "bul"), ("rum", "bul"),
    ("gas", "spa"), ("mar", "spa"), ("por", "spa"),
    ("fin", "stp"), ("lvn", "stp"), ("nwy", "stp"),
    # Sea-to-split-coast
    ("aeg", "bul"), ("bla", "bul"),
    ("mao", "spa"), ("gol", "spa"), ("wes", "spa"),
    ("bar", "stp"), ("bot", "stp"),
]


def build_adjacency_matrix() -> np.ndarray:
    """Build an 81x81 binary adjacency matrix over all areas.

    Bicoastal variants (e.g. bul/ec) inherit the adjacency of their base
    province. Each variant is also connected to its base province.
    """
    adj = np.zeros((NUM_AREAS, NUM_AREAS), dtype=np.float32)

    for a, b in ADJACENCY_PAIRS:
        if a in AREA_INDEX and b in AREA_INDEX:
            i, j = AREA_INDEX[a], AREA_INDEX[b]
            adj[i, j] = 1.0
            adj[j, i] = 1.0

    # Connect bicoastal variants to their base province and propagate
    # adjacencies from base to variants.
    for base, coasts in SPLIT_COASTS.items():
        base_idx = AREA_INDEX[base]
        for coast in coasts:
            variant = f"{base}/{coast}"
            var_idx = AREA_INDEX[variant]
            # Variant is adjacent to base
            adj[base_idx, var_idx] = 1.0
            adj[var_idx, base_idx] = 1.0
            # Variant inherits all base adjacencies
            for k in range(NUM_AREAS):
                if adj[base_idx, k] == 1.0:
                    adj[var_idx, k] = 1.0
                    adj[k, var_idx] = 1.0

    # Self-loops (useful for GNN message passing)
    np.fill_diagonal(adj, 1.0)

    return adj


def _province_type_vec(prov: str) -> list[int]:
    """Return [land, sea, coast] one-hot for a province base code."""
    base = prov.split("/")[0]
    if base in INLAND_PROVINCES:
        return [1, 0, 0]
    if base in SEA_PROVINCES:
        return [0, 1, 0]
    return [0, 0, 1]  # coastal (includes split-coast)


def _parse_unit_string(unit_str: str) -> tuple[str, str, str]:
    """Parse 'A par' or 'F spa/nc' into (unit_type, province, coast).

    Returns ('A'|'F', province_code, coast_or_empty).
    """
    parts = unit_str.strip().split()
    if len(parts) < 2:
        return ("", "", "")
    utype = parts[0].upper()
    loc = parts[1].lower()
    if "/" in loc:
        base, coast = loc.split("/", 1)
        return (utype, base, coast)
    return (utype, loc, "")


def encode_board_state(phase: dict, prev_phase: dict | None = None) -> np.ndarray:
    """Encode a single game phase into an [81, 47] feature tensor.

    Args:
        phase: A phase dict from the parsed game data.
        prev_phase: The previous phase. When provided, channels 36..47
            encode the previous turn's unit positions (type + owner).
            When None (e.g. first turn), those channels are zero-filled
            with "empty" markers.

    Returns:
        np.ndarray of shape (81, 47) with float32 values.
    """
    tensor = np.zeros((NUM_AREAS, NUM_FEATURES), dtype=np.float32)

    # Static province type features (always the same)
    for area, idx in AREA_INDEX.items():
        ptype = _province_type_vec(area)
        tensor[idx, FEAT_PROVINCE_TYPE:FEAT_PROVINCE_TYPE + 3] = ptype

    # Unit positions
    units = phase.get("units", {})
    for power, unit_list in units.items():
        power_idx = POWER_INDEX.get(power)
        if power_idx is None:
            continue
        for unit_str in unit_list:
            utype, prov, coast = _parse_unit_string(unit_str)
            if not prov or prov not in PROVINCE_SET:
                continue
            # Set on base province
            area_idx = AREA_INDEX.get(prov)
            if area_idx is None:
                continue
            _set_unit_features(tensor, area_idx, utype, power_idx)
            # Also set on the specific coast variant if applicable
            if coast:
                variant = f"{prov}/{coast}"
                var_idx = AREA_INDEX.get(variant)
                if var_idx is not None:
                    _set_unit_features(tensor, var_idx, utype, power_idx)

    # Mark empty units
    for idx in range(NUM_AREAS):
        if tensor[idx, FEAT_UNIT_TYPE] == 0 and tensor[idx, FEAT_UNIT_TYPE + 1] == 0:
            tensor[idx, FEAT_UNIT_TYPE + 2] = 1.0  # empty
            tensor[idx, FEAT_UNIT_OWNER + NUM_POWERS] = 1.0  # owner = none

    # Supply center ownership
    centers = phase.get("centers", {})
    owned_centers: set[str] = set()
    for power, center_list in centers.items():
        power_idx = POWER_INDEX.get(power)
        if power_idx is None:
            continue
        for prov in center_list:
            if prov not in PROVINCE_SET:
                continue
            owned_centers.add(prov)
            area_idx = AREA_INDEX.get(prov)
            if area_idx is None:
                continue
            tensor[area_idx, FEAT_SC_OWNER + power_idx] = 1.0
            # Also mark on bicoastal variants
            if prov in SPLIT_COASTS:
                for coast in SPLIT_COASTS[prov]:
                    var_idx = AREA_INDEX.get(f"{prov}/{coast}")
                    if var_idx is not None:
                        tensor[var_idx, FEAT_SC_OWNER + power_idx] = 1.0

    # Mark neutral and non-SC areas
    all_sc = _get_all_supply_centers()
    for area, idx in AREA_INDEX.items():
        base = area.split("/")[0]
        if base in all_sc:
            if base not in owned_centers:
                tensor[idx, FEAT_SC_OWNER + NUM_POWERS] = 1.0  # neutral
        else:
            tensor[idx, FEAT_SC_OWNER + NUM_POWERS + 1] = 1.0  # none (not an SC)

    # Can build / can disband (only meaningful in adjustment phases, but encode always)
    phase_type = phase.get("type", "movement")
    if phase_type == "adjustment":
        _encode_build_disband(tensor, units, centers)

    # Dislodged units (from results)
    results = phase.get("results", {})
    _encode_dislodged(tensor, results, units)

    # Previous-state unit features (channels 36..47)
    _encode_prev_state(tensor, prev_phase)

    return tensor


def _set_unit_features(tensor: np.ndarray, area_idx: int, utype: str, power_idx: int):
    """Set unit type and owner features for an area."""
    if utype == "A":
        tensor[area_idx, FEAT_UNIT_TYPE] = 1.0
    elif utype == "F":
        tensor[area_idx, FEAT_UNIT_TYPE + 1] = 1.0
    tensor[area_idx, FEAT_UNIT_OWNER + power_idx] = 1.0


def _set_prev_unit_features(tensor: np.ndarray, area_idx: int, utype: str, power_idx: int):
    """Set previous-turn unit type and owner features for an area."""
    if utype == "A":
        tensor[area_idx, FEAT_PREV_UNIT_TYPE] = 1.0
    elif utype == "F":
        tensor[area_idx, FEAT_PREV_UNIT_TYPE + 1] = 1.0
    tensor[area_idx, FEAT_PREV_UNIT_OWNER + power_idx] = 1.0


def _encode_prev_state(tensor: np.ndarray, prev_phase: dict | None):
    """Encode previous-turn unit positions into channels 36..47.

    When prev_phase is None (first turn), marks all areas as empty.
    """
    if prev_phase is not None:
        prev_units = prev_phase.get("units", {})
        for power, unit_list in prev_units.items():
            power_idx = POWER_INDEX.get(power)
            if power_idx is None:
                continue
            for unit_str in unit_list:
                utype, prov, coast = _parse_unit_string(unit_str)
                if not prov or prov not in PROVINCE_SET:
                    continue
                area_idx = AREA_INDEX.get(prov)
                if area_idx is None:
                    continue
                _set_prev_unit_features(tensor, area_idx, utype, power_idx)
                # Also set on coast variant if applicable
                if coast:
                    variant = f"{prov}/{coast}"
                    var_idx = AREA_INDEX.get(variant)
                    if var_idx is not None:
                        _set_prev_unit_features(tensor, var_idx, utype, power_idx)

    # Mark empty areas in previous-state channels
    for idx in range(NUM_AREAS):
        if tensor[idx, FEAT_PREV_UNIT_TYPE] == 0 and tensor[idx, FEAT_PREV_UNIT_TYPE + 1] == 0:
            tensor[idx, FEAT_PREV_UNIT_TYPE + 2] = 1.0  # empty
            tensor[idx, FEAT_PREV_UNIT_OWNER + NUM_POWERS] = 1.0  # owner = none


def _get_all_supply_centers() -> frozenset:
    """Return the set of all supply center provinces on the standard map."""
    return frozenset([
        "ank", "bel", "ber", "bre", "bud", "bul", "con", "den", "edi",
        "gre", "hol", "kie", "lon", "lvp", "mar", "mos", "mun", "nap",
        "nwy", "par", "por", "rom", "rum", "ser", "sev", "smy", "spa",
        "stp", "swe", "tri", "tun", "ven", "vie", "war",
    ])


def _encode_build_disband(
    tensor: np.ndarray,
    units: dict[str, list[str]],
    centers: dict[str, list[str]],
):
    """Encode can_build and can_disband flags during adjustment phases."""
    home_centers = {
        "austria": {"vie", "bud", "tri"},
        "england": {"lon", "edi", "lvp"},
        "france": {"par", "bre", "mar"},
        "germany": {"ber", "mun", "kie"},
        "italy": {"rom", "nap", "ven"},
        "russia": {"mos", "sev", "stp", "war"},
        "turkey": {"ank", "con", "smy"},
    }

    for power in POWER_NAMES:
        num_units = len(units.get(power, []))
        num_centers = len(centers.get(power, []))
        power_home = home_centers.get(power, set())
        owned = set(centers.get(power, []))

        if num_centers > num_units:
            # Can build on owned home centers that are unoccupied
            occupied = set()
            for p, ulist in units.items():
                for u in ulist:
                    _, prov, _ = _parse_unit_string(u)
                    occupied.add(prov)
            for hc in power_home & owned:
                if hc not in occupied:
                    idx = AREA_INDEX.get(hc)
                    if idx is not None:
                        tensor[idx, FEAT_CAN_BUILD] = 1.0
        elif num_units > num_centers:
            # Must disband: mark all power's units
            for u in units.get(power, []):
                _, prov, _ = _parse_unit_string(u)
                idx = AREA_INDEX.get(prov)
                if idx is not None:
                    tensor[idx, FEAT_CAN_DISBAND] = 1.0


def _encode_dislodged(
    tensor: np.ndarray,
    results: dict[str, list[str]],
    units: dict[str, list[str]],
):
    """Encode dislodged unit features from phase results.

    A unit is dislodged if its result list contains a non-empty entry
    that indicates retreat is needed (typically contains the province
    name the unit was dislodged from).
    """
    # Build reverse map: unit_str -> power
    unit_to_power: dict[str, str] = {}
    for power, ulist in units.items():
        for u in ulist:
            unit_to_power[u] = power

    for unit_str, res_list in results.items():
        # Check if dislodged (non-empty result that isn't just "")
        dislodged = any(r.strip() for r in res_list if r.strip())
        if not dislodged:
            continue

        utype, prov, coast = _parse_unit_string(unit_str)
        if not prov or prov not in PROVINCE_SET:
            continue
        area_idx = AREA_INDEX.get(prov)
        if area_idx is None:
            continue

        # Dislodged type
        if utype == "A":
            tensor[area_idx, FEAT_DISLODGED_TYPE] = 1.0
        elif utype == "F":
            tensor[area_idx, FEAT_DISLODGED_TYPE + 1] = 1.0

        # Dislodged owner
        power = unit_to_power.get(unit_str)
        if power:
            pidx = POWER_INDEX.get(power)
            if pidx is not None:
                tensor[area_idx, FEAT_DISLODGED_OWNER + pidx] = 1.0

    # Mark non-dislodged slots
    for idx in range(NUM_AREAS):
        if tensor[idx, FEAT_DISLODGED_TYPE] == 0 and tensor[idx, FEAT_DISLODGED_TYPE + 1] == 0:
            tensor[idx, FEAT_DISLODGED_TYPE + 2] = 1.0  # none
            tensor[idx, FEAT_DISLODGED_OWNER + NUM_POWERS] = 1.0  # owner = none


# ---- Order encoding ----

ORDER_TYPES = ["hold", "move", "support", "convoy", "retreat", "build", "disband"]
ORDER_TYPE_INDEX = {t: i for i, t in enumerate(ORDER_TYPES)}

ORDER_RE = re.compile(
    r"^(?P<utype>[AF])\s+(?P<loc>\S+)"
    r"(?:\s+(?P<action>[-HSCBDR]|VIA)"
    r"(?:\s+(?P<rest>.+))?)?\s*$",
    re.IGNORECASE,
)


def parse_order(order_str: str) -> dict | None:
    """Parse an order string into structured components.

    Examples:
        "A par - bur"      -> {type: "move", unit: "A", src: "par", dst: "bur"}
        "A par H"          -> {type: "hold", unit: "A", src: "par"}
        "A par S A mar - bur" -> {type: "support", unit: "A", src: "par", ...}
        "F lon C A yor - bel" -> {type: "convoy", unit: "F", src: "lon", ...}
    """
    s = order_str.strip()
    if not s:
        return None

    tokens = s.split()
    if len(tokens) < 2:
        return None

    utype = tokens[0].upper()
    src = tokens[1].lower().split("/")[0]

    if len(tokens) == 2 or (len(tokens) >= 3 and tokens[2].upper() == "H"):
        return {"type": "hold", "unit": utype, "src": src}

    action = tokens[2].upper()
    if action == "-":
        # Move order
        if len(tokens) >= 4:
            dst = tokens[3].lower().split("/")[0]
            return {"type": "move", "unit": utype, "src": src, "dst": dst}
    elif action == "S":
        return {"type": "support", "unit": utype, "src": src, "rest": tokens[3:]}
    elif action == "C":
        return {"type": "convoy", "unit": utype, "src": src, "rest": tokens[3:]}
    elif action == "R":
        # Retreat
        if len(tokens) >= 4:
            dst = tokens[3].lower().split("/")[0]
            return {"type": "retreat", "unit": utype, "src": src, "dst": dst}
    elif action == "B":
        return {"type": "build", "unit": utype, "src": src}
    elif action == "D":
        return {"type": "disband", "unit": utype, "src": src}

    return {"type": "hold", "unit": utype, "src": src}


def encode_order_label(order_str: str) -> np.ndarray:
    """Encode a single order as a feature vector.

    Returns a vector of length (7 + 81 + 81) = 169:
      [0:7]    order type one-hot
      [7:88]   source area one-hot
      [88:169] destination area one-hot (zeros for hold/build/disband)
    """
    vec_len = len(ORDER_TYPES) + NUM_AREAS + NUM_AREAS
    vec = np.zeros(vec_len, dtype=np.float32)

    parsed = parse_order(order_str)
    if parsed is None:
        return vec

    otype = parsed.get("type", "hold")
    otype_idx = ORDER_TYPE_INDEX.get(otype, 0)
    vec[otype_idx] = 1.0

    src = parsed.get("src", "")
    src_idx = AREA_INDEX.get(src)
    if src_idx is not None:
        vec[len(ORDER_TYPES) + src_idx] = 1.0

    dst = parsed.get("dst", "")
    if dst:
        dst_idx = AREA_INDEX.get(dst)
        if dst_idx is not None:
            vec[len(ORDER_TYPES) + NUM_AREAS + dst_idx] = 1.0

    return vec


# ---- Value labels ----

def encode_value_labels(outcome: dict) -> dict[str, np.ndarray]:
    """Encode game outcome as value labels per power.

    Returns a dict mapping power name to a vector of length 4:
      [0] final SC count (normalized to [0, 1] by dividing by 34)
      [1] win indicator (1.0 if solo victory)
      [2] draw indicator (1.0 if draw)
      [3] survival indicator (1.0 if survived or drew)
    """
    labels = {}
    for power in POWER_NAMES:
        info = outcome.get(power, {"centers": 0, "result": "eliminated"})
        sc = info.get("centers", 0)
        result = info.get("result", "eliminated")

        vec = np.zeros(4, dtype=np.float32)
        vec[0] = sc / 34.0
        if result == "solo":
            vec[1] = 1.0
            vec[3] = 1.0
        elif result == "draw":
            vec[2] = 1.0
            vec[3] = 1.0
        elif result == "survive":
            vec[3] = 1.0
        labels[power] = vec
    return labels


# ---- Dataset construction ----

def extract_game_samples(game: dict) -> list[dict]:
    """Extract training samples from a single parsed game.

    Each movement phase produces one sample per power that has orders.
    Returns a list of sample dicts with keys:
      board: np.ndarray [81, 47]
      orders: list[np.ndarray] (order label vectors)
      value: np.ndarray [4]
      power: str
      phase_name: str
      game_id: str
      year: int
    """
    samples = []
    outcome = game.get("outcome", {})
    value_labels = encode_value_labels(outcome)
    phases = game.get("phases", [])
    game_id = game.get("game_id", "")

    prev_phase = None
    for phase in phases:
        if phase.get("type") != "movement":
            prev_phase = phase
            continue

        board = encode_board_state(phase, prev_phase)
        orders = phase.get("orders", {})

        for power, order_list in orders.items():
            if not order_list:
                continue
            if power not in POWER_INDEX:
                continue

            order_vecs = [encode_order_label(o) for o in order_list]
            if not order_vecs:
                continue

            samples.append({
                "board": board,
                "orders": order_vecs,
                "value": value_labels.get(power, np.zeros(4, dtype=np.float32)),
                "power": power,
                "power_idx": POWER_INDEX[power],
                "phase_name": phase.get("name", ""),
                "game_id": game_id,
                "year": phase.get("year", 0),
            })

        prev_phase = phase

    return samples


def split_dataset(
    samples: list[dict],
    train_ratio: float = 0.90,
    val_ratio: float = 0.05,
    seed: int = 42,
) -> tuple[list[dict], list[dict], list[dict]]:
    """Split samples into train/val/test sets.

    Uses game_id-based splitting (all samples from the same game go
    to the same split) with a deterministic hash for reproducibility.
    """
    game_buckets: dict[str, list[dict]] = defaultdict(list)
    for s in samples:
        game_buckets[s["game_id"]].append(s)

    # Deterministic assignment by hashing game_id
    train, val, test = [], [], []
    for gid, group in sorted(game_buckets.items()):
        h = int(hashlib.md5(f"{seed}_{gid}".encode()).hexdigest(), 16) % 10000
        if h < int(train_ratio * 10000):
            train.extend(group)
        elif h < int((train_ratio + val_ratio) * 10000):
            val.extend(group)
        else:
            test.extend(group)

    return train, val, test


def save_dataset(samples: list[dict], output_path: Path):
    """Save a list of samples to a compressed .npz file.

    Saves:
      boards: [N, 81, 47]
      order_types: [N, max_orders, 169] (padded)
      values: [N, 4]
      power_indices: [N]
      years: [N]
    """
    if not samples:
        log.warning("No samples to save for %s", output_path)
        return

    n = len(samples)
    boards = np.stack([s["board"] for s in samples])

    # Pad orders to max length in this split
    max_orders = max(len(s["orders"]) for s in samples)
    order_dim = len(ORDER_TYPES) + NUM_AREAS + NUM_AREAS  # 169
    order_labels = np.zeros((n, max_orders, order_dim), dtype=np.float32)
    order_masks = np.zeros((n, max_orders), dtype=np.float32)
    for i, s in enumerate(samples):
        for j, ov in enumerate(s["orders"]):
            order_labels[i, j] = ov
            order_masks[i, j] = 1.0

    values = np.stack([s["value"] for s in samples])
    power_indices = np.array([s["power_idx"] for s in samples], dtype=np.int32)
    years = np.array([s["year"] for s in samples], dtype=np.int32)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    np.savez_compressed(
        output_path,
        boards=boards,
        order_labels=order_labels,
        order_masks=order_masks,
        values=values,
        power_indices=power_indices,
        years=years,
    )
    size_mb = output_path.stat().st_size / (1024 * 1024)
    log.info("Saved %d samples to %s (%.1f MB)", n, output_path.name, size_mb)


def main():
    parser = argparse.ArgumentParser(
        description="Extract features from parsed Diplomacy games for neural network training"
    )
    parser.add_argument(
        "--input", type=Path,
        default=PROCESSED_DIR / "games.jsonl",
        help="Input JSONL file from parse.py",
    )
    parser.add_argument(
        "--output-dir", type=Path,
        default=PROCESSED_DIR / "features",
        help="Output directory for .npz files",
    )
    parser.add_argument(
        "--limit", type=int, default=0,
        help="Limit number of games to process (0 = unlimited)",
    )
    parser.add_argument(
        "--seed", type=int, default=42,
        help="Random seed for reproducible train/val/test split",
    )
    args = parser.parse_args()

    if not args.input.exists():
        log.error("Input file not found: %s. Run parse.py first.", args.input)
        sys.exit(1)

    # Load games
    log.info("Loading games from %s ...", args.input)
    games = []
    with open(args.input, "r") as f:
        for idx, line in enumerate(f):
            line = line.strip()
            if not line:
                continue
            try:
                games.append(json.loads(line))
            except json.JSONDecodeError:
                continue
            if args.limit > 0 and len(games) >= args.limit:
                break
    log.info("Loaded %d games", len(games))

    # Extract samples
    log.info("Extracting features ...")
    all_samples = []
    for i, game in enumerate(games):
        samples = extract_game_samples(game)
        all_samples.extend(samples)
        if (i + 1) % 5000 == 0:
            log.info("  ... processed %d games (%d samples)", i + 1, len(all_samples))

    log.info("Extracted %d total samples from %d games", len(all_samples), len(games))

    if not all_samples:
        log.error("No samples extracted. Check input data.")
        sys.exit(1)

    # Split
    train, val, test = split_dataset(all_samples, seed=args.seed)
    log.info("Split: train=%d, val=%d, test=%d", len(train), len(val), len(test))

    # Save
    out = args.output_dir
    save_dataset(train, out / "train.npz")
    save_dataset(val, out / "val.npz")
    save_dataset(test, out / "test.npz")

    # Save adjacency matrix
    adj = build_adjacency_matrix()
    adj_path = out / "adjacency.npy"
    np.save(adj_path, adj)
    log.info("Saved adjacency matrix (%d x %d) to %s", adj.shape[0], adj.shape[1], adj_path.name)

    # Save metadata
    meta = {
        "num_areas": NUM_AREAS,
        "num_features": NUM_FEATURES,
        "area_index": AREA_INDEX,
        "power_index": POWER_INDEX,
        "order_types": ORDER_TYPES,
        "total_samples": len(all_samples),
        "train_samples": len(train),
        "val_samples": len(val),
        "test_samples": len(test),
        "total_games": len(games),
        "seed": args.seed,
    }
    meta_path = out / "metadata.json"
    with open(meta_path, "w") as f:
        json.dump(meta, f, indent=2)
    log.info("Saved metadata to %s", meta_path.name)

    # Summary
    print("\n=== Feature Extraction Summary ===")
    print(f"Games processed:  {len(games)}")
    print(f"Total samples:    {len(all_samples)}")
    print(f"  Train:          {len(train)}")
    print(f"  Validation:     {len(val)}")
    print(f"  Test:           {len(test)}")
    print(f"Board shape:      ({NUM_AREAS}, {NUM_FEATURES})")
    print(f"Adjacency shape:  ({NUM_AREAS}, {NUM_AREAS})")
    print(f"Output directory: {out}")


if __name__ == "__main__":
    main()
