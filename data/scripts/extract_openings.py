#!/usr/bin/env python3
"""Extract opening book data from historical Diplomacy games (Spring 1901 through Fall 1902).

Reads data/processed/games.jsonl (streaming) and produces:
  1. data/processed/opening_book.json — weighted opening entries per power/phase
  2. benchmarks/opening-book-analysis.md — summary statistics

The output JSON maps to the Go opening book structure with position-conditional entries.
"""

import json
import logging
import re
import sys
import time
from collections import Counter, defaultdict
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

# Phases we care about, in order
TARGET_PHASES = ["S1901M", "F1901M", "W1901A", "S1902M", "F1902M"]

# Minimum frequency threshold: a cluster must appear in at least this fraction
# of games for a given power at a given phase to be included.
MIN_FREQUENCY = 0.02  # 2% (relaxed from 5% since later phases fragment more)

# For Spring/Fall 1901 the 5% threshold is reasonable; later phases use 2%
PHASE_THRESHOLDS = {
    "S1901M": 0.05,
    "F1901M": 0.03,
    "W1901A": 0.03,
    "S1902M": 0.02,
    "F1902M": 0.02,
}


def parse_unit(unit_str):
    """Parse 'A par' or 'F stp/sc' into (type, province)."""
    parts = unit_str.strip().split()
    if len(parts) < 2:
        return None, None
    return parts[0], parts[1]


def parse_order(order_str):
    """Parse an order string into a structured dict.

    Handles: 'A par - bur', 'A par H', 'A par S A bur - mun',
             'F nth C A yor - nwy', 'A tri B', 'F tri B', 'A tri D',
             'A ven R pie', 'A ven D'
    """
    tokens = order_str.strip().split()
    if len(tokens) < 2:
        return None

    unit_type = tokens[0]
    unit_loc = tokens[1]

    if len(tokens) == 2:
        # Bare unit, shouldn't happen but treat as hold
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "hold"}

    action = tokens[2]

    if action == "H":
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "hold"}

    if action == "-":
        target = tokens[3] if len(tokens) > 3 else unit_loc
        # Check for VIA (convoy route)
        via = "VIA" in tokens
        result = {"unit": unit_loc, "unit_type": unit_type, "order_type": "move", "target": target}
        if via:
            result["via_convoy"] = True
        return result

    if action == "S":
        # Support: 'A par S A bur - mun' or 'A par S A bur H' or 'A par S A bur'
        if len(tokens) < 5:
            return {"unit": unit_loc, "unit_type": unit_type, "order_type": "support_hold",
                    "target": tokens[4] if len(tokens) > 4 else tokens[3]}
        sup_unit_loc = tokens[4]
        if len(tokens) > 5 and tokens[5] == "-":
            sup_target = tokens[6] if len(tokens) > 6 else sup_unit_loc
            return {"unit": unit_loc, "unit_type": unit_type, "order_type": "support_move",
                    "target": sup_unit_loc, "aux": sup_target}
        else:
            return {"unit": unit_loc, "unit_type": unit_type, "order_type": "support_hold",
                    "target": sup_unit_loc}

    if action == "C":
        # Convoy: 'F nth C A yor - nwy' => tokens [F, nth, C, A, yor, -, nwy]
        # target = convoyed unit's location, aux = destination
        if len(tokens) >= 7 and tokens[5] == "-":
            return {"unit": unit_loc, "unit_type": unit_type, "order_type": "convoy",
                    "target": tokens[4], "aux": tokens[6]}
        elif len(tokens) >= 5:
            return {"unit": unit_loc, "unit_type": unit_type, "order_type": "convoy",
                    "target": tokens[4]}
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "convoy"}

    if action == "B":
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "build"}

    if action == "D":
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "disband"}

    if action == "R":
        target = tokens[3] if len(tokens) > 3 else unit_loc
        return {"unit": unit_loc, "unit_type": unit_type, "order_type": "retreat", "target": target}

    # Unknown action, treat as hold
    return {"unit": unit_loc, "unit_type": unit_type, "order_type": "hold"}


def unit_fingerprint(units_list):
    """Create a hashable fingerprint from a list of unit strings.

    Sorts to ensure deterministic ordering. Returns a tuple like:
    (('A', 'bud'), ('A', 'vie'), ('F', 'tri'))
    """
    parsed = []
    for u in units_list:
        utype, loc = parse_unit(u)
        if utype and loc:
            parsed.append((utype, loc))
    return tuple(sorted(parsed))


