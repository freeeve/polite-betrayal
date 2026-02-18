#!/usr/bin/env python3
"""Extract opening book data from historical Diplomacy games (Spring 1901 through Fall 1907).

Reads data/processed/games.jsonl (streaming) and produces:
  1. data/processed/opening_book.json — matches the Go OpeningBook JSON schema
  2. benchmarks/opening-book-analysis.md — summary statistics

Matching strategy varies by era:
  - 1901: exact unit positions (everyone starts the same)
  - 1902-1903: SC ownership set + exact unit positions
  - 1904-1907: SC count range + theater presence + fleet/army ratio

Every entry includes ALL condition features so the Go side can experiment with
different matching strategies:
  - positions (exact unit locations)
  - owned_scs, sc_count_min/max
  - theaters, fleet_count, army_count
  - neighbor_stance (aggressive/neutral/retreating per neighbor)
  - border_pressure (enemy units adjacent to our SCs)
  - neighbor_sc_counts (each neighbor's SC count)

Output JSON matches the Go struct at api/internal/bot/opening_book.go:
  {"entries": [BookEntry, ...]}
"""

import json
import logging
import re
import sys
import time
from collections import defaultdict
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

DATA_DIR = Path(__file__).resolve().parent.parent
GAMES_PATH = DATA_DIR / "processed" / "games.jsonl"
OUTPUT_PATH = DATA_DIR / "processed" / "opening_book.json"
ANALYSIS_PATH = DATA_DIR.parent / "benchmarks" / "opening-book-analysis.md"

POWERS = ["austria", "england", "france", "germany", "italy", "russia", "turkey"]

# Phases to extract: Spring 1901 through Fall 1907
TARGET_PHASES = []
for _yr in range(1901, 1908):
    TARGET_PHASES.append(f"S{_yr}M")
    TARGET_PHASES.append(f"F{_yr}M")
    if _yr < 1907:
        TARGET_PHASES.append(f"W{_yr}A")

MIN_ABS_COUNT = 10

def get_cond_threshold(year):
    """Conditional frequency threshold within a position cluster."""
    if year <= 1901:
        return 0.10
    elif year <= 1902:
        return 0.08
    elif year <= 1904:
        return 0.02
    else:
        return 0.01

def get_min_pos_games(year):
    """Minimum games for a position cluster to qualify."""
    if year <= 1901:
        return 1000
    elif year <= 1902:
        return 300
    elif year <= 1904:
        return 20
    else:
        return 10

# ---------------------------------------------------------------------------
# Province / theater / adjacency data
# ---------------------------------------------------------------------------

PROVINCE_THEATER = {
    "bre": "west", "par": "west", "mar": "west",
    "gas": "west", "bur": "west", "pic": "west",
    "spa": "west", "por": "west", "bel": "west",
    "mao": "west", "eng": "west", "iri": "west",
    "naf": "west", "nao": "west",
    "lon": "west", "lvp": "west", "wal": "west",
    "yor": "west", "edi": "west", "cly": "west",
    "nwy": "scan", "swe": "scan", "den": "scan",
    "ska": "scan", "nth": "scan", "nrg": "scan",
    "bar": "scan", "fin": "scan", "stp": "scan",
    "tun": "med", "tys": "med", "wes": "med",
    "gol": "med", "ion": "med", "aeg": "med",
    "eas": "med", "rom": "med", "nap": "med",
    "apu": "med", "tus": "med", "pie": "med",
    "ven": "med",
    "gre": "balkans", "ser": "balkans", "bul": "balkans",
    "rum": "balkans", "alb": "balkans", "con": "balkans",
    "smy": "balkans", "ank": "balkans", "arm": "balkans",
    "syr": "balkans", "bla": "balkans", "adr": "balkans",
    "mos": "east", "war": "east", "ukr": "east",
    "sev": "east", "lvn": "east", "pru": "east",
    "sil": "east", "gal": "east", "bot": "east",
    "mun": "center", "ber": "center", "kie": "center",
    "ruh": "center", "hol": "center", "tyr": "center",
    "boh": "center", "vie": "center", "tri": "center",
    "bud": "center", "hel": "center", "bal": "center",
}
ALL_THEATERS = ["west", "scan", "med", "balkans", "east", "center"]

