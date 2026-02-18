import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/auth/auth_notifier.dart';
import '../../core/map/adjacency_data.dart';
import '../../core/models/game_state.dart';
import '../../core/theme/app_theme.dart';
import 'game_notifier.dart';
import 'phase_history_notifier.dart';
import 'order_input/build_order_panel.dart';
import 'order_input/coast_picker.dart';
import 'order_input/order_action_bar.dart';
import 'order_input/order_input_notifier.dart';
import 'order_input/order_state.dart';
import 'order_input/pending_orders_list.dart';
import 'order_input/retreat_order_panel.dart';
import 'widgets/game_map.dart';
import 'widgets/game_over_dialog.dart';
import 'widgets/map_painter.dart';
import 'widgets/phase_bar.dart';
import 'widgets/phase_results.dart';
import 'widgets/phase_history.dart';
import 'widgets/supply_center_table.dart';
import 'replay_notifier.dart';

class GameScreen extends ConsumerStatefulWidget {
  final String gameId;
  const GameScreen({super.key, required this.gameId});

  @override
  ConsumerState<GameScreen> createState() => _GameScreenState();
}

class _GameScreenState extends ConsumerState<GameScreen> {
  late final OrderInputNotifier _orderNotifier;
  OrderInputState _orderState = const OrderInputState();
  String? _currentPhaseId;

  bool _gameOverShown = false;
  bool _showResults = false;
  bool _autoViewedLastPhase = false;
  Set<String> _newUnitProvinces = {};
  Set<String>? _previousPhaseUnitProvinces;

  /// Build/disband animation state.
  Set<String> _buildAnimProvinces = {};
  Set<String> _disbandAnimProvinces = {};
  List<GameUnit> _disbandAnimUnits = [];
  List<GameUnit>? _previousPhaseUnits;
  String? _previousPhaseType;
  bool _isInitialLoad = true;

  /// Tracks the last history viewing index to detect phase changes during replay.
  int? _lastHistoryViewingIndex;

  @override
  void initState() {
    super.initState();
    _orderNotifier = OrderInputNotifier();
    _orderNotifier.addListener((state) {
      if (mounted) setState(() => _orderState = state);

      // Show coast picker when needed.
      if (state.phase == OrderPhase.awaitingCoast && state.target != null && state.selectedUnit != null) {
        final coasts = reachableCoasts(
          state.selectedUnit!.province, state.selectedUnit!.coast, state.target!,
        );
        if (coasts.isNotEmpty) {
          CoastPicker.show(
            context,
            province: state.target!,
            coasts: coasts,
            onSelect: (coast) => _orderNotifier.selectCoast(coast),
          );
        }
      }
    });
  }

  @override
  void dispose() {
    _orderNotifier.dispose();
    super.dispose();
  }

