import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../game_notifier.dart';
import '../phase_history_notifier.dart';
import '../replay_notifier.dart';
import 'replay_controls.dart';

/// Phase history content — used both as a persistent side panel and inside a Drawer.
class PhaseHistoryPanel extends ConsumerStatefulWidget {
  final String gameId;
  final bool inDrawer;
  const PhaseHistoryPanel({super.key, required this.gameId, this.inDrawer = false});

  @override
  ConsumerState<PhaseHistoryPanel> createState() => _PhaseHistoryPanelState();
}

class _PhaseHistoryPanelState extends ConsumerState<PhaseHistoryPanel> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() =>
        ref.read(phaseHistoryProvider(widget.gameId).notifier).loadPhases());
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(phaseHistoryProvider(widget.gameId));
    final replay = ref.watch(replayProvider(widget.gameId));
    final replayActive = replay.isPlaying || replay.totalPhases > 0;
    final gameState = ref.watch(gameProvider(widget.gameId));
    final isFinished = gameState.game?.status == 'finished';

    return SafeArea(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Text(
              'Phase History',
              style: Theme.of(context).textTheme.titleLarge,
            ),
          ),
          // Replay controls or "Watch Replay" button.
          if (!state.loading && state.phases.length > 1)
            replayActive
                ? ReplayControls(gameId: widget.gameId)
                : Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        icon: const Icon(Icons.replay, size: 18),
                        label: const Text('Watch Replay'),
                        onPressed: () {
                          ref
                              .read(replayProvider(widget.gameId).notifier)
                              .play();
                        },
                      ),
                    ),
                  ),
          if (state.loading)
            const Center(child: CircularProgressIndicator())
          else
            Expanded(
              child: ListView.builder(
                itemCount: isFinished
                    ? state.phases.length
                    : state.phases.length + 1,
                itemBuilder: (context, i) {
                  // Hide "Current Phase" item for finished games since
                  // there is no active phase -- the final historical phase
                  // is the canonical end state.
                  if (!isFinished && i == 0) {
                    return ListTile(
                      leading: const Icon(Icons.play_arrow),
                      title: const Text('Current Phase'),
                      selected: state.viewingIndex == null,
                      onTap: () {
                        ref
                            .read(phaseHistoryProvider(widget.gameId).notifier)
                            .viewCurrent();
                        if (widget.inDrawer) Navigator.of(context).pop();
                      },
                    );
                  }

                  final idx = isFinished ? i : i - 1;
                  final phase = state.phases[idx];
                  final season = phase.season[0].toUpperCase() +
                      phase.season.substring(1);
                  return ListTile(
                    leading: Icon(
                      phase.resolvedAt != null
                          ? Icons.check_circle
                          : Icons.hourglass_empty,
                      color: phase.resolvedAt != null
                          ? Colors.green
                          : Colors.orange,
                    ),
                    title: Text('$season ${phase.year} ${phase.phaseType}'),
                    selected: state.viewingIndex == idx,
                    onTap: () {
                      ref
                          .read(phaseHistoryProvider(widget.gameId).notifier)
                          .viewPhase(idx);
                      if (widget.inDrawer) Navigator.of(context).pop();
                    },
                  );
                },
              ),
            ),
        ],
      ),
    );
  }
}

/// Wraps PhaseHistoryPanel in a Drawer for narrow screens.
class PhaseHistory extends StatelessWidget {
  final String gameId;
  const PhaseHistory({super.key, required this.gameId});

  @override
  Widget build(BuildContext context) {
    return Drawer(
      child: PhaseHistoryPanel(gameId: gameId, inDrawer: true),
    );
  }
}