# Build adjacency graph from features.py ADJACENCY_PAIRS (imported inline).
# This is the same data used for the neural network features.
_ADJACENCY_PAIRS = [
    ("adr","ion"),("aeg","eas"),("aeg","ion"),("bal","bot"),
    ("eng","iri"),("eng","mao"),("eng","nth"),("gol","tys"),
    ("gol","wes"),("hel","nth"),("ion","eas"),("ion","tys"),
    ("iri","mao"),("iri","nao"),("mao","nao"),("mao","wes"),
    ("nao","nrg"),("nth","nrg"),("nth","ska"),("nrg","bar"),
    ("tys","wes"),
    ("adr","alb"),("adr","apu"),("adr","tri"),("adr","ven"),
    ("aeg","con"),("aeg","gre"),("aeg","smy"),
    ("bal","ber"),("bal","den"),("bal","kie"),("bal","lvn"),
    ("bal","pru"),("bal","swe"),
    ("bar","nwy"),
    ("bla","ank"),("bla","arm"),("bla","con"),("bla","rum"),("bla","sev"),
    ("bot","fin"),("bot","lvn"),("bot","swe"),
    ("eas","smy"),("eas","syr"),
    ("eng","bel"),("eng","bre"),("eng","lon"),("eng","pic"),("eng","wal"),
    ("gol","mar"),("gol","pie"),("gol","tus"),
    ("hel","den"),("hel","hol"),("hel","kie"),
    ("ion","alb"),("ion","apu"),("ion","gre"),("ion","nap"),("ion","tun"),
    ("iri","lvp"),("iri","wal"),
    ("mao","bre"),("mao","gas"),("mao","naf"),("mao","por"),
    ("nao","cly"),("nao","lvp"),
    ("nth","bel"),("nth","den"),("nth","edi"),("nth","hol"),
    ("nth","lon"),("nth","nwy"),("nth","yor"),
    ("nrg","cly"),("nrg","edi"),("nrg","nwy"),
    ("ska","den"),("ska","nwy"),("ska","swe"),
    ("tys","nap"),("tys","rom"),("tys","tun"),("tys","tus"),
    ("wes","naf"),("wes","tun"),
    ("boh","gal"),("boh","mun"),("boh","sil"),("boh","tyr"),("boh","vie"),
    ("bud","gal"),("bud","vie"),
    ("bur","mun"),("bur","par"),("bur","ruh"),
    ("gal","sil"),("gal","ukr"),("gal","vie"),("gal","war"),
    ("mos","ukr"),("mos","war"),
    ("mun","ruh"),("mun","sil"),("mun","tyr"),
    ("sil","war"),
    ("tyr","vie"),
    ("ukr","war"),
    ("bud","rum"),("bud","ser"),("bud","tri"),
    ("bur","bel"),("bur","gas"),("bur","mar"),("bur","pic"),
    ("gal","rum"),
    ("gas","mar"),
    ("mos","lvn"),("mos","sev"),("mos","stp"),
    ("mun","ber"),("mun","kie"),
    ("par","bre"),("par","gas"),("par","pic"),
    ("ruh","bel"),("ruh","hol"),("ruh","kie"),
    ("ser","alb"),("ser","bul"),("ser","gre"),("ser","rum"),("ser","tri"),
    ("sil","ber"),("sil","pru"),
    ("tyr","pie"),("tyr","tri"),("tyr","ven"),
    ("ukr","rum"),("ukr","sev"),
    ("vie","tri"),
    ("war","lvn"),("war","pru"),
    ("alb","gre"),("alb","tri"),
    ("ank","arm"),("ank","con"),
    ("apu","nap"),("apu","ven"),
    ("bel","hol"),("bel","pic"),
    ("ber","kie"),("ber","pru"),
    ("bre","gas"),("bre","pic"),
    ("cly","edi"),("cly","lvp"),
    ("con","smy"),
    ("den","kie"),("den","swe"),
    ("hol","kie"),
    ("edi","lvp"),("edi","yor"),
    ("fin","nwy"),("fin","swe"),
    ("lon","wal"),("lon","yor"),
    ("lvp","wal"),("lvp","yor"),
    ("mar","pie"),
    ("naf","tun"),
    ("nwy","swe"),
    ("pie","tus"),("pie","ven"),
    ("pru","lvn"),
    ("apu","rom"),("rom","nap"),("rom","tus"),("rom","ven"),
    ("sev","arm"),("sev","rum"),
    ("ank","smy"),("smy","arm"),("smy","syr"),
    ("tri","ven"),
    ("wal","yor"),
    ("con","bul"),("gre","bul"),("rum","bul"),
    ("gas","spa"),("mar","spa"),("por","spa"),
    ("fin","stp"),("lvn","stp"),("nwy","stp"),
    ("aeg","bul"),("bla","bul"),
    ("mao","spa"),("gol","spa"),("wes","spa"),
    ("bar","stp"),("bot","stp"),
]