  void _updateNotifierState(GameViewState gameViewState) {
    final auth = ref.read(authProvider);
    final myPower = gameViewState.powerForUser(auth.user?.id ?? '');

    // Reset order input and history view when phase changes (e.g. deadline expired).
    final phaseId = gameViewState.currentPhase?.id;
    if (phaseId != null && phaseId != _currentPhaseId) {
      if (_currentPhaseId != null) {
        // Detect newly built units by comparing unit provinces from the
        // start of the previous phase against the current state.
        // We use _previousPhaseUnitProvinces (snapshotted when we first
        // observe each phase) rather than _orderNotifier.gameState, because
        // the notifier's game state can silently advance to the post-build
        // state (via stateAfter on a resolved phase) before the phase ID
        // changes, making a live diff always empty.
        final newState = gameViewState.gameState;
        if (_previousPhaseUnitProvinces != null && newState != null) {
          final newProvinces = {for (final u in newState.units) u.province};
          // Detect build phases: either we directly observed the build phase
          // (_previousPhaseType == 'build'), OR the build phase was skipped
          // (bot games resolve instantly) and we detect it by checking if
          // units were added/removed AND we're now in Spring (build phase
          // happens between Fall and Spring).
          final wasBuildPhase = _previousPhaseType == 'build';
          final unitsChanged = newProvinces.length != _previousPhaseUnitProvinces!.length
              || newProvinces.difference(_previousPhaseUnitProvinces!).isNotEmpty;
          final nowSpring = gameViewState.currentPhase?.season == 'spring';
          final buildPhaseOccurred = wasBuildPhase || (unitsChanged && nowSpring);

          if (buildPhaseOccurred) {
            final added = newProvinces.difference(_previousPhaseUnitProvinces!);
            _newUnitProvinces = added.isNotEmpty ? added : {};
          } else {
            _newUnitProvinces = {};
          }

          // Compute build/disband animation sets.
          if (!_isInitialLoad && buildPhaseOccurred) {
            _computeBuildDisbandAnimation(newState, _previousPhaseUnitProvinces!);
          }
        } else {
          _newUnitProvinces = {};
        }

        _orderNotifier.resetForNewPhase();
        // Defer provider modifications to avoid modifying state during build.
        Future(() {
          if (!mounted) return;
          ref.read(phaseHistoryProvider(widget.gameId).notifier).viewCurrent();
          ref.read(phaseHistoryProvider(widget.gameId).notifier).loadPhases();
        });
      }
      // Snapshot the new phase's starting unit provinces so the next
      // transition can diff against this stable baseline.
      if (gameViewState.gameState != null) {
        _previousPhaseUnitProvinces = {for (final u in gameViewState.gameState!.units) u.province};
        _previousPhaseUnits = List.of(gameViewState.gameState!.units);
      }
      _previousPhaseType = gameViewState.currentPhase?.phaseType;
      _currentPhaseId = phaseId;
      _isInitialLoad = false;
    }

    // On initial load, seed the snapshot if not yet set.
    if (_previousPhaseUnitProvinces == null && gameViewState.gameState != null) {
      _previousPhaseUnitProvinces = {for (final u in gameViewState.gameState!.units) u.province};
      _previousPhaseUnits = List.of(gameViewState.gameState!.units);
      _previousPhaseType = gameViewState.currentPhase?.phaseType;
    }

    // Update the order notifier with current game state.
    if (_orderNotifier.gameState != gameViewState.gameState || _orderNotifier.myPower != myPower) {
      _orderNotifier.gameState = gameViewState.gameState;
      _orderNotifier.myPower = myPower;
    }
  }

  void _onProvinceTap(String provinceId) {
    _orderNotifier.selectProvince(provinceId);
  }

