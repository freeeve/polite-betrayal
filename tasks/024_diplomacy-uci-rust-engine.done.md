# 024: Diplomacy UCI Protocol, Rust Engine & Training Pipeline

**Status**: Planning
**Created**: 2026-02-17
**Scope**: Multi-phase initiative spanning protocol design, engine development, and ML training

---

## 1. Executive Summary

Design a UCI-inspired text protocol (DUI — Diplomacy Universal Interface) for external bot engines, build a fast Rust-based Diplomacy engine that speaks this protocol, and establish a training pipeline using human game data. This creates an extensible architecture where any language can implement a Diplomacy bot that plugs into the existing Go game server.

---

## 2. Existing Codebase Analysis

### 2.1 Game Engine (`api/pkg/diplomacy/`)

| File | Purpose |
|------|---------|
| `state.go` | `GameState` struct: Year, Season, Phase, Units, SupplyCenters, Dislodged |
| `unit.go` | `Unit{Type, Power, Province, Coast}`, `UnitType` (Army=0, Fleet=1), `Power` (string: "austria"..."turkey") |
| `order.go` | `Order` struct with Location/Coast, Type (Hold/Move/Support/Convoy), Target/TargetCoast, AuxLoc/AuxTarget/AuxUnitType |
| `map.go` | `DiplomacyMap` with 75 provinces, adjacency graph, coast handling |
| `map_data.go` | Province definitions, adjacency data, split coasts (spa nc/sc, stp nc/sc, bul ec/sc) |
| `resolve.go` | Kruijswijk guess-and-check resolver; `Resolver` struct for zero-alloc hot loops |
| `retreat.go` | `RetreatOrder` (RetreatMove/RetreatDisband), `ResolveRetreats`, `ApplyRetreats` |
| `build.go` | `BuildOrder` (BuildUnit/DisbandUnit), `ResolveBuildOrders`, civil disorder logic |
| `phase.go` | Phase sequencing: Movement -> Retreat? -> Fall Movement -> Build -> Spring |
| `validate.go` | Order validation with `ValidationError` |

### 2.2 Bot System (`api/internal/bot/`)

| File | Purpose |
|------|---------|
| `strategy.go` | `Strategy` interface: `GenerateMovementOrders`, `GenerateRetreatOrders`, `GenerateBuildOrders` |
| `strategy_easy.go` | `HeuristicStrategy` — greedy heuristic |
| `strategy_medium.go` | `TacticalStrategy` — Cartesian search with heuristic eval |
| `strategy_hard.go` | `HardStrategy` — Smooth Regret Matching+ with multi-ply lookahead (10s budget) |
| `eval.go` | `EvaluatePosition` — handcrafted eval: SC count, proximity, threat/defense balance |
| `search_util.go` | `LegalOrdersForUnit`, `TopKOrders`, `searchTopN`, `searchBestOrders`, `simulateAhead` |
| `arena.go` | Full bot-vs-bot game loop with DB persistence |
| `orchestrator.go` | HTTP/WS client-based bot orchestration |
| `client.go` | `OrderInput` struct (JSON: unit_type, location, coast, order_type, target, etc.) |
| `diplomacy_msg.go` | Structured diplomatic intents (request support, propose alliance, threaten, etc.) |

### 2.3 Key Data Types for Protocol Design

```
Order notation (from Order.Describe()):
  A vie Hold
  F tri -> adr
  A bud S A vie Hold
  A gal S A bud -> rum
  F mid C A bre -> spa

Province IDs: 3-letter lowercase (vie, bud, tri, bre, par, mos, etc.)
Coasts: nc, sc, ec (written as spa/nc, stp/sc, bul/ec)
Powers: austria, england, france, germany, italy, russia, turkey
```

---

## 3. DUI Protocol Specification (Diplomacy Universal Interface)

### 3.1 Design Principles