# Build bidirectional adjacency graph: province -> set of adjacent provinces
ADJ_GRAPH = defaultdict(set)
for a, b in _ADJACENCY_PAIRS:
    ADJ_GRAPH[a].add(b)
    ADJ_GRAPH[b].add(a)


# ---------------------------------------------------------------------------
# Utility functions
# ---------------------------------------------------------------------------

def parse_unit(unit_str):
    """Parse 'A par' or 'F stp/sc' into (type, province_with_coast)."""
    parts = unit_str.strip().split()
    if len(parts) < 2:
        return None, None
    return parts[0], parts[1]


def base_province(loc):
    """Strip coast suffix: 'stp/sc' -> 'stp', 'par' -> 'par'."""
    return loc.split("/")[0]


def compute_theater_presence(units_list):
    """Count units per theater from a list like ['A par', 'F bre']."""
    counts = {t: 0 for t in ALL_THEATERS}
    for u in units_list:
        _, loc = parse_unit(u)
        if loc:
            t = PROVINCE_THEATER.get(base_province(loc))
            if t:
                counts[t] += 1
    return counts


def compute_fleet_army(units_list):
    """Count fleets and armies from a unit list."""
    fleets, armies = 0, 0
    for u in units_list:
        utype, _ = parse_unit(u)
        if utype == "F":
            fleets += 1
        elif utype == "A":
            armies += 1
    return fleets, armies


def unit_fingerprint(units_list):
    """Exact position fingerprint: sorted tuple of (type, loc)."""
    parsed = []
    for u in units_list:
        utype, loc = parse_unit(u)
        if utype and loc:
            parsed.append((utype, loc))
    return tuple(sorted(parsed))


def sc_fingerprint(centers_list):
    """SC ownership fingerprint: sorted tuple of owned SC names."""
    return tuple(sorted(centers_list))


def feature_fingerprint(units_list, centers_list):
    """Feature-based fingerprint for 1904+: (sc_count, theater_tuple, fleets, armies)."""
    sc_count = len(centers_list)
    theaters = compute_theater_presence(units_list)
    fleets, armies = compute_fleet_army(units_list)
    theater_tuple = tuple(theaters[t] for t in ALL_THEATERS)
    return (sc_count, theater_tuple, fleets, armies)


def get_phase_year(phase_name):
    """Extract year from phase name like 'S1901M' -> 1901."""
    m = re.match(r"[SFW](\d{4})[MRA]", phase_name)
    return int(m.group(1)) if m else 0


def get_cluster_key(phase_name, units_list, centers_list):
    """Get the appropriate clustering key based on the phase year."""
    year = get_phase_year(phase_name)
    if year <= 1901:
        return ("exact", unit_fingerprint(units_list))
    elif year <= 1903:
        return ("sc", sc_fingerprint(centers_list), unit_fingerprint(units_list))
    else:
        return ("feature", feature_fingerprint(units_list, centers_list))


def orders_fingerprint(orders_list):
    """Hashable fingerprint from a list of order strings."""
    return tuple(sorted(orders_list))


# ---------------------------------------------------------------------------
# Neighbor behavior analysis
# ---------------------------------------------------------------------------

def build_unit_map(phase_data):
    """Build a map of province -> (power, unit_type) from a phase's units dict.

    phase_data["units"] is {power: ["A par", "F bre", ...], ...}
    """
    unit_map = {}
    for power, units in phase_data.get("units", {}).items():
        for u in units:
            utype, loc = parse_unit(u)
            if utype and loc:
                unit_map[base_province(loc)] = (power, utype)
    return unit_map


def compute_border_zone(our_scs, adj_graph):
    """Compute the set of provinces adjacent to our SCs (excluding our own SCs).

    Matches the Go neighbor_stance.go logic.
    """
    border = set()
    for sc in our_scs:
        for adj_prov in adj_graph.get(sc, set()):
            if adj_prov not in our_scs:
                border.add(adj_prov)
    return border


