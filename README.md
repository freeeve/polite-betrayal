# Polite Betrayal

A Diplomacy game engine and web application with neural network-powered AI. Play the classic board game against bots or other players in real-time through a browser-based interface.

## Architecture

```
api/            Go backend (HTTP/WebSocket server, game logic, bot orchestration)
engine/         Rust engine (neural network search, DUI protocol)
ui/             Flutter web frontend (SVG map, Riverpod state management)
data/           ML training pipeline (PyTorch, ONNX export)
```

The Go backend manages games, players, and phases via Postgres. The Rust engine handles move search using ONNX neural networks and communicates over the DUI (Diplomacy Universal Interface) stdin/stdout protocol. The Flutter frontend renders an interactive SVG map with real-time WebSocket updates.

## Prerequisites

- **Go** 1.25+
- **Rust** (stable, 2021 edition)
- **Flutter** 3.11+ (web target enabled)
- **Docker** (for Postgres + Redis)
- **Python** 3.12+ (only for ML training pipeline)

## Quick Start

```bash
# 1. Clone the repo and models
git clone https://github.com/freeeve/polite-betrayal.git
git clone https://github.com/freeeve/polite-betrayal-models.git
cd polite-betrayal

# 2. Symlink ONNX models into the engine
make models

# 3. Start Postgres + Redis
make dev-up

# 4. Build the Rust engine
cd engine && cargo build --release && cd ..

# 5. Start the API server (terminal 1)
./run_dev_api.sh

# 6. Start the Flutter UI (terminal 2)
./run_dev_ui.sh
```

The UI opens at `http://localhost:3009`. In dev mode, log in via the dev auth button (no OAuth setup needed).

## Database Setup

The backend uses Postgres for game state and Redis for phase deadline timers.

```bash
# Start services
make dev-up    # docker compose up -d

# Connection defaults
# Postgres: postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable
# Redis:    redis://localhost:6379/0
```

Migrations run automatically on server startup from `api/migrations/`.

To stop services:
```bash
make dev-down
```

## Building

```bash
# Go backend
make build                    # outputs api/bin/server

# Rust engine
cd engine && cargo build --release
# outputs engine/target/release/realpolitik (DUI engine)
# outputs engine/target/release/selfplay   (self-play game generator)

# Flutter web
cd ui && flutter build web --release
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/polite_betrayal?sslmode=disable` | Postgres connection |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection |
| `PORT` | `8009` | API server port |
| `DEV_MODE` | `false` | Enables `/auth/dev?name=X` endpoint |
| `JWT_SECRET` | `dev-secret-change-me` | JWT signing key |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `REALPOLITIK_PATH` | — | Path to Rust engine binary for bot play |

For Google OAuth (production):
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`

## ONNX Models

The Rust engine requires neural network models in `engine/models/`. These are stored in a separate repo to keep the main repo lightweight:

```bash
git clone https://github.com/freeeve/polite-betrayal-models.git ../polite-betrayal-models
make models   # creates symlinks into engine/models/
```

Models are trained via a self-play reinforcement learning loop. See `data/scripts/` for the training pipeline.

## Testing

```bash
# Go unit tests
make test

# Go integration tests (requires Postgres + Redis + Rust engine)
make test-integration

# Rust tests
cd engine && cargo test

# Rust benchmarks
cd engine && cargo bench

# Flutter analysis
cd ui && flutter analyze

# E2E (Playwright)
./test/run_e2e.sh
```

### Arena Benchmarks

Run the Rust bot against Go-based bots at various difficulty levels:

```bash
cd api
DATABASE_URL=... REALPOLITIK_PATH=../engine/target/release/realpolitik \
  BENCH_GAMES=50 BENCH_SAVE=1 \
  go test -v -tags integration -run TestBenchmark_RustVsMediumAllPowers ./internal/bot/ -timeout 1800m
```

Results are saved to `benchmarks/` as dated markdown files.

## Bot Difficulties

| Bot | Engine | Description |
|-----|--------|-------------|
| Random | Go | Random valid moves |
| Easy | Go | Score-based greedy heuristics with opportunistic supports |
| Medium | Go | Opening book + candidate generation with 1-ply lookahead |
| Hard | Go | Cicero-inspired candidate postures with regret matching (4-ply) |
| Realpolitik | Rust | Neural network policy/value nets with regret matching search |

The Realpolitik bot connects to the Rust engine via `REALPOLITIK_PATH`. It uses the opening book through 1907, then switches to neural network search.

## Development

```bash
# Format before committing
make fmt                          # Go
cd engine && cargo fmt            # Rust
cd ui && flutter analyze          # Dart

# Lint
make lint                         # Go (go vet)
cd engine && cargo clippy         # Rust
```

## ML Training Pipeline

The training pipeline lives in `data/scripts/` and produces ONNX models for the Rust engine:

1. **Supervised pre-training** — Train on human game data (`train_policy.py`, `train_value.py`)
2. **Self-play generation** — Rust engine plays against itself (`selfplay` binary)
3. **RL fine-tuning** — REINFORCE with KL regularization against supervised baseline (`train_policy_rl.py`)
4. **ONNX export** — Convert PyTorch checkpoints to ONNX (`export_onnx.py`)
5. **Deploy** — Copy models to `engine/models/` and rebuild

The full loop is orchestrated by `data/scripts/selfplay_loop.py`.

## License

Private project.
