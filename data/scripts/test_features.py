#!/usr/bin/env python3
"""Tests for the feature extraction pipeline.

Verifies board state encoding, order encoding, value labels,
adjacency matrix, and dataset splitting using synthetic game data.
"""

import json
import tempfile
from pathlib import Path

import numpy as np

from features import (
    AREA_INDEX,
    AREAS,
    BICOASTAL_VARIANTS,
    FEAT_CAN_BUILD,
    FEAT_CAN_DISBAND,
    FEAT_DISLODGED_OWNER,
    FEAT_DISLODGED_TYPE,
    FEAT_PROVINCE_TYPE,
    FEAT_SC_OWNER,
    FEAT_UNIT_OWNER,
    FEAT_UNIT_TYPE,
    INLAND_PROVINCES,
    NUM_AREAS,
    NUM_FEATURES,
    NUM_POWERS,
    ORDER_TYPES,
    POWER_INDEX,
    SEA_PROVINCES,
    build_adjacency_matrix,
    encode_board_state,
    encode_order_label,
    encode_value_labels,
    extract_game_samples,
    parse_order,
    save_dataset,
    split_dataset,
)
from province_map import PROVINCES


def _make_s1901m_phase() -> dict:
    """Create a standard S1901M opening phase for testing."""
    return {
        "name": "S1901M",
        "season": "spring",
        "year": 1901,
        "type": "movement",
        "units": {
            "austria": ["A vie", "A bud", "F tri"],
            "england": ["F lon", "F edi", "A lvp"],
            "france": ["A par", "A mar", "F bre"],
            "germany": ["A ber", "A mun", "F kie"],
            "italy": ["A rom", "A ven", "F nap"],
            "russia": ["A mos", "A war", "F sev", "F stp/sc"],
            "turkey": ["A con", "A smy", "F ank"],
        },
        "centers": {
            "austria": ["vie", "bud", "tri"],
            "england": ["lon", "edi", "lvp"],
            "france": ["par", "bre", "mar"],
            "germany": ["ber", "mun", "kie"],
            "italy": ["rom", "nap", "ven"],
            "russia": ["mos", "war", "sev", "stp"],
            "turkey": ["ank", "con", "smy"],
        },
        "orders": {
            "austria": ["A vie - tri", "A bud - ser", "F tri - alb"],
            "england": ["F lon - nth", "F edi - nrg", "A lvp - yor"],
            "france": ["A par - bur", "A mar - spa", "F bre - mao"],
            "germany": ["A ber - kie", "A mun - ruh", "F kie - den"],
            "italy": ["A rom - apu", "A ven H", "F nap - ion"],
            "russia": ["A mos - ukr", "A war - gal", "F sev - bla", "F stp/sc - bot"],
            "turkey": ["A con - bul", "A smy - arm", "F ank - bla"],
        },
        "results": {
            "A vie": [""],
            "A bud": [""],
            "F tri": [""],
        },
    }


def _make_test_game() -> dict:
    """Create a minimal test game with 2 phases."""
    phase1 = _make_s1901m_phase()
    phase2 = {
        "name": "F1901M",
        "season": "fall",
        "year": 1901,
        "type": "movement",
        "units": {
            "austria": ["A tri", "A ser", "F alb"],
            "england": ["F nth", "F nrg", "A yor"],
            "france": ["A bur", "A spa", "F mao"],
            "germany": ["A kie", "A ruh", "F den"],
            "italy": ["A apu", "A ven", "F ion"],
            "russia": ["A ukr", "A gal", "F bla", "F bot"],
            "turkey": ["A bul", "A arm", "F ank"],
        },
        "centers": {
            "austria": ["vie", "bud", "tri"],
            "england": ["lon", "edi", "lvp"],
            "france": ["par", "bre", "mar"],
            "germany": ["ber", "mun", "kie"],
            "italy": ["rom", "nap", "ven"],
            "russia": ["mos", "war", "sev", "stp"],
            "turkey": ["ank", "con", "smy"],
        },
        "orders": {
            "austria": ["A tri H", "A ser - gre", "F alb S A ser - gre"],
            "england": ["F nth - nwy", "F nrg - bar", "A yor - lon"],
        },
        "results": {},
    }
    return {
        "game_id": "test_001",
        "source": "test",
        "map": "standard",
        "num_phases": 2,
        "year_range": [1901, 1901],
        "outcome": {
            "austria": {"centers": 5, "result": "draw"},
            "england": {"centers": 5, "result": "draw"},
            "france": {"centers": 5, "result": "survive"},
            "germany": {"centers": 5, "result": "survive"},
            "italy": {"centers": 5, "result": "survive"},
            "russia": {"centers": 5, "result": "survive"},
            "turkey": {"centers": 4, "result": "survive"},
        },
        "phases": [phase1, phase2],
    }