def classify_neighbor_stances(phase_data, power, centers_data):
    """Classify each neighbor power's stance toward us based on unit positions.

    Replicates Go ClassifyNeighborStances: count neighbor units in our border
    zone, classify ratio >= 0.5 as aggressive, 0 as retreating, else neutral.
    """
    our_scs = set(centers_data.get(power, []))
    if not our_scs:
        return {}

    border_zone = compute_border_zone(our_scs, ADJ_GRAPH)
    unit_map = build_unit_map(phase_data)

    # Count each neighbor's units in our border zone vs total
    neighbor_stats = defaultdict(lambda: {"adjacent": 0, "total": 0})
    for prov, (unit_power, _) in unit_map.items():
        if unit_power == power:
            continue
        neighbor_stats[unit_power]["total"] += 1
        if prov in border_zone:
            neighbor_stats[unit_power]["adjacent"] += 1

    stances = {}
    for p, stats in neighbor_stats.items():
        if stats["total"] == 0:
            continue
        ratio = stats["adjacent"] / stats["total"]
        if ratio >= 0.5:
            stances[p] = "aggressive"
        elif ratio == 0:
            stances[p] = "retreating"
        else:
            stances[p] = "neutral"
    return stances


def compute_border_pressure(phase_data, power, centers_data):
    """Count enemy units adjacent to this power's SCs.

    Matches Go borderPressure function.
    """
    our_scs = set(centers_data.get(power, []))
    if not our_scs:
        return 0

    border_zone = compute_border_zone(our_scs, ADJ_GRAPH)
    unit_map = build_unit_map(phase_data)

    count = 0
    for prov in border_zone:
        if prov in unit_map:
            unit_power, _ = unit_map[prov]
            if unit_power != power:
                count += 1
    return count


def count_border_bounces(phase_data, results_data, power, centers_data):
    """Count bounces where a neighbor tried to enter our territory.

    A border bounce occurs when an enemy order targets one of our provinces
    and the result includes 'bounce'.
    """
    our_provinces = set()
    for u in phase_data.get("units", {}).get(power, []):
        _, loc = parse_unit(u)
        if loc:
            our_provinces.add(base_province(loc))
    our_provinces.update(centers_data.get(power, []))

    bounces = 0
    for unit_key, result_list in results_data.items():
        if "bounce" not in result_list:
            continue
        # Find the order for this unit — check all powers' orders
        utype, uloc = parse_unit(unit_key)
        if not uloc:
            continue
        unit_power = None
        for p, units in phase_data.get("units", {}).items():
            for u in units:
                _, loc = parse_unit(u)
                if loc and base_province(loc) == base_province(uloc):
                    unit_power = p
                    break
            if unit_power:
                break
        if unit_power == power:
            continue  # We bounced, not a neighbor

        # Check if this unit's order targeted one of our provinces
        for p, orders in phase_data.get("orders", {}).items():
            if p != unit_power:
                continue
            for order_str in orders:
                tokens = order_str.strip().split()
                if len(tokens) >= 4 and tokens[2] == "-":
                    order_loc = base_province(tokens[1])
                    target = base_province(tokens[3])
                    if order_loc == base_province(uloc) and target in our_provinces:
                        bounces += 1
    return bounces


def get_neighbor_sc_counts(centers_data, power):
    """Get SC count for each other power."""
    counts = {}
    for p in POWERS:
        if p != power:
            counts[p] = len(centers_data.get(p, []))
    return counts


def compute_neighbor_features(phase_data, power, centers_data, results_data):
    """Compute all neighbor behavior features for a power at a phase.

    Returns dict with neighbor_stance, border_pressure, border_bounces,
    neighbor_sc_counts.
    """
    stances = classify_neighbor_stances(phase_data, power, centers_data)
    pressure = compute_border_pressure(phase_data, power, centers_data)
    bounces = count_border_bounces(phase_data, results_data, power, centers_data)
    sc_counts = get_neighbor_sc_counts(centers_data, power)

    return {
        "neighbor_stance": stances,
        "border_pressure": pressure,
        "border_bounces": bounces,
        "neighbor_sc_counts": sc_counts,
    }


# ---------------------------------------------------------------------------
# Condition building
# ---------------------------------------------------------------------------

def build_condition(units_list, centers_list, neighbor_features):
    """Build a complete BookCondition dict with all feature tiers."""
    condition = {}

    # Tier 1: exact positions
    positions = {}
    for u in units_list:
        utype, loc = parse_unit(u)
        if utype and loc:
            positions[loc] = "army" if utype == "A" else "fleet"
    condition["positions"] = positions

    # Tier 2: SC ownership
    condition["owned_scs"] = sorted(centers_list)
    sc_count = len(centers_list)
    condition["sc_count_min"] = sc_count
    condition["sc_count_max"] = sc_count

    # Tier 3: neighbor behavior
    condition["neighbor_stance"] = neighbor_features.get("neighbor_stance", {})
    condition["border_pressure"] = neighbor_features.get("border_pressure", 0)
    condition["border_bounces"] = neighbor_features.get("border_bounces", 0)
    condition["neighbor_sc_counts"] = neighbor_features.get("neighbor_sc_counts", {})

    # Tier 4: theater/composition
    theaters = compute_theater_presence(units_list)
    condition["theaters"] = {t: c for t, c in theaters.items() if c > 0}
    fleets, armies = compute_fleet_army(units_list)
    condition["fleet_count"] = fleets
    condition["army_count"] = armies

    return condition


