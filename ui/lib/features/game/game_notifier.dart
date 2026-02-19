import 'dart:async';
import 'dart:convert';
import 'dart:developer' as dev;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/ws_client.dart';
import '../../core/auth/auth_notifier.dart';
import '../../core/models/game.dart';
import '../../core/models/game_state.dart';
import '../../core/models/phase.dart';
import '../../core/models/order.dart';

/// Composite game view state.
class GameViewState {
  final Game? game;
  final Phase? currentPhase;
  final GameState? gameState;
  final GameState? previousGameState;
  final List<Order> phaseOrders;
  final List<Order> resolvedOrders;
  final int readyCount;
  final int drawVoteCount;
  final bool loading;
  final String? error;

  const GameViewState({
    this.game,
    this.currentPhase,
    this.gameState,
    this.previousGameState,
    this.phaseOrders = const [],
    this.resolvedOrders = const [],
    this.readyCount = 0,
    this.drawVoteCount = 0,
    this.loading = true,
    this.error,
  });

  GameViewState copyWith({
    Game? game,
    Phase? currentPhase,
    GameState? gameState,
    GameState? previousGameState,
    bool clearPreviousGameState = false,
    List<Order>? phaseOrders,
    List<Order>? resolvedOrders,
    int? readyCount,
    int? drawVoteCount,
    bool? loading,
    String? error,
  }) {
    return GameViewState(
      game: game ?? this.game,
      currentPhase: currentPhase ?? this.currentPhase,
      gameState: gameState ?? this.gameState,
      previousGameState: clearPreviousGameState
          ? null
          : (previousGameState ?? this.previousGameState),
      phaseOrders: phaseOrders ?? this.phaseOrders,
      resolvedOrders: resolvedOrders ?? this.resolvedOrders,
      readyCount: readyCount ?? this.readyCount,
      drawVoteCount: drawVoteCount ?? this.drawVoteCount,
      loading: loading ?? this.loading,
      error: error ?? this.error,
    );
  }

  /// Returns the power assigned to the current user, if any.
  String? powerForUser(String userId) {
    final player = game?.players.where((p) => p.userId == userId).firstOrNull;
    return player?.power;
  }
}

class GameNotifier extends StateNotifier<GameViewState> {
  final ApiClient _api;
  final WSClient _ws;
  final String gameId;
  StreamSubscription<WSEvent>? _wsSub;
  Timer? _pollTimer;
  bool _fastPolling = false;
  int _loadGeneration = 0;
  String? _lastAnimatedPhaseId;

  /// Phase ID of a retreat that resolved while a movement animation was playing.
  /// Checked in clearPreviousGameState() to chain the retreat animation.
  String? _pendingRetreatPhaseId;

  GameNotifier(this._api, this._ws, this.gameId) : super(const GameViewState()) {
    _ws.subscribe(gameId);
    _wsSub = _ws.events.where((e) => e.gameId == gameId).listen(_onEvent);
    // Start with the default 5s poll; adjusted to 500ms once we know it's bot-only.
    _pollTimer = Timer.periodic(const Duration(seconds: 5), (_) => load());
    load();
  }

