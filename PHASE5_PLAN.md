# Phase 5: Flutter MVP

## Context

Phases 0–4 are complete (game engine, REST API, async turns, WebSocket, integration tests). The backend is fully functional at `localhost:3009`. The `ui/` directory is empty — no Flutter project exists yet. We're building the cross-platform Flutter frontend (Android, iOS, Web) to create a **playable Diplomacy MVP** (`v0.6.0`).

## Key Decisions

- **Map**: SVG base layer + CustomPainter overlay for units/arrows/highlights
- **Platforms**: Android, iOS, Web from the start
- **Auth**: Dev token mode for MVP (add `/auth/dev` endpoint to backend)
- **State management**: Riverpod (with code generation)
- **Routing**: go_router
- **Bundle ID**: `com.politebetrayal.app`

## Critical JSON Gotcha

The `GameState` struct in Go has **no JSON tags** — fields serialize as PascalCase (`Year`, `Season`, `Units`, `SupplyCenters`). The `UnitType` is an `int` (0=Army, 1=Fleet), not a string. The rest of the API models (`Game`, `Phase`, `Order`, `Message`) use snake_case JSON tags. Dart models must handle both conventions.

---

## Sub-Phase A: Project Setup & Core Infrastructure

**Goal**: Flutter project compiles on all platforms, dev login works, API client makes authenticated requests.

### Backend Change

**`api/internal/handler/auth_handler.go`** — Add `DevLogin` handler:
- `GET /auth/dev?name=Alice` → creates/upserts a test user (provider="dev"), returns JWT token pair
- Guard with a `DEV_MODE=true` env var check so it can't run in production

**`api/cmd/server/main.go`** — Register `GET /auth/dev` route

### Flutter Files

| File | Purpose |
|------|---------|
| `pubspec.yaml` | Dependencies: `flutter_riverpod`, `riverpod_annotation`, `go_router`, `web_socket_channel`, `flutter_secure_storage`, `flutter_svg`, `http`, `freezed_annotation`, `json_annotation`. Dev deps: `build_runner`, `riverpod_generator`, `freezed`, `json_serializable` |
| `lib/main.dart` | `ProviderScope` → `MaterialApp.router` with go_router |
| `lib/core/api/api_config.dart` | Base URL constant (`http://localhost:3009/api/v1`), auth URL |
| `lib/core/api/api_client.dart` | HTTP client wrapper with JWT auth header injection, auto-refresh on 401, `get`/`post`/`patch`/`delete` methods |
| `lib/core/api/ws_client.dart` | WebSocket client: connect with token, subscribe/unsubscribe to games, event stream, auto-reconnect with backoff |
| `lib/core/auth/auth_state.dart` | Freezed: `AuthState` (unauthenticated / authenticated with tokens+user) |
| `lib/core/auth/auth_notifier.dart` | Riverpod notifier: `devLogin(name)`, `refresh()`, `logout()`, reads/writes tokens to `flutter_secure_storage` |
| `lib/core/models/user.dart` | Freezed model matching backend `User` JSON (snake_case) |
| `lib/core/models/game.dart` | Freezed model: `Game`, `GamePlayer` (snake_case) |
| `lib/core/models/phase.dart` | Freezed model: `Phase` with `stateBefore`/`stateAfter` as `Map<String, dynamic>` |
| `lib/core/models/game_state.dart` | Freezed model matching Go's `GameState` (**PascalCase** JSON keys, `UnitType` as int) |
| `lib/core/models/order.dart` | Freezed: `Order` (from API, snake_case) and `OrderInput` (for submission) |
| `lib/core/models/message.dart` | Freezed model (snake_case) |
| `lib/core/models/ws_event.dart` | Freezed: `WSEvent` with type, game_id, data |
| `lib/core/theme/app_theme.dart` | Power colors (Austria=red, England=navy, France=lightblue, Germany=gray, Italy=green, Russia=purple, Turkey=yellow), light theme |
| `lib/core/router/app_router.dart` | go_router config with auth redirect: `/login`, `/home`, `/game/create`, `/game/:id`, `/game/:id/lobby`, `/game/:id/messages`, `/profile` |
| `lib/features/auth/login_screen.dart` | Text field for name + "Dev Login" button |

### Verify
- `flutter run -d chrome` shows login screen
- Dev login → token stored → redirects to `/home` (placeholder)
- Restart app → auto-login from stored token
- `GET /api/v1/users/me` returns user data

---

## Sub-Phase B: Home & Lobby

**Goal**: Create, join, and start games. Real-time lobby updates via WebSocket.

### Files