# ---------------------------------------------------------------------------
# Order parsing
# ---------------------------------------------------------------------------

def parse_order_to_input(order_str):
    """Parse a textual order string into a Go-compatible OrderInput dict."""
    tokens = order_str.strip().split()
    if len(tokens) < 2:
        return None

    unit_type_char = tokens[0]
    unit_loc = tokens[1]
    unit_type = "army" if unit_type_char == "A" else "fleet"

    loc_parts = unit_loc.split("/")
    location = loc_parts[0]
    coast = loc_parts[1] if len(loc_parts) > 1 else ""

    result = {"unit_type": unit_type, "location": location}
    if coast:
        result["coast"] = coast

    if len(tokens) == 2:
        result["order_type"] = "hold"
        return result

    action = tokens[2]

    if action == "H":
        result["order_type"] = "hold"
        return result

    if action == "-":
        target_raw = tokens[3] if len(tokens) > 3 else location
        target_parts = target_raw.split("/")
        result["order_type"] = "move"
        result["target"] = target_parts[0]
        if len(target_parts) > 1:
            result["target_coast"] = target_parts[1]
        return result

    if action == "S":
        if len(tokens) < 5:
            aux_loc = tokens[3] if len(tokens) > 3 else location
            result["order_type"] = "support"
            result["aux_loc"] = aux_loc
            result["aux_target"] = aux_loc
            result["aux_unit_type"] = "army"
            return result

        aux_unit_type = "army" if tokens[3] == "A" else "fleet"
        aux_loc_raw = tokens[4]
        aux_loc = aux_loc_raw.split("/")[0]

        if len(tokens) > 5 and tokens[5] == "-":
            aux_target_raw = tokens[6] if len(tokens) > 6 else aux_loc
            result["order_type"] = "support"
            result["aux_loc"] = aux_loc
            result["aux_target"] = aux_target_raw.split("/")[0]
            result["aux_unit_type"] = aux_unit_type
        else:
            result["order_type"] = "support"
            result["aux_loc"] = aux_loc
            result["aux_target"] = aux_loc
            result["aux_unit_type"] = aux_unit_type
        return result

    if action == "C":
        if len(tokens) >= 7 and tokens[5] == "-":
            result["order_type"] = "convoy"
            result["aux_loc"] = tokens[4]
            result["aux_target"] = tokens[6]
            result["aux_unit_type"] = "army" if tokens[3] == "A" else "fleet"
        elif len(tokens) >= 5:
            result["order_type"] = "convoy"
            result["aux_loc"] = tokens[4] if len(tokens) > 4 else location
            result["aux_unit_type"] = "army" if tokens[3] == "A" else "fleet"
        return result

    if action == "B":
        result["order_type"] = "build"
        return result

    if action == "D":
        result["order_type"] = "disband"
        return result

    if action == "R":
        target_raw = tokens[3] if len(tokens) > 3 else location
        target_parts = target_raw.split("/")
        result["order_type"] = "retreat"
        result["target"] = target_parts[0]
        if len(target_parts) > 1:
            result["target_coast"] = target_parts[1]
        return result

    result["order_type"] = "hold"
    return result


def parse_phase_to_fields(phase_name):
    """Parse 'S1901M' into (year, season, phase_type) for Go BookEntry."""
    m = re.match(r"([SFW])(\d{4})([MRA])", phase_name)
    if not m:
        return None, None, None
    season_map = {"S": "spring", "F": "fall", "W": "winter"}
    type_map = {"M": "movement", "R": "retreat", "A": "build"}
    return int(m.group(2)), season_map[m.group(1)], type_map[m.group(3)]


# ---------------------------------------------------------------------------
# Main processing
# ---------------------------------------------------------------------------