- **Text-based stdin/stdout** — same as UCI chess: one command per line, newline-terminated
- **Stateless position** — server sends full board state each turn (no incremental updates)
- **Simultaneous moves** — all 7 powers submit orders at once (unlike chess's alternating turns)
- **Phase-aware** — protocol distinguishes movement, retreat, and build phases
- **Power-scoped** — engine plays one power at a time (server spawns 7 processes or queries one engine for each power)

### 3.2 Commands: Server -> Engine

```
dui
    Tell engine to switch to DUI mode. Engine must respond with
    `id` lines and `duiok`.

isready
    Synchronization ping. Engine responds with `readyok`.

setoption name <id> [value <x>]
    Set engine parameters. Common options:
    - Threads <n>
    - SearchTime <ms>
    - ModelPath <path>
    - Strength <1-100>
    - Personality <aggressive|defensive|balanced>

newgame
    Reset engine state for a new game.

position <dfen>
    Set the current board state. See Section 3.3 for DFEN format.

setpower <power>
    Tell the engine which power it is playing.
    power = austria | england | france | germany | italy | russia | turkey

go [movetime <ms>] [depth <n>] [nodes <n>] [infinite]
    Start calculating orders for the current position and assigned power.
    Engine must respond with `bestorders` when done.
    - movetime: time limit in milliseconds
    - depth: search depth limit (plies/phases)
    - nodes: node limit
    - infinite: search until `stop` is sent

stop
    Stop calculating and output `bestorders` immediately.

press <from_power> <message_type> [args...]
    Deliver a diplomatic message from another power.
    message_type:
    - request_support <from_prov> <to_prov>
    - propose_nonaggression [<province>...]
    - propose_alliance [against <power>]
    - threaten <province>
    - offer_deal <i_take_prov> <you_take_prov>
    - accept
    - reject
    - freetext <base64_encoded_text>

quit
    Terminate the engine process.
```

### 3.3 Commands: Engine -> Server

```
id name <engine_name>
id author <author_name>
    Engine identification.

option name <id> type <type> [default <x>] [min <x>] [max <x>] [var <x>]
    Declare supported options. Types: check, spin, combo, button, string.

duiok
    Signals DUI initialization is complete.

readyok
    Response to `isready`.

info [depth <n>] [nodes <n>] [nps <n>] [time <ms>] [score <cp>] [pv <orders...>]
    Search progress information.
    - score: centipawn evaluation from the engine's power's perspective
    - pv: principal variation (sequence of order sets)

bestorders <order> [; <order>]...
    The engine's chosen orders for all its units.
    Order format is defined in Section 3.4.

press_out <to_power> <message_type> [args...]
    Engine wants to send a diplomatic message.
    Same message_type format as inbound `press`.
```

### 3.4 Order Notation (DSON — Diplomacy Standard Order Notation)

Compact, unambiguous, parseable order strings. Based on the existing `Order.Describe()` format but formalized:

```
Movement Phase:
  A vie H                        # Army Vienna Hold
  A bud - rum                    # Army Budapest Move to Rumania
  F tri - adr                    # Fleet Trieste Move to Adriatic Sea
  A gal S A bud - rum            # Army Galicia Support Army Budapest -> Rumania
  A tyr S A vie H                # Army Tyrolia Support Army Vienna Hold
  F mid C A bre - spa            # Fleet Mid-Atlantic Convoy Army Brest -> Spain
  F nwg - stp/nc                 # Fleet Norwegian Sea Move to St Petersburg North Coast

Retreat Phase:
  A vie R boh                    # Army Vienna Retreat to Bohemia
  F tri D                        # Fleet Trieste Disband

Build Phase:
  A vie B                        # Build Army in Vienna
  F stp/sc B                     # Build Fleet in St Petersburg South Coast
  A war D                        # Disband Army in Warsaw
  W                              # Waive (skip a build)

Grammar:
  <order>     ::= <unit> <action>
  <unit>      ::= ('A' | 'F') <location>
  <location>  ::= <prov_id> ['/' <coast>]
  <prov_id>   ::= [a-z]{3}
  <coast>     ::= 'nc' | 'sc' | 'ec'
  <action>    ::= 'H'                           # hold
              |   '-' <location>                 # move
              |   'S' <unit> 'H'                 # support hold
              |   'S' <unit> '-' <location>      # support move
              |   'C' 'A' <location> '-' <location>  # convoy
              |   'R' <location>                 # retreat
              |   'D'                            # disband
              |   'B'                            # build
```

### 3.5 DFEN — Diplomacy FEN (Board State Encoding)

A single-line text encoding of the complete board state, analogous to chess FEN:

```
DFEN = <phase_info> '/' <units> '/' <supply_centers> '/' <dislodged>

phase_info = <year> <season><phase>
  year = integer (e.g. 1901)
  season = 's' (spring) | 'f' (fall)
  phase = 'm' (movement) | 'r' (retreat) | 'b' (build)
  Example: "1901sm" = Spring 1901 Movement

units = <unit_entry> [',' <unit_entry>]*
  unit_entry = <power_char> <unit_type> <location>
  power_char = A(ustria) | E(ngland) | F(rance) | G(ermany) | I(taly) | R(ussia) | T(urkey)
  unit_type = 'a' (army) | 'f' (fleet)
  location = <prov_id> ['.' <coast>]   (dot separator to avoid / ambiguity)
  Example: "Aavie,Aabud,Aftri" = Austria: A vie, A bud, F tri

supply_centers = <sc_entry> [',' <sc_entry>]*
  sc_entry = <power_char> <prov_id>
  Neutral SCs use 'N' as power_char.
  Only list non-initial or all SCs — TBD based on compactness.
  Example: "Avie,Abud,Atri,Elon,Eedi,Elvp,..."

dislodged = <dislodged_entry> [',' <dislodged_entry>]* | '-'
  dislodged_entry = <power_char> <unit_type> <location> '<' <attacker_from>
  '<' indicates "dislodged by unit from"
  Example: "Aavie<boh" = Austrian Army at Vienna dislodged from Bohemia
  '-' means no dislodged units

Full example (initial position):
1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Avie,Abud,Atri,Elon,Eedi,Elvp,Fbre,Fpar,Fmar,Gkie,Gber,Gmun,Inap,Irom,Iven,Rstp,Rmos,Rwar,Rsev,Tank,Tcon,Tsmy,Nnwy,Nswe,Nden,Nhol,Nbel,Nspa,Npor,Ntun,Ngre,Nser,Nbul,Nrum/-
```

### 3.6 Session Flow Example

```
Server: dui
Engine: id name RustDiplomacyBot
Engine: id author polite-betrayal
Engine: option name Threads type spin default 4 min 1 max 64
Engine: option name ModelPath type string default models/v1.onnx
Engine: option name Strength type spin default 100 min 1 max 100
Engine: duiok

Server: isready
Engine: readyok

Server: newgame
Server: setpower austria
Server: position 1901sm/Aavie,Aabud,Aftri,.../-
Server: go movetime 5000

Engine: info depth 1 nodes 1234 score 0 time 100
Engine: info depth 2 nodes 15000 score 5 time 800
Engine: info depth 3 nodes 120000 score 12 time 3200
Engine: bestorders A vie - tri ; A bud - ser ; F tri - alb

Server: position 1901fm/.../-
Server: go movetime 5000
Engine: bestorders A tri H ; A ser - rum ; F alb - gre

Server: quit
```

### 3.7 Comparison: DUI vs DAIDE vs webDiplomacy API

| Feature | DUI (proposed) | DAIDE | webDiplomacy API |
|---------|---------------|-------|------------------|
| Transport | stdin/stdout | TCP binary | HTTP/JSON |
| Encoding | Text lines | Binary tokens (2-octet) | JSON |
| Statefulness | Stateless (full position each turn) | Stateful (incremental) | Stateful (session) |
| Multi-power | One power per session | One power per connection | One power per API key |
| Negotiation | Structured press commands | Full DAIDE language (complex) | Chat messages |
| Complexity | Low (~15 commands) | High (~50+ token types) | Medium (REST endpoints) |
| Language support | Any (stdin/stdout) | Needs binary parser | Needs HTTP client |

---

## 4. Rust Engine Architecture

### 4.1 Project Structure

```
engine/
├── Cargo.toml
├── src/
│   ├── main.rs              # DUI protocol loop
│   ├── protocol/
│   │   ├── mod.rs
│   │   ├── parser.rs        # Parse DUI commands
│   │   ├── dfen.rs          # DFEN encode/decode
│   │   └── dson.rs          # Order notation encode/decode
│   ├── board/
│   │   ├── mod.rs
│   │   ├── state.rs         # GameState (bitboard-style)
│   │   ├── province.rs      # Province enum (0..74), Coast enum
│   │   ├── adjacency.rs     # Adjacency graph (compile-time const)
│   │   ├── unit.rs          # Unit representation
│   │   └── order.rs         # Order types
│   ├── movegen/
│   │   ├── mod.rs
│   │   ├── movement.rs      # Legal move generation
│   │   ├── retreat.rs       # Legal retreat generation
│   │   └── build.rs         # Legal build generation
│   ├── resolve/
│   │   ├── mod.rs
│   │   └── kruijswijk.rs    # Order adjudication (port from Go)
│   ├── search/
│   │   ├── mod.rs
│   │   ├── regret_matching.rs  # RM+ for opponent modeling
│   │   ├── mcts.rs          # Monte Carlo Tree Search (future)
│   │   └── beam.rs          # Beam search over order combinations
│   ├── eval/
│   │   ├── mod.rs
│   │   ├── heuristic.rs     # Handcrafted eval (port from Go)
│   │   └── neural.rs        # Neural network eval via ONNX
│   ├── nn/
│   │   ├── mod.rs
│   │   ├── policy.rs        # Policy network (order prediction)
│   │   └── value.rs         # Value network (position evaluation)
│   └── data/
│       ├── map_data.rs       # Province/adjacency constants
│       └── opening_book.rs   # Common opening moves (optional)
├── models/                   # ONNX model files
└── tests/
    ├── datc_tests.rs         # DATC compliance tests
    ├── protocol_tests.rs     # DUI protocol tests
    └── search_tests.rs       # Search algorithm tests
```

### 4.2 Board Representation

Optimized for speed — fixed-size arrays, no heap allocation for core operations:

```rust
// Province indices (compile-time const, 0..74)
#[repr(u8)]
enum Province { Vie=0, Bud=1, Tri=2, ..., Smy=74 }

// Bitboard-style unit tracking
struct BoardState {
    // Per-province data (75 entries, cache-friendly)
    units: [Option<UnitEntry>; 75],       // unit at each province
    sc_owner: [PowerMask; 75],            // SC ownership (0 for non-SC)
    dislodged: [Option<DislodgedEntry>; 75],

    // Per-power data
    unit_count: [u8; 7],
    sc_count: [u8; 7],

    // Phase info
    year: u16,
    season: Season,
    phase: Phase,
}

// Compact unit representation
struct UnitEntry {
    unit_type: UnitType,  // 1 bit
    power: Power,         // 3 bits
    coast: Coast,         // 2 bits
}
// Fits in 1 byte with bitpacking
```

### 4.3 Move Generation

Port `LegalOrdersForUnit` from Go, but with compile-time adjacency tables:

```rust
// Static adjacency graph (generated from map_data.go)
const ADJACENCIES: &[&[AdjEntry]] = &[
    /* vie */ &[AdjEntry { to: Province::Boh, army: true, fleet: false, .. }, ...],
    /* bud */ &[...],
    ...
];

fn legal_orders(unit: &UnitEntry, prov: Province, state: &BoardState) -> Vec<Order> {
    // Same logic as Go's LegalOrdersForUnit, but using const adjacency table
}
```

### 4.4 Resolution Engine

Direct port of `resolve.go` Kruijswijk algorithm:

```rust
struct Resolver {
    lookup: [i16; 75],          // province -> adjBuf index
    adj_buf: Vec<AdjResult>,    // dense results
}

impl Resolver {
    fn resolve(&mut self, orders: &[Order], state: &BoardState) -> Vec<ResolvedOrder> {
        // Same Kruijswijk guess-and-check as Go implementation
    }
}
```

### 4.5 Search Architecture

Two-tier search matching the existing Go `HardStrategy`:

**Tier 1: Candidate Generation**
- Enumerate legal orders per unit
- Score with heuristic + neural policy (if available)
- Keep top-K per unit using adaptive K

**Tier 2: Opponent Modeling via Regret Matching+**
- Same Smooth RM+ algorithm as `strategy_hard.go`
- K candidate order sets per power
- Counterfactual regret updates
- Best-response with multi-ply lookahead

**Future Tier 3: MCTS with Neural Guidance**
- Policy network guides tree expansion
- Value network for leaf evaluation
- UCB1 or PUCT selection

### 4.6 Neural Network Integration

Using `ort` (Rust ONNX Runtime bindings) or `tract` (pure Rust):

```rust
// Board state -> tensor encoding (inspired by DeepMind's format)
fn encode_state(state: &BoardState) -> Tensor {
    // 81 areas (75 + 6 bicoastal) x feature_vector_length
    // Features per area:
    //   - unit_present: [army, fleet, empty]     (3)
    //   - unit_owner: [A,E,F,G,I,R,T,none]      (8)
    //   - sc_owner: [A,E,F,G,I,R,T,neutral,none] (9)
    //   - can_build: bool                         (1)
    //   - can_disband: bool                       (1)
    //   - dislodged: [army, fleet, none]          (3)
    //   - dislodged_owner: [A,E,F,G,I,R,T,none]  (8)
    //   - province_type: [land, sea, coast]       (3)
    //   Total: 36 features per area
    // Shape: [81, 36] = 2,916 floats
}

// Action space encoding
// ~16,000 possible unit orders (75 provinces * ~200 legal orders each, max)
// Policy output: probability over all legal orders for each unit
```

---

## 5. Go Server Integration

### 5.1 External Process Strategy

Add a new `ExternalStrategy` to the existing bot system that communicates with any DUI-compatible engine:

```go
// api/internal/bot/strategy_external.go

type ExternalStrategy struct {
    enginePath string
    process    *exec.Cmd
    stdin      io.Writer
    stdout     *bufio.Scanner
    power      diplomacy.Power
    timeout    time.Duration
}

func (s *ExternalStrategy) GenerateMovementOrders(
    gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap,
) []OrderInput {
    dfen := EncodeDFEN(gs)
    fmt.Fprintf(s.stdin, "position %s\n", dfen)
    fmt.Fprintf(s.stdin, "setpower %s\n", power)
    fmt.Fprintf(s.stdin, "go movetime %d\n", s.timeout.Milliseconds())

    // Read bestorders response
    for s.stdout.Scan() {
        line := s.stdout.Text()
        if strings.HasPrefix(line, "bestorders ") {
            return ParseDSON(line[len("bestorders "):])
        }
    }
    return nil
}
```

### 5.2 Engine Pool

For arena games and live play, maintain a pool of engine processes:

```go
type EnginePool struct {
    engines   []*ExternalStrategy
    available chan *ExternalStrategy
}

func (p *EnginePool) Acquire(power diplomacy.Power) *ExternalStrategy { ... }
func (p *EnginePool) Release(e *ExternalStrategy) { ... }
```

### 5.3 Difficulty Mapping

```go
func StrategyForDifficulty(difficulty string) Strategy {
    switch difficulty {
    case "impossible":
        return NewExternalStrategy("./engine/target/release/dui-engine",
            WithOption("ModelPath", "models/v3.onnx"),
            WithOption("Strength", "100"),
            WithTimeout(10*time.Second))
    // ... existing strategies unchanged
    }
}
```

---

## 6. Training Pipeline

### 6.1 Data Sources

| Dataset | Size | Content | Format | Access |
|---------|------|---------|--------|--------|
| webDiplomacy.net | ~156K games | Orders + some press | Custom JSON | Email admin@webdiplomacy.net |
| Kaggle Diplomacy Game Dataset | ~5K games | Moves, outcomes | CSV/JSON | Public download |
| Kaggle Betrayal Dataset | 500 games | Press messages + betrayal labels | JSON | Public download |
| Facebook Diplomacy Research | 46K games | Orders (no-press subset) | JSON (game format) | GitHub |
| Cicero Training Data | ~125K games | Orders + full press | Licensed from webDip | Not public |

### 6.2 Data Pipeline Architecture

```
Raw game data (JSON/CSV)
    │
    ▼
┌────────────────────┐
│  Parse & Normalize  │  Python scripts
│  - Validate orders  │  - Convert territory IDs to our 3-letter codes
│  - Fix known issues │  - Handle variant differences
│  - Dedup games      │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Feature Extraction │  Python + NumPy
│  - Board tensors    │  - 81 x 36 per phase
│  - Order labels     │  - One-hot over legal orders
│  - Value labels     │  - Final SC count / win/draw/loss
│  - Phase metadata   │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Training Dataset   │  Parquet / HDF5
│  - ~2M phase records│
│  - Train/Val/Test   │  90/5/5 split
│  - Balanced by power│
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Model Training     │  PyTorch
│  - Policy network   │  - Predict human orders (supervised)
│  - Value network    │  - Predict game outcome
│  - Joint training   │  - Multi-task loss
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Export to ONNX     │  torch.onnx.export
│  - Quantize (INT8)  │  - For fast inference
│  - Validate output  │  - Compare PyTorch vs ONNX
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Rust Engine Load   │  ort / tract crate
│  - Load .onnx       │
│  - Batch inference   │
│  - Integrate w/search│
└────────────────────┘
```

### 6.3 Model Architecture

**Policy Network (Order Prediction)**
```
Input: Board tensor [81 x 36] + prev_orders [81 x 36]
  │
  ▼
Graph Neural Network (adjacency-aware):
  - 3 GNN layers (message passing along adjacency graph)
  - 256-dim hidden state per province
  │
  ▼
Per-unit order decoder (autoregressive):
  - For each unit of the active power:
    - Attention over all province embeddings
    - Project to order vocabulary (~200 per unit)
    - Softmax → order probability
  │
  ▼
Output: Per-unit order probabilities
```

**Value Network (Position Evaluation)**
```
Input: Board tensor [81 x 36]
  │
  ▼
Same GNN encoder (shared weights with policy)
  │
  ▼
Global pooling → FC layers → 7-dim output
  │
  ▼
Output: Expected share of supply centers per power [7]
  (sum to 34, or normalized to sum to 1.0)
```

**Combined Model Size Target**: 5-20M parameters (fast inference on CPU)

### 6.4 Training Stages

**Stage 1: Supervised Learning (SL)**
- Train policy network on human game data
- Cross-entropy loss on order prediction
- ~50K games, ~1M movement phases
- Target: >40% top-1 accuracy on held-out games (baseline from DipNet: ~45%)

**Stage 2: Value Network Training**
- Train value network on game outcomes
- MSE loss on final SC distribution
- Use same dataset but label with terminal position

**Stage 3: Self-Play Reinforcement Learning (RL)**
- Use Rust engine for fast game rollouts
- Policy gradient (REINFORCE or PPO) on game outcomes
- Arena evaluation against SL baseline
- Target: beat SL model >60% in head-to-head

**Stage 4: Iterative Improvement**
- Alternate between self-play data generation and retraining
- Similar to AlphaZero's training loop
- Each iteration produces a new model checkpoint

### 6.5 NLP Pipeline (Future — Phase 4+)

For press-enabled games:

```
Incoming message
    │
    ▼
Intent classifier (fine-tuned BERT/DistilBERT)
    │
    ▼
Structured DiplomaticIntent (matches existing Go types)
    │
    ▼
Feed to engine via `press` DUI command
    │
    ▼
Engine factors into search (trust model + order adjustment)
    │
    ▼
Engine outputs `press_out` with structured response
    │
    ▼
Response generator (template-based or LLM)
    │
    ▼
Natural language message to opponent
```

---

## 7. Phased Implementation Plan

### Phase 1: Protocol & Foundation (4-6 weeks)

**Milestone**: DUI protocol implemented, Rust engine plays legal random moves

| Task | Effort | Dependencies |
|------|--------|--------------|
| Define DUI protocol spec (finalize DFEN, DSON formats) | 3d | None |
| Implement DFEN encoder/decoder in Go | 2d | Protocol spec |
| Implement DSON parser in Go | 2d | Protocol spec |
| Implement `ExternalStrategy` in Go | 3d | DFEN, DSON |
| Scaffold Rust engine with Cargo workspace | 1d | None |
| Port province/adjacency data to Rust | 2d | map_data.go |
| Implement DUI protocol parser in Rust | 3d | Protocol spec |
| Implement DFEN/DSON in Rust | 2d | Protocol spec |
| Implement random legal move generator in Rust | 3d | Adjacency data |
| Integration test: Go server <-> Rust engine | 2d | All above |
| DATC compliance tests for Rust move gen | 3d | Move generator |

**Deliverable**: `go run ./cmd/server` can launch Rust engine subprocess, send it positions, and receive legal orders back.

### Phase 2: Resolution & Heuristic Search (4-6 weeks)

**Milestone**: Rust engine plays at "medium" Go bot strength

| Task | Effort | Dependencies |
|------|--------|--------------|
| Port Kruijswijk resolver to Rust | 5d | Phase 1 |
| DATC resolution compliance tests | 3d | Resolver |
| Port heuristic evaluation function | 2d | Resolver |
| Port retreat/build order generation | 2d | Resolver |
| Implement Cartesian search (like TacticalStrategy) | 3d | Eval + movegen |
| Implement RM+ opponent modeling (like HardStrategy) | 5d | Search |
| Benchmark: Rust engine vs Go bots in arena | 2d | All above |
| Performance optimization (cache-friendly data, SIMD) | 3d | Benchmark results |

**Deliverable**: Rust engine beats Go "easy" bot >80% of the time, competitive with "medium".

### Phase 3: Neural Network Integration (6-8 weeks)

**Milestone**: Trained model guides search, engine plays at "hard" or better strength

| Task | Effort | Dependencies |
|------|--------|--------------|
| Data pipeline: download/parse webDiplomacy games | 5d | Data access |
| Feature extraction: board tensors + order labels | 5d | Data pipeline |
| Implement GNN policy network in PyTorch | 5d | Features |
| Implement value network in PyTorch | 3d | GNN encoder |
| Train supervised learning model | 5d | Networks + data |
| Export to ONNX, validate correctness | 2d | Trained model |
| Integrate ort/tract into Rust engine | 3d | ONNX model |
| Neural-guided candidate generation | 3d | ort integration |
| Neural value function in search | 2d | ort integration |
| Arena evaluation vs Go hard bot | 2d | Integration |

**Deliverable**: Neural-guided Rust engine beats Go "hard" bot >60%.

### Phase 4: Self-Play & Advanced Training (8-12 weeks)

**Milestone**: Self-play trained engine at superhuman (or near) strength

| Task | Effort | Dependencies |
|------|--------|--------------|
| Self-play game generation pipeline | 5d | Phase 3 engine |
| RL training loop (policy gradient) | 5d | Self-play data |
| Value iteration training | 3d | Self-play data |
| Iterative training (AlphaZero-style loop) | 10d | RL pipeline |
| Arena evaluation vs best engine version | 3d | Each iteration |
| NLP intent classifier for press games | 5d | Press dataset |
| Press integration into DUI protocol | 3d | Classifier |
| Template response generator | 3d | Press integration |

**Deliverable**: Engine that can beat all Go bots convincingly, optionally handle press.

### Phase 5: Polish & Production (4 weeks)

| Task | Effort | Dependencies |
|------|--------|--------------|
| Engine packaging (static binary, model bundling) | 2d | Phase 4 |
| Configuration UI for engine settings | 3d | Flutter UI |
| Performance profiling & optimization | 3d | All phases |
| Documentation: protocol spec, engine README | 2d | All phases |
| CI/CD for engine builds | 2d | Packaging |
| Third-party engine support (anyone can write a DUI bot) | 2d | Protocol finalized |

---

## 8. Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| webDiplomacy data access denied or delayed | Blocks Phase 3 | Medium | Use Kaggle dataset + synthetic self-play data; Facebook research dataset as fallback |
| Rust resolver produces different results than Go | Correctness bugs in games | High initially | Extensive DATC test suite; cross-validate with Go resolver on random positions |
| Neural model too large for fast inference | Engine too slow for real-time play | Medium | Start with small model (5M params); INT8 quantization; `tract` for pure-Rust inference |
| Action space too large for effective policy learning | Poor model accuracy | Medium | Use per-unit autoregressive decoding; prune illegal moves; focus on top-K orders |
| Self-play divergence / mode collapse | Engine finds degenerate strategies | Medium | Diverse initial policies; population-based training; mix in human data |
| DUI protocol needs revision after implementation | Breaking changes to integrations | High | Start with minimal protocol; version the protocol; keep backwards compat |
| Rust <-> Go process communication overhead | Latency issues in real-time games | Low | Unix pipes are fast; batch multiple queries; consider FFI via CGo if needed |

---

## 9. Design Decisions (Resolved)

1. **Engine scope**: External bot strategy only — Go engine stays for server-side adjudication. Rust engine is an additional difficulty level ("impossible") launched as an external process via DUI protocol.

2. **Protocol transport**: stdin/stdout only. Simple Unix pipes, same as UCI chess.

3. **Press/NLP priority**: Structured intents in v1 — support the existing DiplomaticIntent types (request support, propose alliance, threaten, etc.) from the start. Full NLP deferred.

4. **Training compute**: Apple MPS (M-series Mac) — use PyTorch MPS backend on Apple Silicon. Suitable for 5-20M param models.

5. **Model architecture**: GNN (Graph Neural Network) — matches board topology naturally, proven in DeepMind's Diplomacy work. Consider Transformer for v2.

6. **Target strength**: Beat all Go bots — consistently beat the Go hard bot. Achievable with supervised learning + light self-play.

7. **Licensing**: TBD — investigate webDiplomacy data access. Fallback: Kaggle public data + Facebook research dataset + self-play.

8. **Engine name**: **realpolitik** — Diplomacy term for pragmatic power politics.

---

## 10. References

- [UCI Protocol Specification](https://gist.github.com/DOBRO/2592c6dad754ba67e6dcaec8c90165bf)
- [DAIDE Protocol](http://www.daide.org.uk/commodel.html) — Binary protocol for Diplomacy AI
- [DipNet / No Press Diplomacy](https://arxiv.org/abs/1909.02128) — Supervised + RL Diplomacy agent
- [Facebook SearchBot](https://github.com/facebookresearch/diplomacy_searchbot) — Equilibrium search via regret matching (ICLR 2021)
- [Meta Cicero](https://github.com/facebookresearch/diplomacy_cicero) — Human-level press Diplomacy (Science 2022)
- [Google DeepMind Diplomacy](https://github.com/google-deepmind/diplomacy) — GNN-based board encoding
- [Kaggle Diplomacy Game Dataset](https://www.kaggle.com/datasets/gowripreetham/diplomacy-game-dataset)
- [Kaggle Diplomacy Betrayal Dataset](https://www.kaggle.com/datasets/rtatman/diplomacy-betrayal-dataset)
- [ort — Rust ONNX Runtime](https://github.com/pykeio/ort)
- [tract — Pure Rust ONNX inference](https://github.com/sonos/tract)