def orders_fingerprint(orders_list):
    """Create a hashable fingerprint from a list of order strings.

    Returns sorted tuple of normalized order strings.
    """
    return tuple(sorted(orders_list))


def extract_game_openings(game):
    """Extract opening data for each power from a single game.

    Returns a dict: {power: {phase_name: {"orders": [...], "units_after": [...], "centers_after": [...]}}}
    """
    phases_by_name = {}
    for p in game.get("phases", []):
        phases_by_name[p["name"]] = p

    outcome = game.get("outcome", {})
    result = {}

    for power in POWERS:
        power_data = {}
        for phase_name in TARGET_PHASES:
            phase = phases_by_name.get(phase_name)
            if not phase:
                break  # If a phase is missing, skip remaining phases

            orders = phase.get("orders", {}).get(power, [])
            units = phase.get("units", {}).get(power, [])
            centers = phase.get("centers", {}).get(power, [])

            # For retreat phases that may follow, check for them
            retreat_name = phase_name[0] + phase_name[1:5] + "R"
            retreat_phase = phases_by_name.get(retreat_name)
            retreat_orders = []
            if retreat_phase:
                retreat_orders = retreat_phase.get("orders", {}).get(power, [])

            power_data[phase_name] = {
                "orders": orders,
                "units": units,
                "centers": centers,
                "retreat_orders": retreat_orders,
            }

        power_outcome = outcome.get(power, {})
        result[power] = {
            "phases": power_data,
            "final_centers": power_outcome.get("centers", 0),
            "result": power_outcome.get("result", "unknown"),
        }

    return result


def get_position_key(power_data, phase_name):
    """Get a position fingerprint for a power at a given phase.

    Uses the units at the START of that phase (which is what the orders act on).
    """
    phase = power_data.get("phases", {}).get(phase_name)
    if not phase:
        return None
    return unit_fingerprint(phase["units"])


def process_games():
    """Stream through games.jsonl and aggregate opening data."""
    log.info("Reading games from %s", GAMES_PATH)

    # Structure: power -> phase -> position_key -> orders_key -> {count, total_centers, wins}
    clusters = defaultdict(lambda: defaultdict(lambda: defaultdict(lambda: defaultdict(
        lambda: {"count": 0, "total_centers": 0, "wins": 0, "orders": None}
    ))))

    # Also track per-power/phase totals for frequency calculation
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

            # Only standard map games
            if game.get("map") != "standard":
                skipped += 1
                continue

            # Need at least through F1901M (first 2 movement phases)
            phase_names = {p["name"] for p in game.get("phases", [])}
            if "S1901M" not in phase_names or "F1901M" not in phase_names:
                skipped += 1
                continue

            total_games += 1
            openings = extract_game_openings(game)

            for power in POWERS:
                pdata = openings[power]
                is_win = pdata["result"] in ("solo", "draw")
                final_sc = pdata["final_centers"]

                for phase_name in TARGET_PHASES:
                    phase_info = pdata["phases"].get(phase_name)
                    if not phase_info or not phase_info["orders"]:
                        break  # Stop processing further phases for this power

                    pos_key = get_position_key(pdata, phase_name)
                    if pos_key is None:
                        break

                    ord_key = orders_fingerprint(phase_info["orders"])

                    entry = clusters[power][phase_name][pos_key][ord_key]
                    entry["count"] += 1
                    entry["total_centers"] += final_sc
                    if is_win:
                        entry["wins"] += 1
                    if entry["orders"] is None:
                        entry["orders"] = phase_info["orders"]

                    phase_totals[power][phase_name] += 1

            if (line_num + 1) % 20000 == 0:
                elapsed = time.time() - start_time
                rate = (line_num + 1) / elapsed
                log.info(
                    "  Processed %d lines (%d games, %d skipped) — %.0f lines/sec",
                    line_num + 1, total_games, skipped, rate
                )

    elapsed = time.time() - start_time
    log.info(
        "Done: %d games processed, %d skipped in %.1fs",
        total_games, skipped, elapsed
    )

    return clusters, phase_totals, total_games


