import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/ws_client.dart';
import '../../core/auth/auth_notifier.dart';
import '../../core/models/message.dart';

class MessagesState {
  final List<Message> messages;
  final bool loading;
  final String? error;

  const MessagesState({
    this.messages = const [],
    this.loading = true,
    this.error,
  });

  MessagesState copyWith({
    List<Message>? messages,
    bool? loading,
    String? error,
  }) {
    return MessagesState(
      messages: messages ?? this.messages,
      loading: loading ?? this.loading,
      error: error ?? this.error,
    );
  }
}

class MessagesNotifier extends StateNotifier<MessagesState> {
  final ApiClient _api;
  final WSClient _ws;
  final String gameId;
  StreamSubscription<WSEvent>? _wsSub;

  MessagesNotifier(this._api, this._ws, this.gameId)
      : super(const MessagesState()) {
    _ws.subscribe(gameId);
    _wsSub = _ws.events
        .where((e) => e.gameId == gameId && e.type == 'message')
        .listen(_onMessage);
    load();
  }

  Future<void> load() async {
    state = state.copyWith(loading: true);
    try {
      final resp = await _api.get('/games/$gameId/messages');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Message.fromJson(e as Map<String, dynamic>))
            .toList();
        state = state.copyWith(messages: list, loading: false);
      } else {
        state = state.copyWith(loading: false, error: 'Failed to load messages');
      }
    } catch (e) {
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  void _onMessage(WSEvent event) {
    try {
      final msg = Message.fromJson(event.data);
      state = state.copyWith(
        messages: [...state.messages, msg],
      );
    } catch (_) {
      // Refresh on parse error.
      load();
    }
  }

  Future<bool> sendMessage(String content, {String? recipientId}) async {
    final body = <String, dynamic>{'content': content};
    if (recipientId != null) body['recipient_id'] = recipientId;

    final resp = await _api.post('/games/$gameId/messages', body: body);
    return resp.statusCode == 201 || resp.statusCode == 200;
  }

  @override
  void dispose() {
    _wsSub?.cancel();
    super.dispose();
  }
}

final messagesProvider = StateNotifierProvider.family<MessagesNotifier, MessagesState, String>(
  (ref, gameId) {
    return MessagesNotifier(
      ref.watch(apiClientProvider),
      ref.watch(wsClientProvider),
      gameId,
    );
  },
);