| File | Purpose |
|------|---------|
| `lib/shared/widgets/app_shell.dart` | Shell with bottom nav: Home, Profile |
| `lib/shared/widgets/game_card.dart` | Card showing game name, status chip, player count |
| `lib/shared/widgets/power_badge.dart` | Small colored badge for a power |
| `lib/features/home/home_screen.dart` | Two tabs: "Open Games" / "My Games", pull-to-refresh, FAB for create |
| `lib/features/home/game_list_notifier.dart` | Riverpod async notifier fetching game lists |
| `lib/features/lobby/create_game_screen.dart` | Form: name, turn/retreat/build duration dropdowns, Create button |
| `lib/features/lobby/lobby_screen.dart` | Waiting room: player list, Join button, Start button (creator, 7 players) |
| `lib/features/lobby/lobby_notifier.dart` | State for lobby, listens to WS for player join / game start events |

### Verify
- Open Games tab lists waiting games
- Create game → appears in list
- Join game → lobby shows updated player list
- Start game (7 players) → powers assigned, navigates to game screen

---

## Sub-Phase C: Map & Game View

**Goal**: Interactive Diplomacy map with units, supply centers, phase bar, and countdown timer.

### SVG Map Strategy
- Place a standard Diplomacy SVG at `assets/map/diplomacy_map.svg` with province elements having IDs matching backend province codes (e.g., `id="par"`)
- Render as background with `flutter_svg`
- Overlay with `CustomPainter` for all dynamic elements
- Wrap in `InteractiveViewer` for zoom/pan
- Hit-test province taps via nearest-center proximity (simpler than path-based for MVP)

### Files

| File | Purpose |
|------|---------|
| `assets/map/diplomacy_map.svg` | Standard Diplomacy map SVG asset |
| `lib/core/map/province_data.dart` | Province center (x,y) coordinates, names, types for all 75 provinces. Must be calibrated to the SVG's coordinate space |
| `lib/core/map/hit_testing.dart` | `hitTestProvince(Offset tap, scale, offset)` — finds nearest province center within threshold |
| `lib/features/game/game_screen.dart` | Main game view: PhaseBar (top) + GameMap (center) + action bar (bottom) |
| `lib/features/game/game_notifier.dart` | Core state: fetches game, current phase, parses GameState, subscribes WS, handles events |
| `lib/features/game/widgets/game_map.dart` | `InteractiveViewer` + `GestureDetector` + SVG background + `CustomPaint` overlay |
| `lib/features/game/widgets/map_painter.dart` | Draws units (army/fleet icons colored by power), SC dots, province highlight, order arrows |
| `lib/features/game/widgets/phase_bar.dart` | Season/year, phase type, countdown, ready count |
| `lib/features/game/widgets/countdown_timer.dart` | `Timer.periodic` computing `deadline - now`, displays HH:MM:SS |
| `lib/features/game/widgets/supply_center_table.dart` | Compact per-power SC count / unit count display |

### Verify
- Game screen shows SVG map, zoomable/pannable
- Units appear at correct provinces with power colors
- Army vs fleet visually distinct
- SC dots match ownership
- Phase bar shows Spring 1901 Movement
- Timer counts down
- Province taps detected and logged

---

## Sub-Phase D: Order Input

**Goal**: Full order entry flow — select unit, choose type, pick targets, submit, mark ready.

### Order UX (Movement Phase)
1. Tap your unit → highlight, show order type buttons (Hold/Move/Support/Convoy)
2. **Hold**: add immediately
3. **Move**: tap target → if fleet to split-coast, show coast picker → done
4. **Support**: tap supported unit → tap its destination (or none for support-hold) → done
5. **Convoy**: tap convoyed army → tap destination → done
6. Pending orders shown as arrows on map + list at bottom
7. "Submit" → `POST /api/v1/games/{id}/orders`
8. "Ready" → `POST /api/v1/games/{id}/orders/ready`

### Files

| File | Purpose |
|------|---------|
| `lib/core/map/adjacency_data.dart` | Dart port of `map_data.go` adjacency graph — enables highlighting valid targets client-side |
| `lib/features/game/order_input/order_state.dart` | Freezed state machine: idle → unitSelected → awaitingTarget → awaitingAuxLoc → awaitingAuxTarget → complete |
| `lib/features/game/order_input/order_input_notifier.dart` | Manages transitions: `selectProvince()`, `selectOrderType()`, `selectCoast()`, `cancel()`, `confirmOrder()` |
| `lib/features/game/order_input/order_action_bar.dart` | Bottom bar: order type buttons, prompt text, cancel, submit/ready buttons |
| `lib/features/game/order_input/pending_orders_list.dart` | Draggable bottom sheet listing pending orders with delete |
| `lib/features/game/order_input/coast_picker.dart` | Bottom sheet for spa/stp/bul coast selection |
| `lib/features/game/order_input/build_order_panel.dart` | Build phase: shows needed builds/disbands, tap home SC or unit |
| `lib/features/game/order_input/retreat_order_panel.dart` | Retreat phase: shows dislodged units, retreat or disband options |
| `lib/features/game/widgets/map_painter.dart` | **Update**: draw pending order arrows (move=solid, support=dashed, convoy=dotted) |
| `lib/features/game/game_screen.dart` | **Update**: integrate order panels, pass `onProvinceTap` to map |

