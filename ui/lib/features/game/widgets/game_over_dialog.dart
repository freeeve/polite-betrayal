import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_theme.dart';

/// Dialog shown when the game ends.
class GameOverDialog extends StatelessWidget {
  final String? winner;

  const GameOverDialog({super.key, this.winner});

  static Future<void> show(BuildContext context, {String? winner}) {
    return showDialog(
      context: context,
      barrierDismissible: true,
      builder: (_) => GameOverDialog(winner: winner),
    );
  }

  @override
  Widget build(BuildContext context) {
    final hasWinner = winner != null && winner!.isNotEmpty;

    return AlertDialog(
      title: const Text('Game Over'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (hasWinner) ...[
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: PowerColors.forPower(winner!),
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Text(
                winner![0].toUpperCase(),
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 24,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
            const SizedBox(height: 16),
            Text(
              '${PowerColors.label(winner!)} wins!',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
          ] else
            const Text('The game has ended.'),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () {
            Navigator.of(context).pop();
            GoRouter.of(context).go('/home');
          },
          child: const Text('Back to Games'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Continue Viewing'),
        ),
      ],
    );
  }
}
