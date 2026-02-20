#!/usr/bin/env python3
"""Convert self-play JSONL output from the Rust engine into NPZ training data.

Reads JSONL game records produced by `cargo run --release --bin selfplay`
and converts them into the same NPZ tensor format consumed by
`train_policy.py` and `train_value.py`.

Usage:
    python3 convert_selfplay.py --input games.jsonl --output-dir data/processed/ [--val-split 0.1]
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

# ---- Area index: 75 provinces + 6 bicoastal variants = 81 ----
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

# ---- Order encoding ----
ORDER_TYPES = ["hold", "move", "support", "convoy", "retreat", "build", "disband"]
ORDER_TYPE_INDEX = {t: i for i, t in enumerate(ORDER_TYPES)}
ORDER_VOCAB_SIZE = len(ORDER_TYPES) + NUM_AREAS + NUM_AREAS  # 169

# ---- Home centers for build logic ----
HOME_CENTERS = {
    "austria": {"vie", "bud", "tri"},
    "england": {"lon", "edi", "lvp"},
    "france": {"par", "bre", "mar"},
    "germany": {"ber", "mun", "kie"},
    "italy": {"rom", "nap", "ven"},
    "russia": {"mos", "sev", "stp", "war"},
    "turkey": {"ank", "con", "smy"},
}

ALL_SUPPLY_CENTERS = frozenset([
    "ank", "bel", "ber", "bre", "bud", "bul", "con", "den", "edi",
    "gre", "hol", "kie", "lon", "lvp", "mar", "mos", "mun", "nap",
    "nwy", "par", "por", "rom", "rum", "ser", "sev", "smy", "spa",
    "stp", "swe", "tri", "tun", "ven", "vie", "war",
])

# ---- DFEN power characters ----
DFEN_POWER_MAP = {
    'A': "austria",
    'E': "england",
    'F': "france",
    'G': "germany",
    'I': "italy",
    'R': "russia",
    'T': "turkey",
}

DFEN_UNIT_MAP = {
    'a': 'A',  # army
    'f': 'F',  # fleet
}

DFEN_SEASON_MAP = {
    's': 'spring',
    'f': 'fall',
}

DFEN_PHASE_MAP = {
    'm': 'movement',
    'r': 'retreat',
    'b': 'build',
}


# ===========================================================================
# DFEN Parser
# ===========================================================================

def parse_dfen(dfen_str: str) -> dict:
    """Parse a DFEN string into a board state dict.

    Returns a dict with keys:
        year: int
        season: str
        phase: str (movement/retreat/build)
        units: {power_name: ["A vie", "F tri", ...]}
        centers: {power_name: ["vie", "bud", ...]}
        dislodged: {power_name: ["A ser", ...]}  (optional)
    """
    sections = dfen_str.split('/')
    if len(sections) != 4:
        raise ValueError(f"DFEN must have 4 sections, got {len(sections)}: {dfen_str[:60]}")

    phase_info, units_str, sc_str, dislodged_str = sections

    # Parse phase info: e.g. "1901sm"
    if len(phase_info) < 3:
        raise ValueError(f"Phase info too short: {phase_info}")
    phase_char = phase_info[-1]
    season_char = phase_info[-2]
    year_str = phase_info[:-2]
    year = int(year_str)
    season = DFEN_SEASON_MAP.get(season_char, season_char)
    phase = DFEN_PHASE_MAP.get(phase_char, phase_char)

    # Parse units: comma-separated entries like "Aavie", "Rfstp.sc"
    units: dict[str, list[str]] = defaultdict(list)
    if units_str != "-":
        for entry in units_str.split(','):
            if len(entry) < 4:
                continue
            power_char = entry[0]
            unit_char = entry[1]
            location = entry[2:]

            power = DFEN_POWER_MAP.get(power_char)
            utype = DFEN_UNIT_MAP.get(unit_char)
            if not power or not utype:
                continue

            # Convert location: "stp.sc" -> "stp/sc", "vie" -> "vie"
            prov, coast = _parse_dfen_location(location)
            if not prov:
                continue
            if coast:
                units[power].append(f"{utype} {prov}/{coast}")
            else:
                units[power].append(f"{utype} {prov}")

    # Parse supply centers: comma-separated entries like "Avie", "Nbel"
    centers: dict[str, list[str]] = defaultdict(list)
    for entry in sc_str.split(','):
        if len(entry) < 4:
            continue
        power_char = entry[0]
        prov_str = entry[1:]

        if power_char == 'N':
            continue  # neutral, skip

        power = DFEN_POWER_MAP.get(power_char)
        if not power:
            continue

        prov = prov_str.lower()
        if prov in PROVINCE_SET:
            centers[power].append(prov)

    # Parse dislodged units
    dislodged: dict[str, list[str]] = defaultdict(list)
    if dislodged_str != "-":
        for entry in dislodged_str.split(','):
            parts = entry.split('<')
            if len(parts) != 2:
                continue
            unit_part = parts[0]
            if len(unit_part) < 4:
                continue
            power_char = unit_part[0]
            unit_char = unit_part[1]
            location = unit_part[2:]

            power = DFEN_POWER_MAP.get(power_char)
            utype = DFEN_UNIT_MAP.get(unit_char)
            if not power or not utype:
                continue

            prov, coast = _parse_dfen_location(location)
            if not prov:
                continue
            if coast:
                dislodged[power].append(f"{utype} {prov}/{coast}")
            else:
                dislodged[power].append(f"{utype} {prov}")

    return {
        "year": year,
        "season": season,
        "phase": phase,
        "units": dict(units),
        "centers": dict(centers),
        "dislodged": dict(dislodged),
    }


def _parse_dfen_location(loc_str: str) -> tuple[str, str]:
    """Parse DFEN location like 'stp.sc' into (province, coast) or ('vie', '')."""
    if '.' in loc_str:
        parts = loc_str.split('.', 1)
        prov = parts[0].lower()
        coast = parts[1].lower()
        if prov in PROVINCE_SET and coast in ("nc", "sc", "ec"):
            return prov, coast
        return prov if prov in PROVINCE_SET else "", ""
    prov = loc_str.lower()
    if prov in PROVINCE_SET:
        return prov, ""
    return "", ""


# ===========================================================================
# Board State Encoder (DFEN -> [81, 47] tensor)
# ===========================================================================

def _province_type_vec(prov: str) -> list[int]:
    """Return [land, sea, coast] one-hot for a province base code."""
    base = prov.split("/")[0]
    if base in INLAND_PROVINCES:
        return [1, 0, 0]
    if base in SEA_PROVINCES:
        return [0, 1, 0]
    return [0, 0, 1]  # coastal


def _parse_unit_string(unit_str: str) -> tuple[str, str, str]:
    """Parse 'A par' or 'F spa/nc' into (unit_type, province, coast)."""
    parts = unit_str.strip().split()
    if len(parts) < 2:
        return ("", "", "")
    utype = parts[0].upper()
    loc = parts[1].lower()
    if "/" in loc:
        base, coast = loc.split("/", 1)
        return (utype, base, coast)
    return (utype, loc, "")


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


def encode_board_state(board: dict, prev_board: dict | None = None) -> np.ndarray:
    """Encode a parsed DFEN board dict into an [81, 47] feature tensor.

    Matches the encoding in features.py exactly.
    """
    tensor = np.zeros((NUM_AREAS, NUM_FEATURES), dtype=np.float32)

    # Static province type features
    for area, idx in AREA_INDEX.items():
        ptype = _province_type_vec(area)
        tensor[idx, FEAT_PROVINCE_TYPE:FEAT_PROVINCE_TYPE + 3] = ptype

    # Unit positions
    units = board.get("units", {})
    for power, unit_list in units.items():
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
            _set_unit_features(tensor, area_idx, utype, power_idx)
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
    centers = board.get("centers", {})
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
            if prov in SPLIT_COASTS:
                for coast in SPLIT_COASTS[prov]:
                    var_idx = AREA_INDEX.get(f"{prov}/{coast}")
                    if var_idx is not None:
                        tensor[var_idx, FEAT_SC_OWNER + power_idx] = 1.0

    # Mark neutral and non-SC areas
    for area, idx in AREA_INDEX.items():
        base = area.split("/")[0]
        if base in ALL_SUPPLY_CENTERS:
            if base not in owned_centers:
                tensor[idx, FEAT_SC_OWNER + NUM_POWERS] = 1.0  # neutral
        else:
            tensor[idx, FEAT_SC_OWNER + NUM_POWERS + 1] = 1.0  # none (not an SC)

    # Build/disband flags (adjustment phase)
    phase_type = board.get("phase", "movement")
    if phase_type == "build":
        _encode_build_disband(tensor, units, centers)

    # Dislodged units
    dislodged = board.get("dislodged", {})
    _encode_dislodged(tensor, dislodged)

    # Previous-state unit features (channels 36..47)
    _encode_prev_state(tensor, prev_board)

    return tensor


def _encode_build_disband(
    tensor: np.ndarray,
    units: dict[str, list[str]],
    centers: dict[str, list[str]],
):
    """Encode can_build and can_disband flags during adjustment phases."""
    for power in POWER_NAMES:
        num_units = len(units.get(power, []))
        num_centers = len(centers.get(power, []))
        power_home = HOME_CENTERS.get(power, set())
        owned = set(centers.get(power, []))

        if num_centers > num_units:
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
            for u in units.get(power, []):
                _, prov, _ = _parse_unit_string(u)
                idx = AREA_INDEX.get(prov)
                if idx is not None:
                    tensor[idx, FEAT_CAN_DISBAND] = 1.0


def _encode_dislodged(tensor: np.ndarray, dislodged: dict[str, list[str]]):
    """Encode dislodged unit features from parsed DFEN dislodged section."""
    for power, unit_list in dislodged.items():
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
            if utype == "A":
                tensor[area_idx, FEAT_DISLODGED_TYPE] = 1.0
            elif utype == "F":
                tensor[area_idx, FEAT_DISLODGED_TYPE + 1] = 1.0
            tensor[area_idx, FEAT_DISLODGED_OWNER + power_idx] = 1.0

    # Mark non-dislodged slots
    for idx in range(NUM_AREAS):
        if tensor[idx, FEAT_DISLODGED_TYPE] == 0 and tensor[idx, FEAT_DISLODGED_TYPE + 1] == 0:
            tensor[idx, FEAT_DISLODGED_TYPE + 2] = 1.0  # none
            tensor[idx, FEAT_DISLODGED_OWNER + NUM_POWERS] = 1.0  # owner = none


def _encode_prev_state(tensor: np.ndarray, prev_board: dict | None):
    """Encode previous-turn unit positions into channels 36..47."""
    if prev_board is not None:
        prev_units = prev_board.get("units", {})
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


# ===========================================================================
# DSON Order Parser & Encoder
# ===========================================================================

def parse_dson_order(order_str: str) -> dict | None:
    """Parse a single DSON order string into structured components.

    DSON format examples:
        "A vie H"              -> hold
        "A bud - rum"          -> move
        "A gal S A bud - rum"  -> support (move)
        "A tyr S A vie H"      -> support (hold)
        "F mao C A bre - spa"  -> convoy
        "A vie R boh"          -> retreat
        "A vie B"              -> build
        "A war D"              -> disband
        "W"                    -> waive (skip)
    """
    s = order_str.strip()
    if not s:
        return None

    tokens = s.split()
    if not tokens:
        return None

    # Waive
    if tokens[0] == "W":
        return None  # Waive has no unit, skip for training

    if len(tokens) < 3:
        return None

    utype = tokens[0].upper()
    src = _normalize_prov(tokens[1])
    if not src:
        return None
    action = tokens[2].upper()

    if action == "H":
        return {"type": "hold", "unit": utype, "src": src}
    elif action == "-":
        if len(tokens) >= 4:
            dst = _normalize_prov(tokens[3])
            return {"type": "move", "unit": utype, "src": src, "dst": dst or src}
    elif action == "S":
        return {"type": "support", "unit": utype, "src": src, "rest": tokens[3:]}
    elif action == "C":
        return {"type": "convoy", "unit": utype, "src": src, "rest": tokens[3:]}
    elif action == "R":
        if len(tokens) >= 4:
            dst = _normalize_prov(tokens[3])
            return {"type": "retreat", "unit": utype, "src": src, "dst": dst or src}
    elif action == "B":
        return {"type": "build", "unit": utype, "src": src}
    elif action == "D":
        return {"type": "disband", "unit": utype, "src": src}

    return {"type": "hold", "unit": utype, "src": src}


def _normalize_prov(token: str) -> str | None:
    """Normalize a province token, stripping coast for area index lookup."""
    loc = token.lower()
    if "/" in loc:
        base = loc.split("/")[0]
    else:
        base = loc
    if base in PROVINCE_SET:
        return base
    return None


def encode_order_label(order_str: str) -> np.ndarray:
    """Encode a single DSON order as a 169-dim feature vector.

    [0:7]    order type one-hot
    [7:88]   source area one-hot
    [88:169] destination area one-hot (zeros for hold/build/disband)
    """
    vec = np.zeros(ORDER_VOCAB_SIZE, dtype=np.float32)

    parsed = parse_dson_order(order_str)
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


def split_dson_orders(dson_str: str) -> list[str]:
    """Split a DSON multi-order string 'A vie H ; A bud - ser' into individual orders."""
    return [o.strip() for o in dson_str.split(" ; ") if o.strip()]


# ===========================================================================
# Adjacency Matrix
# ===========================================================================

ADJACENCY_PAIRS = [
    ("adr", "ion"), ("aeg", "eas"), ("aeg", "ion"), ("bal", "bot"),
    ("eng", "iri"), ("eng", "mao"), ("eng", "nth"), ("gol", "tys"),
    ("gol", "wes"), ("hel", "nth"), ("ion", "eas"), ("ion", "tys"),
    ("iri", "mao"), ("iri", "nao"), ("mao", "nao"), ("mao", "wes"),
    ("nao", "nrg"), ("nth", "nrg"), ("nth", "ska"), ("nrg", "bar"),
    ("tys", "wes"),
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
    ("boh", "gal"), ("boh", "mun"), ("boh", "sil"), ("boh", "tyr"), ("boh", "vie"),
    ("bud", "gal"), ("bud", "vie"),
    ("bur", "mun"), ("bur", "par"), ("bur", "ruh"),
    ("gal", "sil"), ("gal", "ukr"), ("gal", "vie"), ("gal", "war"),
    ("mos", "ukr"), ("mos", "war"),
    ("mun", "ruh"), ("mun", "sil"), ("mun", "tyr"),
    ("sil", "war"),
    ("tyr", "vie"),
    ("ukr", "war"),
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
    ("con", "bul"), ("gre", "bul"), ("rum", "bul"),
    ("gas", "spa"), ("mar", "spa"), ("por", "spa"),
    ("fin", "stp"), ("lvn", "stp"), ("nwy", "stp"),
    ("aeg", "bul"), ("bla", "bul"),
    ("mao", "spa"), ("gol", "spa"), ("wes", "spa"),
    ("bar", "stp"), ("bot", "stp"),
]


def build_adjacency_matrix() -> np.ndarray:
    """Build an 81x81 binary adjacency matrix, matching features.py."""
    adj = np.zeros((NUM_AREAS, NUM_AREAS), dtype=np.float32)

    for a, b in ADJACENCY_PAIRS:
        if a in AREA_INDEX and b in AREA_INDEX:
            i, j = AREA_INDEX[a], AREA_INDEX[b]
            adj[i, j] = 1.0
            adj[j, i] = 1.0

    for base, coasts in SPLIT_COASTS.items():
        base_idx = AREA_INDEX[base]
        for coast in coasts:
            variant = f"{base}/{coast}"
            var_idx = AREA_INDEX[variant]
            adj[base_idx, var_idx] = 1.0
            adj[var_idx, base_idx] = 1.0
            for k in range(NUM_AREAS):
                if adj[base_idx, k] == 1.0:
                    adj[var_idx, k] = 1.0
                    adj[k, var_idx] = 1.0

    np.fill_diagonal(adj, 1.0)
    return adj


# ===========================================================================
# Value Label Computation
# ===========================================================================

def compute_value_labels(final_sc_counts: list[int], winner: str | None) -> dict[str, np.ndarray]:
    """Compute value targets per power from final game outcome.

    Returns a dict: power_name -> [sc_share, win_prob, draw_prob, survival_prob]
    """
    labels = {}
    for i, power in enumerate(POWER_NAMES):
        sc = final_sc_counts[i]
        vec = np.zeros(4, dtype=np.float32)
        vec[0] = sc / 34.0
        if winner is not None and winner == power:
            vec[1] = 1.0  # win
            vec[3] = 1.0  # survived
        elif winner is None and sc > 0:
            vec[2] = 1.0  # draw
            vec[3] = 1.0  # survived
        elif sc > 0:
            vec[3] = 1.0  # survived but not draw winner
        labels[power] = vec
    return labels


def compute_reward(final_sc_counts: list[int], winner: str | None, power_idx: int) -> float:
    """Compute per-phase reward for RL training."""
    sc = final_sc_counts[power_idx]
    power = POWER_NAMES[power_idx]
    if winner is not None and winner == power:
        return 1.0   # solo win
    elif winner is None and sc > 0:
        return 0.2   # draw (survived)
    elif sc > 0:
        return 0.0   # survived but someone else won
    else:
        return -0.5  # eliminated


# ===========================================================================
# Game Conversion
# ===========================================================================

def convert_game(game: dict, verbose: bool = False) -> list[dict]:
    """Convert a single self-play game record into training samples.

    Each movement/retreat/build phase produces one sample per power that has orders.
    """
    samples = []

    phases = game.get("phases", [])
    winner = game.get("winner")  # None for draws
    final_sc_counts = game.get("final_sc_counts", [0] * 7)
    game_id = str(game.get("game_id", 0))

    value_labels = compute_value_labels(final_sc_counts, winner)

    prev_board = None
    for phase_data in phases:
        dfen = phase_data.get("dfen", "")
        if not dfen:
            continue

        try:
            board = parse_dfen(dfen)
        except (ValueError, IndexError) as e:
            if verbose:
                log.warning("Failed to parse DFEN: %s (%s)", dfen[:40], e)
            continue

        # Encode board state
        board_tensor = encode_board_state(board, prev_board)

        # Get orders per power
        orders_dict = phase_data.get("orders", {})
        year = phase_data.get("year", board.get("year", 0))

        for power, dson_str in orders_dict.items():
            if not dson_str:
                continue
            power_idx = POWER_INDEX.get(power)
            if power_idx is None:
                continue

            # Split and encode individual orders
            individual_orders = split_dson_orders(dson_str)
            order_vecs = []
            for o in individual_orders:
                vec = encode_order_label(o)
                # Only include non-zero orders (skip waive / unparseable)
                if vec.sum() > 0:
                    order_vecs.append(vec)

            if not order_vecs:
                continue

            reward = compute_reward(final_sc_counts, winner, power_idx)

            samples.append({
                "board": board_tensor,
                "orders": order_vecs,
                "value": value_labels.get(power, np.zeros(4, dtype=np.float32)),
                "reward": reward,
                "power_idx": power_idx,
                "year": year,
                "game_id": game_id,
            })

        prev_board = board

    return samples


# ===========================================================================
# Dataset Saving
# ===========================================================================

def save_dataset(samples: list[dict], output_path: Path):
    """Save samples to a compressed .npz file matching train_policy.py format.

    Saves:
        boards: [N, 81, 47]
        order_labels: [N, max_orders, 169]
        order_masks: [N, max_orders]
        values: [N, 4]
        power_indices: [N]
        years: [N]
        rewards: [N]
    """
    if not samples:
        log.warning("No samples to save for %s", output_path)
        return

    n = len(samples)
    boards = np.stack([s["board"] for s in samples])

    max_orders = max(len(s["orders"]) for s in samples)
    order_labels = np.zeros((n, max_orders, ORDER_VOCAB_SIZE), dtype=np.float32)
    order_masks = np.zeros((n, max_orders), dtype=np.float32)
    for i, s in enumerate(samples):
        for j, ov in enumerate(s["orders"]):
            order_labels[i, j] = ov
            order_masks[i, j] = 1.0

    values = np.stack([s["value"] for s in samples])
    power_indices = np.array([s["power_idx"] for s in samples], dtype=np.int32)
    years = np.array([s["year"] for s in samples], dtype=np.int32)
    rewards = np.array([s["reward"] for s in samples], dtype=np.float32)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    np.savez_compressed(
        output_path,
        boards=boards,
        order_labels=order_labels,
        order_masks=order_masks,
        values=values,
        power_indices=power_indices,
        years=years,
        rewards=rewards,
    )
    size_mb = output_path.stat().st_size / (1024 * 1024)
    log.info("Saved %d samples to %s (%.1f MB)", n, output_path.name, size_mb)


def split_by_game(
    samples: list[dict],
    val_ratio: float = 0.1,
    seed: int = 42,
) -> tuple[list[dict], list[dict]]:
    """Split samples into train/val sets using game-id-based splitting."""
    game_buckets: dict[str, list[dict]] = defaultdict(list)
    for s in samples:
        game_buckets[s["game_id"]].append(s)

    train, val = [], []
    for gid, group in sorted(game_buckets.items()):
        h = int(hashlib.md5(f"{seed}_{gid}".encode()).hexdigest(), 16) % 10000
        if h < int(val_ratio * 10000):
            val.extend(group)
        else:
            train.extend(group)

    return train, val


# ===========================================================================
# Main
# ===========================================================================

def main():
    parser = argparse.ArgumentParser(
        description="Convert self-play JSONL to NPZ training data"
    )
    parser.add_argument(
        "--input", type=Path, required=True,
        help="Input JSONL file from selfplay binary",
    )
    parser.add_argument(
        "--output-dir", type=Path, required=True,
        help="Output directory for .npz files",
    )
    parser.add_argument(
        "--val-split", type=float, default=0.1,
        help="Fraction of games to use for validation (default: 0.1)",
    )
    parser.add_argument(
        "--seed", type=int, default=42,
        help="Random seed for train/val split",
    )
    parser.add_argument(
        "--verbose", action="store_true",
        help="Enable verbose debug output",
    )
    args = parser.parse_args()

    if not args.input.exists():
        log.error("Input file not found: %s", args.input)
        sys.exit(1)

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

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
            except json.JSONDecodeError as e:
                log.warning("JSON decode error at line %d: %s", idx + 1, e)
                continue
    log.info("Loaded %d games", len(games))

    if not games:
        log.error("No games loaded. Check input file.")
        sys.exit(1)

    # Convert games to samples
    log.info("Converting games to training samples ...")
    all_samples = []
    games_with_winner = 0
    games_draw = 0
    for i, game in enumerate(games):
        samples = convert_game(game, verbose=args.verbose)
        all_samples.extend(samples)
        if game.get("winner"):
            games_with_winner += 1
        else:
            games_draw += 1
        if (i + 1) % 100 == 0:
            log.info("  ... processed %d games (%d samples)", i + 1, len(all_samples))

    log.info("Extracted %d total samples from %d games", len(all_samples), len(games))

    if not all_samples:
        log.error("No samples extracted. Check input data format.")
        sys.exit(1)

    # Split
    train, val = split_by_game(all_samples, val_ratio=args.val_split, seed=args.seed)
    log.info("Split: train=%d, val=%d", len(train), len(val))

    # Save
    out = args.output_dir
    save_dataset(train, out / "train.npz")
    if val:
        save_dataset(val, out / "val.npz")
    else:
        log.warning("No validation samples (too few games for the split ratio)")

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
        "order_vocab_size": ORDER_VOCAB_SIZE,
        "total_samples": len(all_samples),
        "train_samples": len(train),
        "val_samples": len(val),
        "total_games": len(games),
        "games_with_winner": games_with_winner,
        "games_draw": games_draw,
        "seed": args.seed,
        "source": str(args.input),
    }
    meta_path = out / "metadata.json"
    with open(meta_path, "w") as f:
        json.dump(meta, f, indent=2)

    # Summary
    # Compute class balance
    all_values = np.stack([s["value"] for s in all_samples])
    all_rewards = np.array([s["reward"] for s in all_samples])
    power_counts = np.zeros(NUM_POWERS, dtype=int)
    for s in all_samples:
        power_counts[s["power_idx"]] += 1
    max_orders = max(len(s["orders"]) for s in all_samples)

    print("\n=== Self-Play Conversion Summary ===")
    print(f"Input file:       {args.input}")
    print(f"Games loaded:     {len(games)}")
    print(f"  Solo wins:      {games_with_winner}")
    print(f"  Draws:          {games_draw}")
    print(f"Total samples:    {len(all_samples)}")
    print(f"  Train:          {len(train)}")
    print(f"  Validation:     {len(val)}")
    print(f"Board shape:      ({NUM_AREAS}, {NUM_FEATURES})")
    print(f"Order vocab:      {ORDER_VOCAB_SIZE}")
    print(f"Max orders/sample:{max_orders}")
    print(f"Output directory: {out}")
    print()
    print("Value label stats:")
    print(f"  SC share:       mean={all_values[:, 0].mean():.3f} std={all_values[:, 0].std():.3f}")
    print(f"  Win prob:       mean={all_values[:, 1].mean():.3f}")
    print(f"  Draw prob:      mean={all_values[:, 2].mean():.3f}")
    print(f"  Survival prob:  mean={all_values[:, 3].mean():.3f}")
    print(f"  Reward:         mean={all_rewards.mean():.3f} std={all_rewards.std():.3f}")
    print()
    print("Samples per power:")
    for i, power in enumerate(POWER_NAMES):
        print(f"  {power:>8}: {power_counts[i]:>6} ({100.0 * power_counts[i] / len(all_samples):.1f}%)")


if __name__ == "__main__":
    main()
