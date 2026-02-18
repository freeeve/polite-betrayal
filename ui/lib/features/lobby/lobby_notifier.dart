import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/ws_client.dart';
import '../../core/auth/auth_notifier.dart';
import '../../core/models/game.dart';

/// State notifier for a game lobby, listens to WS for player join / game start events.
class LobbyNotifier extends StateNotifier<AsyncValue<Game>> {
  final ApiClient _api;
  final WSClient _ws;
  final String gameId;
  StreamSubscription<WSEvent>? _wsSub;

  LobbyNotifier(this._api, this._ws, this.gameId) : super(const AsyncValue.loading()) {
    _ws.subscribe(gameId);
    _wsSub = _ws.events.where((e) => e.gameId == gameId).listen(_onEvent);
    refresh();
  }

  Future<void> refresh() async {
    try {
      final resp = await _api.get('/games/$gameId');
      if (resp.statusCode == 200) {
        state = AsyncValue.data(Game.fromJson(jsonDecode(resp.body) as Map<String, dynamic>));
      } else {
        state = AsyncValue.error('Failed to load game', StackTrace.current);
      }
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  void _onEvent(WSEvent event) {
    switch (event.type) {
      case 'game_started':
      case 'player_joined':
      case 'power_changed':
        refresh();
    }
  }

  Future<String?> joinGame() async {
    final resp = await _api.post('/games/$gameId/join');
    if (resp.statusCode == 200) {
      await refresh();
      return null;
    }
    return 'Failed to join: ${resp.body}';
  }

  Future<String?> startGame() async {
    final resp = await _api.post('/games/$gameId/start');
    if (resp.statusCode == 200) {
      await refresh();
      return null;
    }
    return 'Failed to start: ${resp.body}';
  }

  Future<String?> updatePlayerPower(String userId, String power) async {
    final resp = await _api.patch(
      '/games/$gameId/players/$userId/power',
      body: {'power': power},
    );
    if (resp.statusCode == 200) {
      await refresh();
      return null;
    }
    return 'Failed to update power: ${resp.body}';
  }

  Future<String?> updateBotDifficulty(String botUserId, String difficulty) async {
    final resp = await _api.patch(
      '/games/$gameId/players/$botUserId/bot-difficulty',
      body: {'difficulty': difficulty},
    );
    if (resp.statusCode == 200) {
      await refresh();
      return null;
    }
    return 'Failed to update difficulty';
  }

  @override
  void dispose() {
    _wsSub?.cancel();
    _ws.unsubscribe(gameId);
    super.dispose();
  }
}

final lobbyProvider =
    StateNotifierProvider.family<LobbyNotifier, AsyncValue<Game>, String>((ref, gameId) {
  return LobbyNotifier(
    ref.watch(apiClientProvider),
    ref.watch(wsClientProvider),
    gameId,
  );
});