def build_opening_book(clusters, phase_totals, total_games):
    """Convert raw clusters into the opening book JSON format.

    Filters by frequency threshold and computes statistics.
    """
    book_entries = []
    stats = defaultdict(lambda: defaultdict(lambda: {
        "total": 0, "covered": 0, "entries": []
    }))

    for power in POWERS:
        for phase_name in TARGET_PHASES:
            total_for_phase = phase_totals[power][phase_name]
            if total_for_phase == 0:
                continue

            threshold = PHASE_THRESHOLDS.get(phase_name, MIN_FREQUENCY)
            min_count = max(10, int(total_for_phase * threshold))

            phase_clusters = clusters[power][phase_name]
            covered_games = 0

            for pos_key, order_variants in phase_clusters.items():
                # Build condition from position key
                condition = {}
                for utype, loc in pos_key:
                    condition[loc] = "army" if utype == "A" else "fleet"

                entries_for_position = []

                for ord_key, data in order_variants.items():
                    if data["count"] < min_count:
                        continue

                    freq = data["count"] / total_for_phase
                    avg_centers = data["total_centers"] / data["count"]
                    win_rate = data["wins"] / data["count"]

                    # Parse orders into structured format
                    parsed_orders = []
                    for o in data["orders"]:
                        parsed = parse_order(o)
                        if parsed:
                            parsed_orders.append(parsed)

                    entry = {
                        "weight": round(freq, 4),
                        "avg_centers": round(avg_centers, 2),
                        "win_rate": round(win_rate, 4),
                        "games": data["count"],
                        "orders": parsed_orders,
                    }
                    entries_for_position.append(entry)
                    covered_games += data["count"]

                if entries_for_position:
                    # Sort by weight descending
                    entries_for_position.sort(key=lambda e: -e["weight"])

                    # Name the entries by rank
                    for i, e in enumerate(entries_for_position):
                        e["name"] = f"{power}_{phase_name}_var{i+1}"

                    book_entries.append({
                        "power": power,
                        "phase": phase_name.lower().replace("1901", "_1901_").replace(
                            "1902", "_1902_").rstrip("_"),
                        "phase_code": phase_name,
                        "condition": condition,
                        "entries": entries_for_position,
                    })

            stats[power][phase_name]["total"] = total_for_phase
            stats[power][phase_name]["covered"] = min(covered_games, total_for_phase)
            stats[power][phase_name]["entries"] = []

    return book_entries, stats


def generate_analysis(book_entries, phase_totals, total_games, stats):
    """Generate the markdown analysis report."""
    lines = [
        "# Opening Book Analysis",
        "",
        f"**Total games analyzed:** {total_games:,}",
        f"**Phases covered:** Spring 1901 through Fall 1902",
        f"**Map:** Standard only",
        "",
    ]

    # Per-power coverage
    lines.append("## Coverage Statistics")
    lines.append("")
    lines.append("Percentage of games where at least one book entry matches.")
    lines.append("")
    header = "| Power |"
    sep = "|-------|"
    for phase in TARGET_PHASES:
        header += f" {phase} |"
        sep += "--------|"
    lines.append(header)
    lines.append(sep)

    for power in POWERS:
        row = f"| {power.capitalize()} |"
        for phase in TARGET_PHASES:
            total = phase_totals[power][phase]
            if total == 0:
                row += " N/A |"
                continue
            # Count covered games
            covered = 0
            for entry in book_entries:
                if entry["power"] == power and entry["phase_code"] == phase:
                    for e in entry["entries"]:
                        covered += e["games"]
            pct = min(100.0, 100.0 * covered / total)
            row += f" {pct:.1f}% |"
        lines.append(row)
    lines.append("")

    # Top openings per power per phase
    lines.append("## Top Openings by Power and Phase")
    lines.append("")

    for power in POWERS:
        lines.append(f"### {power.capitalize()}")
        lines.append("")

        for phase in TARGET_PHASES:
            phase_entries = [
                e for e in book_entries
                if e["power"] == power and e["phase_code"] == phase
            ]
            if not phase_entries:
                continue

            lines.append(f"#### {phase}")
            lines.append("")

            # Flatten all variants, sort by weight
            all_variants = []
            for pe in phase_entries:
                for v in pe["entries"]:
                    all_variants.append((pe["condition"], v))

            all_variants.sort(key=lambda x: -x[1]["weight"])

            lines.append("| # | Weight | Games | Avg SCs | Win Rate | Orders |")
            lines.append("|---|--------|-------|---------|----------|--------|")

            for i, (cond, v) in enumerate(all_variants[:5]):
                orders_str = "; ".join(
                    format_order_brief(o) for o in v["orders"]
                )
                if len(orders_str) > 80:
                    orders_str = orders_str[:77] + "..."
                lines.append(
                    f"| {i+1} | {v['weight']:.1%} | {v['games']:,} | "
                    f"{v['avg_centers']:.1f} | {v['win_rate']:.1%} | {orders_str} |"
                )

            lines.append("")

    # Book vs off-book comparison
    lines.append("## Book vs Off-Book Outcome Comparison")
    lines.append("")
    lines.append("Average final supply centers for games matching a book entry vs those that don't.")
    lines.append("")
    lines.append("| Power | Phase | Book Avg SCs | Book Win% | Total Games |")
    lines.append("|-------|-------|-------------|-----------|-------------|")

    for power in POWERS:
        for phase in TARGET_PHASES:
            phase_entries = [
                e for e in book_entries
                if e["power"] == power and e["phase_code"] == phase
            ]
            if not phase_entries:
                continue
            total_games_phase = 0
            total_sc = 0
            total_wins = 0
            for pe in phase_entries:
                for v in pe["entries"]:
                    total_games_phase += v["games"]
                    total_sc += v["avg_centers"] * v["games"]
                    total_wins += v["win_rate"] * v["games"]
            if total_games_phase > 0:
                avg_sc = total_sc / total_games_phase
                avg_win = total_wins / total_games_phase
                lines.append(
                    f"| {power.capitalize()} | {phase} | {avg_sc:.2f} | "
                    f"{avg_win:.1%} | {total_games_phase:,} |"
                )

    lines.append("")
    lines.append(f"*Generated by `data/scripts/extract_openings.py`*")

    return "\n".join(lines)


