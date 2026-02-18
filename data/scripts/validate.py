#!/usr/bin/env python3
"""Validate parsed Diplomacy game data and produce summary statistics.

Reads data/processed/games.jsonl and checks for:
  - Schema completeness (required fields present)
  - Province name validity (all 3-letter codes in canonical set)
  - Phase sequence validity (chronological order, valid phase names)
  - Order count sanity (at least some orders per movement phase)
  - Supply center consistency (no more than 34 total centers)
  - Game completeness (at least a few phases)

Outputs a validation report and flags quarantined games.
"""

import argparse
import json
import logging
import re
import sys
from collections import Counter
from pathlib import Path

from province_map import PROVINCE_SET, SPLIT_COASTS

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

PROCESSED_DIR = Path(__file__).resolve().parent.parent / "processed"

PHASE_RE = re.compile(r"^[SFW]\d{4}[MRA]$")
REQUIRED_GAME_FIELDS = {"game_id", "source", "map", "num_phases", "year_range", "outcome", "phases"}
REQUIRED_PHASE_FIELDS = {"name", "season", "year", "type", "units", "centers", "orders", "results"}
MAX_SUPPLY_CENTERS = 34


class ValidationResult:
    """Accumulates validation issues for a single game."""

    def __init__(self, game_id: str):
        self.game_id = game_id
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, msg: str):
        self.errors.append(msg)

    def warn(self, msg: str):
        self.warnings.append(msg)

    @property
    def is_valid(self) -> bool:
        return len(self.errors) == 0

    @property
    def is_quarantined(self) -> bool:
        return len(self.errors) > 0


def validate_province_in_unit(unit_str: str) -> str | None:
    """Extract and validate province from a unit string like 'A par' or 'F spa/nc'."""
    parts = unit_str.split()
    if len(parts) < 2:
        return f"invalid unit format: {unit_str}"
    loc = parts[1]
    base = loc.split("/")[0]
    if base not in PROVINCE_SET:
        return f"unknown province in unit: {unit_str}"
    # Validate coast if present
    if "/" in loc:
        coast = loc.split("/")[1]
        if base in SPLIT_COASTS:
            if coast not in SPLIT_COASTS[base]:
                return f"invalid coast {coast} for {base} in unit: {unit_str}"
        else:
            return f"coast specified for non-split province: {unit_str}"
    return None


def validate_game(game: dict) -> ValidationResult:
    """Validate a single game record."""
    gid = game.get("game_id", "unknown")
    result = ValidationResult(gid)

    # Check required fields
    missing = REQUIRED_GAME_FIELDS - set(game.keys())
    if missing:
        result.error(f"missing top-level fields: {missing}")
        return result

    phases = game["phases"]
    if not isinstance(phases, list):
        result.error("phases is not a list")
        return result

    if len(phases) < 2:
        result.error(f"too few phases: {len(phases)}")

    # Check phase sequence
    prev_year = 0
    movement_phases = 0
    total_orders = 0

    for i, phase in enumerate(phases):
        pmissing = REQUIRED_PHASE_FIELDS - set(phase.keys())
        if pmissing:
            result.error(f"phase {i} missing fields: {pmissing}")
            continue

        name = phase["name"]
        if not PHASE_RE.match(name):
            result.error(f"invalid phase name: {name}")
            continue

        year = phase["year"]
        if year < prev_year:
            result.warn(f"phase {i} year {year} < previous {prev_year}")
        prev_year = year

        # Validate units
        for power, units in phase.get("units", {}).items():
            for u in units:
                err = validate_province_in_unit(u)
                if err:
                    result.warn(err)

        # Validate centers
        all_centers = set()
        for power, centers in phase.get("centers", {}).items():
            for c in centers:
                if c not in PROVINCE_SET:
                    result.warn(f"unknown center: {c}")
                all_centers.add(c)
        if len(all_centers) > MAX_SUPPLY_CENTERS:
            result.warn(f"phase {i}: total unique centers {len(all_centers)} > {MAX_SUPPLY_CENTERS}")

        # Count orders for movement phases
        if phase["type"] == "movement":
            movement_phases += 1
            phase_orders = sum(len(v) for v in phase.get("orders", {}).values())
            total_orders += phase_orders

    if movement_phases > 0 and total_orders == 0:
        result.error("no orders in any movement phase")

    # Validate outcome
    outcome = game.get("outcome", {})
    total_sc = sum(v.get("centers", 0) for v in outcome.values())
    if total_sc > MAX_SUPPLY_CENTERS:
        result.warn(f"outcome total centers {total_sc} > {MAX_SUPPLY_CENTERS}")

    return result


