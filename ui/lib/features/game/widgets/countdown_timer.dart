import 'dart:async';

import 'package:flutter/material.dart';

/// Countdown timer widget that computes remaining time from a deadline.
class CountdownTimer extends StatefulWidget {
  final DateTime deadline;

  /// Total phase duration for scaling warning thresholds.
  /// When null, falls back to fixed thresholds for backward compatibility.
  final Duration? totalDuration;

  /// Fires once when remaining time crosses the urgent threshold.
  final VoidCallback? onUrgent;

  const CountdownTimer({
    super.key,
    required this.deadline,
    this.totalDuration,
    this.onUrgent,
  });

  @override
  State<CountdownTimer> createState() => _CountdownTimerState();
}

class _CountdownTimerState extends State<CountdownTimer> {
  Timer? _timer;
  Duration _remaining = Duration.zero;
  bool _firedUrgent = false;

  /// 15% of total duration (floor 10s), or 15s fallback.
  Duration get _urgentThreshold {
    if (widget.totalDuration != null) {
      final scaled = widget.totalDuration! * 0.15;
      return scaled < const Duration(seconds: 10)
          ? const Duration(seconds: 10)
          : scaled;
    }
    return const Duration(seconds: 15);
  }

  /// 33% of total duration (floor 30s), or 2m fallback.
  Duration get _warningThreshold {
    if (widget.totalDuration != null) {
      final scaled = widget.totalDuration! * 0.33;
      return scaled < const Duration(seconds: 30)
          ? const Duration(seconds: 30)
          : scaled;
    }
    return const Duration(minutes: 2);
  }

  @override
  void initState() {
    super.initState();
    _update();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) => _update());
  }

  @override
  void didUpdateWidget(CountdownTimer old) {
    super.didUpdateWidget(old);
    if (old.deadline != widget.deadline) {
      _firedUrgent = false;
      _update();
    }
  }

  void _update() {
    final remaining = widget.deadline.difference(DateTime.now());
    setState(() => _remaining = remaining.isNegative ? Duration.zero : remaining);
    if (!_firedUrgent
        && _remaining <= _urgentThreshold
        && _remaining > Duration.zero
        && widget.onUrgent != null) {
      _firedUrgent = true;
      widget.onUrgent!();
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final hours = _remaining.inHours;
    final minutes = _remaining.inMinutes.remainder(60);
    final seconds = _remaining.inSeconds.remainder(60);

    final text = hours > 0
        ? '${hours.toString().padLeft(2, '0')}:${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}'
        : '${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}';

    final isUrgent = _remaining <= _warningThreshold && _remaining > Duration.zero;

    return Text(
      text,
      style: TextStyle(
        fontWeight: FontWeight.bold,
        fontSize: 16,
        color: _remaining == Duration.zero
            ? Colors.grey
            : isUrgent
                ? Colors.red
                : null,
        fontFeatures: const [FontFeature.tabularFigures()],
      ),
    );
  }
}
