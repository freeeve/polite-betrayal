# DUI Protocol Specification v1

## Diplomacy Universal Interface

**Version**: 1
**Engine**: realpolitik
**Status**: Draft
**Date**: 2026-02-17

---

## Table of Contents

1. [Protocol Overview](#1-protocol-overview)
2. [DFEN Format](#2-dfen-format)
3. [DSON Format](#3-dson-format)
4. [Command Set](#4-command-set)
5. [Session Flow](#5-session-flow)
6. [Protocol Versioning](#6-protocol-versioning)
7. [DFEN Examples](#7-dfen-examples)

---

## 1. Protocol Overview

DUI (Diplomacy Universal Interface) is a text-based protocol for communication between a game server and an external Diplomacy engine. It is modeled after the UCI (Universal Chess Interface) protocol.

### Design Principles

- **Text-based stdin/stdout** -- one command per line, newline-terminated (`\n`).
- **Stateless position** -- the server sends the full board state on every turn. The engine does not need to maintain state across positions.
- **Simultaneous moves** -- all seven powers submit orders at once. The engine plays one power at a time; the server queries it once per power per phase.
- **Phase-aware** -- the protocol distinguishes movement, retreat, and build phases.
- **Power-scoped** -- the engine plays one power at a time. The server may spawn seven processes or query one engine for each power sequentially.

### Transport

- Server launches the engine as a child process.
- Server writes commands to the engine's stdin.
- Engine writes responses to its stdout.
- All lines are UTF-8 encoded and terminated with `\n`.
- Lines must not exceed 65535 bytes.
- The engine must not write to stderr during normal operation (stderr is reserved for debug logging).

---

## 2. DFEN Format

DFEN (Diplomacy FEN) is a single-line text encoding of the complete board state, analogous to FEN in chess. A DFEN string encodes the phase, all units, all supply center ownership, and any dislodged units.

### Structure

```
DFEN = <phase_info> "/" <units> "/" <supply_centers> "/" <dislodged>
```

The four sections are separated by forward slashes (`/`).

### 2.1 Phase Info

```
phase_info = <year> <season> <phase>
year       = integer (e.g., 1901)
season     = "s" | "f"
phase      = "m" | "r" | "b"
```

Season values:
- `s` -- Spring
- `f` -- Fall

Phase values:
- `m` -- Movement
- `r` -- Retreat
- `b` -- Build (adjustment)

Examples:
- `1901sm` -- Spring 1901 Movement
- `1902fr` -- Fall 1902 Retreat
- `1901fb` -- Fall 1901 Build

### 2.2 Units

```
units      = <unit_entry> ["," <unit_entry>]*
unit_entry = <power_char> <unit_type> <location>
power_char = "A" | "E" | "F" | "G" | "I" | "R" | "T"
unit_type  = "a" | "f"
location   = <prov_id> ["." <coast>]
prov_id    = [a-z]{3}
coast      = "nc" | "sc" | "ec"
```

Power character mapping:
| Character | Power   |
|-----------|---------|
| `A`       | Austria |
| `E`       | England |
| `F`       | France  |
| `G`       | Germany |
| `I`       | Italy   |
| `R`       | Russia  |
| `T`       | Turkey  |

Unit type:
- `a` -- Army
- `f` -- Fleet

Location uses a dot (`.`) separator for coasts to avoid ambiguity with the `/` field separator. Coasts are only specified for fleets on split-coast provinces (spa, stp, bul).

Examples:
- `Aavie` -- Austrian Army at Vienna
- `Eftri` -- English Fleet at Trieste
- `Rfstp.sc` -- Russian Fleet at St. Petersburg, South Coast

If there are no units on the board (theoretical edge case), the units section is a single dash: `-`.

### 2.3 Supply Centers

```
supply_centers = <sc_entry> ["," <sc_entry>]*
sc_entry       = <power_char> <prov_id>
```

All 34 supply centers are listed explicitly, regardless of whether their ownership matches the default starting position. Neutral supply centers use `N` as the power character.

This design choice avoids ambiguity: a parser never needs to know the default SC assignments to reconstruct the board. The cost is approximately 150 additional characters in the DFEN string, which is negligible for a text protocol.

Supply centers are listed in a fixed order: Austria, England, France, Germany, Italy, Russia, Turkey, then Neutral. Within each power's group, the order is alphabetical by province ID.

Example (initial position):
```
Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun
```

### 2.4 Dislodged Units

```
dislodged       = <dislodged_entry> ["," <dislodged_entry>]* | "-"
dislodged_entry = <power_char> <unit_type> <location> "<" <prov_id>
```

Each dislodged entry records:
- The dislodged unit (power, type, location including coast)
- The province the attacker came from (after the `<` character)

The `<` character is read as "dislodged by attack from". The attacker-from province is needed because the dislodged unit cannot retreat to it.

If there are no dislodged units, this section is a single dash: `-`.

Example:
- `Aavie<boh` -- Austrian Army at Vienna, dislodged by an attack from Bohemia
- `Rfsev<rum` -- Russian Fleet at Sevastopol, dislodged by an attack from Rumania

### 2.5 Formal Grammar

```
dfen            = phase_info "/" units_section "/" sc_section "/" dislodged_section
phase_info      = year season phase
year            = DIGIT+
season          = "s" | "f"
phase           = "m" | "r" | "b"

units_section   = "-" | unit_entry ("," unit_entry)*
unit_entry      = power_char unit_type location

sc_section      = sc_entry ("," sc_entry)*
sc_entry        = (power_char | "N") prov_id

dislodged_section = "-" | dislodged_entry ("," dislodged_entry)*
dislodged_entry   = power_char unit_type location "<" prov_id

power_char      = "A" | "E" | "F" | "G" | "I" | "R" | "T"
unit_type       = "a" | "f"
location        = prov_id ("." coast)?
prov_id         = LOWER LOWER LOWER
coast           = "nc" | "sc" | "ec"

DIGIT           = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
LOWER           = "a" | "b" | ... | "z"
```

---

## 3. DSON Format

DSON (Diplomacy Standard Order Notation) is a compact, unambiguous, parseable notation for Diplomacy orders. It is used in the `bestorders` response and in `info` lines.

### 3.1 Movement Phase Orders

```
A vie H                     -- Army Vienna Hold
A bud - rum                 -- Army Budapest Move to Rumania
F tri - adr                 -- Fleet Trieste Move to Adriatic Sea
A gal S A bud - rum         -- Army Galicia Support Army Budapest -> Rumania
A tyr S A vie H             -- Army Tyrolia Support Army Vienna Hold
F mao C A bre - spa         -- Fleet Mid-Atlantic Convoy Army Brest -> Spain
F nrg - stp/nc              -- Fleet Norwegian Sea Move to St. Petersburg North Coast
```

### 3.2 Retreat Phase Orders

```
A vie R boh                 -- Army Vienna Retreat to Bohemia
F tri D                     -- Fleet Trieste Disband
F stp/nc R nwy              -- Fleet St. Petersburg (NC) Retreat to Norway
```

### 3.3 Build Phase Orders

```
A vie B                     -- Build Army in Vienna
F stp/sc B                  -- Build Fleet in St. Petersburg South Coast
A war D                     -- Disband Army in Warsaw
W                           -- Waive (voluntarily skip one build)
```

### 3.4 Formal Grammar

```
order           = movement_order | retreat_order | build_order | waive_order

movement_order  = unit SP action
retreat_order   = unit SP retreat_action
build_order     = unit SP build_action
waive_order     = "W"

unit            = unit_char SP location
unit_char       = "A" | "F"
location        = prov_id ("/" coast)?
prov_id         = LOWER LOWER LOWER
coast           = "nc" | "sc" | "ec"

action          = hold | move | support_hold | support_move | convoy
hold            = "H"
move            = "-" SP location
support_hold    = "S" SP unit SP "H"
support_move    = "S" SP unit SP "-" SP location
convoy          = "C" SP "A" SP location SP "-" SP location

retreat_action  = retreat_move | disband
retreat_move    = "R" SP location
disband         = "D"

build_action    = build | disband
build           = "B"

SP              = " "
LOWER           = "a" | "b" | ... | "z"
```

Note on locations: In DSON, the coast separator is `/` (e.g., `stp/nc`). In DFEN, the coast separator is `.` (e.g., `stp.nc`). This distinction avoids conflicts with the DFEN field separator `/`.

### 3.5 Multiple Orders

When the engine returns orders for all of its units, they are separated by ` ; ` (space-semicolon-space):

```
bestorders A vie - tri ; A bud - ser ; F tri - alb
```

### 3.6 Province IDs

Province IDs are always 3-letter lowercase. The standard map uses 75 provinces.

**Inland (14)**: boh, bud, bur, gal, mos, mun, par, ruh, ser, sil, tyr, ukr, vie, war

**Coastal (39)**: alb, ank, apu, arm, bel, ber, bre, cly, con, den, edi, fin, gas, gre, hol, kie, lon, lvn, lvp, mar, naf, nap, nwy, pic, pie, por, pru, rom, rum, sev, smy, swe, syr, tri, tun, tus, ven, wal, yor

**Split-coast (3)**: bul (ec, sc), spa (nc, sc), stp (nc, sc)

**Sea (19)**: adr, aeg, bal, bar, bla, bot, eas, eng, gol, hel, ion, iri, mao, nao, nrg, nth, ska, tys, wes

### 3.7 Power Names

Full power names are used in commands like `setpower`. They are always lowercase:
`austria`, `england`, `france`, `germany`, `italy`, `russia`, `turkey`

---

## 4. Command Set

### 4.1 Server to Engine

#### `dui`

Initialize the DUI protocol. This must be the first command sent. The engine must respond with `id` lines, optionally `option` lines, and finally `duiok`.

```
Server: dui
Engine: id name realpolitik
Engine: id author polite-betrayal
Engine: option name Threads type spin default 4 min 1 max 64
Engine: option name SearchTime type spin default 5000 min 100 max 60000
Engine: option name ModelPath type string default models/v1.onnx
Engine: option name Strength type spin default 100 min 1 max 100
Engine: option name Personality type combo default balanced var aggressive var defensive var balanced
Engine: duiok
```

#### `isready`

Synchronization ping. The engine must respond with `readyok` only after it has finished processing all previous commands and is ready to accept new ones. This can be used after `setoption` or `position` to confirm the engine is in a known state.

```
Server: isready
Engine: readyok
```

#### `setoption name <id> [value <x>]`

Set an engine parameter. The option name and value are engine-specific. Common options:

| Option | Type | Description |
|--------|------|-------------|
| `Threads` | spin | Number of search threads |
| `SearchTime` | spin | Default search time in milliseconds |
| `ModelPath` | string | Path to neural network model file (ONNX) |
| `Strength` | spin | Playing strength (1-100) |
| `Personality` | combo | Strategic personality |

```
Server: setoption name Threads value 8
Server: setoption name ModelPath value /opt/models/v2.onnx
```

#### `newgame`

Reset the engine's internal state for a new game. The engine should clear any cached data, transposition tables, or game history.

```
Server: newgame
```

#### `position <dfen>`

Set the current board state. The DFEN string is passed as defined in Section 2.

```
Server: position 1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-
```

#### `setpower <power>`

Set which power the engine is playing for the current position. Must be one of: `austria`, `england`, `france`, `germany`, `italy`, `russia`, `turkey`.

```
Server: setpower austria
```

#### `go [movetime <ms>] [depth <n>] [nodes <n>] [infinite]`

Start calculating orders for the current position and assigned power. The engine must eventually respond with `bestorders`. Search constraints are optional and combinable:

| Parameter | Description |
|-----------|-------------|
| `movetime <ms>` | Hard time limit in milliseconds |
| `depth <n>` | Search depth limit (in plies or phases) |
| `nodes <n>` | Node count limit |
| `infinite` | Search until `stop` is sent |

If no constraints are given, the engine uses its default search time.

```
Server: go movetime 5000
Server: go depth 3
Server: go infinite
```

#### `stop`

Interrupt the current search. The engine must immediately output `bestorders` with the best orders found so far.

```
Server: stop
```

#### `press <from_power> <message_type> [args...]`

Deliver a diplomatic message from another power. This command is optional -- the engine may ignore press entirely.

Message types:
| Type | Arguments | Description |
|------|-----------|-------------|
| `request_support` | `<from_prov> <to_prov>` | Request support for a move |
| `propose_nonaggression` | `[<province>...]` | Propose non-aggression pact |
| `propose_alliance` | `[against <power>]` | Propose alliance |
| `threaten` | `<province>` | Threaten a province |
| `offer_deal` | `<i_take_prov> <you_take_prov>` | Propose territorial trade |
| `accept` | | Accept the last proposal |
| `reject` | | Reject the last proposal |
| `freetext` | `<base64_text>` | Free-form text (base64-encoded) |

```
Server: press france propose_alliance against germany
Server: press russia request_support war gal
Server: press england freetext SSBwcm9wb3NlIHdlIHdvcmsgdG9nZXRoZXI=
```

#### `quit`

Terminate the engine process. The engine should clean up and exit.

```
Server: quit
```

### 4.2 Engine to Server

#### `id name <engine_name>`

Engine identification. Sent during the `dui` handshake.

```
Engine: id name realpolitik
```

#### `id author <author_name>`

Author identification. Sent during the `dui` handshake.

```
Engine: id author polite-betrayal
```

#### `option name <id> type <type> [default <x>] [min <x>] [max <x>] [var <x> ...]`

Declare a supported configuration option. Sent during the `dui` handshake.

Option types:
| Type | Description |
|------|-------------|
| `check` | Boolean (true/false) |
| `spin` | Integer with min/max range |
| `combo` | One of several predefined strings |
| `button` | Trigger action (no value) |
| `string` | Arbitrary string value |

```
Engine: option name Threads type spin default 4 min 1 max 64
Engine: option name Personality type combo default balanced var aggressive var defensive var balanced
Engine: option name ModelPath type string default models/v1.onnx
Engine: option name UseBook type check default true
```

#### `duiok`

Signals that the DUI initialization handshake is complete. The engine is ready to receive commands.

```
Engine: duiok
```

#### `readyok`

Response to `isready`. The engine has processed all previous commands and is ready.

```
Engine: readyok
```

#### `info [depth <n>] [nodes <n>] [nps <n>] [time <ms>] [score <cp>] [pv <orders...>]`

Search progress information. Sent periodically during a `go` search. All fields are optional.

| Field | Description |
|-------|-------------|
| `depth <n>` | Search depth reached |
| `nodes <n>` | Number of positions evaluated |
| `nps <n>` | Nodes per second |
| `time <ms>` | Time spent searching so far |
| `score <cp>` | Centipawn-equivalent evaluation from the engine's power's perspective. Positive is good for the engine. |
| `pv <orders...>` | Principal variation -- best order set found so far (DSON format, semicolon-separated) |

```
Engine: info depth 1 nodes 1234 score 0 time 100
Engine: info depth 2 nodes 15000 score 5 time 800
Engine: info depth 3 nodes 120000 score 12 time 3200 pv A vie - tri ; A bud - ser ; F tri - alb
```

#### `bestorders <order> [; <order>]...`

The engine's chosen orders for all its units in the current position for the assigned power. Orders are in DSON format, separated by ` ; `.

For movement phases, the engine must emit one order per unit it controls:
```
Engine: bestorders A vie - tri ; A bud - ser ; F tri - alb
```

For retreat phases, the engine must emit one order per dislodged unit it controls:
```
Engine: bestorders A vie R boh ; F tri D
```

For build phases, the engine emits build, disband, or waive orders as needed. The number of orders should match the delta between supply centers and units:
```
Engine: bestorders A vie B ; F stp/sc B
Engine: bestorders A rum D
Engine: bestorders W
```

#### `press_out <to_power> <message_type> [args...]`

Engine wants to send a diplomatic message. Uses the same message type format as the inbound `press` command.

```
Engine: press_out france propose_alliance against germany
Engine: press_out russia reject
```

---

## 5. Session Flow

### 5.1 Full Example: Movement, Retreat, and Build Phases

This example shows a complete game turn cycle with the engine playing Austria.

```
-- Phase 1: Handshake --

Server: dui
Engine: id name realpolitik
Engine: id author polite-betrayal
Engine: option name Threads type spin default 4 min 1 max 64
Engine: option name ModelPath type string default models/v1.onnx
Engine: option name Strength type spin default 100 min 1 max 100
Engine: option name Personality type combo default balanced var aggressive var defensive var balanced
Engine: protocol_version 1
Engine: duiok

Server: isready
Engine: readyok

Server: setoption name Threads value 8
Server: setoption name Strength value 80
Server: isready
Engine: readyok

-- Phase 2: Spring 1901 Movement --

Server: newgame
Server: setpower austria
Server: position 1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-
Server: go movetime 5000

Engine: info depth 1 nodes 512 score 0 time 80
Engine: info depth 2 nodes 8400 score 3 time 620
Engine: info depth 3 nodes 95000 score 8 time 3100
Engine: bestorders A vie - tri ; A bud - ser ; F tri - alb

-- Phase 3: Fall 1901 Movement --
-- (after all 7 powers' spring orders are resolved)
-- Austria now has A tri, A ser, F alb (assume all succeeded)

Server: position 1901fm/Aatri,Aaser,Afalb,Eflon,Efnth,Ealvp,Ffbre,Fapar,Faspa,Gfkie,Gaden,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Ragal,Rfsev,Tfank,Tacon,Tabul/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-
Server: go movetime 5000

Engine: info depth 1 nodes 640 score 5 time 90
Engine: info depth 2 nodes 11200 score 10 time 750
Engine: bestorders A tri H ; A ser - rum ; F alb - gre

-- Phase 4: Fall 1901 Retreat --
-- Suppose after resolution, Russia's army in Galicia was dislodged
-- by a German army moving from Silesia. The server sends a retreat-phase
-- DFEN to the Russian engine (not Austria).

Server: setpower russia
Server: position 1901fr/Aatri,Aarum,Afgre,Eflon,Efnth,Ealvp,Ffbre,Fapar,Faspa,Gfkie,Gaden,Gasil,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tabul/Abud,Atri,Avie,Arum,Agre,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Nhol,Nnwy,Npor,Nser,Nspa,Nswe,Ntun/Ragal<sil
Server: go movetime 2000

Engine: info depth 1 nodes 12 score -5 time 10
Engine: bestorders A gal R ukr

-- Phase 5: Fall 1901 Build --
-- Austria captured rum and gre (2 new SCs), has 3 units, now has 5 SCs.
-- Austria can build 2 units.

Server: setpower austria
Server: position 1901fb/Aatri,Aarum,Afgre,Eflon,Efnth,Ealvp,Ffbre,Fapar,Faspa,Gfkie,Gaden,Gasil,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Raukr,Rawar,Rfsev,Tfank,Tacon,Tabul/Abud,Atri,Avie,Arum,Agre,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Gden,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nhol,Nnwy,Npor,Nser,Nspa,Nswe,Ntun/-
Server: go movetime 3000

Engine: info depth 1 nodes 24 score 15 time 20
Engine: bestorders A vie B ; A bud B

-- Phase 6: Termination --

Server: quit
```

### 5.2 Minimal Session

The simplest possible session for a single position query:

```
Server: dui
Engine: id name realpolitik
Engine: id author polite-betrayal
Engine: protocol_version 1
Engine: duiok

Server: isready
Engine: readyok

Server: newgame
Server: setpower austria
Server: position 1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-
Server: go movetime 5000
Engine: bestorders A vie - tri ; A bud - ser ; F tri - alb

Server: quit
```

### 5.3 Multi-Power Query

The server can query the same engine for multiple powers sequentially without restarting:

```
Server: setpower austria
Server: position 1901sm/.../-
Server: go movetime 5000
Engine: bestorders A vie - tri ; A bud - ser ; F tri - alb

Server: setpower england
Server: position 1901sm/.../-
Server: go movetime 5000
Engine: bestorders F lon - nth ; F edi - nrg ; A lvp - yor

Server: setpower france
Server: position 1901sm/.../-
Server: go movetime 5000
Engine: bestorders F bre - mao ; A par - bur ; A mar S A par - bur
```

---

## 6. Protocol Versioning

### 6.1 Version Announcement

During the `dui` handshake, the engine announces its supported protocol version:

```
Engine: protocol_version 1
```

This line must appear before `duiok`. If omitted, the server assumes protocol version 1.

### 6.2 Version Negotiation

The server may send a `protocol_version` command before `dui` to request a specific version:

```
Server: protocol_version 1
Server: dui
Engine: id name realpolitik
Engine: id author polite-betrayal
Engine: protocol_version 1
Engine: duiok
```

If the engine does not support the requested version, it should respond with its highest supported version. The server then decides whether to proceed or terminate.

### 6.3 Versioning Policy

- Version 1 is the initial release defined in this document.
- New optional commands may be added without changing the version (additive changes).
- Breaking changes to existing commands or formats require a version bump.
- Engines should ignore unrecognized commands gracefully (log and skip, do not crash).

---

## 7. DFEN Examples

### 7.1 Initial Position (Spring 1901 Movement)

The standard Diplomacy starting position with all 22 units and 34 supply centers.

**Units (22):**
- Austria: A vie, A bud, F tri
- England: F lon, F edi, A lvp
- France: F bre, A par, A mar
- Germany: F kie, A ber, A mun
- Italy: F nap, A rom, A ven
- Russia: F stp/sc, A mos, A war, F sev
- Turkey: F ank, A con, A smy

**Supply Centers (34):**
- Austria (3): bud, tri, vie
- England (3): edi, lon, lvp
- France (3): bre, mar, par
- Germany (3): ber, kie, mun
- Italy (3): nap, rom, ven
- Russia (4): mos, sev, stp, war
- Turkey (3): ank, con, smy
- Neutral (12): bel, bul, den, gre, hol, nwy, por, rum, ser, spa, swe, tun

**Dislodged:** none

```
1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-
```

Verification:
- 22 unit entries in the units section (matches 22 starting units)
- 34 SC entries in the supply centers section (22 owned + 12 neutral)
- Russia's St. Petersburg fleet is on the south coast (`Rfstp.sc`)
- No dislodged units (`-`)

### 7.2 Mid-Game Position (Fall 1903 Movement)

A plausible mid-game position after several turns of play. Austria has expanded into the Balkans, Germany controls the north, and Turkey holds the east.

**Units (27):**
- Austria (4): A bud, A rum, F gre, A vie
- England (4): F nth, F nwy, A yor, F lon
- France (4): F mao, A bur, A mar, F por
- Germany (5): A den, A hol, A mun, F kie, F ska
- Italy (3): F tys, A ven, A rom
- Russia (3): F sev, A mos, A war
- Turkey (4): F ank, A bul, A con, A smy

**Supply Centers (34):**
- Austria (5): bud, gre, rum, tri, vie
- England (4): edi, lon, lvp, nwy
- France (4): bre, mar, par, spa
- Germany (5): ber, den, hol, kie, mun
- Italy (3): nap, rom, ven
- Russia (3): mos, sev, war
- Turkey (4): ank, bul, con, smy
- Neutral (6): bel, por, ser, stp, swe, tun

**Dislodged:** none

```
1903fm/Aabud,Aarum,Afgre,Aavie,Efnth,Efnwy,Eayor,Eflon,Ffmao,Fabur,Famar,Ffpor,Gaden,Gahol,Gamun,Gfkie,Gfska,Iftys,Iaven,Iarom,Rfsev,Ramos,Rawar,Tfank,Tabul,Tacon,Tasmy/Abud,Agre,Arum,Atri,Avie,Eedi,Elon,Elvp,Enwy,Fbre,Fmar,Fpar,Fspa,Gber,Gden,Ghol,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rwar,Tank,Tbul,Tcon,Tsmy,Nbel,Npor,Nser,Nstp,Nswe,Ntun/-
```

Verification:
- 27 unit entries (Austria 4 + England 4 + France 4 + Germany 5 + Italy 3 + Russia 3 + Turkey 4)
- 34 SC entries (Austria 5 + England 4 + France 4 + Germany 5 + Italy 3 + Russia 3 + Turkey 4 + Neutral 6)
- Austria's unit count (4) is less than SC count (5), meaning Austria can build 1 unit in the next build phase
- No dislodged units

### 7.3 Retreat Phase Position (Fall 1902 Retreat)

A position where two units are dislodged and must retreat or disband.

Scenario: After Fall 1902 movement resolution, Austria's army in Serbia was dislodged by a Turkish army attacking from Bulgaria, and Russia's fleet in Sevastopol was dislodged by a Turkish fleet attacking from the Black Sea.

**Units (28):**
- Austria (4): A bud, A vie, F tri, A gre
- England (4): F nth, F nwy, A bel, F lon
- France (4): F mao, A bur, A par, F bre
- Germany (4): A den, A mun, F kie, A ber
- Italy (3): F nap, A ven, A rom
- Russia (4): A mos, A war, A gal, F stp.sc
- Turkey (5): A bul, F bla, A con, A smy, F ank

Note: The dislodged units at ser and sev are not in the main units list.

**Dislodged (2):**
- `Aaser<bul` -- Austrian Army at Serbia, dislodged by attack from Bulgaria
- `Rfsev<bla` -- Russian Fleet at Sevastopol, dislodged by attack from Black Sea

**Supply Centers (34):**
- Austria (4): bud, gre, tri, vie
- England (4): bel, edi, lon, lvp
- France (3): bre, mar, par
- Germany (4): ber, den, kie, mun
- Italy (3): nap, rom, ven
- Russia (4): mos, sev, stp, war
- Turkey (4): ank, bul, con, smy
- Neutral (8): hol, nwy, por, rum, ser, spa, swe, tun

Note: SC ownership updates happen after retreats are resolved and before the build phase. During the retreat phase, SCs reflect the ownership from the previous adjustment phase.

```
1902fr/Aabud,Aavie,Aftri,Aagre,Efnth,Efnwy,Eabel,Eflon,Ffmao,Fabur,Fapar,Ffbre,Gaden,Gamun,Gfkie,Gaber,Ifnap,Iaven,Iarom,Ramos,Rawar,Ragal,Rfstp.sc,Tabul,Tfbla,Tacon,Tasmy,Tfank/Abud,Agre,Atri,Avie,Ebel,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gden,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tbul,Tcon,Tsmy,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/Aaser<bul,Rfsev<bla
```

Verification:
- Phase: `1902fr` -- Fall 1902 Retreat
- 28 units in main units section (dislodged units are not in the main list)
- 2 dislodged entries: Austrian Army at ser (attacker from bul), Russian Fleet at sev (attacker from bla)
- 34 SC entries (4+4+3+4+3+4+4+8 = 34)
- The Austrian Army at ser cannot retreat to bul (where the attacker came from)
- The Russian Fleet at sev cannot retreat to bla (where the attacker came from)

---

## Appendix A: Summary of All Commands

### Server to Engine

| Command | Description |
|---------|-------------|
| `dui` | Initialize DUI protocol |
| `isready` | Synchronization ping |
| `setoption name <id> [value <x>]` | Set engine option |
| `newgame` | Reset engine state |
| `position <dfen>` | Set board position |
| `setpower <power>` | Set active power |
| `go [movetime <ms>] [depth <n>] [nodes <n>] [infinite]` | Start search |
| `stop` | Stop search immediately |
| `press <from_power> <type> [args...]` | Deliver diplomatic message |
| `quit` | Terminate engine |

### Engine to Server

| Command | Description |
|---------|-------------|
| `id name <name>` | Engine name |
| `id author <name>` | Author name |
| `option name <id> type <type> [...]` | Declare supported option |
| `protocol_version <n>` | Announce protocol version |
| `duiok` | Handshake complete |
| `readyok` | Ready confirmation |
| `info [depth <n>] [nodes <n>] [...]` | Search progress |
| `bestorders <order> [; <order>]...` | Final orders |
| `press_out <to_power> <type> [args...]` | Outbound diplomatic message |

---

## Appendix B: Differences from UCI Chess

| Aspect | UCI (Chess) | DUI (Diplomacy) |
|--------|-------------|-----------------|
| Position format | FEN (single line) | DFEN (single line, 4 sections) |
| Move notation | Algebraic (e2e4) | DSON (A vie - tri) |
| Move output | `bestmove e2e4` | `bestorders A vie - tri ; A bud - ser` |
| Players | 2 (alternating) | 7 (simultaneous) |
| Phases | Always movement | Movement, Retreat, Build |
| Diplomacy | N/A | `press` / `press_out` commands |
| Power selection | N/A (color in FEN) | `setpower` command |
| Protocol init | `uci` / `uciok` | `dui` / `duiok` |

---

## Appendix C: Comparison with Other Diplomacy Protocols

| Feature | DUI | DAIDE | webDiplomacy API |
|---------|-----|-------|------------------|
| Transport | stdin/stdout | TCP binary | HTTP/JSON |
| Encoding | Text lines | Binary tokens (2-octet) | JSON |
| Statefulness | Stateless (full position each turn) | Stateful (incremental) | Stateful (session) |
| Multi-power | One power per session | One power per connection | One power per API key |
| Negotiation | Structured press commands | Full DAIDE language | Chat messages |
| Complexity | Low (~15 commands) | High (~50+ token types) | Medium (REST endpoints) |
| Language support | Any (stdin/stdout) | Needs binary parser | Needs HTTP client |
