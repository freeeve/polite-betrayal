import 'package:flutter/material.dart';

import '../../core/models/game.dart';
import 'power_badge.dart';

/// Card showing game name, status chip, and player count/powers.
class GameCard extends StatelessWidget {
  final Game game;
  final VoidCallback? onTap;
  final VoidCallback? onStop;
  final VoidCallback? onDelete;

  const GameCard({super.key, required this.game, this.onTap, this.onStop, this.onDelete});

  static String _timeAgo(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inDays > 30) return '${diff.inDays ~/ 30}mo ago';
    if (diff.inDays > 0) return '${diff.inDays}d ago';
    if (diff.inHours > 0) return '${diff.inHours}h ago';
    return '${diff.inMinutes}m ago';
  }

  @override
  Widget build(BuildContext context) {
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      game.name,
                      style: Theme.of(context).textTheme.titleMedium,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  if (onDelete != null)
                    Padding(
                      padding: const EdgeInsets.only(right: 8),
                      child: SizedBox(
                        height: 28,
                        width: 28,
                        child: IconButton(
                          icon: const Icon(Icons.delete_outline, size: 20),
                          padding: EdgeInsets.zero,
                          tooltip: 'Delete game',
                          color: Colors.red.shade700,
                          onPressed: onDelete,
                        ),
                      ),
                    ),
                  if (onStop != null)
                    Padding(
                      padding: const EdgeInsets.only(right: 8),
                      child: SizedBox(
                        height: 28,
                        width: 28,
                        child: IconButton(
                          icon: const Icon(Icons.stop_circle_outlined, size: 20),
                          padding: EdgeInsets.zero,
                          tooltip: 'Stop game',
                          color: Colors.red.shade700,
                          onPressed: onStop,
                        ),
                      ),
                    ),
                  _StatusChip(status: game.status),
                ],
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  Icon(Icons.people, size: 16, color: Theme.of(context).colorScheme.onSurfaceVariant),
                  const SizedBox(width: 4),
                  Text(
                    '${game.players.length}/7',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(width: 16),
                  ...game.players
                      .where((p) => p.power != null && p.power!.isNotEmpty)
                      .map((p) => Padding(
                            padding: const EdgeInsets.only(right: 4),
                            child: PowerBadge(power: p.power!, size: 22),
                          )),
                ],
              ),
              if (game.status == 'finished') ...[
                const SizedBox(height: 8),
                Row(
                  children: [
                    const Icon(Icons.emoji_events, size: 16, color: Colors.amber),
                    const SizedBox(width: 4),
                    Text(
                      game.winner != null
                          ? 'Winner: ${game.winner![0].toUpperCase()}${game.winner!.substring(1)}'
                          : 'Draw',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                    ),
                    if (game.finishedAt != null) ...[
                      const Spacer(),
                      Text(
                        _timeAgo(game.finishedAt!),
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                              color: Theme.of(context).colorScheme.onSurfaceVariant,
                            ),
                      ),
                    ],
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  final String status;
  const _StatusChip({required this.status});

  @override
  Widget build(BuildContext context) {
    final (Color bg, Color fg) = switch (status) {
      'waiting' => (Colors.orange.shade100, Colors.orange.shade800),
      'active' => (Colors.green.shade100, Colors.green.shade800),
      'finished' => (Colors.grey.shade200, Colors.grey.shade700),
      _ => (Colors.grey.shade200, Colors.grey.shade700),
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        status[0].toUpperCase() + status.substring(1),
        style: TextStyle(color: fg, fontSize: 12, fontWeight: FontWeight.w600),
      ),
    );
  }
}
