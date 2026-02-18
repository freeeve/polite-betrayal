#!/usr/bin/env python3
"""Summarize the opening book JSON into a human-readable report.

Reads data/processed/opening_book.json and prints a formatted summary
showing the most popular openings for each power and phase, with
order descriptions, win rates, and positional context.

Usage:
    python data/scripts/summarize_openings.py [--power france] [--year 1901] [--top 5]
"""

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path

DATA_DIR = Path(__file__).resolve().parent.parent
BOOK_PATH = DATA_DIR / "processed" / "opening_book.json"

POWERS = ["austria", "england", "france", "germany", "italy", "russia", "turkey"]

SEASON_ABBREV = {"spring": "S", "fall": "F", "winter": "W"}
PHASE_ABBREV = {"movement": "M", "retreat": "R", "build": "A"}

# Standard Diplomacy province full names for readability.
PROVINCE_NAMES = {
    "alb": "Albania", "ank": "Ankara", "apu": "Apulia", "arm": "Armenia",
    "bal": "Baltic Sea", "bar": "Barents Sea", "bel": "Belgium",
    "ber": "Berlin", "bla": "Black Sea", "boh": "Bohemia",
    "bot": "Gulf of Bothnia", "bre": "Brest", "bud": "Budapest",
    "bul": "Bulgaria", "bur": "Burgundy", "cly": "Clyde",
    "con": "Constantinople", "den": "Denmark", "eas": "Eastern Med",
    "edi": "Edinburgh", "eng": "English Channel", "fin": "Finland",
    "gal": "Galicia", "gas": "Gascony", "gol": "Gulf of Lyon",
    "gre": "Greece", "hel": "Helgoland Bight", "hol": "Holland",
    "ion": "Ionian Sea", "iri": "Irish Sea", "kie": "Kiel",
    "lon": "London", "lvn": "Livonia", "lvp": "Liverpool",
    "mao": "Mid-Atlantic", "mar": "Marseilles", "mos": "Moscow",
    "mun": "Munich", "naf": "North Africa", "nao": "North Atlantic",
    "nap": "Naples", "nrg": "Norwegian Sea", "nth": "North Sea",
    "nwy": "Norway", "par": "Paris", "pic": "Picardy",
    "pie": "Piedmont", "por": "Portugal", "pru": "Prussia",
    "rom": "Rome", "ruh": "Ruhr", "rum": "Rumania",
    "ser": "Serbia", "sev": "Sevastopol", "sil": "Silesia",
    "ska": "Skagerrak", "smy": "Smyrna", "spa": "Spain",
    "stp": "St. Petersburg", "swe": "Sweden", "syr": "Syria",
    "tri": "Trieste", "tun": "Tunisia", "tus": "Tuscany",
    "tyr": "Tyrolia", "tys": "Tyrrhenian Sea", "ukr": "Ukraine",
    "ven": "Venice", "vie": "Vienna", "wal": "Wales",
    "war": "Warsaw", "wes": "Western Med", "yor": "Yorkshire",
}


def phase_code(entry):
    """Reconstruct phase code like 'S1901M' from entry fields."""
    s = SEASON_ABBREV.get(entry["season"], "?")
    t = PHASE_ABBREV.get(entry["phase"], "?")
    return f"{s}{entry['year']}{t}"


def phase_label(entry):
    """Human-readable phase label like 'Spring 1901 Movement'."""
    return f"{entry['season'].capitalize()} {entry['year']} {entry['phase'].capitalize()}"


def prov(loc):
    """Province abbreviation, uppercased for display."""
    return loc.upper()