  Future<void> _submitOrders() async {
    final orders = _orderState.pendingOrders;
    if (orders.isEmpty) return;

    final notifier = ref.read(gameProvider(widget.gameId).notifier);
    final (result, errorMsg) = await notifier.submitOrders(orders);
    if (!mounted) return;
    if (result != null) {
      _orderNotifier.markSubmitted();
      // Auto-mark ready after successful submit.
      final ok = await notifier.markReady();
      if (!mounted) return;
      if (ok) {
        _orderNotifier.markReady();
      }
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(errorMsg ?? 'Failed to submit orders')),
      );
    }
  }

  Future<void> _markReady() async {
    final notifier = ref.read(gameProvider(widget.gameId).notifier);
    final ok = await notifier.markReady();
    if (!mounted) return;
    if (ok) {
      _orderNotifier.markReady();
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Failed to mark ready')),
      );
    }
  }

  Future<void> _editOrders() async {
    final notifier = ref.read(gameProvider(widget.gameId).notifier);
    await notifier.unmarkReady();
    if (!mounted) return;
    _orderNotifier.unsubmit();
  }

  /// Computes which units were built and which were disbanded by comparing
  /// the previous phase snapshot against the new game state, then stores
  /// the sets to drive the build/disband animation in MapPainter.
  ///
  /// Only called when the resolved phase was a build/adjustment phase.
  /// Uses set difference to detect changes -- handles the common case
  /// where builds and disbands happen in the same phase and the net
  /// unit count is unchanged.
  void _computeBuildDisbandAnimation(GameState newState, Set<String> previousProvinces) {
    final newProvinces = {for (final u in newState.units) u.province};

    final built = newProvinces.difference(previousProvinces);
    final disbanded = previousProvinces.difference(newProvinces);

    if (built.isEmpty && disbanded.isEmpty) return;

    // For disbanded units we need the actual unit data (type, power) to draw
    // them as ghosts. We stored the previous province set but not the units
    // themselves, so we need to reconstruct from the old order notifier state.
    // Since _previousPhaseUnitProvinces only has province IDs, we look at the
    // old gameState snapshot that was set when the notifier was last updated.
    // The notifier's gameState is already updated to the new state, but the
    // units in disbanded provinces have been removed. We can infer the unit
    // data from the previous game state that may still be in the notifier.
    //
    // A simpler approach: store the previous units list alongside the province set.
    // But since we only have provinces, we look at what's no longer present.
    // We stored units in the game notifier's previousGameState for movement,
    // but for build phases there's no previousGameState set.
    //
    // Solution: we snapshotted _previousPhaseUnits below for this purpose.
    final disbandUnits = <GameUnit>[];
    if (_previousPhaseUnits != null) {
      for (final unit in _previousPhaseUnits!) {
        if (disbanded.contains(unit.province)) {
          disbandUnits.add(unit);
        }
      }
    }

    setState(() {
      _buildAnimProvinces = built;
      _disbandAnimProvinces = disbanded;
      _disbandAnimUnits = disbandUnits;
    });
  }

  /// Computes build/disband animation data from history phases when viewing
  /// a build/adjustment phase during replay. Compares the previous phase's
  /// stateAfter (or current phase's stateBefore) against the current phase's
  /// stateAfter to detect which units were added or removed.
  void _computeHistoryBuildDisbandAnimation(PhaseHistoryState historyState) {
    final idx = historyState.viewingIndex;
    if (idx == null || idx >= historyState.phases.length) return;

    final phase = historyState.phases[idx];

    // Only compute for build/adjustment phases that have been resolved.
    if (phase.phaseType != 'build' || phase.stateAfter == null) return;

    final stateBefore = GameState.fromJson(phase.stateBefore);
    final stateAfter = GameState.fromJson(phase.stateAfter!);

    final beforeProvinces = {for (final u in stateBefore.units) u.province};
    final afterProvinces = {for (final u in stateAfter.units) u.province};

    final built = afterProvinces.difference(beforeProvinces);
    final disbanded = beforeProvinces.difference(afterProvinces);

    if (built.isEmpty && disbanded.isEmpty) return;

    // Collect ghost units for disbanded units from the before-state.
    final disbandUnits = <GameUnit>[];
    for (final unit in stateBefore.units) {
      if (disbanded.contains(unit.province)) {
        disbandUnits.add(unit);
      }
    }

    setState(() {
      _buildAnimProvinces = built;
      _disbandAnimProvinces = disbanded;
      _disbandAnimUnits = disbandUnits;
    });
  }

  /// Clears build/disband animation state after animation completes.
  /// During replay, notifies the replay controller so it can advance
  /// after the build/disband visual finishes.
  void _onBuildDisbandAnimationComplete() {
    if (!mounted) return;
    setState(() {
      _buildAnimProvinces = {};
      _disbandAnimProvinces = {};
      _disbandAnimUnits = [];
    });
    final historyState = ref.read(phaseHistoryProvider(widget.gameId));
    if (historyState.viewingIndex != null) {
      ref.read(replayProvider(widget.gameId).notifier)
          .notifyAnimationComplete();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(gameProvider(widget.gameId));
    final auth = ref.watch(authProvider);

    if (state.loading) {
      return Scaffold(
        appBar: AppBar(title: const Text('Game')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (state.error != null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Game')),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('Error: ${state.error}'),
              const SizedBox(height: 8),
              FilledButton(
                onPressed: () => ref.read(gameProvider(widget.gameId).notifier).load(),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      );
    }

    final game = state.game!;
    final myPower = state.powerForUser(auth.user?.id ?? '');
    _updateNotifierState(state);

    // Show game-over dialog once when first viewing a finished game.
    if (game.status == 'finished' && !_gameOverShown) {
      _gameOverShown = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) GameOverDialog.show(context, winner: game.winner);
      });
    }

    final isFinished = game.status == 'finished';

    // Check if viewing historical state.
    final historyState = ref.watch(phaseHistoryProvider(widget.gameId));

    // Auto-navigate to the last phase for finished games so the final
    // SC counts, territory shading, and winner highlight are correct.
    if (isFinished && !_autoViewedLastPhase
        && historyState.phases.isNotEmpty && historyState.viewingIndex == null) {
      _autoViewedLastPhase = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ref.read(phaseHistoryProvider(widget.gameId).notifier)
            .viewPhase(historyState.phases.length - 1);
      });
    }
    final displayState = historyState.historicalState ?? state.gameState;

    // Convert pending orders to map arrows.
    final pendingMapOrders = _orderState.pendingOrders.map((o) => PendingOrder(
          location: o.location,
          orderType: o.orderType,
          target: o.orderType == 'convoy' ? o.auxTarget : o.target,
          auxLoc: o.auxLoc,
          auxTarget: o.auxTarget,
        )).toList();

    final replayState = ref.watch(replayProvider(widget.gameId));
    final isReplaying = replayState.isPlaying || replayState.totalPhases > 0;

    final isMovement = state.currentPhase?.phaseType == 'movement';
    final isRetreat = state.currentPhase?.phaseType == 'retreat';
    final isBuild = state.currentPhase?.phaseType == 'build';
    final isViewingHistory = historyState.viewingIndex != null;

    // Detect history phase changes and compute build/disband animation data.
    if (isViewingHistory && historyState.viewingIndex != _lastHistoryViewingIndex) {
      _lastHistoryViewingIndex = historyState.viewingIndex;
      // Clear any previous build/disband animation before computing new one.
      _buildAnimProvinces = {};
      _disbandAnimProvinces = {};
      _disbandAnimUnits = [];
      // Defer the computation to avoid setState during build.
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        _computeHistoryBuildDisbandAnimation(historyState);
      });
    } else if (!isViewingHistory && _lastHistoryViewingIndex != null) {
      _lastHistoryViewingIndex = null;
    }

    const wideBreakpoint = 900.0;
    const sidePanelWidth = 250.0;
    const actionsPanelWidth = 280.0;

    return LayoutBuilder(
      builder: (context, constraints) {
        final isWide = constraints.maxWidth >= wideBreakpoint;

        // Map section: phase bar + interactive map + supply center table
        final gameMap = GameMap(
          gameState: displayState,
          previousGameState: isViewingHistory
              ? historyState.previousHistoricalState
              : (isFinished ? null : state.previousGameState),
          myPower: myPower,
          selectedProvince: isViewingHistory || isFinished ? null : _orderState.selectedProvince,
          validTargets: isViewingHistory || isFinished ? {} : _orderState.validTargets,
          pendingOrders: isViewingHistory || isFinished ? [] : pendingMapOrders,
          resolvedOrders: isViewingHistory
              ? historyState.historicalOrders
              : (state.resolvedOrders.isEmpty ? null : state.resolvedOrders),
          onProvinceTap: myPower != null && !isViewingHistory && !isFinished ? _onProvinceTap : null,
          onAnimationComplete: isViewingHistory
              ? () {
                  ref.read(phaseHistoryProvider(widget.gameId).notifier)
                      .clearPreviousHistoricalState();
                  ref.read(replayProvider(widget.gameId).notifier)
                      .notifyAnimationComplete();
                }
              : () => ref.read(gameProvider(widget.gameId).notifier)
                  .clearPreviousGameState(),
          replayMode: isViewingHistory,
          replaySpeed: isViewingHistory ? replayState.speed : null,
          newUnitProvinces: isViewingHistory ? const {} : _newUnitProvinces,
          buildProvinces: _buildAnimProvinces,
          disbandProvinces: _disbandAnimProvinces,
          disbandUnits: _disbandAnimUnits,
          onBuildDisbandAnimationComplete: _onBuildDisbandAnimationComplete,
        );

        // Action widgets: pending orders, order panels, action bar, submit buttons
        final actionWidgets = <Widget>[
          if (!isFinished && _orderState.pendingOrders.isNotEmpty && isMovement && !isReplaying)
            PendingOrdersList(
              orders: _orderState.pendingOrders,
              onRemove: _orderState.submitted ? null : _orderNotifier.removeOrder,
            ),
          if (!isFinished && isRetreat && myPower != null && !isReplaying
              && !_orderState.submitted && !_orderState.ready)
            RetreatOrderPanel(
              gameState: state.gameState!,
              myPower: myPower,
              pendingOrders: _orderState.pendingOrders,
              onAddOrder: (o) => _orderNotifier.addOrder(o),
              onRemoveOrder: _orderNotifier.removeOrder,
            ),
          if (!isFinished && isBuild && myPower != null && !isReplaying
              && !_orderState.submitted && !_orderState.ready)
            BuildOrderPanel(
              gameState: state.gameState!,
              myPower: myPower,
              pendingOrders: _orderState.pendingOrders,
              onAddOrder: (o) => _orderNotifier.addOrder(o),
              onRemoveOrder: _orderNotifier.removeOrder,
            ),
          if (!isFinished && isMovement && myPower != null && !isReplaying)
            OrderActionBar(
              state: _orderState,
              onOrderType: _orderNotifier.selectOrderType,
              onCancel: _orderNotifier.cancel,
              onSubmit: _submitOrders,
              onEdit: _editOrders,
              onSkip: _markReady,
            ),
          if (!isFinished && (isRetreat || isBuild) && myPower != null && !isReplaying)
            Padding(
              padding: const EdgeInsets.all(8),
              child: _orderState.ready
                  ? Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        FilledButton.icon(
                          onPressed: _editOrders,
                          icon: const Icon(Icons.edit),
                          label: const Text('Edit Orders'),
                        ),
                        const SizedBox(width: 12),
                        const Chip(
                          avatar: Icon(Icons.check_circle, color: Colors.green),
                          label: Text('Ready'),
                        ),
                      ],
                    )
                  : Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        if (_orderState.pendingOrders.isNotEmpty)
                          FilledButton.icon(
                            onPressed: _submitOrders,
                            icon: const Icon(Icons.send),
                            label: Text('Submit (${_orderState.pendingOrders.length})'),
                          ),
                        if (_orderState.pendingOrders.isNotEmpty)
                          const SizedBox(width: 12),
                        FilledButton.tonal(
                          onPressed: _markReady,
                          child: const Text('Ready (skip)'),
                        ),
                      ],
                    ),
            ),
        ];

        final phaseHeader = isFinished
            ? _GameOverBanner(winner: game.winner)
            : PhaseBar(phase: state.currentPhase, readyCount: state.readyCount);

        final phaseResultsOverlay = _showResults && state.resolvedOrders.isNotEmpty && !isViewingHistory
            ? PhaseResults(
                orders: state.resolvedOrders,
                onDismiss: () => setState(() => _showResults = false),
              )
            : null;

        Widget body;
        if (isWide) {
          body = Row(
            children: [
              SizedBox(
                width: sidePanelWidth,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    border: Border(
                      right: BorderSide(color: Theme.of(context).dividerColor),
                    ),
                  ),
                  child: PhaseHistoryPanel(gameId: widget.gameId),
                ),
              ),
              Expanded(
                child: Column(
                  children: [
                    phaseHeader,
                    Expanded(
                      child: Stack(
                        children: [
                          gameMap,
                          ?phaseResultsOverlay,
                        ],
                      ),
                    ),
                    if (displayState != null)
                      SupplyCenterTable(gameState: displayState, players: game.players),
                  ],
                ),
              ),
              SizedBox(
                width: actionsPanelWidth,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    border: Border(
                      left: BorderSide(color: Theme.of(context).dividerColor),
                    ),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const _MapLegend(compact: false),
                      const Divider(height: 1),
                      Expanded(
                        child: SingleChildScrollView(
                          child: Column(children: actionWidgets),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          );
        } else {
          body = Column(
            children: [
              phaseHeader,
              Expanded(
                child: Stack(
                  children: [
                    gameMap,
                    ?phaseResultsOverlay,
                  ],
                ),
              ),
              if (displayState != null)
                SupplyCenterTable(gameState: displayState, players: game.players),
              const _MapLegend(compact: true),
              ...actionWidgets,
            ],
          );
        }

        return Scaffold(
          appBar: AppBar(
            title: Text(game.name),
            leading: isWide
                ? null
                : Builder(
                    builder: (ctx) => IconButton(
                      icon: const Icon(Icons.history),
                      onPressed: () => Scaffold.of(ctx).openDrawer(),
                    ),
                  ),
            automaticallyImplyLeading: false,
            actions: [
              if (isViewingHistory && !isFinished)
                TextButton(
                  onPressed: () => ref.read(phaseHistoryProvider(widget.gameId).notifier).viewCurrent(),
                  child: const Text('Back to Current'),
                ),
              if (!isFinished && !isViewingHistory && myPower != null)
                IconButton(
                  icon: const Icon(Icons.handshake),
                  tooltip: 'Vote for Draw (${state.drawVoteCount}/${_alivePowerCount(state)})',
                  onPressed: () => ref.read(gameProvider(widget.gameId).notifier).voteForDraw(),
                ),
              if (state.resolvedOrders.isNotEmpty && !isViewingHistory)
                IconButton(
                  icon: const Icon(Icons.assessment),
                  tooltip: 'Phase Results',
                  onPressed: () => setState(() => _showResults = !_showResults),
                ),
              if (myPower != null && !isViewingHistory)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  child: Chip(label: Text(myPower[0].toUpperCase() + myPower.substring(1))),
                ),
              IconButton(
                icon: const Icon(Icons.chat),
                onPressed: () => context.push('/game/${widget.gameId}/messages'),
              ),
              if (isFinished)
                IconButton(
                  icon: const Icon(Icons.home),
                  tooltip: 'Back to Games',
                  onPressed: () => context.go('/home'),
                ),
            ],
          ),
          drawer: isWide ? null : PhaseHistory(gameId: widget.gameId),
          body: body,
        );
      },
    );
  }
}

