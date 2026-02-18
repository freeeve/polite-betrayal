#!/usr/bin/env python3
"""Parse downloaded Diplomacy datasets into a unified JSONL format.

Reads raw JSONL files from data/raw/ and writes normalized game records
to data/processed/games.jsonl (one JSON object per line).

Unified game record schema:
{
  "game_id": "research_12345",
  "source": "research",
  "map": "standard",
  "num_phases": 42,
  "year_range": [1901, 1910],
  "outcome": {
    "austria": {"centers": 0, "result": "loss"},
    "england": {"centers": 18, "result": "win"},
    ...
  },
  "phases": [
    {
      "name": "S1901M",
      "season": "spring",
      "year": 1901,
      "type": "movement",
      "units": {
        "austria": ["A vie", "A bud", "F tri"],
        ...
      },
      "centers": {
        "austria": ["vie", "bud", "tri"],
        ...
      },
      "orders": {
        "austria": ["A vie - tri", "A bud - ser", "F tri - alb"],
        ...
      },
      "results": {
        "A vie": [""],
        ...
      }
    }
  ]
}
"""

import argparse
import hashlib
import json
import logging
import re
import sys
from collections import Counter
from pathlib import Path

from province_map import (
    POWER_NAMES,
    PROVINCE_SET,
    extract_coast,
    normalize_power,
    normalize_province,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

RAW_DIR = Path(__file__).resolve().parent.parent / "raw"
PROCESSED_DIR = Path(__file__).resolve().parent.parent / "processed"

# Phase name pattern: S1901M, F1901M, W1901A, S1902R, etc.
PHASE_RE = re.compile(r"^([SFW])(\d{4})([MRA])$")

SEASON_MAP = {"S": "spring", "F": "fall", "W": "winter"}
TYPE_MAP = {"M": "movement", "R": "retreat", "A": "adjustment"}


def parse_phase_name(name: str) -> dict | None:
    """Parse a phase name like 'S1901M' into structured components."""
    m = PHASE_RE.match(name)
    if not m:
        return None
    return {
        "season": SEASON_MAP[m.group(1)],
        "year": int(m.group(2)),
        "type": TYPE_MAP[m.group(3)],
    }


def normalize_unit(unit_str: str) -> str | None:
    """Normalize a unit string like 'A PAR' or 'F SPA/NC' to lowercase.

    Returns e.g. 'A par', 'F spa/nc', or None if invalid.
    """
    parts = unit_str.strip().split()
    if len(parts) < 2:
        return None
    unit_type = parts[0].upper()
    if unit_type not in ("A", "F"):
        return None
    loc = parts[1].strip()
    coast = extract_coast(loc)
    prov = normalize_province(loc)
    if not prov:
        return None
    if coast:
        return f"{unit_type} {prov}/{coast}"
    return f"{unit_type} {prov}"


def normalize_order(order_str: str) -> str | None:
    """Best-effort normalization of an order string.

    Keeps the original structure but normalizes province names to 3-letter codes.
    Returns None if order cannot be parsed at all.
    """
    s = order_str.strip()
    if not s:
        return None
    # Replace province tokens (3+ letter uppercase words) with normalized forms
    tokens = s.split()
    result = []
    for tok in tokens:
        # Skip operator tokens
        if tok in ("-", "S", "H", "C", "B", "D", "R", "VIA"):
            result.append(tok)
            continue
        # Handle unit type prefix
        if tok in ("A", "F"):
            result.append(tok)
            continue
        # Try to normalize as province
        prov = normalize_province(tok)
        if prov:
            result.append(prov)
        else:
            # Keep as-is (could be coast suffix like /NC)
            result.append(tok.lower())
    return " ".join(result)


def normalize_orders_list(orders: list[str]) -> list[str]:
    """Normalize a list of order strings, dropping unparseable ones."""
    normalized = []
    for o in orders:
        n = normalize_order(o)
        if n:
            normalized.append(n)
    return normalized


def normalize_units_dict(units: dict) -> dict[str, list[str]]:
    """Normalize a power->units mapping.

    Input: {"AUSTRIA": ["A VIE", "A BUD", "F TRI"], ...}
    Output: {"austria": ["A vie", "A bud", "F tri"], ...}
    """
    result = {}
    for power_raw, unit_list in units.items():
        power = normalize_power(power_raw)
        if not power:
            continue
        normalized = []
        for u in unit_list:
            # Strip dislodged marker (leading *)
            clean = u.lstrip("* ")
            n = normalize_unit(clean)
            if n:
                normalized.append(n)
        result[power] = normalized
    return result


def normalize_centers_dict(centers: dict) -> dict[str, list[str]]:
    """Normalize a power->centers mapping.

    Input: {"AUSTRIA": ["VIE", "BUD", "TRI"], ...}
    Output: {"austria": ["vie", "bud", "tri"], ...}
    """
    result = {}
    for power_raw, center_list in centers.items():
        power = normalize_power(power_raw)
        if not power:
            continue
        normalized = []
        for c in center_list:
            p = normalize_province(c)
            if p:
                normalized.append(p)
        result[power] = normalized
    return result


def compute_outcome(final_centers: dict[str, list[str]]) -> dict:
    """Compute game outcome from the final center distribution."""
    outcome = {}
    max_centers = 0
    for power in POWER_NAMES:
        count = len(final_centers.get(power, []))
        max_centers = max(max_centers, count)

    for power in POWER_NAMES:
        count = len(final_centers.get(power, []))
        if count >= 18:
            result = "solo"
        elif count == max_centers and count > 0:
            result = "draw"
        elif count > 0:
            result = "survive"
        else:
            result = "eliminated"
        outcome[power] = {"centers": count, "result": result}
    return outcome


def parse_research_game(raw: dict, game_idx: int) -> dict | None:
    """Parse a single game from the diplomacy/research dataset format."""
    phases_raw = raw.get("phases", [])
    if not phases_raw:
        return None

    game_id = raw.get("id", f"research_{game_idx}")
    map_name = raw.get("map", "standard")

    # Only process standard map games
    if map_name != "standard":
        return None

    phases = []
    final_centers = {}

    for phase_raw in phases_raw:
        name = phase_raw.get("name", "")
        if name in ("FORMING", "COMPLETED"):
            continue

        parsed = parse_phase_name(name)
        if not parsed:
            continue

        state = phase_raw.get("state", {})
        orders_raw = phase_raw.get("orders", {})
        results_raw = phase_raw.get("results", {})

        units = normalize_units_dict(state.get("units", {}))
        centers = normalize_centers_dict(state.get("centers", {}))

        # Normalize orders: input is {power: [order_str, ...]}
        orders = {}
        for power_raw, order_list in orders_raw.items():
            power = normalize_power(power_raw)
            if power and order_list:
                orders[power] = normalize_orders_list(order_list)

        # Normalize results: input is {unit_str: [result_code, ...]}
        results = {}
        for unit_raw, res_list in results_raw.items():
            unit_n = normalize_unit(unit_raw)
            if unit_n:
                results[unit_n] = res_list

        phase = {
            "name": name,
            "season": parsed["season"],
            "year": parsed["year"],
            "type": parsed["type"],
            "units": units,
            "centers": centers,
            "orders": orders,
            "results": results,
        }
        phases.append(phase)
        if centers:
            final_centers = centers

    if not phases:
        return None

    years = [p["year"] for p in phases]
    outcome = compute_outcome(final_centers)

    return {
        "game_id": str(game_id),
        "source": "research",
        "map": map_name,
        "num_phases": len(phases),
        "year_range": [min(years), max(years)],
        "outcome": outcome,
        "phases": phases,
    }


def parse_research_file(path: Path, seen_ids: set) -> tuple[list[dict], int, int]:
    """Parse a JSONL file from the diplomacy/research dataset.

    Returns (games, parsed_count, skipped_count).
    """
    games = []
    skipped = 0
    log.info("Parsing %s ...", path.name)
    with open(path, "r") as f:
        for idx, line in enumerate(f):
            line = line.strip()
            if not line:
                continue
            try:
                raw = json.loads(line)
            except json.JSONDecodeError as e:
                log.warning("JSON decode error at line %d in %s: %s", idx + 1, path.name, e)
                skipped += 1
                continue

            game = parse_research_game(raw, idx)
            if game is None:
                skipped += 1
                continue

            # Deduplication by game_id
            gid = game["game_id"]
            if gid in seen_ids:
                skipped += 1
                continue
            seen_ids.add(gid)
            games.append(game)

            if (idx + 1) % 10000 == 0:
                log.info("  ... processed %d lines (%d games)", idx + 1, len(games))

    log.info("  Parsed %d games, skipped %d from %s", len(games), skipped, path.name)
    return games, len(games), skipped


def parse_all(source_filter: str = "all") -> tuple[list[dict], dict]:
    """Parse all available raw datasets. Returns (games, stats)."""
    games = []
    seen_ids: set[str] = set()
    stats = {
        "files_processed": 0,
        "total_parsed": 0,
        "total_skipped": 0,
        "by_source": {},
    }

    if source_filter in ("research", "all"):
        # Look for JSONL files from the research dataset
        research_files = sorted(RAW_DIR.glob("*.jsonl"))
        if not research_files:
            log.warning("No JSONL files found in %s. Run download.py first.", RAW_DIR)
        for path in research_files:
            file_games, parsed, skipped = parse_research_file(path, seen_ids)
            games.extend(file_games)
            stats["files_processed"] += 1
            stats["total_parsed"] += parsed
            stats["total_skipped"] += skipped
            stats["by_source"][path.name] = {"parsed": parsed, "skipped": skipped}

    return games, stats


def write_output(games: list[dict], output_path: Path):
    """Write games to JSONL file."""
    output_path.parent.mkdir(parents=True, exist_ok=True)
    log.info("Writing %d games to %s ...", len(games), output_path)
    with open(output_path, "w") as f:
        for game in games:
            f.write(json.dumps(game, separators=(",", ":")) + "\n")
    size_mb = output_path.stat().st_size / (1024 * 1024)
    log.info("Wrote %s (%.1f MB)", output_path.name, size_mb)


def main():
    parser = argparse.ArgumentParser(description="Parse Diplomacy datasets into unified format")
    parser.add_argument(
        "--source",
        choices=["research", "all"],
        default="all",
        help="Which source to parse (default: all)",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=PROCESSED_DIR / "games.jsonl",
        help="Output JSONL file path",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="Limit number of games to process (0 = unlimited, for testing)",
    )
    args = parser.parse_args()

    games, stats = parse_all(source_filter=args.source)

    if args.limit > 0:
        games = games[: args.limit]
        log.info("Limited to %d games (--limit)", args.limit)

    if not games:
        log.error("No games parsed. Check that raw data exists in %s", RAW_DIR)
        sys.exit(1)

    write_output(games, args.output)

    # Print summary
    print("\n=== Parse Summary ===")
    print(f"Files processed: {stats['files_processed']}")
    print(f"Games parsed:    {stats['total_parsed']}")
    print(f"Games skipped:   {stats['total_skipped']}")
    for fname, fstats in stats["by_source"].items():
        print(f"  {fname}: {fstats['parsed']} parsed, {fstats['skipped']} skipped")
    print(f"Output:          {args.output}")
    print(f"Total games:     {len(games)}")


if __name__ == "__main__":
    main()
