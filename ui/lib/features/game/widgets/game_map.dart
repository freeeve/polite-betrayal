import 'dart:developer' as dev;

import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../../../core/map/hit_testing.dart';
import '../../../core/map/province_data.dart';
import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';
import 'map_painter.dart';

/// Interactive Diplomacy map: SVG background + CustomPaint overlay, wrapped in InteractiveViewer.
class GameMap extends StatefulWidget {
  final GameState? gameState;
  final GameState? previousGameState;
  final String? selectedProvince;
  final Set<String> validTargets;
  final List<PendingOrder> pendingOrders;
  final List<Order>? resolvedOrders;
  final String? myPower;
  final void Function(String provinceId)? onProvinceTap;
  final VoidCallback? onAnimationComplete;
  final bool replayMode;
  final Set<String> newUnitProvinces;

  /// Build/disband animation: provinces where units were built or disbanded,
  /// and ghost units for disbanded units no longer in current game state.
  final Set<String> buildProvinces;
  final Set<String> disbandProvinces;
  final List<GameUnit> disbandUnits;
  final VoidCallback? onBuildDisbandAnimationComplete;

  /// Replay speed in seconds per phase (used to scale animation durations).
  /// Only applies when replayMode is true. Null means use default durations.
  final double? replaySpeed;

  const GameMap({
    super.key,
    this.gameState,
    this.previousGameState,
    this.selectedProvince,
    this.validTargets = const {},
    this.pendingOrders = const [],
    this.resolvedOrders,
    this.myPower,
    this.onProvinceTap,
    this.onAnimationComplete,
    this.replayMode = false,
    this.newUnitProvinces = const {},
    this.buildProvinces = const {},
    this.disbandProvinces = const {},
    this.disbandUnits = const [],
    this.onBuildDisbandAnimationComplete,
    this.replaySpeed,
  });

  @override
  State<GameMap> createState() => _GameMapState();
}

class _GameMapState extends State<GameMap> with TickerProviderStateMixin {
  final _transformController = TransformationController();
  late final AnimationController _animController;
  late final AnimationController _buildDisbandController;
  Offset? _pointerDown;

  @override
  void initState() {
    super.initState();
    _animController = AnimationController(
      duration: const Duration(seconds: 7),
      vsync: this,
    )..addListener(() => setState(() {}))
     ..addStatusListener((status) {
       if (status == AnimationStatus.completed) {
         widget.onAnimationComplete?.call();
       }
     });
    _buildDisbandController = AnimationController(
      duration: const Duration(milliseconds: 1500),
      vsync: this,
    )..addListener(() => setState(() {}))
     ..addStatusListener((status) {
       if (status == AnimationStatus.completed) {
         widget.onBuildDisbandAnimationComplete?.call();
       }
     });
  }

  /// Computes the movement animation duration based on replay mode and speed.
  Duration _movementDuration() {
    if (widget.replayMode && widget.replaySpeed != null) {
      final ms = (widget.replaySpeed! * 1000).round();
      return Duration(milliseconds: ms);
    }
    return const Duration(seconds: 7);
  }

  /// Computes the build/disband animation duration based on replay mode and speed.
  Duration _buildDisbandDuration() {
    if (widget.replayMode && widget.replaySpeed != null) {
      final ms = (widget.replaySpeed! * 750).round();
      return Duration(milliseconds: ms);
    }
    return const Duration(milliseconds: 1500);
  }