/// Returns the number of alive powers (those owning at least 1 supply center).
int _alivePowerCount(GameViewState state) {
  final gs = state.gameState;
  if (gs == null) return 7;
  return gs.supplyCenters.values.toSet().length;
}

/// Color legend for map arrows and unit markers.
class _MapLegend extends StatelessWidget {
  final bool compact;
  const _MapLegend({this.compact = false});

  static final _arrowEntries = [
    ('Move', Colors.green.shade700, false),
    ('Support', Colors.yellow.shade700, true),
    ('Convoy', Colors.purple.shade700, true),
    ('Retreat', Colors.blue.shade600, false),
    ('Bounced', Colors.pink.shade300, true),
  ];

  @override
  Widget build(BuildContext context) {
    final items = <Widget>[
      ..._arrowEntries.map((e) => _arrowItem(e.$1, e.$2, e.$3)),
      _symbolItem('New Unit', const _LegendStarPainter()),
      _symbolItem('Destroyed', const _LegendXPainter()),
    ];

    if (compact) {
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        child: Wrap(spacing: 16, runSpacing: 2, children: items),
      );
    }

    return Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text('Legend', style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: Colors.grey.shade400,
          )),
          const SizedBox(height: 6),
          ...items,
        ],
      ),
    );
  }

  Widget _arrowItem(String label, Color color, bool dashed) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 28,
            height: 14,
            child: CustomPaint(painter: _LegendArrowPainter(color, dashed)),
          ),
          const SizedBox(width: 6),
          Text(label, style: const TextStyle(fontSize: 11)),
        ],
      ),
    );
  }

  Widget _symbolItem(String label, CustomPainter painter) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 28,
            height: 14,
            child: CustomPaint(painter: painter),
          ),
          const SizedBox(width: 6),
          Text(label, style: const TextStyle(fontSize: 11)),
        ],
      ),
    );
  }
}

