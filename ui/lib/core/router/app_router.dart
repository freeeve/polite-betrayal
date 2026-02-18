import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../auth/auth_notifier.dart';
import '../auth/auth_state.dart';
import '../../features/auth/login_screen.dart';
import '../../features/home/home_screen.dart';
import '../../features/lobby/create_game_screen.dart';
import '../../features/lobby/lobby_screen.dart';
import '../../features/game/game_screen.dart';
import '../../features/messages/messages_screen.dart';
import '../../features/profile/profile_screen.dart';

/// Listenable that triggers router redirect re-evaluation when auth state changes.
class _AuthRefreshListenable extends ChangeNotifier {
  _AuthRefreshListenable(Ref ref) {
    ref.listen<AuthState>(authProvider, (prev, next) => notifyListeners());
  }
}

final routerProvider = Provider<GoRouter>((ref) {
  final refreshListenable = _AuthRefreshListenable(ref);

  return GoRouter(
    initialLocation: '/home',
    refreshListenable: refreshListenable,
    redirect: (context, state) {
      // Read current auth state at redirect time (not captured at router creation).
      final container = ProviderScope.containerOf(context);
      final isAuthenticated = container.read(authProvider).isAuthenticated;
      final isLoginRoute = state.matchedLocation == '/login';

      if (!isAuthenticated && !isLoginRoute) return '/login';
      if (isAuthenticated && isLoginRoute) return '/home';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        builder: (context, state) => const LoginScreen(),
      ),
      GoRoute(
        path: '/home',
        builder: (context, state) => const HomeScreen(),
      ),
      GoRoute(
        path: '/profile',
        builder: (context, state) => const ProfileScreen(),
      ),
      GoRoute(
        path: '/game/create',
        builder: (context, state) => const CreateGameScreen(),
      ),
      GoRoute(
        path: '/game/:id',
        builder: (context, state) => GameScreen(
          gameId: state.pathParameters['id']!,
        ),
      ),
      GoRoute(
        path: '/game/:id/lobby',
        builder: (context, state) => LobbyScreen(
          gameId: state.pathParameters['id']!,
        ),
      ),
      GoRoute(
        path: '/game/:id/messages',
        builder: (context, state) => MessagesScreen(
          gameId: state.pathParameters['id']!,
        ),
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      body: Center(child: Text('Page not found: ${state.error}')),
    ),
  );
});
