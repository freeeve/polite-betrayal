import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../api/api_client.dart';
import '../api/ws_client.dart';
import '../models/user.dart';
import 'auth_state.dart';

const _keyAccess = 'access_token';
const _keyRefresh = 'refresh_token';

/// Token storage that tries FlutterSecureStorage, falls back to in-memory
/// when Keychain is unavailable (e.g. unsigned macOS builds).
class TokenStorage {
  final FlutterSecureStorage _secure;
  final Map<String, String> _mem = {};
  bool _useMemory = false;

  TokenStorage([this._secure = const FlutterSecureStorage()]);

  Future<String?> read(String key) async {
    if (_useMemory) return _mem[key];
    try {
      return await _secure.read(key: key);
    } catch (_) {
      _useMemory = true;
      return _mem[key];
    }
  }

  Future<void> write(String key, String value) async {
    _mem[key] = value;
    if (_useMemory) return;
    try {
      await _secure.write(key: key, value: value);
    } catch (_) {
      _useMemory = true;
    }
  }

  Future<void> deleteAll() async {
    _mem.clear();
    if (_useMemory) return;
    try {
      await _secure.deleteAll();
    } catch (_) {
      _useMemory = true;
    }
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiClient _api;
  final WSClient wsClient;
  final TokenStorage _storage;

  AuthNotifier(this._api, this.wsClient, this._storage) : super(const AuthState.unauthenticated()) {
    _api.onTokenRefreshed = _onTokenRefreshed;
    _api.onAuthExpired = logout;
    wsClient.onTokenExpired = _refreshTokenForWS;
  }

  /// Try restoring auth from stored tokens on app start.
  Future<void> tryRestore() async {
    final access = await _storage.read(_keyAccess);
    final refresh = await _storage.read(_keyRefresh);
    if (access == null || refresh == null) return;

    _api.setTokens(access, refresh);
    try {
      final resp = await _api.get('/users/me');
      if (resp.statusCode == 200) {
        final user = User.fromJson(jsonDecode(resp.body) as Map<String, dynamic>);
        state = AuthState.authenticated(
          accessToken: access,
          refreshToken: refresh,
          user: user,
        );
        wsClient.connect(state.accessToken!);
        return;
      }
    } catch (_) {}

    // Token restore failed
    _api.clearTokens();
    await _storage.deleteAll();
  }

  /// Perform dev login with the given display name.
  Future<String?> devLogin(String name) async {
    final resp = await _api.devLogin(name);
    if (resp.statusCode != 200) {
      return 'Login failed: ${resp.body}';
    }

    final tokens = jsonDecode(resp.body) as Map<String, dynamic>;
    final accessToken = tokens['access_token'] as String;
    final refreshToken = tokens['refresh_token'] as String;

    _api.setTokens(accessToken, refreshToken);
    await _storage.write(_keyAccess, accessToken);
    await _storage.write(_keyRefresh, refreshToken);

    // Fetch user profile
    final userResp = await _api.get('/users/me');
    if (userResp.statusCode != 200) {
      return 'Failed to fetch user profile';
    }

    final user = User.fromJson(jsonDecode(userResp.body) as Map<String, dynamic>);
    state = AuthState.authenticated(
      accessToken: accessToken,
      refreshToken: refreshToken,
      user: user,
    );
    wsClient.connect(accessToken);
    return null; // success
  }

  Future<void> logout() async {
    _api.clearTokens();
    wsClient.disconnect();
    await _storage.deleteAll();
    state = const AuthState.unauthenticated();
  }

  void _onTokenRefreshed(String access, String refresh) {
    _storage.write(_keyAccess, access);
    _storage.write(_keyRefresh, refresh);
    wsClient.updateToken(access);
    if (state.user != null) {
      state = AuthState.authenticated(
        accessToken: access,
        refreshToken: refresh,
        user: state.user!,
      );
    }
  }

  /// Called by WSClient when a handshake fails. Refreshes the token and
  /// returns the new access token, or null if refresh failed.
  /// Only triggers logout when the server explicitly rejects the refresh
  /// (token truly expired), not on network errors (server unreachable).
  Future<String?> _refreshTokenForWS() async {
    if (await _api.refreshTokens()) {
      return _api.accessToken;
    }
    // refreshTokens returns false for both network errors and genuine
    // token expiry. The ApiClient.onAuthExpired callback handles the
    // genuine expiry case and calls logout() directly. If we reach here
    // without onAuthExpired having fired, it was a network error -- do
    // not logout, just return null so the WS client stops reconnecting
    // until the next poll-driven reconnect picks it up.
    return null;
  }
}

final tokenStorageProvider = Provider<TokenStorage>(
  (ref) => TokenStorage(),
);

final wsClientProvider = Provider<WSClient>((ref) {
  final client = WSClient();
  ref.onDispose(() => client.dispose());
  return client;
});

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(
    ref.watch(apiClientProvider),
    ref.watch(wsClientProvider),
    ref.watch(tokenStorageProvider),
  );
});

/// Tracks WebSocket connection status as a StateNotifier for UI indicators.
/// Listens to the WSClient status stream and exposes the current status
/// synchronously so widgets can immediately show banners when disconnected.
class ConnectionStatusNotifier extends StateNotifier<ConnectionStatus> {
  StreamSubscription<ConnectionStatus>? _sub;

  ConnectionStatusNotifier(WSClient ws) : super(ws.connectionStatus) {
    _sub = ws.statusStream.listen((status) => state = status);
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}

final connectionStatusProvider =
    StateNotifierProvider<ConnectionStatusNotifier, ConnectionStatus>((ref) {
  return ConnectionStatusNotifier(ref.watch(wsClientProvider));
});