def process_games():
    """Stream through games.jsonl and aggregate opening data.

    For each power at each phase, stores:
      - Order clusters keyed by (position_cluster_key, orders_fingerprint)
      - Representative data: units, centers, orders, neighbor features
    """
    log.info("Reading games from %s", GAMES_PATH)

    # power -> phase -> cluster_key -> orders_key ->
    #   {count, total_centers, wins, orders, units, centers, neighbor_features}
    clusters = defaultdict(lambda: defaultdict(lambda: defaultdict(lambda: defaultdict(
        lambda: {"count": 0, "total_centers": 0, "wins": 0,
                 "orders": None, "units": None, "centers": None,
                 "neighbor_features": None}
    ))))

    phase_totals = defaultdict(lambda: defaultdict(int))
    total_games = 0
    skipped = 0
    start_time = time.time()

    with open(GAMES_PATH, "r") as f:
        for line_num, line in enumerate(f):
            line = line.strip()
            if not line:
                continue

            try:
                game = json.loads(line)
            except json.JSONDecodeError:
                skipped += 1
                continue

            if game.get("map") != "standard":
                skipped += 1
                continue

            phase_names_set = {p["name"] for p in game.get("phases", [])}
            if "S1901M" not in phase_names_set or "F1901M" not in phase_names_set:
                skipped += 1
                continue

            total_games += 1
            phases_by_name = {p["name"]: p for p in game.get("phases", [])}
            outcome = game.get("outcome", {})

            for power in POWERS:
                power_outcome = outcome.get(power, {})
                is_win = power_outcome.get("result") in ("solo", "draw")
                final_sc = power_outcome.get("centers", 0)

                for phase_name in TARGET_PHASES:
                    phase = phases_by_name.get(phase_name)
                    if not phase:
                        break

                    orders = phase.get("orders", {}).get(power, [])
                    units = phase.get("units", {}).get(power, [])
                    centers_data = phase.get("centers", {})
                    centers = centers_data.get(power, [])
                    results = phase.get("results", {})

                    if not orders:
                        break

                    ckey = get_cluster_key(phase_name, units, centers)
                    okey = orders_fingerprint(orders)

                    entry = clusters[power][phase_name][ckey][okey]
                    entry["count"] += 1
                    entry["total_centers"] += final_sc
                    if is_win:
                        entry["wins"] += 1
                    if entry["orders"] is None:
                        entry["orders"] = orders
                        entry["units"] = units
                        entry["centers"] = centers
                        # Compute neighbor features from the full phase data
                        entry["neighbor_features"] = compute_neighbor_features(
                            phase, power, centers_data, results
                        )

                    phase_totals[power][phase_name] += 1

            if (line_num + 1) % 20000 == 0:
                elapsed = time.time() - start_time
                rate = (line_num + 1) / elapsed
                log.info(
                    "  Processed %d lines (%d games, %d skipped) — %.0f lines/sec",
                    line_num + 1, total_games, skipped, rate,
                )

    elapsed = time.time() - start_time
    log.info("Done: %d games processed, %d skipped in %.1fs", total_games, skipped, elapsed)
    return clusters, phase_totals, total_games


def build_opening_book(clusters, phase_totals, total_games):
    """Convert raw clusters into the Go-compatible OpeningBook JSON format."""
    book_entries = []

    for power in POWERS:
        for phase_name in TARGET_PHASES:
            total_for_phase = phase_totals[power][phase_name]
            if total_for_phase == 0:
                continue

            year = get_phase_year(phase_name)
            min_pos = get_min_pos_games(year)
            cond_threshold = get_cond_threshold(year)

            phase_clusters = clusters[power][phase_name]

            for ckey, order_variants in phase_clusters.items():
                pos_total = sum(d["count"] for d in order_variants.values())
                if pos_total < min_pos:
                    continue

                options = []
                for okey, data in order_variants.items():
                    cond_freq = data["count"] / pos_total
                    if cond_freq < cond_threshold or data["count"] < MIN_ABS_COUNT:
                        continue

                    parsed_orders = []
                    for o in data["orders"]:
                        parsed = parse_order_to_input(o)
                        if parsed:
                            parsed_orders.append(parsed)

                    options.append({
                        "name": "",
                        "weight": round(cond_freq, 4),
                        "orders": parsed_orders,
                        "_games": data["count"],
                        "_pos_games": pos_total,
                        "_global_freq": round(data["count"] / total_for_phase, 4),
                        "_avg_centers": round(data["total_centers"] / data["count"], 2),
                        "_win_rate": round(data["wins"] / data["count"], 4),
                    })

                if not options:
                    continue

                options.sort(key=lambda e: -e["weight"])
                for i, opt in enumerate(options):
                    opt["name"] = f"{power}_{phase_name}_{i+1}"

                # Use representative data from the highest-count variant
                best = max(order_variants.values(), key=lambda d: d["count"])
                rep_units = best["units"] or []
                rep_centers = best["centers"] or []
                rep_nf = best["neighbor_features"] or {}

                yr, season, phase_type = parse_phase_to_fields(phase_name)
                condition = build_condition(rep_units, rep_centers, rep_nf)

                book_entries.append({
                    "power": power,
                    "year": yr,
                    "season": season,
                    "phase": phase_type,
                    "condition": condition,
                    "options": options,
                })

    return {"entries": book_entries}