# ---- Tests ----


class TestAreaIndex:
    """Verify area index structure."""

    def test_num_areas(self):
        assert NUM_AREAS == 81, f"Expected 81 areas, got {NUM_AREAS}"

    def test_all_provinces_present(self):
        for p in PROVINCES:
            assert p in AREA_INDEX, f"Province {p} missing from AREA_INDEX"

    def test_bicoastal_variants_present(self):
        for v in BICOASTAL_VARIANTS:
            assert v in AREA_INDEX, f"Bicoastal variant {v} missing from AREA_INDEX"

    def test_indices_are_contiguous(self):
        indices = sorted(AREA_INDEX.values())
        assert indices == list(range(NUM_AREAS))

    def test_feature_count(self):
        assert NUM_FEATURES == 36, f"Expected 36 features, got {NUM_FEATURES}"


class TestBoardStateEncoding:
    """Verify board state tensor encoding."""

    def test_output_shape(self):
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)
        assert tensor.shape == (81, 36), f"Expected (81, 36), got {tensor.shape}"

    def test_dtype(self):
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)
        assert tensor.dtype == np.float32

    def test_unit_positions(self):
        """Verify Austrian units are correctly encoded at S1901M."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Vienna should have army
        vie_idx = AREA_INDEX["vie"]
        assert tensor[vie_idx, FEAT_UNIT_TYPE] == 1.0, "Vienna should have army"
        assert tensor[vie_idx, FEAT_UNIT_TYPE + 1] == 0.0, "Vienna should not have fleet"
        assert tensor[vie_idx, FEAT_UNIT_TYPE + 2] == 0.0, "Vienna should not be empty"
        # Owner should be Austria (index 0)
        assert tensor[vie_idx, FEAT_UNIT_OWNER + 0] == 1.0, "Vienna army should be Austrian"

        # Trieste should have fleet
        tri_idx = AREA_INDEX["tri"]
        assert tensor[tri_idx, FEAT_UNIT_TYPE + 1] == 1.0, "Trieste should have fleet"
        assert tensor[tri_idx, FEAT_UNIT_OWNER + 0] == 1.0, "Trieste fleet should be Austrian"

    def test_russian_fleet_on_coast(self):
        """Verify Russian fleet on stp/sc is encoded on both base and variant."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Base province stp should have fleet
        stp_idx = AREA_INDEX["stp"]
        assert tensor[stp_idx, FEAT_UNIT_TYPE + 1] == 1.0, "STP should have fleet"
        # Russia is index 5
        assert tensor[stp_idx, FEAT_UNIT_OWNER + 5] == 1.0, "STP fleet should be Russian"

        # Coast variant stp/sc should also have fleet
        stp_sc_idx = AREA_INDEX["stp/sc"]
        assert tensor[stp_sc_idx, FEAT_UNIT_TYPE + 1] == 1.0, "STP/SC should have fleet"
        assert tensor[stp_sc_idx, FEAT_UNIT_OWNER + 5] == 1.0

    def test_empty_provinces(self):
        """Verify provinces with no units are marked empty."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Galicia should be empty
        gal_idx = AREA_INDEX["gal"]
        assert tensor[gal_idx, FEAT_UNIT_TYPE + 2] == 1.0, "Galicia should be empty"
        assert tensor[gal_idx, FEAT_UNIT_OWNER + NUM_POWERS] == 1.0, "Empty unit owner should be 'none'"

    def test_supply_center_ownership(self):
        """Verify SC ownership is correctly encoded."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Vienna should be owned by Austria (index 0)
        vie_idx = AREA_INDEX["vie"]
        assert tensor[vie_idx, FEAT_SC_OWNER + 0] == 1.0, "Vienna SC should be Austrian"

        # London should be owned by England (index 1)
        lon_idx = AREA_INDEX["lon"]
        assert tensor[lon_idx, FEAT_SC_OWNER + 1] == 1.0, "London SC should be English"

    def test_neutral_sc(self):
        """Verify unowned SCs are marked neutral."""
        # Create a phase where Serbia (an SC) is not owned by anyone
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Serbia is an SC but not in any power's center list at start
        ser_idx = AREA_INDEX["ser"]
        # Should be marked as neutral
        assert tensor[ser_idx, FEAT_SC_OWNER + NUM_POWERS] == 1.0, "Serbia SC should be neutral"

    def test_non_sc_provinces(self):
        """Verify non-SC provinces have 'none' SC owner."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Galicia is not an SC
        gal_idx = AREA_INDEX["gal"]
        assert tensor[gal_idx, FEAT_SC_OWNER + NUM_POWERS + 1] == 1.0, "Galicia should have SC=none"

    def test_province_type_features(self):
        """Verify province type encoding."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)

        # Bohemia is inland -> land
        boh_idx = AREA_INDEX["boh"]
        assert tensor[boh_idx, FEAT_PROVINCE_TYPE] == 1.0, "Bohemia should be land"
        assert tensor[boh_idx, FEAT_PROVINCE_TYPE + 1] == 0.0
        assert tensor[boh_idx, FEAT_PROVINCE_TYPE + 2] == 0.0

        # North Sea is sea
        nth_idx = AREA_INDEX["nth"]
        assert tensor[nth_idx, FEAT_PROVINCE_TYPE + 1] == 1.0, "North Sea should be sea"

        # London is coastal
        lon_idx = AREA_INDEX["lon"]
        assert tensor[lon_idx, FEAT_PROVINCE_TYPE + 2] == 1.0, "London should be coastal"

    def test_values_are_binary(self):
        """Verify all tensor values are 0.0 or 1.0."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)
        unique_vals = np.unique(tensor)
        for v in unique_vals:
            assert v in (0.0, 1.0), f"Unexpected value {v} in tensor"

    def test_exactly_22_units_at_start(self):
        """Verify 22 units total at game start (3 each + Russia has 4)."""
        phase = _make_s1901m_phase()
        tensor = encode_board_state(phase)
        num_armies = int(tensor[:, FEAT_UNIT_TYPE].sum())
        num_fleets = int(tensor[:, FEAT_UNIT_TYPE + 1].sum())
        # Russia's fleet on stp/sc counts on both base + variant = extra
        # So base count is 22, but stp/sc variant adds 1 more fleet
        total_units_base = 0
        for p in PROVINCES:
            idx = AREA_INDEX[p]
            if tensor[idx, FEAT_UNIT_TYPE] == 1.0 or tensor[idx, FEAT_UNIT_TYPE + 1] == 1.0:
                total_units_base += 1
        assert total_units_base == 22, f"Expected 22 units on base provinces, got {total_units_base}"


class TestAdjacencyMatrix:
    """Verify the GNN adjacency matrix."""

    def test_shape(self):
        adj = build_adjacency_matrix()
        assert adj.shape == (81, 81), f"Expected (81, 81), got {adj.shape}"

    def test_symmetric(self):
        adj = build_adjacency_matrix()
        assert np.allclose(adj, adj.T), "Adjacency matrix should be symmetric"

    def test_self_loops(self):
        adj = build_adjacency_matrix()
        for i in range(NUM_AREAS):
            assert adj[i, i] == 1.0, f"Missing self-loop at index {i}"

    def test_known_adjacency(self):
        """Verify known adjacent provinces."""
        adj = build_adjacency_matrix()
        pairs = [
            ("vie", "boh"), ("vie", "bud"), ("vie", "gal"), ("vie", "tyr"), ("vie", "tri"),
            ("lon", "wal"), ("lon", "yor"), ("lon", "nth"), ("lon", "eng"),
            ("par", "bur"), ("par", "gas"), ("par", "bre"), ("par", "pic"),
        ]
        for a, b in pairs:
            i, j = AREA_INDEX[a], AREA_INDEX[b]
            assert adj[i, j] == 1.0, f"{a} should be adjacent to {b}"
            assert adj[j, i] == 1.0, f"{b} should be adjacent to {a}"

    def test_known_non_adjacency(self):
        """Verify provinces that should NOT be adjacent."""
        adj = build_adjacency_matrix()
        non_pairs = [
            ("vie", "ven"),  # Vienna and Venice are NOT adjacent
            ("smy", "ank"),  # Smyrna and Ankara share fleet adjacency via con path
        ]
        # Correction: ank-smy IS adjacent in our data (addBothAdj("ank", "smy"))
        # Just test Vienna-Venice
        i, j = AREA_INDEX["vie"], AREA_INDEX["ven"]
        assert adj[i, j] == 0.0, "Vienna and Venice should NOT be adjacent"

    def test_bicoastal_inherits_adjacency(self):
        """Verify bicoastal variants inherit base province adjacency."""
        adj = build_adjacency_matrix()
        # bul is adjacent to ser; so bul/ec and bul/sc should also be adjacent to ser
        ser_idx = AREA_INDEX["ser"]
        bul_ec_idx = AREA_INDEX["bul/ec"]
        bul_sc_idx = AREA_INDEX["bul/sc"]
        assert adj[ser_idx, bul_ec_idx] == 1.0, "Serbia should be adjacent to bul/ec"
        assert adj[ser_idx, bul_sc_idx] == 1.0, "Serbia should be adjacent to bul/sc"

    def test_edge_count_reasonable(self):
        """Verify the total number of edges is in a reasonable range."""
        adj = build_adjacency_matrix()
        # Remove self-loops for counting
        np.fill_diagonal(adj, 0)
        num_edges = int(adj.sum()) // 2  # undirected
        # Standard map has ~152 base adjacency pairs, bicoastal variants add more
        assert 150 < num_edges < 500, f"Edge count {num_edges} seems unreasonable"


class TestOrderEncoding:
    """Verify order parsing and encoding."""

    def test_parse_move(self):
        o = parse_order("A par - bur")
        assert o is not None
        assert o["type"] == "move"
        assert o["src"] == "par"
        assert o["dst"] == "bur"

    def test_parse_hold(self):
        o = parse_order("A ven H")
        assert o is not None
        assert o["type"] == "hold"
        assert o["src"] == "ven"

    def test_parse_support(self):
        o = parse_order("F alb S A ser - gre")
        assert o is not None
        assert o["type"] == "support"
        assert o["src"] == "alb"

    def test_parse_convoy(self):
        o = parse_order("F nth C A yor - bel")
        assert o is not None
        assert o["type"] == "convoy"
        assert o["src"] == "nth"

    def test_encode_order_shape(self):
        vec = encode_order_label("A par - bur")
        expected_len = len(ORDER_TYPES) + NUM_AREAS + NUM_AREAS
        assert vec.shape == (expected_len,), f"Expected ({expected_len},), got {vec.shape}"

    def test_encode_move_order(self):
        vec = encode_order_label("A par - bur")
        # Move type should be set (index 1)
        assert vec[1] == 1.0, "Move type should be set"
        # Source: par
        par_idx = AREA_INDEX["par"]
        assert vec[len(ORDER_TYPES) + par_idx] == 1.0, "Source par should be set"
        # Destination: bur
        bur_idx = AREA_INDEX["bur"]
        assert vec[len(ORDER_TYPES) + NUM_AREAS + bur_idx] == 1.0, "Dest bur should be set"

    def test_encode_hold_order(self):
        vec = encode_order_label("A ven H")
        assert vec[0] == 1.0, "Hold type should be set"
        ven_idx = AREA_INDEX["ven"]
        assert vec[len(ORDER_TYPES) + ven_idx] == 1.0, "Source ven should be set"
        # No destination for hold
        dst_section = vec[len(ORDER_TYPES) + NUM_AREAS:]
        assert dst_section.sum() == 0.0, "Hold should have no destination"


class TestValueLabels:
    """Verify value label encoding."""

    def test_solo_victory(self):
        outcome = {
            "austria": {"centers": 0, "result": "eliminated"},
            "england": {"centers": 18, "result": "solo"},
            "france": {"centers": 5, "result": "survive"},
            "germany": {"centers": 4, "result": "survive"},
            "italy": {"centers": 3, "result": "survive"},
            "russia": {"centers": 4, "result": "survive"},
            "turkey": {"centers": 0, "result": "eliminated"},
        }
        labels = encode_value_labels(outcome)
        assert labels["england"][1] == 1.0, "England should have win=1"
        assert labels["england"][3] == 1.0, "England should have survival=1"
        assert labels["austria"][1] == 0.0, "Austria should have win=0"
        assert labels["austria"][3] == 0.0, "Austria should have survival=0"

    def test_draw(self):
        outcome = {
            "austria": {"centers": 8, "result": "draw"},
            "england": {"centers": 8, "result": "draw"},
            "france": {"centers": 0, "result": "eliminated"},
            "germany": {"centers": 0, "result": "eliminated"},
            "italy": {"centers": 0, "result": "eliminated"},
            "russia": {"centers": 10, "result": "draw"},
            "turkey": {"centers": 8, "result": "draw"},
        }
        labels = encode_value_labels(outcome)
        assert labels["austria"][2] == 1.0, "Austria should have draw=1"
        assert labels["austria"][3] == 1.0, "Austria should survive in draw"
        assert labels["france"][2] == 0.0, "France should have draw=0"

    def test_sc_normalization(self):
        outcome = {"austria": {"centers": 17, "result": "survive"}}
        labels = encode_value_labels(outcome)
        assert abs(labels["austria"][0] - 17.0 / 34.0) < 1e-6


class TestGameSampleExtraction:
    """Verify end-to-end sample extraction."""

    def test_extracts_samples(self):
        game = _make_test_game()
        samples = extract_game_samples(game)
        assert len(samples) > 0, "Should extract at least some samples"

    def test_sample_structure(self):
        game = _make_test_game()
        samples = extract_game_samples(game)
        s = samples[0]
        assert "board" in s
        assert "orders" in s
        assert "value" in s
        assert "power" in s
        assert "game_id" in s
        assert s["board"].shape == (81, 36)
        assert s["value"].shape == (4,)

    def test_all_powers_with_orders_get_samples(self):
        """S1901M should produce 7 samples (one per power)."""
        game = _make_test_game()
        samples = extract_game_samples(game)
        # Phase 1: 7 powers have orders; Phase 2: 2 powers
        phase1_samples = [s for s in samples if s["phase_name"] == "S1901M"]
        assert len(phase1_samples) == 7, f"Expected 7 samples for S1901M, got {len(phase1_samples)}"

    def test_only_movement_phases(self):
        """Samples should only come from movement phases."""
        game = _make_test_game()
        # Add a retreat phase
        game["phases"].append({
            "name": "F1901R",
            "season": "fall",
            "year": 1901,
            "type": "retreat",
            "units": {},
            "centers": {},
            "orders": {},
            "results": {},
        })
        samples = extract_game_samples(game)
        for s in samples:
            assert s["phase_name"].endswith("M"), f"Got non-movement phase: {s['phase_name']}"


class TestDatasetSplit:
    """Verify train/val/test splitting."""

    def test_split_covers_all_samples(self):
        game = _make_test_game()
        samples = extract_game_samples(game)
        # Add more games for a meaningful split
        all_samples = []
        for i in range(20):
            g = _make_test_game()
            g["game_id"] = f"test_{i:03d}"
            all_samples.extend(extract_game_samples(g))

        train, val, test = split_dataset(all_samples)
        total = len(train) + len(val) + len(test)
        assert total == len(all_samples), f"Split lost samples: {total} vs {len(all_samples)}"

    def test_split_is_reproducible(self):
        all_samples = []
        for i in range(20):
            g = _make_test_game()
            g["game_id"] = f"test_{i:03d}"
            all_samples.extend(extract_game_samples(g))

        train1, val1, test1 = split_dataset(all_samples, seed=42)
        train2, val2, test2 = split_dataset(all_samples, seed=42)
        assert len(train1) == len(train2)
        assert len(val1) == len(val2)
        assert len(test1) == len(test2)

    def test_game_not_split_across_sets(self):
        """All samples from the same game should be in the same split."""
        all_samples = []
        for i in range(50):
            g = _make_test_game()
            g["game_id"] = f"test_{i:03d}"
            all_samples.extend(extract_game_samples(g))

        train, val, test = split_dataset(all_samples)
        train_games = {s["game_id"] for s in train}
        val_games = {s["game_id"] for s in val}
        test_games = {s["game_id"] for s in test}

        assert not (train_games & val_games), "Train and val should not share games"
        assert not (train_games & test_games), "Train and test should not share games"
        assert not (val_games & test_games), "Val and test should not share games"


class TestSaveLoad:
    """Verify saving and loading datasets."""

    def test_save_and_load(self):
        game = _make_test_game()
        samples = extract_game_samples(game)

        with tempfile.TemporaryDirectory() as tmpdir:
            path = Path(tmpdir) / "test.npz"
            save_dataset(samples, path)

            assert path.exists(), "Output file should exist"

            data = np.load(path)
            assert "boards" in data
            assert "order_labels" in data
            assert "order_masks" in data
            assert "values" in data
            assert "power_indices" in data
            assert "years" in data

            n = len(samples)
            assert data["boards"].shape == (n, 81, 36)
            assert data["values"].shape == (n, 4)
            assert data["power_indices"].shape == (n,)


def run_all_tests():
    """Run all tests and report results."""
    test_classes = [
        TestAreaIndex,
        TestBoardStateEncoding,
        TestAdjacencyMatrix,
        TestOrderEncoding,
        TestValueLabels,
        TestGameSampleExtraction,
        TestDatasetSplit,
        TestSaveLoad,
    ]

    total = 0
    passed = 0
    failed = 0
    errors = []

    for cls in test_classes:
        instance = cls()
        methods = [m for m in dir(instance) if m.startswith("test_")]
        for method_name in sorted(methods):
            total += 1
            test_name = f"{cls.__name__}.{method_name}"
            try:
                getattr(instance, method_name)()
                passed += 1
                print(f"  PASS  {test_name}")
            except Exception as e:
                failed += 1
                errors.append((test_name, str(e)))
                print(f"  FAIL  {test_name}: {e}")

    print(f"\n{'=' * 50}")
    print(f"Results: {passed}/{total} passed, {failed} failed")
    if errors:
        print("\nFailures:")
        for name, err in errors:
            print(f"  {name}: {err}")
    print(f"{'=' * 50}")

    return failed == 0


if __name__ == "__main__":
    import sys
    success = run_all_tests()
    sys.exit(0 if success else 1)
