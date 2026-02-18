import '../models/user.dart';

enum AuthStatus { unauthenticated, authenticated }

class AuthState {
  final AuthStatus status;
  final String? accessToken;
  final String? refreshToken;
  final User? user;

  const AuthState({
    this.status = AuthStatus.unauthenticated,
    this.accessToken,
    this.refreshToken,
    this.user,
  });

  const AuthState.unauthenticated()
      : status = AuthStatus.unauthenticated,
        accessToken = null,
        refreshToken = null,
        user = null;

  AuthState.authenticated({
    required String this.accessToken,
    required String this.refreshToken,
    required User this.user,
  }) : status = AuthStatus.authenticated;

  bool get isAuthenticated => status == AuthStatus.authenticated;

  AuthState copyWith({
    AuthStatus? status,
    String? accessToken,
    String? refreshToken,
    User? user,
  }) {
    return AuthState(
      status: status ?? this.status,
      accessToken: accessToken ?? this.accessToken,
      refreshToken: refreshToken ?? this.refreshToken,
      user: user ?? this.user,
    );
  }
}
