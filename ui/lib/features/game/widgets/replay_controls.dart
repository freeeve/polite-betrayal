import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../phase_history_notifier.dart';
import '../replay_notifier.dart';

/// Compact replay controls for the phase history panel.
class ReplayControls extends ConsumerWidget {
  final String gameId;
  const ReplayControls({super.key, required this.gameId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final replay = ref.watch(replayProvider(gameId));
    final notifier = ref.read(replayProvider(gameId).notifier);
    final historyState = ref.watch(phaseHistoryProvider(gameId));
    final theme = Theme.of(context);

    // Build season/phase label from the current replay phase.
    String phaseLabel;
    if (replay.totalPhases > 0 &&
        replay.currentIndex < historyState.phases.length) {
      final phase = historyState.phases[replay.currentIndex];
      final season =
          phase.season[0].toUpperCase() + phase.season.substring(1);
      final phaseType =
          phase.phaseType[0].toUpperCase() + phase.phaseType.substring(1);
      phaseLabel = '$season ${phase.year} \u2014 $phaseType';
    } else {
      phaseLabel = 'No phases';
    }

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Season, year, and phase type header.
            Text(
              phaseLabel,
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            if (replay.totalPhases > 0)
              Text(
                'Phase ${replay.currentIndex + 1} / ${replay.totalPhases}',
                style: theme.textTheme.bodySmall,
              ),
            const SizedBox(height: 4),

            // Phase seek slider.
            if (replay.totalPhases > 1)
              SliderTheme(
                data: SliderTheme.of(context).copyWith(
                  trackHeight: 3,
                  thumbShape:
                      const RoundSliderThumbShape(enabledThumbRadius: 6),
                ),
                child: Slider(
                  value: replay.currentIndex.toDouble(),
                  min: 0,
                  max: (replay.totalPhases - 1).toDouble(),
                  divisions:
                      replay.totalPhases > 1 ? replay.totalPhases - 1 : null,
                  onChanged: (v) => notifier.seekTo(v.round()),
                ),
              ),

            // Transport controls row.
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                // Stop button.
                IconButton(
                  icon: const Icon(Icons.stop),
                  tooltip: 'Stop replay',
                  iconSize: 20,
                  onPressed: () => notifier.stop(),
                ),
                // Play / Pause button.
                IconButton(
                  icon: Icon(replay.isPlaying ? Icons.pause : Icons.play_arrow),
                  tooltip: replay.isPlaying ? 'Pause' : 'Play',
                  iconSize: 28,
                  onPressed: () {
                    if (replay.isPlaying) {
                      notifier.pause();
                    } else {
                      notifier.play();
                    }
                  },
                ),
              ],
            ),

            // Speed slider.
            Row(
              children: [
                Text('Speed', style: theme.textTheme.bodySmall),
                Expanded(
                  child: Slider(
                    value: replay.speed,
                    min: 0.5,
                    max: 10.0,
                    divisions: 19,
                    label: '${replay.speed.toStringAsFixed(1)}s',
                    onChanged: (v) => notifier.setSpeed(v),
                  ),
                ),
                SizedBox(
                  width: 56,
                  child: Text(
                    '${replay.speed.toStringAsFixed(1)}s',
                    style: theme.textTheme.bodySmall,
                    textAlign: TextAlign.end,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