# ---------------------------------------------------------------------------
# Analysis report
# ---------------------------------------------------------------------------

def generate_analysis(book_data, phase_totals, total_games):
    """Generate the markdown analysis report."""
    entries = book_data["entries"]
    lines = [
        "# Opening Book Analysis",
        "",
        f"**Total games analyzed:** {total_games:,}",
        f"**Phases covered:** Spring 1901 through Fall 1907",
        f"**Map:** Standard only",
        f"**Clustering:** exact positions (1901), SC ownership (1902-1903), features (1904+)",
        f"**Neighbor features:** stance classification, border pressure, bounces, SC counts",
        "",
    ]

    total_options = sum(len(e["options"]) for e in entries)
    lines.append(f"**Total position clusters:** {len(entries):,}")
    lines.append(f"**Total order variants:** {total_options:,}")
    lines.append("")

    # Phase distribution
    lines.append("## Phase Distribution")
    lines.append("")
    lines.append("| Phase | Clusters | Variants |")
    lines.append("|-------|----------|----------|")
    for phase in TARGET_PHASES:
        phase_entries = [e for e in entries if _phase_code(e) == phase]
        n_variants = sum(len(e["options"]) for e in phase_entries)
        if phase_entries or n_variants:
            lines.append(f"| {phase} | {len(phase_entries)} | {n_variants} |")
    lines.append("")

    # Coverage by year
    lines.append("## Coverage Statistics")
    lines.append("")
    lines.append("Percentage of games where at least one book entry matches.")

    for yr in sorted(set(get_phase_year(p) for p in TARGET_PHASES)):
        year_phases = [p for p in TARGET_PHASES if get_phase_year(p) == yr]
        if not year_phases:
            continue
        lines.append("")
        lines.append(f"### {yr}")
        lines.append("")
        header = "| Power |"
        sep = "|-------|"
        for phase in year_phases:
            header += f" {phase} |"
            sep += "--------|"
        lines.append(header)
        lines.append(sep)

        for power in POWERS:
            row = f"| {power.capitalize()} |"
            for phase in year_phases:
                total = phase_totals[power][phase]
                if total == 0:
                    row += " N/A |"
                    continue
                covered = sum(
                    opt["_games"]
                    for e in entries
                    if e["power"] == power and _phase_code(e) == phase
                    for opt in e["options"]
                )
                pct = min(100.0, 100.0 * covered / total)
                row += f" {pct:.1f}% |"
            lines.append(row)
    lines.append("")

    # Neighbor stance distribution sample
    lines.append("## Neighbor Stance Distribution (Sample)")
    lines.append("")
    lines.append("Distribution of neighbor stances in book entries for selected powers at F1901M.")
    lines.append("")

    for power in ["france", "germany", "austria"]:
        f1901_entries = [
            e for e in entries
            if e["power"] == power and _phase_code(e) == "F1901M"
        ]
        if not f1901_entries:
            continue
        lines.append(f"**{power.capitalize()} F1901M** ({len(f1901_entries)} clusters):")
        lines.append("")
        # Aggregate stances across clusters
        stance_counts = defaultdict(lambda: defaultdict(int))
        for e in f1901_entries:
            for neighbor, stance in e["condition"].get("neighbor_stance", {}).items():
                stance_counts[neighbor][stance] += 1
        for neighbor in sorted(stance_counts.keys()):
            stances = stance_counts[neighbor]
            parts = [f"{s}: {c}" for s, c in sorted(stances.items())]
            lines.append(f"- vs {neighbor}: {', '.join(parts)}")
        lines.append("")

    # Top openings per power per phase
    lines.append("## Top Openings by Power and Phase")
    lines.append("")

    for power in POWERS:
        power_entries = [e for e in entries if e["power"] == power]
        if not power_entries:
            continue

        lines.append(f"### {power.capitalize()}")
        lines.append("")

        for phase in TARGET_PHASES:
            phase_entries = [e for e in power_entries if _phase_code(e) == phase]
            if not phase_entries:
                continue

            lines.append(f"#### {phase}")
            lines.append("")

            all_opts = []
            for pe in phase_entries:
                for opt in pe["options"]:
                    all_opts.append((pe, opt))
            all_opts.sort(key=lambda x: -x[1]["weight"])

            lines.append("| # | Cond% | Games | Avg SCs | Win% | Pressure | Orders |")
            lines.append("|---|-------|-------|---------|------|----------|--------|")

            for i, (pe, v) in enumerate(all_opts[:5]):
                orders_str = "; ".join(format_order_brief(o) for o in v["orders"])
                if len(orders_str) > 60:
                    orders_str = orders_str[:57] + "..."
                pressure = pe["condition"].get("border_pressure", 0)
                lines.append(
                    f"| {i+1} | {v['weight']:.1%} "
                    f"| {v['_games']:,} "
                    f"| {v['_avg_centers']:.1f} | {v['_win_rate']:.1%} "
                    f"| {pressure} | {orders_str} |"
                )

            lines.append("")

    lines.append(f"*Generated by `data/scripts/extract_openings.py`*")
    return "\n".join(lines)