def format_order_brief(order):
    """Format an order dict into a brief string."""
    ot = order.get("order_type", "?")
    unit = order.get("unit", "?")
    ut = order.get("unit_type", "?")

    if ot == "hold":
        return f"{ut} {unit} H"
    elif ot == "move":
        via = " VIA" if order.get("via_convoy") else ""
        return f"{ut} {unit}-{order.get('target', '?')}{via}"
    elif ot == "support_hold":
        return f"{ut} {unit} S {order.get('target', '?')}"
    elif ot == "support_move":
        return f"{ut} {unit} S {order.get('target', '?')}-{order.get('aux', '?')}"
    elif ot == "convoy":
        return f"{ut} {unit} C {order.get('target', '?')}-{order.get('aux', '?')}"
    elif ot == "build":
        return f"{ut} {unit} B"
    elif ot == "disband":
        return f"{ut} {unit} D"
    elif ot == "retreat":
        return f"{ut} {unit} R {order.get('target', '?')}"
    return f"{ut} {unit} {ot}"


def main():
    if not GAMES_PATH.exists():
        log.error("games.jsonl not found at %s", GAMES_PATH)
        sys.exit(1)

    log.info("Starting opening book extraction")

    clusters, phase_totals, total_games = process_games()

    log.info("Building opening book entries")
    book_entries, stats = build_opening_book(clusters, phase_totals, total_games)

    # Compute total entry count
    total_entries = sum(len(e["entries"]) for e in book_entries)
    log.info("Generated %d book entries across %d position clusters", total_entries, len(book_entries))

    # Write JSON output
    OUTPUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(OUTPUT_PATH, "w") as f:
        json.dump(book_entries, f, indent=2)
    log.info("Wrote opening book to %s (%.1f KB)", OUTPUT_PATH, OUTPUT_PATH.stat().st_size / 1024)

    # Write analysis
    analysis = generate_analysis(book_entries, phase_totals, total_games, stats)
    ANALYSIS_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(ANALYSIS_PATH, "w") as f:
        f.write(analysis)
    log.info("Wrote analysis to %s", ANALYSIS_PATH)

    # Print quick summary
    print(f"\n=== Opening Book Summary ===")
    print(f"Games analyzed: {total_games:,}")
    print(f"Position clusters: {len(book_entries)}")
    print(f"Total order variants: {total_entries}")
    print(f"Output: {OUTPUT_PATH}")
    print(f"Analysis: {ANALYSIS_PATH}")


if __name__ == "__main__":
    main()