  Future<void> load() async {
    final generation = ++_loadGeneration;
    final hasData = state.game != null;
    // Only show loading spinner on initial load, not during retries.
    if (!hasData) {
      state = state.copyWith(loading: true, error: null);
    }
    try {
      // Fetch game and current phase in parallel.
      final results = await Future.wait([
        _api.get('/games/$gameId'),
        _api.get('/games/$gameId/phases/current'),
      ]);

      // Discard stale results if a newer load() was triggered.
      if (generation != _loadGeneration) return;

      final gameResp = results[0];
      final phaseResp = results[1];

      if (gameResp.statusCode != 200) {
        if (!hasData) {
          state = state.copyWith(loading: false, error: 'Failed to load game');
        }
        return;
      }

      final game = Game.fromJson(jsonDecode(gameResp.body) as Map<String, dynamic>);

      // Switch to fast polling (500ms) for bot-only games.
      if (game.isBotOnly && !_fastPolling) {
        _fastPolling = true;
        _pollTimer?.cancel();
        _pollTimer = Timer.periodic(
            const Duration(seconds: 5), (_) => load());
      }

      Phase? phase;
      GameState? gs;
      if (phaseResp.statusCode == 200) {
        phase = Phase.fromJson(jsonDecode(phaseResp.body) as Map<String, dynamic>);
        gs = GameState.fromJson(phase.stateAfter ?? phase.stateBefore);
      }

      // Detect movement phase transition for animation — works even when
      // the WebSocket phase_resolved event is missed (hot restart, etc.).
      // Guard with _lastAnimatedPhaseId to prevent re-triggering the same
      // animation after the freeze ends (stale currentPhase would otherwise
      // cause an infinite replay loop in fast bot games).
      final animatablePhase = state.currentPhase?.phaseType == 'movement'
          || state.currentPhase?.phaseType == 'retreat';
      if (phase != null
          && state.currentPhase != null
          && phase.id != state.currentPhase!.id
          && animatablePhase
          && state.gameState != null
          && state.previousGameState == null
          && state.currentPhase!.id != _lastAnimatedPhaseId) {
        dev.log('phase transition detected in load(): '
            '${state.currentPhase!.phaseType} → ${phase.phaseType}',
            name: 'GameNotifier');
        _lastAnimatedPhaseId = state.currentPhase!.id;
        state = state.copyWith(
          previousGameState: state.gameState,
          resolvedOrders: [],
        );
        _fetchResolvedOrders(state.currentPhase!.id);
      }

      // Freeze gameState during animation to prevent visual jumps when
      // subsequent phases resolve faster than the animation plays.
      final freezeGameState = state.previousGameState != null;
      state = state.copyWith(
        game: game,
        currentPhase: freezeGameState ? state.currentPhase : (phase ?? state.currentPhase),
        gameState: freezeGameState ? state.gameState : (gs ?? state.gameState),
        readyCount: game.readyCount,
        drawVoteCount: game.drawVoteCount,
        loading: false,
        error: null,
      );
    } catch (e) {
      if (generation != _loadGeneration) return;
      // Only show error if we have no data yet; otherwise silently retry via poll.
      if (!hasData) {
        state = state.copyWith(loading: false, error: e.toString());
      }
    }
  }

  void _onEvent(WSEvent event) {
    dev.log('WS event: ${event.type}', name: 'GameNotifier');
    switch (event.type) {
      case 'phase_resolved':
        // Use event payload's type field — more reliable than currentPhase
        // which may already reflect the NEW phase if a poll raced the event.
        final resolvedType = event.data['type'] as String?;
        final phaseId = event.data['phase_id'] as String?;
        dev.log('phase_resolved: type=$resolvedType, phaseId=$phaseId, '
            'hasGameState=${state.gameState != null}',
            name: 'GameNotifier');
        if ((resolvedType == 'movement' || resolvedType == 'retreat')
            && state.gameState != null
            && phaseId != _lastAnimatedPhaseId) {
          if (state.previousGameState == null) {
            _lastAnimatedPhaseId = phaseId;
            _pendingRetreatPhaseId = null;
            state = state.copyWith(
              previousGameState: state.gameState,
              resolvedOrders: [],
            );
            dev.log('animation snapshot set (${state.gameState!.units.length} units)',
                name: 'GameNotifier');
          } else if (resolvedType == 'retreat') {
            // Retreat resolved while movement animation is still playing.
            // Queue it so clearPreviousGameState() can chain the retreat animation.
            _pendingRetreatPhaseId = phaseId;
            dev.log('retreat animation queued (movement animation in progress)',
                name: 'GameNotifier');
          }
        }
        if (phaseId != null) _fetchResolvedOrders(phaseId);
        state = state.copyWith(readyCount: 0);
        load();
      case 'phase_changed':
        state = state.copyWith(readyCount: 0);
        load();
      case 'player_ready':
        final count = event.data['ready_count'];
        if (count is int) {
          state = state.copyWith(readyCount: count);
        }
      case 'draw_vote':
        final count = event.data['draw_vote_count'];
        if (count is int) {
          state = state.copyWith(drawVoteCount: count);
        }
      case 'game_ended':
        load();
    }
  }