### Verify
- Select own unit → order type buttons appear. Cannot select opponent's units
- Move flow: unit → Move → valid targets highlighted → tap target → arrow drawn
- Support flow: 3-step works end-to-end
- Split-coast picker appears for spa/stp/bul fleet moves
- Submit orders → backend accepts, pending list clears
- Mark ready → ready count updates via WS
- Phase resolves → new state loads, map updates
- Retreat and build phases work

---

## Sub-Phase E: Results, Chat & Polish

**Goal**: Phase results visualization, game history, messaging, profile, game-over.

### Files

| File | Purpose |
|------|---------|
| `lib/features/game/widgets/phase_results.dart` | Overlay showing each order's result (succeeded=green, bounced=red, cut=orange), color-coded arrows on map |
| `lib/features/game/widgets/phase_history.dart` | Side drawer listing all phases — tap to view that phase's historical state |
| `lib/features/game/phase_history_notifier.dart` | Fetches all phases, manages "viewing past phase" state |
| `lib/features/messages/messages_screen.dart` | Full chat: tabs for Public / per-power DMs, message list grouped by phase, input field |
| `lib/features/messages/messages_notifier.dart` | Fetches messages, sends messages, receives WS message events |
| `lib/features/messages/message_bubble.dart` | Single message: sender power colored, content, timestamp |
| `lib/features/game/widgets/game_over_dialog.dart` | Dialog on `game_ended` WS event: shows winner, links to final state |
| `lib/features/profile/profile_screen.dart` | Display name, edit, logout |
| `lib/core/router/app_router.dart` | **Update**: add `/game/:id/messages` route |

### Verify
- After resolution: results overlay with color-coded outcomes
- Phase history: browse past phases, map renders historical state
- Chat: send/receive public and private messages in real time
- Game over dialog appears when a power wins
- Profile: edit display name, logout clears tokens

---

## Backend API Reference (Quick)

### Routes (all JSON, snake_case except GameState)

```
Public:
  GET  /auth/dev?name=Alice          → { access_token, refresh_token, expires_in }
  GET  /auth/google/login            → redirect
  GET  /auth/google/callback         → { access_token, refresh_token, expires_in }
  POST /auth/refresh                 → { access_token, refresh_token, expires_in }

Protected (Authorization: Bearer <token>):
  GET    /api/v1/users/me
  PATCH  /api/v1/users/me            → { display_name }
  GET    /api/v1/users/{id}
  POST   /api/v1/games               → { name, turn_duration, retreat_duration, build_duration }
  GET    /api/v1/games?filter=        → waiting|active|mine
  GET    /api/v1/games/{id}
  POST   /api/v1/games/{id}/join
  POST   /api/v1/games/{id}/start
  POST   /api/v1/games/{id}/orders   → [{ unit_type, location, order_type, target?, aux_loc?, aux_target?, aux_unit_type? }]
  POST   /api/v1/games/{id}/orders/ready
  GET    /api/v1/games/{id}/phases
  GET    /api/v1/games/{id}/phases/current
  GET    /api/v1/games/{id}/messages?recipient_id=
  POST   /api/v1/games/{id}/messages → { content, recipient_id? }

WebSocket:
  GET  /api/v1/ws?token=<JWT>
  Client sends: { "action": "subscribe"|"unsubscribe", "game_id": "..." }
  Server sends: { "type": "phase_changed"|"phase_resolved"|"player_ready"|"message"|"game_started"|"game_ended", "game_id": "...", "data": {...} }
```

### GameState JSON (PascalCase, no tags)

```json
{
  "Year": 1901,
  "Season": "spring",
  "Phase": "movement",
  "Units": [
    { "Type": 0, "Power": "austria", "Province": "vie", "Coast": "" },
    { "Type": 1, "Power": "england", "Province": "lon", "Coast": "" }
  ],
  "SupplyCenters": { "vie": "austria", "lon": "england" },
  "Dislodged": []
}
```

UnitType: 0=Army, 1=Fleet

---

## Google Auth Preparation (Future)

When ready to add real Google OAuth (Phase 6), you'll need:

1. **Google Cloud Console**: Create project, enable Google Sign-In API, configure consent screen
2. **OAuth 2.0 Credentials**: Web (redirect URI), Android (SHA-1), iOS (bundle ID)
3. **Backend env vars**: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`
4. **Flutter packages**: `google_sign_in`

The dev token system uses the same JWT token format, so the rest of the app works identically with real OAuth — only the login screen changes.

---

## Verification (End-to-End)

1. `flutter create` in `ui/`, `flutter run -d chrome` compiles on all platforms
2. Dev login → create game → 7 players join → start → map renders with units
3. Enter movement orders → submit → mark ready → phase resolves → map updates
4. Chat works (public + private), phase history browsable
5. Artificial game-over → dialog appears
6. `flutter analyze` clean, `gofmt -s && go vet` clean on backend changes
