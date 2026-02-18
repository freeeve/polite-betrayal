import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import 'api_config.dart';

/// WebSocket event from the server.
class WSEvent {
  final String type;
  final String gameId;
  final Map<String, dynamic> data;

  WSEvent({required this.type, required this.gameId, required this.data});

  factory WSEvent.fromJson(Map<String, dynamic> json) {
    return WSEvent(
      type: json['type'] as String,
      gameId: json['game_id'] as String,
      data: json['data'] as Map<String, dynamic>? ?? {},
    );
  }
}

/// Connection status exposed to the UI for displaying connection indicators.
enum ConnectionStatus { connected, disconnected, reconnecting }

/// WebSocket client with auto-reconnect and game subscription management.
class WSClient {
  WebSocketChannel? _channel;
  StreamSubscription? _subscription;
  final _eventController = StreamController<WSEvent>.broadcast();
  final _statusController = StreamController<ConnectionStatus>.broadcast();
  final Set<String> _subscriptions = {};
  final List<Map<String, dynamic>> _pendingMessages = [];

  String? _token;
  Timer? _reconnectTimer;
  int _reconnectAttempts = 0;
  bool _disposed = false;
  bool _connected = false;
  bool _reconnecting = false;
  bool _hasEverConnected = false;
  ConnectionStatus _status = ConnectionStatus.disconnected;

  /// Called when a WS handshake fails (likely expired token).
  /// Should attempt a token refresh and return the new access token,
  /// or null if refresh failed (triggers logout).
  Future<String?> Function()? onTokenExpired;

  Stream<WSEvent> get events => _eventController.stream;
  Stream<ConnectionStatus> get statusStream => _statusController.stream;
  bool get isConnected => _connected;
  ConnectionStatus get connectionStatus => _status;

  void _setStatus(ConnectionStatus status) {
    if (_status == status) return;
    _status = status;
    if (!_statusController.isClosed) {
      _statusController.add(status);
    }
  }

  /// Updates the stored token without reconnecting. Called when the API client
  /// refreshes tokens so the next WS reconnect uses the fresh token.
  void updateToken(String token) {
    _token = token;
  }

  void connect(String token) {
    _token = token;
    _reconnectAttempts = 0;
    _reconnecting = false;
    _hasEverConnected = false;
    _setStatus(ConnectionStatus.reconnecting);
    _doConnect();
  }

  void _doConnect() {
    if (_disposed || _token == null) return;

    _connected = false;
    _reconnecting = false;
    _setStatus(ConnectionStatus.reconnecting);
    _subscription?.cancel();
    _channel?.sink.close();

    try {
      final uri = Uri.parse('$wsUrl?token=$_token');
      _channel = WebSocketChannel.connect(uri);

      _channel!.ready.then((_) {
        if (_disposed) return;
        _connected = true;
        _hasEverConnected = true;
        _reconnectAttempts = 0;
        _setStatus(ConnectionStatus.connected);
        _flushPending();
      }).catchError((_) => _onDisconnected());

      _subscription = _channel!.stream.listen(
        (message) {
          // Receiving a message proves the connection is alive. Mark connected
          // here as a fallback in case the `ready` future didn't complete
          // (observed on some platforms).
          if (!_connected && !_disposed) {
            _connected = true;
            _hasEverConnected = true;
            _reconnectAttempts = 0;
            _setStatus(ConnectionStatus.connected);
            _flushPending();
          }
          _reconnectAttempts = 0;
          try {
            final json = jsonDecode(message as String) as Map<String, dynamic>;
            _eventController.add(WSEvent.fromJson(json));
          } catch (_) {}
        },
        onDone: _onDisconnected,
        onError: (_) => _onDisconnected(),
      );

      // Resubscribe to previously tracked games.
      for (final gameId in _subscriptions) {
        _send({'action': 'subscribe', 'game_id': gameId});
      }
    } catch (_) {
      _onDisconnected();
    }
  }

  void _flushPending() {
    for (final msg in _pendingMessages) {
      _channel?.sink.add(jsonEncode(msg));
    }
    _pendingMessages.clear();
  }

  void _onDisconnected() {
    if (_disposed || _reconnecting) return;
    _reconnecting = true;

    final wasConnected = _connected;
    _channel = null;
    _connected = false;
    _reconnectTimer?.cancel();

    if (wasConnected || _hasEverConnected) {
      _setStatus(ConnectionStatus.disconnected);
    }

    if (!wasConnected && !_hasEverConnected && onTokenExpired != null) {
      // First-ever handshake failed -- likely an expired token.
      // Try to refresh before reconnecting.
      onTokenExpired!().then((newToken) {
        if (_disposed) return;
        if (newToken != null) {
          _token = newToken;
          _reconnectAttempts = 0;
          _doConnect();
        }
        // null means refresh failed -- auth system handles logout,
        // so we stop reconnecting.
      });
      return;
    }

    // Reconnect with backoff. Either we were connected and lost connection,
    // or we previously connected but the server is temporarily down.
    _reconnectAttempts++;
    final delay = Duration(seconds: _backoffSeconds());
    _setStatus(ConnectionStatus.reconnecting);
    _reconnectTimer = Timer(delay, _doConnect);
  }

  int _backoffSeconds() {
    const maxDelay = 30;
    final delay = 1 << _reconnectAttempts;
    return delay > maxDelay ? maxDelay : delay;
  }

  void subscribe(String gameId) {
    _subscriptions.add(gameId);
    _send({'action': 'subscribe', 'game_id': gameId});
  }

  void unsubscribe(String gameId) {
    _subscriptions.remove(gameId);
    _send({'action': 'unsubscribe', 'game_id': gameId});
  }

  void _send(Map<String, dynamic> message) {
    if (_connected && _channel != null) {
      _channel!.sink.add(jsonEncode(message));
    } else {
      _pendingMessages.add(message);
    }
  }

  void disconnect() {
    _reconnectTimer?.cancel();
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close();
    _channel = null;
    _connected = false;
    _reconnecting = false;
    _hasEverConnected = false;
    _subscriptions.clear();
    _pendingMessages.clear();
    _setStatus(ConnectionStatus.disconnected);
  }

  void dispose() {
    _disposed = true;
    disconnect();
    _eventController.close();
    _statusController.close();
  }
}
