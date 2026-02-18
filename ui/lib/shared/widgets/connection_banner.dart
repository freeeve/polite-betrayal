import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/ws_client.dart';
import '../../core/auth/auth_notifier.dart';

/// A persistent banner displayed at the top of the screen when the WebSocket
/// connection to the server is lost. Automatically hides when the connection
/// is restored. Wrapped in a Material widget to ensure proper rendering
/// when used above the router in the MaterialApp builder.
class ConnectionBanner extends ConsumerWidget {
  const ConnectionBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(connectionStatusProvider);
    final authState = ref.watch(authProvider);

    // Only show the banner when the user is authenticated and the connection
    // is not healthy. When unauthenticated, the user is on the login screen
    // and the WS is intentionally disconnected.
    if (!authState.isAuthenticated) return const SizedBox.shrink();
    if (status == ConnectionStatus.connected) return const SizedBox.shrink();

    final isReconnecting = status == ConnectionStatus.reconnecting;
    final message = isReconnecting
        ? 'Reconnecting to server...'
        : 'Server connection lost';

    return Material(
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        color: Colors.red.shade700,
        child: SafeArea(
          bottom: false,
          child: Row(
            children: [
              if (isReconnecting)
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              else
                const Icon(Icons.cloud_off, color: Colors.white, size: 16),
              const SizedBox(width: 10),
              Text(
                message,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
