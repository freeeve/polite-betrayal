import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/auth/auth_notifier.dart';
import '../../core/models/game.dart';
import '../../shared/widgets/game_card.dart';
import 'game_list_notifier.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Polite Betrayal'),
          actions: [
            IconButton(
              icon: const Icon(Icons.person),
              onPressed: () => context.push('/profile'),
            ),
          ],
          bottom: const TabBar(
            tabs: [
              Tab(text: 'Open Games'),
              Tab(text: 'My Games'),
              Tab(text: 'Past Games'),
            ],
          ),
        ),
        body: TabBarView(
          children: [
            _GameListTab(
              provider: openGamesProvider,
              onRefresh: () => ref.read(openGamesProvider.notifier).refresh(),
              emptyMessage: 'No open games. Create one!',
              onTap: (game) => context.push('/game/${game.id}/lobby'),
            ),
            _MyGamesTab(ref: ref),
            _GameListTab(
              provider: pastGamesProvider,
              onRefresh: () => ref.read(myGamesProvider.notifier).refresh(),
              emptyMessage: 'No completed games yet.',
              onTap: (game) => context.push('/game/${game.id}'),
            ),
          ],
        ),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: () => context.push('/game/create'),
          icon: const Icon(Icons.add),
          label: const Text('New Game'),
        ),
      ),
    );
  }
}

class _MyGamesTab extends ConsumerWidget {
  final WidgetRef ref;
  const _MyGamesTab({required this.ref});

  void _confirmDelete(BuildContext context, Game game) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete Game?'),
        content: Text('Delete "${game.name}"? This cannot be undone.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              ref.read(myGamesProvider.notifier).deleteGame(game.id).then((err) {
                if (err != null && context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('Failed to delete game: $err')),
                  );
                }
              });
            },
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
  }

  void _confirmStop(BuildContext context, Game game) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Stop Game?'),
        content: Text('End "${game.name}" as a draw? This cannot be undone.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              ref.read(myGamesProvider.notifier).stopGame(game.id).then((err) {
                if (err != null && context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('Failed to stop game: $err')),
                  );
                }
              });
            },
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            child: const Text('Stop'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final games = ref.watch(activeGamesProvider);
    final userId = ref.watch(authProvider).user?.id;

    return games.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Error: $err'),
            const SizedBox(height: 8),
            FilledButton(
              onPressed: () => ref.read(myGamesProvider.notifier).refresh(),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (list) {
        if (list.isEmpty) {
          return const Center(child: Text('No active games.'));
        }
        return RefreshIndicator(
          onRefresh: () => ref.read(myGamesProvider.notifier).refresh(),
          child: ListView.builder(
            padding: const EdgeInsets.all(12),
            itemCount: list.length,
            itemBuilder: (context, i) {
              final game = list[i];
              final isCreator = game.creatorId == userId;
              final canStop = game.status == 'active' && isCreator;
              final canDelete = game.status == 'waiting' && isCreator;
              return GameCard(
                game: game,
                onTap: () {
                  if (game.status == 'waiting') {
                    context.push('/game/${game.id}/lobby');
                  } else {
                    context.push('/game/${game.id}');
                  }
                },
                onStop: canStop ? () => _confirmStop(context, game) : null,
                onDelete: canDelete ? () => _confirmDelete(context, game) : null,
              );
            },
          ),
        );
      },
    );
  }
}

class _GameListTab extends ConsumerWidget {
  final ProviderListenable<AsyncValue<List<Game>>> provider;
  final Future<void> Function() onRefresh;
  final String emptyMessage;
  final void Function(Game) onTap;

  const _GameListTab({
    required this.provider,
    required this.onRefresh,
    required this.emptyMessage,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final games = ref.watch(provider);

    return games.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Error: $err'),
            const SizedBox(height: 8),
            FilledButton(onPressed: onRefresh, child: const Text('Retry')),
          ],
        ),
      ),
      data: (list) {
        if (list.isEmpty) {
          return Center(child: Text(emptyMessage));
        }
        return RefreshIndicator(
          onRefresh: onRefresh,
          child: ListView.builder(
            padding: const EdgeInsets.all(12),
            itemCount: list.length,
            itemBuilder: (context, i) => GameCard(
              game: list[i],
              onTap: () => onTap(list[i]),
            ),
          ),
        );
      },
    );
  }
}
