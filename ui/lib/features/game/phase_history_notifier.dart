import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/models/game_state.dart';
import '../../core/models/order.dart';
import '../../core/models/phase.dart';

/// State for phase history browsing.
class PhaseHistoryState {
  final List<Phase> phases;
  final int? viewingIndex; // null = viewing current phase
  final GameState? historicalState;
  final GameState? previousHistoricalState; // pre-resolution state for replay animation
  final List<Order> historicalOrders;
  final bool loading;

  const PhaseHistoryState({
    this.phases = const [],
    this.viewingIndex,
    this.historicalState,
    this.previousHistoricalState,
    this.historicalOrders = const [],
    this.loading = false,
  });

  PhaseHistoryState copyWith({
    List<Phase>? phases,
    int? viewingIndex,
    GameState? historicalState,
    GameState? previousHistoricalState,
    List<Order>? historicalOrders,
    bool? loading,
    bool clearViewing = false,
    bool clearPreviousHistorical = false,
  }) {
    return PhaseHistoryState(
      phases: phases ?? this.phases,
      viewingIndex: clearViewing ? null : (viewingIndex ?? this.viewingIndex),
      historicalState: clearViewing ? null : (historicalState ?? this.historicalState),
      previousHistoricalState: clearViewing || clearPreviousHistorical
          ? null : (previousHistoricalState ?? this.previousHistoricalState),
      historicalOrders: clearViewing ? const [] : (historicalOrders ?? this.historicalOrders),
      loading: loading ?? this.loading,
    );
  }
}

class PhaseHistoryNotifier extends StateNotifier<PhaseHistoryState> {
  final ApiClient _api;
  final String gameId;

  PhaseHistoryNotifier(this._api, this.gameId) : super(const PhaseHistoryState());

  Future<void> loadPhases({bool viewLast = false}) async {
    state = state.copyWith(loading: true);
    try {
      final resp = await _api.get('/games/$gameId/phases');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Phase.fromJson(e as Map<String, dynamic>))
            .toList();
        state = state.copyWith(phases: list, loading: false);
        if (viewLast && list.isNotEmpty) {
          viewPhase(list.length - 1);
        }
      } else {
        state = state.copyWith(loading: false);
      }
    } catch (_) {
      state = state.copyWith(loading: false);
    }
  }

  /// View a historical phase by index.
  void viewPhase(int index) {
    if (index < 0 || index >= state.phases.length) return;
    final phase = state.phases[index];
    final gs = GameState.fromJson(phase.stateAfter ?? phase.stateBefore);

    // For resolved phases with stateAfter, set the pre-resolution state for replay animation.
    // Exclude build phases: they don't have movement animations, only build/disband overlays.
    final prevGs = (phase.stateAfter != null && phase.phaseType != 'build')
        ? GameState.fromJson(phase.stateBefore)
        : null;

    state = state.copyWith(
      viewingIndex: index,
      historicalState: gs,
      previousHistoricalState: prevGs,
      historicalOrders: const [],
    );
    if (phase.resolvedAt != null) {
      _fetchHistoricalOrders(phase.id);
    }
  }

  /// Clear the previous historical state after replay animation completes.
  void clearPreviousHistoricalState() {
    state = state.copyWith(clearPreviousHistorical: true);
  }

  Future<void> _fetchHistoricalOrders(String phaseId) async {
    try {
      final resp = await _api.get('/games/$gameId/phases/$phaseId/orders');
      if (resp.statusCode == 200) {
        final list = (jsonDecode(resp.body) as List<dynamic>)
            .map((e) => Order.fromJson(e as Map<String, dynamic>))
            .toList();
        state = state.copyWith(historicalOrders: list);
      }
    } catch (_) {
      // Non-critical: historical orders are a UX enhancement.
    }
  }

  /// Return to viewing the current phase.
  void viewCurrent() {
    state = state.copyWith(clearViewing: true);
  }
}

final phaseHistoryProvider =
    StateNotifierProvider.family<PhaseHistoryNotifier, PhaseHistoryState, String>((ref, gameId) {
  return PhaseHistoryNotifier(ref.watch(apiClientProvider), gameId);
});