def compute_statistics(games: list[dict]) -> dict:
    """Compute summary statistics from validated games."""
    stats = {
        "total_games": len(games),
        "total_phases": 0,
        "total_orders": 0,
        "phase_type_dist": Counter(),
        "source_dist": Counter(),
        "year_range_dist": Counter(),
        "outcome_dist": Counter(),
        "phases_per_game": [],
        "orders_per_phase": [],
        "solo_wins": 0,
        "draws": 0,
        "games_by_year_span": Counter(),
    }

    for game in games:
        source = game.get("source", "unknown")
        stats["source_dist"][source] += 1

        phases = game.get("phases", [])
        stats["total_phases"] += len(phases)
        stats["phases_per_game"].append(len(phases))

        yr = game.get("year_range", [0, 0])
        if len(yr) == 2:
            span = yr[1] - yr[0]
            stats["games_by_year_span"][span] += 1

        for phase in phases:
            ptype = phase.get("type", "unknown")
            stats["phase_type_dist"][ptype] += 1
            phase_orders = sum(len(v) for v in phase.get("orders", {}).values())
            stats["total_orders"] += phase_orders
            if ptype == "movement":
                stats["orders_per_phase"].append(phase_orders)

        # Outcome analysis
        outcome = game.get("outcome", {})
        has_solo = any(v.get("result") == "solo" for v in outcome.values())
        if has_solo:
            stats["solo_wins"] += 1
            for power, v in outcome.items():
                if v.get("result") == "solo":
                    stats["outcome_dist"][f"solo_{power}"] += 1
        else:
            draw_powers = [p for p, v in outcome.items() if v.get("result") == "draw"]
            if draw_powers:
                stats["draws"] += 1
                stats["outcome_dist"][f"draw_{len(draw_powers)}way"] += 1

    return stats


def print_report(
    total: int,
    valid: int,
    quarantined: int,
    error_counts: Counter,
    warning_counts: Counter,
    stats: dict,
):
    """Print the validation report."""
    print("\n" + "=" * 60)
    print("VALIDATION REPORT")
    print("=" * 60)

    print(f"\nTotal games:       {total}")
    print(f"Valid games:       {valid} ({valid * 100 / total:.1f}%)" if total > 0 else "")
    print(f"Quarantined:       {quarantined} ({quarantined * 100 / total:.1f}%)" if total > 0 else "")

    if error_counts:
        print("\n--- Error distribution ---")
        for err, count in error_counts.most_common(20):
            print(f"  {count:6d}  {err}")

    if warning_counts:
        print("\n--- Warning distribution ---")
        for warn, count in warning_counts.most_common(20):
            print(f"  {count:6d}  {warn}")

    print("\n--- Dataset statistics ---")
    print(f"Sources:           {dict(stats['source_dist'])}")
    print(f"Total phases:      {stats['total_phases']:,}")
    print(f"Total orders:      {stats['total_orders']:,}")
    print(f"Phase types:       {dict(stats['phase_type_dist'])}")

    ppg = stats["phases_per_game"]
    if ppg:
        print(f"Phases/game:       min={min(ppg)}, max={max(ppg)}, avg={sum(ppg)/len(ppg):.1f}")

    oppg = stats["orders_per_phase"]
    if oppg:
        print(f"Orders/move phase: min={min(oppg)}, max={max(oppg)}, avg={sum(oppg)/len(oppg):.1f}")

    print(f"\nSolo victories:    {stats['solo_wins']}")
    print(f"Draws:             {stats['draws']}")

    if stats["outcome_dist"]:
        print("\n--- Outcome distribution ---")
        for outcome, count in stats["outcome_dist"].most_common(20):
            print(f"  {count:6d}  {outcome}")

    if stats["games_by_year_span"]:
        print("\n--- Game length (years) ---")
        for span in sorted(stats["games_by_year_span"]):
            count = stats["games_by_year_span"][span]
            print(f"  {span:3d} years: {count:6d} games")

    print("\n" + "=" * 60)


