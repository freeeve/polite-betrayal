import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'api_config.dart';

/// HTTP client wrapper with JWT auth header injection and auto-refresh on 401.
class ApiClient {
  final http.Client _client = http.Client();

  String? _accessToken;
  String? _refreshToken;

  /// Callback to persist new tokens after a refresh.
  void Function(String accessToken, String refreshToken)? onTokenRefreshed;

  /// Callback invoked when tokens are expired and refresh fails.
  void Function()? onAuthExpired;

  void setTokens(String access, String refresh) {
    _accessToken = access;
    _refreshToken = refresh;
  }

  void clearTokens() {
    _accessToken = null;
    _refreshToken = null;
  }

  bool get hasTokens => _accessToken != null;

  String? get accessToken => _accessToken;

  /// Public wrapper for token refresh. Returns true if successful.
  Future<bool> refreshTokens() => _tryRefresh();

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (_accessToken != null) 'Authorization': 'Bearer $_accessToken',
      };

  Future<http.Response> get(String path) async {
    final resp = await _client.get(Uri.parse('$baseUrl$path'), headers: _headers);
    if (resp.statusCode == 401 && await _tryRefresh()) {
      return _client.get(Uri.parse('$baseUrl$path'), headers: _headers);
    }
    return resp;
  }

  Future<http.Response> post(String path, {Object? body}) async {
    final encoded = body != null ? jsonEncode(body) : null;
    final resp = await _client.post(Uri.parse('$baseUrl$path'), headers: _headers, body: encoded);
    if (resp.statusCode == 401 && await _tryRefresh()) {
      return _client.post(Uri.parse('$baseUrl$path'), headers: _headers, body: encoded);
    }
    return resp;
  }

  Future<http.Response> patch(String path, {Object? body}) async {
    final encoded = body != null ? jsonEncode(body) : null;
    final resp = await _client.patch(Uri.parse('$baseUrl$path'), headers: _headers, body: encoded);
    if (resp.statusCode == 401 && await _tryRefresh()) {
      return _client.patch(Uri.parse('$baseUrl$path'), headers: _headers, body: encoded);
    }
    return resp;
  }

  Future<http.Response> delete(String path) async {
    final resp = await _client.delete(Uri.parse('$baseUrl$path'), headers: _headers);
    if (resp.statusCode == 401 && await _tryRefresh()) {
      return _client.delete(Uri.parse('$baseUrl$path'), headers: _headers);
    }
    return resp;
  }

  /// Perform dev login (outside the normal API path).
  Future<http.Response> devLogin(String name) {
    return _client.get(
      Uri.parse('$authUrl/dev?name=${Uri.encodeComponent(name)}'),
      headers: {'Content-Type': 'application/json'},
    );
  }

  /// Attempt a token refresh. Returns true if successful.
  /// Only calls onAuthExpired for genuine auth failures (server returned non-200),
  /// not for network errors (server unreachable).
  Future<bool> _tryRefresh() async {
    if (_refreshToken == null) {
      onAuthExpired?.call();
      return false;
    }
    try {
      final resp = await _client.post(
        Uri.parse('$authUrl/refresh'),
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode({'refresh_token': _refreshToken}),
      );
      if (resp.statusCode == 200) {
        final data = jsonDecode(resp.body) as Map<String, dynamic>;
        _accessToken = data['access_token'] as String;
        _refreshToken = data['refresh_token'] as String;
        onTokenRefreshed?.call(_accessToken!, _refreshToken!);
        return true;
      }
      // Server responded but rejected the refresh — tokens are truly expired.
      onAuthExpired?.call();
      return false;
    } catch (_) {
      // Network error (server down) — don't log out, just fail silently.
      return false;
    }
  }
}

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());
