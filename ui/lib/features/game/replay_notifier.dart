import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'phase_history_notifier.dart';

/// State for the game replay feature.
class ReplayState {
  final bool isPlaying;
  final double speed; // seconds per phase
  final int currentIndex;
  final int totalPhases;

  const ReplayState({
    this.isPlaying = false,
    this.speed = 2.0,
    this.currentIndex = 0,
    this.totalPhases = 0,
  });

  ReplayState copyWith({
    bool? isPlaying,
    double? speed,
    int? currentIndex,
    int? totalPhases,
  }) {
    return ReplayState(
      isPlaying: isPlaying ?? this.isPlaying,
      speed: speed ?? this.speed,
      currentIndex: currentIndex ?? this.currentIndex,
      totalPhases: totalPhases ?? this.totalPhases,
    );
  }
}

/// Manages auto-advancing replay through phase history.
class ReplayNotifier extends StateNotifier<ReplayState> {
  final PhaseHistoryNotifier _historyNotifier;
  final PhaseHistoryState Function() _readHistoryState;
  Timer? _timer;

  ReplayNotifier(this._historyNotifier, this._readHistoryState)
      : super(const ReplayState());

  /// Start or resume replay from the current index.
  void play() {
    final historyState = _readHistoryState();
    final total = historyState.phases.length;
    if (total == 0) return;

    var startIndex = state.currentIndex;
    // If we're at the end, restart from the beginning.
    if (startIndex >= total - 1) {
      startIndex = 0;
    }

    state = state.copyWith(
      isPlaying: true,
      currentIndex: startIndex,
      totalPhases: total,
    );
    _viewPhaseAndSchedule(startIndex);
  }

  /// Pause replay at the current phase.
  void pause() {
    _cancelTimer();
    state = state.copyWith(isPlaying: false);
  }

  /// Stop replay and return to the live/current view.
  /// For finished games, navigate to the last phase instead of clearing the
  /// view, since there is no active phase and clearing would leave no state.
  void stop({bool isFinished = false}) {
    _cancelTimer();
    if (isFinished) {
      final historyState = _readHistoryState();
      if (historyState.phases.isNotEmpty) {
        _historyNotifier.viewPhase(historyState.phases.length - 1);
      }
    } else {
      _historyNotifier.viewCurrent();
    }
    state = const ReplayState();
  }

  /// Update replay speed in seconds per phase (clamped to 0.5-10.0).
  void setSpeed(double seconds) {
    final clamped = seconds.clamp(0.5, 10.0);
    state = state.copyWith(speed: clamped);
  }

  /// Jump to a specific phase index.
  void seekTo(int index) {
    final historyState = _readHistoryState();
    final total = historyState.phases.length;
    if (total == 0) return;

    final clamped = index.clamp(0, total - 1);
    state = state.copyWith(currentIndex: clamped, totalPhases: total);
    _historyNotifier.viewPhase(clamped);
  }

  /// Called by the game screen when the map animation completes during replay.
  /// Schedules the next advance after the configured delay.
  void notifyAnimationComplete() {
    if (!state.isPlaying) return;
    _scheduleAdvance();
  }

  /// Views the phase and schedules the next advance. For phases without
  /// animation (no stateAfter), the delay starts immediately. For animated
  /// phases, the delay starts after [notifyAnimationComplete] is called.
  ///
  /// Build phases use a build/disband overlay animation instead of movement
  /// animation, so notifyAnimationComplete is called from that callback.
  /// Retreat phases with no movement (all disbands / empty orders) also skip
  /// the movement animation -- we detect this by checking whether viewPhase
  /// actually set a previousHistoricalState for the movement animation.
  void _viewPhaseAndSchedule(int index) {
    _historyNotifier.viewPhase(index);

    final historyState = _readHistoryState();
    final phase = historyState.phases[index];
    final hasMovementAnimation = phase.stateAfter != null
        && phase.phaseType != 'build'
        && historyState.previousHistoricalState != null;
    if (!hasMovementAnimation) {
      _scheduleAdvance();
    }
    // Otherwise wait for notifyAnimationComplete() from the map widget.
  }

  void _scheduleAdvance() {
    _cancelTimer();
    _timer = Timer(
      Duration(milliseconds: (state.speed * 1000).round()),
      _advance,
    );
  }

  void _advance() {
    final historyState = _readHistoryState();
    final total = historyState.phases.length;
    final next = state.currentIndex + 1;

    if (next >= total) {
      // Reached the end -- pause on the last phase instead of clearing the map.
      pause();
      return;
    }

    state = state.copyWith(currentIndex: next, totalPhases: total);
    _viewPhaseAndSchedule(next);
  }

  void _cancelTimer() {
    _timer?.cancel();
    _timer = null;
  }

  @override
  void dispose() {
    _cancelTimer();
    super.dispose();
  }
}

/// Family provider keyed by gameId.
final replayProvider =
    StateNotifierProvider.family<ReplayNotifier, ReplayState, String>(
        (ref, gameId) {
  final historyNotifier = ref.watch(phaseHistoryProvider(gameId).notifier);
  // Provide a closure that reads the current history state snapshot.
  PhaseHistoryState readState() => ref.read(phaseHistoryProvider(gameId));
  return ReplayNotifier(historyNotifier, readState);
});
