import 'package:flutter/material.dart';

import '../../../core/models/phase.dart';
import 'countdown_timer.dart';

/// Top bar showing season/year, phase type, countdown, and ready count.
/// Supports a resolving indicator and phase-flash animation.
class PhaseBar extends StatefulWidget {
  final Phase? phase;
  final int readyCount;
  final VoidCallback? onUrgent;
  final bool isResolving;
  final bool isNewPhase;

  /// Total phase duration for scaling countdown warning thresholds.
  final Duration? totalDuration;

  const PhaseBar({
    super.key,
    this.phase,
    this.readyCount = 0,
    this.onUrgent,
    this.isResolving = false,
    this.isNewPhase = false,
    this.totalDuration,
  });

  @override
  State<PhaseBar> createState() => _PhaseBarState();
}

class _PhaseBarState extends State<PhaseBar>
    with SingleTickerProviderStateMixin {
  late final AnimationController _flashController;
  late Animation<Color?> _flashAnimation;

  @override
  void initState() {
    super.initState();
    _flashController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    );
    _flashAnimation = _buildFlashTween().animate(
      CurvedAnimation(parent: _flashController, curve: Curves.easeOut),
    );

    if (widget.isNewPhase) {
      _flashController.forward(from: 0);
    }
  }

  @override
  void didUpdateWidget(PhaseBar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.isNewPhase && !oldWidget.isNewPhase) {
      _flashAnimation = _buildFlashTween().animate(
        CurvedAnimation(parent: _flashController, curve: Curves.easeOut),
      );
      _flashController.forward(from: 0);
    }
  }

  @override
  void dispose() {
    _flashController.dispose();
    super.dispose();
  }

  ColorTween _buildFlashTween() {
    final accentColor = _phaseAccentColor(widget.phase?.phaseType);
    return ColorTween(begin: accentColor, end: Colors.transparent);
  }

  Color _phaseAccentColor(String? type) {
    return switch (type) {
      'movement' => Colors.blue.withValues(alpha: 0.3),
      'retreat' => Colors.orange.withValues(alpha: 0.3),
      'build' => Colors.green.withValues(alpha: 0.3),
      _ => Colors.grey.withValues(alpha: 0.2),
    };
  }

  Color _phaseBadgeColor(String type) {
    return switch (type) {
      'movement' => Colors.blue.shade600,
      'retreat' => Colors.orange.shade600,
      'build' => Colors.green.shade600,
      _ => Colors.grey,
    };
  }

  @override
  Widget build(BuildContext context) {
    if (widget.phase == null) {
      return const SizedBox(
        height: 48,
        child: Center(child: Text('Loading phase...')),
      );
    }

    final p = widget.phase!;
    final seasonLabel = p.season[0].toUpperCase() + p.season.substring(1);
    final typeLabel = p.phaseType[0].toUpperCase() + p.phaseType.substring(1);

    return AnimatedBuilder(
      animation: _flashController,
      builder: (context, child) {
        return AnimatedContainer(
          duration: const Duration(milliseconds: 300),
          decoration: BoxDecoration(
            color: _flashAnimation.value ??
                Theme.of(context).colorScheme.surfaceContainer,
            border: Border(
              bottom: BorderSide(color: Theme.of(context).dividerColor),
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(
                height: 48,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Row(
                    children: [
                      Text(
                        '$seasonLabel ${p.year}',
                        style: Theme.of(context)
                            .textTheme
                            .titleSmall
                            ?.copyWith(fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 2),
                        decoration: BoxDecoration(
                          color: _phaseBadgeColor(p.phaseType),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Text(
                          typeLabel,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      const Spacer(),
                      if (p.resolvedAt == null) ...[
                        Icon(Icons.people,
                            size: 16,
                            color: Theme.of(context)
                                .colorScheme
                                .onSurfaceVariant),
                        const SizedBox(width: 4),
                        Text('Ready ${widget.readyCount}/7'),
                        const SizedBox(width: 16),
                        CountdownTimer(
                            deadline: p.deadline,
                            totalDuration: widget.totalDuration,
                            onUrgent: widget.onUrgent),
                      ] else
                        const Text('Resolved',
                            style: TextStyle(color: Colors.grey)),
                    ],
                  ),
                ),
              ),
              AnimatedCrossFade(
                firstChild: const SizedBox.shrink(),
                secondChild: Padding(
                  padding:
                      const EdgeInsets.only(left: 16, right: 16, bottom: 8),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        'Resolving orders...',
                        style: TextStyle(
                          fontSize: 12,
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                crossFadeState: widget.isResolving
                    ? CrossFadeState.showSecond
                    : CrossFadeState.showFirst,
                duration: const Duration(milliseconds: 300),
              ),
            ],
          ),
        );
      },
    );
  }
}