def format_order(order):
    """Format an OrderInput dict into a readable order string."""
    ot = order.get("order_type", "?")
    loc = order.get("location", "?")
    ut = "A" if order.get("unit_type") == "army" else "F"
    coast = f"/{order['coast']}" if order.get("coast") else ""

    if ot == "hold":
        return f"{ut} {prov(loc)}{coast} Hold"
    elif ot == "move":
        tc = f"/{order['target_coast']}" if order.get("target_coast") else ""
        return f"{ut} {prov(loc)}{coast} -> {prov(order.get('target', '?'))}{tc}"
    elif ot == "support":
        aux_loc = order.get("aux_loc", "?")
        aux_target = order.get("aux_target", "?")
        aut = "A" if order.get("aux_unit_type") == "army" else "F"
        if aux_loc == aux_target:
            return f"{ut} {prov(loc)}{coast} Support {aut} {prov(aux_loc)}"
        return f"{ut} {prov(loc)}{coast} Support {aut} {prov(aux_loc)} -> {prov(aux_target)}"
    elif ot == "convoy":
        return f"{ut} {prov(loc)} Convoy A {prov(order.get('aux_loc', '?'))} -> {prov(order.get('aux_target', '?'))}"
    elif ot == "build":
        return f"Build {ut} {prov(loc)}{coast}"
    elif ot == "disband":
        return f"Disband {ut} {prov(loc)}{coast}"
    elif ot == "retreat":
        tc = f"/{order['target_coast']}" if order.get("target_coast") else ""
        return f"{ut} {prov(loc)}{coast} Retreat -> {prov(order.get('target', '?'))}{tc}"
    return f"{ut} {prov(loc)} {ot}"


def format_positions(condition):
    """Format unit positions from condition into readable string."""
    positions = condition.get("positions", {})
    parts = []
    for loc, utype in sorted(positions.items()):
        ut = "A" if utype == "army" else "F"
        parts.append(f"{ut} {prov(loc)}")
    return ", ".join(parts)


def format_stances(condition, power):
    """Format neighbor stances into a compact readable string."""
    stances = condition.get("neighbor_stance", {})
    if not stances:
        return "N/A"
    parts = []
    for neighbor, stance in sorted(stances.items()):
        if neighbor == power:
            continue
        icon = {"aggressive": "!", "neutral": "~", "retreating": "-"}.get(stance, "?")
        parts.append(f"{neighbor[:3].upper()}{icon}")
    return " ".join(parts)


def format_theaters(condition):
    """Format theater presence into a compact string."""
    theaters = condition.get("theaters", {})
    if not theaters:
        return "N/A"
    parts = []
    for theater, count in sorted(theaters.items()):
        if count > 0:
            parts.append(f"{theater}:{count}")
    return ", ".join(parts)


def print_separator(char="─", width=80):
    print(char * width)


def print_entry(entry, top_n, rank_offset=0):
    """Print a single opening book entry with its top options."""
    cond = entry["condition"]
    scs = cond.get("owned_scs", [])
    pressure = cond.get("border_pressure", 0)

    print(f"  Units: {format_positions(cond)}")
    if scs:
        print(f"  SCs ({len(scs)}): {', '.join(prov(sc) for sc in scs)}")
    print(f"  Theaters: {format_theaters(cond)}")
    print(f"  Fleets: {cond.get('fleet_count', 0)}, Armies: {cond.get('army_count', 0)}")
    print(f"  Border pressure: {pressure}")
    print(f"  Neighbors: {format_stances(cond, entry['power'])}")
    print()

    options = entry["options"][:top_n]
    for i, opt in enumerate(options):
        rank = rank_offset + i + 1
        games = opt.get("_games", 0)
        weight = opt.get("weight", 0)
        avg_sc = opt.get("_avg_centers", 0)
        win_rate = opt.get("_win_rate", 0)

        print(f"  #{rank}  {weight:5.1%} of games in this position  "
              f"({games:,} games, avg {avg_sc:.1f} SCs, {win_rate:.1%} win rate)")

        for order in opt.get("orders", []):
            print(f"        {format_order(order)}")
        print()