  Future<void> _fetchResolvedOrders(String phaseId) async {
    try {
      final resp = await _api.get('/games/$gameId/phases/$phaseId/orders');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Order.fromJson(e as Map<String, dynamic>))
            .toList();
        // Don't overwrite animation orders with orders from a later phase.
        if (state.previousGameState != null && state.resolvedOrders.isNotEmpty) {
          return;
        }
        final moves = list.where((o) => o.orderType == 'move').length;
        dev.log('resolved orders loaded: ${list.length} total, $moves moves, '
            'hasPrevState=${state.previousGameState != null}',
            name: 'GameNotifier');
        state = state.copyWith(resolvedOrders: list);
      }
    } catch (_) {
      // Non-critical: resolved orders are a UX enhancement.
    }
  }

  Future<(List<Order>?, String?)> submitOrders(List<OrderInput> orders) async {
    try {
      final resp = await _api.post(
        '/games/$gameId/orders',
        body: {'orders': orders.map((o) => o.toJson()).toList()},
      );
      if (resp.statusCode == 200 || resp.statusCode == 201) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Order.fromJson(e as Map<String, dynamic>))
            .toList();
        state = state.copyWith(phaseOrders: list);
        return (list, null);
      }
      // Extract error message from response body.
      String? errorMsg;
      try {
        final body = jsonDecode(resp.body) as Map<String, dynamic>;
        errorMsg = body['error'] as String?;
      } catch (_) {
        errorMsg = resp.body;
      }
      return (null, errorMsg ?? 'Order submission failed (${resp.statusCode})');
    } catch (e) {
      return (null, 'Connection error — server may be restarting. Try again.');
    }
  }

  /// Clears the animation snapshot after the animation completes.
  /// If a retreat phase resolved during the movement animation, chains
  /// into the retreat animation instead of returning to the live state.
  void clearPreviousGameState() {
    final pendingRetreat = _pendingRetreatPhaseId;
    if (pendingRetreat != null) {
      _pendingRetreatPhaseId = null;
      _lastAnimatedPhaseId = pendingRetreat;
      dev.log('chaining retreat animation for phase $pendingRetreat',
          name: 'GameNotifier');
      // Fetch the retreat phase to get its stateBefore (which has dislodged units).
      _loadRetreatPhaseForAnimation(pendingRetreat);
      return;
    }
    state = state.copyWith(clearPreviousGameState: true);
  }

  /// Loads a retreat phase's stateBefore and resolved orders to start the
  /// retreat animation after a movement animation completes.
  Future<void> _loadRetreatPhaseForAnimation(String phaseId) async {
    try {
      final resp = await _api.get('/games/$gameId/phases');
      if (resp.statusCode == 200) {
        final phases = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Phase.fromJson(e as Map<String, dynamic>))
            .toList();
        final retreatPhase = phases.where((p) => p.id == phaseId).firstOrNull;
        if (retreatPhase != null) {
          final retreatState = GameState.fromJson(retreatPhase.stateBefore);
          state = state.copyWith(
            previousGameState: retreatState,
            gameState: retreatState,
            currentPhase: retreatPhase,
            resolvedOrders: [],
          );
          _fetchResolvedOrders(phaseId);
          dev.log('retreat phase loaded for animation '
              '(${retreatState.dislodged.length} dislodged units)',
              name: 'GameNotifier');
          return;
        }
      }
    } catch (e) {
      dev.log('failed to load retreat phase for animation: $e',
          name: 'GameNotifier');
    }
    // Fallback: if we can't load the retreat phase, just clear the animation.
    state = state.copyWith(clearPreviousGameState: true);
  }

  Future<bool> markReady() async {
    try {
      final resp = await _api.post('/games/$gameId/orders/ready');
      return resp.statusCode == 200;
    } catch (_) {
      return false;
    }
  }

  Future<bool> unmarkReady() async {
    try {
      final resp = await _api.delete('/games/$gameId/orders/ready');
      return resp.statusCode == 200;
    } catch (_) {
      return false;
    }
  }

  Future<bool> voteForDraw() async {
    try {
      final resp = await _api.post('/games/$gameId/draw/vote');
      return resp.statusCode == 200;
    } catch (_) {
      return false;
    }
  }

  Future<bool> removeDrawVote() async {
    try {
      final resp = await _api.delete('/games/$gameId/draw/vote');
      return resp.statusCode == 200;
    } catch (_) {
      return false;
    }
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _wsSub?.cancel();
    _ws.unsubscribe(gameId);
    super.dispose();
  }
}

final gameProvider =
    StateNotifierProvider.family<GameNotifier, GameViewState, String>((ref, gameId) {
  return GameNotifier(
    ref.watch(apiClientProvider),
    ref.watch(wsClientProvider),
    gameId,
  );
});