def _phase_code(entry):
    """Reconstruct phase code like 'S1901M' from BookEntry fields."""
    season_map = {"spring": "S", "fall": "F", "winter": "W"}
    type_map = {"movement": "M", "retreat": "R", "build": "A"}
    s = season_map.get(entry["season"], "S")
    t = type_map.get(entry["phase"], "M")
    return f"{s}{entry['year']}{t}"


def format_order_brief(order):
    """Format an OrderInput dict into a brief human-readable string."""
    ot = order.get("order_type", "?")
    loc = order.get("location", "?")
    ut = "A" if order.get("unit_type") == "army" else "F"
    coast = f"/{order['coast']}" if order.get("coast") else ""

    if ot == "hold":
        return f"{ut} {loc}{coast} H"
    elif ot == "move":
        tc = f"/{order['target_coast']}" if order.get("target_coast") else ""
        return f"{ut} {loc}{coast}-{order.get('target', '?')}{tc}"
    elif ot == "support":
        aux_loc = order.get("aux_loc", "?")
        aux_target = order.get("aux_target", "?")
        if aux_loc == aux_target:
            return f"{ut} {loc}{coast} S {aux_loc}"
        return f"{ut} {loc}{coast} S {aux_loc}-{aux_target}"
    elif ot == "convoy":
        return f"{ut} {loc} C {order.get('aux_loc', '?')}-{order.get('aux_target', '?')}"
    elif ot == "build":
        return f"{ut} {loc}{coast} B"
    elif ot == "disband":
        return f"{ut} {loc}{coast} D"
    elif ot == "retreat":
        tc = f"/{order['target_coast']}" if order.get("target_coast") else ""
        return f"{ut} {loc}{coast} R {order.get('target', '?')}{tc}"
    return f"{ut} {loc} {ot}"


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    if not GAMES_PATH.exists():
        log.error("games.jsonl not found at %s", GAMES_PATH)
        sys.exit(1)

    log.info("Starting opening book extraction (S1901M through F1907M)")

    clusters, phase_totals, total_games = process_games()

    log.info("Building opening book entries")
    book_data = build_opening_book(clusters, phase_totals, total_games)

    total_entries = len(book_data["entries"])
    total_options = sum(len(e["options"]) for e in book_data["entries"])
    log.info("Generated %d order variants across %d position clusters", total_options, total_entries)

    OUTPUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(OUTPUT_PATH, "w") as f:
        json.dump(book_data, f, indent=2)
    size_kb = OUTPUT_PATH.stat().st_size / 1024
    log.info("Wrote opening book to %s (%.1f KB)", OUTPUT_PATH, size_kb)

    analysis = generate_analysis(book_data, phase_totals, total_games)
    ANALYSIS_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(ANALYSIS_PATH, "w") as f:
        f.write(analysis)
    log.info("Wrote analysis to %s", ANALYSIS_PATH)

    print(f"\n=== Opening Book Summary ===")
    print(f"Games analyzed: {total_games:,}")
    print(f"Position clusters: {total_entries:,}")
    print(f"Total order variants: {total_options:,}")
    print(f"Output: {OUTPUT_PATH}")
    print(f"Analysis: {ANALYSIS_PATH}")


if __name__ == "__main__":
    main()