/// Draws a small arrow segment for the legend (solid or dashed).
class _LegendArrowPainter extends CustomPainter {
  final Color color;
  final bool dashed;

  _LegendArrowPainter(this.color, this.dashed);

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 3
      ..style = PaintingStyle.stroke;

    final y = size.height / 2;
    if (dashed) {
      const dashLen = 4.0;
      const gapLen = 3.0;
      var x = 0.0;
      while (x < size.width - 5) {
        final end = math.min(x + dashLen, size.width - 5);
        canvas.drawLine(Offset(x, y), Offset(end, y), paint);
        x += dashLen + gapLen;
      }
    } else {
      canvas.drawLine(Offset(0, y), Offset(size.width - 5, y), paint);
    }

    // Arrowhead
    final tip = Offset(size.width, y);
    final path = Path()
      ..moveTo(tip.dx, tip.dy)
      ..lineTo(tip.dx - 6, tip.dy - 3.5)
      ..lineTo(tip.dx - 6, tip.dy + 3.5)
      ..close();
    canvas.drawPath(path, Paint()..color = color);
  }

  @override
  bool shouldRepaint(covariant _LegendArrowPainter old) =>
      old.color != color || old.dashed != dashed;
}

/// Draws a small gold star for the "New Unit" legend entry (matches _drawBuildStar in MapPainter).
class _LegendStarPainter extends CustomPainter {
  const _LegendStarPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final center = Offset(size.width / 2, size.height / 2);
    const outerR = 6.0;
    const innerR = 2.7;
    final path = Path();
    for (var i = 0; i < 8; i++) {
      final angle = (i * math.pi / 4) - math.pi / 2;
      final r = i.isEven ? outerR : innerR;
      final x = center.dx + r * math.cos(angle);
      final y = center.dy + r * math.sin(angle);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    path.close();
    canvas.drawPath(path, Paint()..color = const Color(0xFFFFD700));
    canvas.drawPath(path, Paint()
      ..color = const Color(0xFF8B6914)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.0);
  }

