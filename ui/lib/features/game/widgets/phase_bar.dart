import 'package:flutter/material.dart';

import '../../../core/models/phase.dart';
import 'countdown_timer.dart';

/// Top bar showing season/year, phase type, countdown, and ready count.
class PhaseBar extends StatelessWidget {
  final Phase? phase;
  final int readyCount;

  const PhaseBar({super.key, this.phase, this.readyCount = 0});

  @override
  Widget build(BuildContext context) {
    if (phase == null) {
      return const SizedBox(
        height: 48,
        child: Center(child: Text('Loading phase...')),
      );
    }

    final p = phase!;
    final seasonLabel = p.season[0].toUpperCase() + p.season.substring(1);
    final typeLabel = p.phaseType[0].toUpperCase() + p.phaseType.substring(1);

    return Container(
      height: 48,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(
          bottom: BorderSide(color: Theme.of(context).dividerColor),
        ),
      ),
      child: Row(
        children: [
          Text(
            '$seasonLabel ${p.year}',
            style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.bold),
          ),
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(
              color: _phaseColor(p.phaseType),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Text(
              typeLabel,
              style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w600),
            ),
          ),
          const Spacer(),
          if (p.resolvedAt == null) ...[
            Icon(Icons.people, size: 16, color: Theme.of(context).colorScheme.onSurfaceVariant),
            const SizedBox(width: 4),
            Text('Ready $readyCount/7'),
            const SizedBox(width: 16),
            CountdownTimer(deadline: p.deadline),
          ] else
            const Text('Resolved', style: TextStyle(color: Colors.grey)),
        ],
      ),
    );
  }

  Color _phaseColor(String type) {
    return switch (type) {
      'movement' => Colors.blue.shade600,
      'retreat' => Colors.orange.shade600,
      'build' => Colors.green.shade600,
      _ => Colors.grey,
    };
  }
}