  @override
  void didUpdateWidget(GameMap oldWidget) {
    super.didUpdateWidget(oldWidget);

    // Update animation durations when replay mode or speed changes.
    if (widget.replayMode != oldWidget.replayMode
        || widget.replaySpeed != oldWidget.replaySpeed) {
      _animController.duration = _movementDuration();
      _buildDisbandController.duration = _buildDisbandDuration();
    }

    // Start animation when both previousGameState and resolved orders are
    // available. Resolved orders load asynchronously so they may arrive a
    // frame or two after previousGameState is set.
    if (widget.previousGameState != null && !_animController.isAnimating) {
      final ready = widget.resolvedOrders != null
          && widget.resolvedOrders!.isNotEmpty;
      final wasReady = oldWidget.previousGameState != null
          && oldWidget.resolvedOrders != null
          && oldWidget.resolvedOrders!.isNotEmpty;
      // Also trigger when previousGameState changed identity (e.g. returning
      // from history/replay view to live while a phase resolved in the background).
      final stateChanged = widget.previousGameState != oldWidget.previousGameState;
      if (ready && (!wasReady || stateChanged)) {
        dev.log('starting animation (${widget.resolvedOrders!.length} orders, replay=${widget.replayMode})',
            name: 'GameMap');
        _animController.forward(from: 0);
      }
    }

    // Cancel animation if previousGameState is cleared.
    if (widget.previousGameState == null && oldWidget.previousGameState != null) {
      _animController.reset();
    }

    // Start build/disband animation when build or disband provinces become non-empty.
    final hasBuildDisband = widget.buildProvinces.isNotEmpty || widget.disbandProvinces.isNotEmpty;
    final hadBuildDisband = oldWidget.buildProvinces.isNotEmpty || oldWidget.disbandProvinces.isNotEmpty;
    if (hasBuildDisband && !hadBuildDisband) {
      dev.log('starting build/disband animation (${widget.buildProvinces.length} builds, '
          '${widget.disbandProvinces.length} disbands)', name: 'GameMap');
      _buildDisbandController.forward(from: 0);
    }
    // Also re-trigger if the province sets changed (e.g. replay advancing).
    if (hasBuildDisband && hadBuildDisband
        && (widget.buildProvinces != oldWidget.buildProvinces
            || widget.disbandProvinces != oldWidget.disbandProvinces)) {
      _buildDisbandController.forward(from: 0);
    }

    // Cancel build/disband animation if sets are cleared.
    if (!hasBuildDisband && hadBuildDisband) {
      _buildDisbandController.reset();
    }
  }

  @override
  void dispose() {
    _animController.dispose();
    _buildDisbandController.dispose();
    _transformController.dispose();
    super.dispose();
  }

  /// Handles tap detection via raw pointer events to avoid gesture arena
  /// conflicts with InteractiveViewer's internal pan/scale recognizers.
  void _onPointerDown(PointerDownEvent event) {
    _pointerDown = event.localPosition;
  }

  void _onPointerUp(PointerUpEvent event) {
    if (!mounted) return;
    if (widget.onProvinceTap == null || _pointerDown == null) return;

    // Only treat as a tap if the pointer didn't move much (not a drag).
    final distance = (event.localPosition - _pointerDown!).distance;
    if (distance > 10) return;

    // Convert screen position through InteractiveViewer transform to SVG space.
    final matrix = _transformController.value;
    final inverse = Matrix4.inverted(matrix);
    final transformed = MatrixUtils.transformPoint(inverse, event.localPosition);

    final renderBox = context.findRenderObject() as RenderBox;
    final size = renderBox.size;
    final svgX = transformed.dx * svgViewBoxWidth / size.width;
    final svgY = transformed.dy * svgViewBoxHeight / size.height;

    final province = hitTestProvince(Offset(svgX, svgY));
    if (province != null) {
      widget.onProvinceTap!(province);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Listener(
      onPointerDown: _onPointerDown,
      onPointerUp: _onPointerUp,
      child: InteractiveViewer(
        transformationController: _transformController,
        minScale: 0.5,
        maxScale: 4.0,
        boundaryMargin: const EdgeInsets.all(100),
        child: AspectRatio(
          aspectRatio: svgViewBoxWidth / svgViewBoxHeight,
          child: LayoutBuilder(
            builder: (context, constraints) {
              return Stack(
                children: [
                  // SVG background
                  SvgPicture.asset(
                    'assets/map/diplomacy_map_hq.svg',
                    width: constraints.maxWidth,
                    height: constraints.maxHeight,
                    fit: BoxFit.contain,
                  ),
                  // Dynamic overlay
                  CustomPaint(
                    size: Size(constraints.maxWidth, constraints.maxHeight),
                    painter: MapPainter(
                      gameState: widget.gameState,
                      previousGameState: _animController.isAnimating
                          ? widget.previousGameState : null,
                      animationProgress: _animController.isAnimating
                          ? _animController.value : null,
                      selectedProvince: widget.selectedProvince,
                      validTargets: widget.validTargets,
                      pendingOrders: widget.pendingOrders,
                      resolvedOrders: widget.resolvedOrders,
                      myPower: widget.myPower,
                      newUnitProvinces: widget.newUnitProvinces,
                      buildProvinces: _buildDisbandController.isAnimating
                          ? widget.buildProvinces : const {},
                      disbandProvinces: _buildDisbandController.isAnimating
                          ? widget.disbandProvinces : const {},
                      disbandUnits: _buildDisbandController.isAnimating
                          ? widget.disbandUnits : const [],
                      buildDisbandProgress: _buildDisbandController.isAnimating
                          ? _buildDisbandController.value : null,
                    ),
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );
  }
}
