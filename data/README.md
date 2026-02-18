# Diplomacy Game Data Pipeline

Download, parse, and validate Diplomacy game datasets for ML training.

## Quick Start

```bash
cd data/

# 1. Download raw datasets
python3 scripts/download.py

# 2. Parse into unified format
python3 scripts/parse.py

# 3. Validate and generate statistics
python3 scripts/validate.py

# 4. Extract features for neural network training
pip install -r requirements.txt  # numpy
python3 scripts/features.py
```

Download, parse, and validate require only Python stdlib. Feature extraction requires numpy (`pip install -r requirements.txt`).

## Data Sources

| Source | Games | Format | Access |
|--------|-------|--------|--------|
| [diplomacy/research](https://github.com/diplomacy/research) | ~156K | JSONL | Public S3 download |
| [Kaggle diplomacy-game-dataset](https://www.kaggle.com/datasets/gowripreetham/diplomacy-game-dataset) | ~5K | CSV/JSON | Requires Kaggle account |

The primary dataset is from `diplomacy/research`, containing games from webdiplomacy.net in the `diplomacy` Python library's saved-game JSONL format. Only standard-map games are processed (non-standard maps are skipped).

## Directory Layout

```
data/
  scripts/
    download.py       # Download raw datasets
    parse.py          # Parse and normalize to unified format
    validate.py       # Validate data and produce statistics
    features.py       # Extract training features (board tensors, order labels)
    province_map.py   # Province name normalization (3-letter codes)
  raw/                # Downloaded files (gitignored)
  processed/          # Parsed output (gitignored)
    games.jsonl       # Unified game records (one per line)
    quarantined.jsonl # Games that failed validation
    stats.json        # Summary statistics
    features/         # Training-ready numpy arrays
      train.npz       # Training split
      val.npz         # Validation split
      test.npz        # Test split
      adjacency.npy   # 81x81 GNN adjacency matrix
      metadata.json   # Feature dimensions, area/power indices
  requirements.txt
  .gitignore
```

## Unified Game Record Schema

Each line in `games.jsonl` is a JSON object:

```json
{
  "game_id": "12345",
  "source": "research",
  "map": "standard",
  "num_phases": 42,
  "year_range": [1901, 1910],
  "outcome": {
    "austria": {"centers": 0, "result": "eliminated"},
    "england": {"centers": 18, "result": "solo"},
    "france": {"centers": 5, "result": "survive"},
    "germany": {"centers": 4, "result": "survive"},
    "italy": {"centers": 3, "result": "survive"},
    "russia": {"centers": 4, "result": "survive"},
    "turkey": {"centers": 0, "result": "eliminated"}
  },
  "phases": [
    {
      "name": "S1901M",
      "season": "spring",
      "year": 1901,
      "type": "movement",
      "units": {"austria": ["A vie", "A bud", "F tri"], ...},
      "centers": {"austria": ["vie", "bud", "tri"], ...},
      "orders": {"austria": ["A vie - tri", "A bud - ser", "F tri - alb"], ...},
      "results": {"A vie": [""], ...}
    }
  ]
}
```

### Province Codes

All territory identifiers use lowercase 3-letter codes matching `engine/src/board/province.rs`. Split-coast provinces use `/nc`, `/sc`, `/ec` suffixes (e.g., `spa/nc`, `stp/sc`, `bul/ec`).

### Outcome Results

- `solo` -- achieved 18+ supply centers (solo victory)
- `draw` -- tied for most centers at game end
- `survive` -- alive but not in the draw
- `eliminated` -- 0 centers

## Feature Extraction

The `features.py` script converts parsed game data into training-ready numpy arrays for GNN policy and value networks.

### Board State Tensor: `[81, 36]`

81 areas = 75 provinces + 6 bicoastal variants (spa/nc, spa/sc, stp/nc, stp/sc, bul/ec, bul/sc).

36 features per area:

| Offset | Width | Feature |
|--------|-------|---------|
| 0 | 3 | Unit present: army, fleet, empty |
| 3 | 8 | Unit owner: A, E, F, G, I, R, T, none |
| 11 | 9 | SC owner: A, E, F, G, I, R, T, neutral, none |
| 20 | 1 | Can build |
| 21 | 1 | Can disband |
| 22 | 3 | Dislodged unit: army, fleet, none |
| 25 | 8 | Dislodged owner: A, E, F, G, I, R, T, none |
| 33 | 3 | Province type: land, sea, coast |

### Order Labels: `[N, max_orders, 169]`

Each order is encoded as a 169-dim vector: 7 order types + 81 source area + 81 destination area.

### Value Labels: `[N, 4]`

Per-power outcome: normalized SC count, win indicator, draw indicator, survival indicator.

### Adjacency Matrix: `[81, 81]`

Binary symmetric matrix with self-loops. Bicoastal variants inherit base province adjacency.

### Dataset Splits

90/5/5 train/val/test split by game ID (all phases from the same game stay together). Reproducible via `--seed`.

## Idempotency

All scripts are idempotent:
- `download.py` skips files already present (use `--force` to re-download)
- `parse.py` overwrites output from scratch (deterministic from raw input)
- `validate.py` overwrites reports from scratch

## Options

```bash
# Download only the research dataset
python3 scripts/download.py --source research

# Parse with a limit (for testing)
python3 scripts/parse.py --limit 100

# Custom output path
python3 scripts/parse.py --output /tmp/test_games.jsonl

# Extract features (limit for testing)
python3 scripts/features.py --limit 100

# Custom output directory
python3 scripts/features.py --output-dir /tmp/features
```