  @override
  bool shouldRepaint(covariant _LegendStarPainter old) => false;
}

/// Draws a small red X for the "Destroyed" legend entry (matches _drawDisbandX in MapPainter).
class _LegendXPainter extends CustomPainter {
  const _LegendXPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final center = Offset(size.width / 2, size.height / 2);
    const half = 5.0;
    final paint = Paint()
      ..color = const Color(0xFFFF0000)
      ..strokeWidth = 2.5
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    canvas.drawLine(
      center + const Offset(-half, -half),
      center + const Offset(half, half),
      paint,
    );
    canvas.drawLine(
      center + const Offset(half, -half),
      center + const Offset(-half, half),
      paint,
    );
  }

  @override
  bool shouldRepaint(covariant _LegendXPainter old) => false;
}

/// Persistent banner shown instead of PhaseBar for finished games.
class _GameOverBanner extends StatelessWidget {
  final String? winner;
  const _GameOverBanner({this.winner});

  @override
  Widget build(BuildContext context) {
    final hasWinner = winner != null && winner!.isNotEmpty;
    return Container(
      height: 48,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: Colors.grey.shade800,
        border: Border(
          bottom: BorderSide(color: Theme.of(context).dividerColor),
        ),
      ),
      child: Row(
        children: [
          const Icon(Icons.emoji_events, color: Colors.amber, size: 20),
          const SizedBox(width: 8),
          Text(
            hasWinner ? '${PowerColors.label(winner!)} wins!' : 'Game Over — Draw',
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.bold,
              fontSize: 14,
            ),
          ),
          if (hasWinner) ...[
            const SizedBox(width: 8),
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                color: PowerColors.forPower(winner!),
                shape: BoxShape.circle,
              ),
            ),
          ],
          const Spacer(),
          const Text(
            'Use history to replay',
            style: TextStyle(color: Colors.white70, fontSize: 12),
          ),
        ],
      ),
    );
  }
}
