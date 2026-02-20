import 'package:flutter/material.dart';

import '../../../core/models/game.dart';
import '../../../core/models/game_state.dart';
import '../../../core/theme/app_theme.dart';

/// Compact per-power supply center count, unit count, and bot difficulty.
class SupplyCenterTable extends StatelessWidget {
  final GameState gameState;
  final List<GamePlayer> players;

  const SupplyCenterTable({
    super.key,
    required this.gameState,
    this.players = const [],
  });

  String? _botDifficulty(String power) {
    for (final p in players) {
      if (p.power == power && p.isBot) return p.botDifficulty;
    }
    return null;
  }

  static const _difficultyAbbrev = {
    'random': 'R',
    'easy': 'E',
    'medium': 'M',
    'hard': 'H',
    'rust': 'Rs',
  };

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(
          top: BorderSide(color: Theme.of(context).dividerColor),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: allPowers.map((power) {
          final sc = gameState.supplyCenterCount(power);
          final units = gameState.unitsOf(power).length;
          if (sc == 0 && units == 0) return const SizedBox.shrink();

          final diff = _botDifficulty(power);

          return Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 20,
                  height: 20,
                  decoration: BoxDecoration(
                    color: PowerColors.forPower(power),
                    shape: BoxShape.circle,
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    power[0].toUpperCase(),
                    style: const TextStyle(color: Colors.white, fontSize: 10, fontWeight: FontWeight.bold),
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  '$sc/$units',
                  style: const TextStyle(fontSize: 10),
                ),
                if (diff != null)
                  Text(
                    _difficultyAbbrev[diff] ?? diff,
                    style: TextStyle(
                      fontSize: 8,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
              ],
            ),
          );
        }).toList(),
      ),
    );
  }
}
