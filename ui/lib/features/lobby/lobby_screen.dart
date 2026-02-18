import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/auth/auth_notifier.dart';
import '../../core/theme/app_theme.dart';
import '../../shared/widgets/power_badge.dart';
import 'lobby_notifier.dart';

class LobbyScreen extends ConsumerWidget {
  final String gameId;
  const LobbyScreen({super.key, required this.gameId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final lobby = ref.watch(lobbyProvider(gameId));
    final auth = ref.watch(authProvider);
    final userId = auth.user?.id;

    return Scaffold(
      appBar: AppBar(title: const Text('Game Lobby')),
      body: lobby.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(child: Text('Error: $err')),
        data: (game) {
          // Navigate to game screen if the game has started.
          if (game.status == 'active') {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              context.go('/game/${game.id}');
            });
            return const Center(child: CircularProgressIndicator());
          }

          final isCreator = game.creatorId == userId;
          final hasJoined = game.players.any((p) => p.userId == userId);
          final canStart = isCreator && game.players.length == 7;
          final allBots = game.players.length == 7 && game.players.every((p) => p.isBot);
          final isManual = game.powerAssignment == 'manual';
          final allPowers = ['austria', 'england', 'france', 'germany', 'italy', 'russia', 'turkey'];
          final takenPowers = <String, String>{};
          for (final p in game.players) {
            if (p.power != null && p.power!.isNotEmpty) {
              takenPowers[p.power!] = p.userId;
            }
          }

          return ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Text(game.name, style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 8),
              Text('Turn: ${game.turnDuration} | Retreat: ${game.retreatDuration} | Build: ${game.buildDuration}'),
              if (isManual) ...[
                const SizedBox(height: 4),
                Text('Power Assignment: Choose in Lobby',
                    style: Theme.of(context).textTheme.bodySmall),
              ],
              const SizedBox(height: 24),
              Text('Players (${game.players.length}/7)',
                  style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 8),
              ...game.players.map((p) {
                final label = p.userId == userId
                    ? 'You'
                    : p.isBot
                        ? 'Bot (${p.botDifficulty[0].toUpperCase()}${p.botDifficulty.substring(1)})'
                        : 'Player';
                final canEditPower = isManual &&
                    game.status == 'waiting' &&
                    ((p.userId == userId && !p.isBot) || (isCreator && p.isBot));
                final availablePowers = allPowers.where((pow) {
                  final owner = takenPowers[pow];
                  return owner == null || owner == p.userId;
                }).toList();

                return Column(
                  children: [
                    ListTile(
                      leading: p.power != null && p.power!.isNotEmpty
                          ? PowerBadge(power: p.power!)
                          : CircleAvatar(
                              child: Icon(p.isBot ? Icons.smart_toy : Icons.person)),
                      title: Text(label),
                      subtitle: p.power != null && p.power!.isNotEmpty
                          ? Text(PowerColors.label(p.power!))
                          : isManual
                              ? const Text('Unassigned')
                              : null,
                      trailing: isCreator && p.isBot
                          ? DropdownButton<String>(
                              value: p.botDifficulty,
                              underline: const SizedBox.shrink(),
                              items: ['random', 'easy', 'medium', 'hard']
                                  .map((d) => DropdownMenuItem(
                                        value: d,
                                        child: Text(d[0].toUpperCase() + d.substring(1)),
                                      ))
                                  .toList(),
                              onChanged: (val) {
                                if (val != null) {
                                  ref.read(lobbyProvider(gameId).notifier).updateBotDifficulty(p.userId, val);
                                }
                              },
                            )
                          : null,
                    ),
                    if (canEditPower)
                      Padding(
                        padding: const EdgeInsets.only(left: 72, right: 16, bottom: 8),
                        child: InputDecorator(
                          decoration: const InputDecoration(
                            labelText: 'Power',
                            border: OutlineInputBorder(),
                            isDense: true,
                          ),
                          child: DropdownButton<String>(
                            value: (p.power != null && p.power!.isNotEmpty) ? p.power : null,
                            hint: const Text('Select power'),
                            underline: const SizedBox.shrink(),
                            isExpanded: true,
                            isDense: true,
                            items: availablePowers
                                .map((pow) => DropdownMenuItem(
                                      value: pow,
                                      child: Row(
                                        children: [
                                          PowerBadge(power: pow, size: 20),
                                          const SizedBox(width: 8),
                                          Text(PowerColors.label(pow)),
                                        ],
                                      ),
                                    ))
                                .toList(),
                            onChanged: (val) {
                              if (val != null) {
                                ref.read(lobbyProvider(gameId).notifier).updatePlayerPower(p.userId, val);
                              }
                            },
                          ),
                        ),
                      ),
                  ],
                );
              }),
              if (game.players.length < 7) ...[
                for (var i = game.players.length; i < 7; i++)
                  ListTile(
                    leading: CircleAvatar(
                      backgroundColor: Theme.of(context).colorScheme.surfaceContainerHighest,
                      child: const Icon(Icons.hourglass_empty),
                    ),
                    title: const Text('Waiting for player...'),
                  ),
              ],
              const SizedBox(height: 24),
              if (!hasJoined && !allBots)
                FilledButton(
                  onPressed: () async {
                    final err = await ref.read(lobbyProvider(gameId).notifier).joinGame();
                    if (err != null && context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(err)));
                    }
                  },
                  child: const Text('Join Game'),
                ),
              if (canStart)
                FilledButton(
                  onPressed: () async {
                    if (isManual) {
                      final unassigned = game.players.where(
                        (p) => p.power == null || p.power!.isEmpty,
                      ).length;
                      if (unassigned > 0) {
                        final confirmed = await showDialog<bool>(
                          context: context,
                          builder: (ctx) => AlertDialog(
                            title: const Text('Unassigned Powers'),
                            content: Text('$unassigned player(s) will be randomly assigned. Continue?'),
                            actions: [
                              TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
                              FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Start')),
                            ],
                          ),
                        );
                        if (confirmed != true) return;
                      }
                    }
                    final err = await ref.read(lobbyProvider(gameId).notifier).startGame();
                    if (err != null && context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(err)));
                    }
                  },
                  child: const Text('Start Game'),
                ),
              if (hasJoined && !canStart)
                const Center(
                  child: Padding(
                    padding: EdgeInsets.all(16),
                    child: Text('Waiting for more players...'),
                  ),
                ),
            ],
          );
        },
      ),
    );
  }
}