def summarize_power_phase(entries, power, phase_entries, top_n):
    """Summarize all clusters for a power at a specific phase."""
    if not phase_entries:
        return

    label = phase_label(phase_entries[0])
    code = phase_code(phase_entries[0])
    total_options = sum(len(e["options"]) for e in phase_entries)
    total_games = sum(
        opt["_games"]
        for e in phase_entries
        for opt in e["options"]
    )

    print(f"\n  {label} ({code})")
    print(f"  {len(phase_entries)} position cluster(s), {total_options} order variant(s), ~{total_games:,} game observations")
    print_separator("·", 80)

    if len(phase_entries) == 1:
        print_entry(phase_entries[0], top_n)
    else:
        # Multiple clusters: show each with a header
        for ci, pe in enumerate(sorted(phase_entries, key=lambda e: -sum(o["_games"] for o in e["options"]))):
            cluster_games = sum(o["_games"] for o in pe["options"])
            print(f"\n  Cluster {ci + 1} ({cluster_games:,} games)")
            print_entry(pe, top_n)
            if ci >= 4:
                remaining = len(phase_entries) - ci - 1
                if remaining > 0:
                    print(f"  ... and {remaining} more cluster(s)")
                break


def main():
    parser = argparse.ArgumentParser(description="Summarize Diplomacy opening book")
    parser.add_argument("--power", "-p", type=str, default=None,
                        help="Filter to a specific power (e.g. france)")
    parser.add_argument("--year", "-y", type=int, default=None,
                        help="Filter to a specific year (e.g. 1901)")
    parser.add_argument("--phase", type=str, default=None,
                        help="Filter to a specific phase code (e.g. S1901M)")
    parser.add_argument("--top", "-t", type=int, default=5,
                        help="Number of top options to show per cluster (default: 5)")
    parser.add_argument("--book", type=str, default=str(BOOK_PATH),
                        help="Path to opening_book.json")
    args = parser.parse_args()

    book_path = Path(args.book)
    if not book_path.exists():
        print(f"Error: opening book not found at {book_path}", file=sys.stderr)
        sys.exit(1)

    with open(book_path) as f:
        data = json.load(f)

    entries = data["entries"]

    # Apply filters
    if args.power:
        power_filter = args.power.lower()
        entries = [e for e in entries if e["power"] == power_filter]
        if not entries:
            print(f"No entries found for power: {args.power}")
            sys.exit(0)

    if args.year:
        entries = [e for e in entries if e["year"] == args.year]
        if not entries:
            print(f"No entries found for year: {args.year}")
            sys.exit(0)

    if args.phase:
        entries = [e for e in entries if phase_code(e) == args.phase.upper()]
        if not entries:
            print(f"No entries found for phase: {args.phase}")
            sys.exit(0)

    # Global summary
    total_clusters = len(entries)
    total_options = sum(len(e["options"]) for e in entries)
    powers_present = sorted(set(e["power"] for e in entries))
    years_present = sorted(set(e["year"] for e in entries))

    print()
    print_separator("═", 80)
    print("  DIPLOMACY OPENING BOOK SUMMARY")
    print_separator("═", 80)
    print(f"  Clusters: {total_clusters:,}   Options: {total_options:,}")
    print(f"  Powers: {', '.join(p.capitalize() for p in powers_present)}")
    print(f"  Years: {min(years_present)}-{max(years_present)}")
    print()

    # Per-power breakdown
    by_power = defaultdict(list)
    for e in entries:
        by_power[e["power"]].append(e)

    for power in POWERS:
        if power not in by_power:
            continue

        power_entries = by_power[power]
        power_options = sum(len(e["options"]) for e in power_entries)
        power_years = sorted(set(e["year"] for e in power_entries))

        print_separator("═", 80)
        print(f"  {power.upper()}")
        print(f"  {len(power_entries)} clusters, {power_options} variants "
              f"({min(power_years)}-{max(power_years)})")
        print_separator("═", 80)

        # Group by phase
        by_phase = defaultdict(list)
        for e in power_entries:
            by_phase[phase_code(e)].append(e)

        # Sort phases chronologically
        phase_order = []
        for yr in sorted(power_years):
            for s in ["S", "F", "W"]:
                for t in ["M", "R", "A"]:
                    code = f"{s}{yr}{t}"
                    if code in by_phase:
                        phase_order.append(code)

        for code in phase_order:
            phase_entries = by_phase[code]
            summarize_power_phase(entries, power, phase_entries, args.top)

        print()


if __name__ == "__main__":
    import signal
    signal.signal(signal.SIGPIPE, signal.SIG_DFL)
    main()
