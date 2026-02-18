import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/models/game.dart';

/// Fetches open games (waiting to be joined).
class OpenGamesNotifier extends StateNotifier<AsyncValue<List<Game>>> {
  final ApiClient _api;

  OpenGamesNotifier(this._api) : super(const AsyncValue.loading()) {
    refresh();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    try {
      final resp = await _api.get('/games?filter=waiting');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Game.fromJson(e as Map<String, dynamic>))
            .toList();
        state = AsyncValue.data(list);
      } else {
        state = AsyncValue.error('Failed to load games', StackTrace.current);
      }
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }
}

/// Fetches games the current user is in.
class MyGamesNotifier extends StateNotifier<AsyncValue<List<Game>>> {
  final ApiClient _api;

  MyGamesNotifier(this._api) : super(const AsyncValue.loading()) {
    refresh();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    try {
      final resp = await _api.get('/games?filter=my');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Game.fromJson(e as Map<String, dynamic>))
            .toList();
        state = AsyncValue.data(list);
      } else {
        state = AsyncValue.error('Failed to load games', StackTrace.current);
      }
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  /// Stops an active game (creator only). Returns null on success or error message.
  Future<String?> stopGame(String gameId) async {
    try {
      final resp = await _api.post('/games/$gameId/stop');
      if (resp.statusCode == 200) {
        await refresh();
        return null;
      }
      final body = jsonDecode(resp.body) as Map<String, dynamic>;
      return body['error'] as String? ?? 'Failed to stop game';
    } catch (e) {
      return e.toString();
    }
  }

  /// Deletes a waiting game (creator only). Returns null on success or error message.
  Future<String?> deleteGame(String gameId) async {
    try {
      final resp = await _api.delete('/games/$gameId');
      if (resp.statusCode == 200) {
        await refresh();
        return null;
      }
      final body = jsonDecode(resp.body) as Map<String, dynamic>;
      return body['error'] as String? ?? 'Failed to delete game';
    } catch (e) {
      return e.toString();
    }
  }
}

final openGamesProvider =
    StateNotifierProvider<OpenGamesNotifier, AsyncValue<List<Game>>>((ref) {
  return OpenGamesNotifier(ref.watch(apiClientProvider));
});

final myGamesProvider =
    StateNotifierProvider<MyGamesNotifier, AsyncValue<List<Game>>>((ref) {
  return MyGamesNotifier(ref.watch(apiClientProvider));
});

/// Active + waiting games only (excludes finished).
final activeGamesProvider = Provider<AsyncValue<List<Game>>>((ref) {
  return ref.watch(myGamesProvider).whenData(
    (games) => games.where((g) => g.status != 'finished').toList(),
  );
});

/// Fetches all finished games (including botmatch games), with optional search.
class FinishedGamesNotifier extends StateNotifier<AsyncValue<List<Game>>> {
  final ApiClient _api;
  String _search = '';

  FinishedGamesNotifier(this._api) : super(const AsyncValue.loading()) {
    refresh();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    try {
      var path = '/games?filter=finished';
      if (_search.isNotEmpty) {
        path += '&search=${Uri.encodeComponent(_search)}';
      }
      final resp = await _api.get(path);
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Game.fromJson(e as Map<String, dynamic>))
            .toList();
        state = AsyncValue.data(list);
      } else {
        state = AsyncValue.error('Failed to load games', StackTrace.current);
      }
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  /// Updates the search term and re-fetches from the server.
  Future<void> search(String query) async {
    _search = query.trim();
    await refresh();
  }
}

final finishedGamesProvider =
    StateNotifierProvider<FinishedGamesNotifier, AsyncValue<List<Game>>>((ref) {
  return FinishedGamesNotifier(ref.watch(apiClientProvider));
});

/// Finished games only (combines user's finished games + all finished games).
final pastGamesProvider = Provider<AsyncValue<List<Game>>>((ref) {
  return ref.watch(finishedGamesProvider);
});