def main():
    parser = argparse.ArgumentParser(description="Validate parsed Diplomacy game data")
    parser.add_argument(
        "--input",
        type=Path,
        default=PROCESSED_DIR / "games.jsonl",
        help="Input JSONL file",
    )
    parser.add_argument(
        "--quarantine",
        type=Path,
        default=PROCESSED_DIR / "quarantined.jsonl",
        help="Output file for quarantined games",
    )
    parser.add_argument(
        "--stats-json",
        type=Path,
        default=PROCESSED_DIR / "stats.json",
        help="Output file for statistics JSON",
    )
    args = parser.parse_args()

    if not args.input.exists():
        log.error("Input file not found: %s. Run parse.py first.", args.input)
        sys.exit(1)

    log.info("Validating %s ...", args.input)

    games = []
    valid_games = []
    quarantined_games = []
    error_counts: Counter = Counter()
    warning_counts: Counter = Counter()

    with open(args.input, "r") as f:
        for idx, line in enumerate(f):
            line = line.strip()
            if not line:
                continue
            try:
                game = json.loads(line)
            except json.JSONDecodeError as e:
                log.warning("JSON error at line %d: %s", idx + 1, e)
                continue

            games.append(game)
            result = validate_game(game)

            if result.is_quarantined:
                quarantined_games.append(game)
                for err in result.errors:
                    error_counts[err] += 1
            else:
                valid_games.append(game)

            for warn in result.warnings:
                warning_counts[warn] += 1

            if (idx + 1) % 10000 == 0:
                log.info("  ... validated %d games", idx + 1)

    log.info(
        "Validation complete: %d valid, %d quarantined out of %d",
        len(valid_games),
        len(quarantined_games),
        len(games),
    )

    # Write quarantined games
    if quarantined_games:
        args.quarantine.parent.mkdir(parents=True, exist_ok=True)
        with open(args.quarantine, "w") as f:
            for g in quarantined_games:
                f.write(json.dumps(g, separators=(",", ":")) + "\n")
        log.info("Quarantined games written to %s", args.quarantine)

    # Compute and save statistics
    stats = compute_statistics(valid_games)
    args.stats_json.parent.mkdir(parents=True, exist_ok=True)

    # Convert Counters to dicts for JSON serialization
    stats_serializable = {
        k: (dict(v) if isinstance(v, Counter) else v)
        for k, v in stats.items()
        if k not in ("phases_per_game", "orders_per_phase")
    }
    ppg = stats["phases_per_game"]
    oppg = stats["orders_per_phase"]
    stats_serializable["phases_per_game_summary"] = {
        "min": min(ppg) if ppg else 0,
        "max": max(ppg) if ppg else 0,
        "avg": round(sum(ppg) / len(ppg), 1) if ppg else 0,
    }
    stats_serializable["orders_per_phase_summary"] = {
        "min": min(oppg) if oppg else 0,
        "max": max(oppg) if oppg else 0,
        "avg": round(sum(oppg) / len(oppg), 1) if oppg else 0,
    }

    with open(args.stats_json, "w") as f:
        json.dump(stats_serializable, f, indent=2)
    log.info("Statistics written to %s", args.stats_json)

    print_report(
        total=len(games),
        valid=len(valid_games),
        quarantined=len(quarantined_games),
        error_counts=error_counts,
        warning_counts=warning_counts,
        stats=stats,
    )

    # Exit with error if quarantine rate > 5%
    if len(games) > 0:
        quarantine_rate = len(quarantined_games) / len(games)
        if quarantine_rate > 0.05:
            log.warning("Quarantine rate %.1f%% exceeds 5%% threshold", quarantine_rate * 100)
            sys.exit(1)


if __name__ == "__main__":
    main()
