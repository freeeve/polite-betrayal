import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/map/adjacency_data.dart';
import '../../../core/map/province_data.dart';
import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';
import 'order_state.dart';

/// Manages the order input state machine transitions.
class OrderInputNotifier extends StateNotifier<OrderInputState> {
  GameState? gameState;
  String? myPower;

  OrderInputNotifier({this.gameState, this.myPower}) : super(const OrderInputState());

  /// Handle a province tap based on current state.
  void selectProvince(String provinceId) {
    switch (state.phase) {
      case OrderPhase.idle:
        _selectUnit(provinceId);
      case OrderPhase.unitSelected:
        // Tapping the same unit deselects it.
        if (provinceId == state.selectedProvince) {
          state = state.reset();
          return;
        }
        // Otherwise treat as if we're tapping a different unit.
        _selectUnit(provinceId);
      case OrderPhase.awaitingTarget:
        _selectTarget(provinceId);
      case OrderPhase.awaitingAuxLoc:
        _selectAuxLoc(provinceId);
      case OrderPhase.awaitingAuxTarget:
        _selectAuxTarget(provinceId);
      default:
        break;
    }
  }

  void _selectUnit(String provinceId) {
    if (gameState == null || myPower == null) return;

    final unit = gameState!.units.where(
      (u) => u.province == provinceId && u.power == myPower,
    ).firstOrNull;

    if (unit == null) return; // Not our unit

    state = state.copyWith(
      phase: OrderPhase.unitSelected,
      selectedProvince: provinceId,
      selectedUnit: unit,
      prompt: 'Choose order type',
      validTargets: {},
    );
  }

  /// Select the order type (hold, move, support, convoy).
  void selectOrderType(String type) {
    if (state.phase != OrderPhase.unitSelected) return;
    final unit = state.selectedUnit!;

    switch (type) {
      case 'hold':
        addOrder(OrderInput(
          unitType: unit.type.fullName,
          location: unit.province,
          coast: unit.coast.isNotEmpty ? unit.coast : null,
          orderType: 'hold',
        ));
        state = state.reset();

      case 'move':
        var targets = unit.type == UnitType.army
            ? armyTargets(unit.province)
            : fleetTargets(unit.province, coast: unit.coast);
        if (unit.type == UnitType.army) {
          targets = {...targets, ..._convoyDestinations(unit.province)};
        }
        state = state.copyWith(
          phase: OrderPhase.awaitingTarget,
          orderType: 'move',
          prompt: 'Select move target',
          validTargets: targets,
        );

      case 'support':
        // Provinces the supporting unit can reach (type-aware).
        final supporterReach = unit.type == UnitType.army
            ? armyTargets(unit.province)
            : fleetTargets(unit.province, coast: unit.coast);

        // Show any unit that is adjacent (support-hold/move) OR can move
        // to a province the supporter can reach (support-move only).
        final candidates = <String>{};
        for (final u in gameState!.units) {
          if (u.province == unit.province) continue;
          if (supporterReach.contains(u.province)) {
            candidates.add(u.province);
            continue;
          }
          final unitReach = u.type == UnitType.army
              ? armyTargets(u.province)
              : fleetTargets(u.province, coast: u.coast);
          if (unitReach.intersection(supporterReach).isNotEmpty) {
            candidates.add(u.province);
          }
        }
        state = state.copyWith(
          phase: OrderPhase.awaitingAuxLoc,
          orderType: 'support',
          prompt: 'Select unit to support',
          validTargets: candidates,
        );

      case 'convoy':
        if (unit.type != UnitType.fleet) return;
        // Select army to convoy — only show adjacent provinces with armies.
        final adj = allAdjacent(unit.province);
        final armies = adj.where((p) =>
          gameState!.units.any((u) => u.province == p && u.type == UnitType.army),
        ).toSet();
        state = state.copyWith(
          phase: OrderPhase.awaitingAuxLoc,
          orderType: 'convoy',
          prompt: 'Select army to convoy',
          validTargets: armies,
        );
    }
  }

  void _selectTarget(String target) {
    if (!state.validTargets.contains(target)) return;

    final unit = state.selectedUnit!;

    // Check if target is a split-coast province for fleet moves.
    if (unit.type == UnitType.fleet) {
      final coasts = reachableCoasts(
        unit.province, unit.coast, target,
      );
      if (coasts.length == 1) {
        // Only one coast reachable — auto-select it.
        _addOrderAndReset(OrderInput(
          unitType: unit.type.fullName,
          location: state.selectedProvince!,
          coast: unit.coast.isNotEmpty ? unit.coast : null,
          orderType: 'move',
          target: target,
          targetCoast: coasts.first,
        ));
        return;
      }
      if (coasts.length > 1) {
        state = state.copyWith(
          phase: OrderPhase.awaitingCoast,
          target: target,
          prompt: 'Select coast',
        );
        return;
      }
    }

    _addOrderAndReset(OrderInput(
      unitType: unit.type.fullName,
      location: state.selectedProvince!,
      coast: unit.coast.isNotEmpty ? unit.coast : null,
      orderType: 'move',
      target: target,
    ));
  }

  /// Select coast for split-coast targets.
  void selectCoast(String coast) {
    if (state.phase != OrderPhase.awaitingCoast) return;

    final unit = state.selectedUnit!;
    _addOrderAndReset(OrderInput(
      unitType: unit.type.fullName,
      location: state.selectedProvince!,
      coast: unit.coast.isNotEmpty ? unit.coast : null,
      orderType: 'move',
      target: state.target,
      targetCoast: coast,
    ));
  }

  /// Add an order and reset in a single state update to avoid intermediate notifications.
  void _addOrderAndReset(OrderInput order) {
    final orders = state.pendingOrders
        .where((o) => o.location != order.location)
        .toList()
      ..add(order);
    state = const OrderInputState().copyWith(pendingOrders: orders);
  }

  void _selectAuxLoc(String auxLoc) {
    if (!state.validTargets.contains(auxLoc)) return;

    if (state.orderType == 'support') {
      // Compute where the supported unit can move.
      final supportedUnit = gameState?.units.where((u) => u.province == auxLoc).firstOrNull;
      final supportedAdj = supportedUnit != null
          ? (supportedUnit.type == UnitType.army
              ? armyTargets(auxLoc)
              : fleetTargets(auxLoc, coast: supportedUnit.coast))
          : allAdjacent(auxLoc);

      // Compute where the supporting unit can move (must be able to reach destination).
      final supportingUnit = state.selectedUnit!;
      final supportingAdj = supportingUnit.type == UnitType.army
          ? armyTargets(supportingUnit.province)
          : fleetTargets(supportingUnit.province, coast: supportingUnit.coast);

      // Valid move destinations: intersection of both.
      final targets = supportedAdj.intersection(supportingAdj);

      // Support-hold is valid only if the supporting unit can reach auxLoc.
      if (supportingAdj.contains(auxLoc)) {
        targets.add(auxLoc);
      }

      state = state.copyWith(
        phase: OrderPhase.awaitingAuxTarget,
        auxLoc: auxLoc,
        auxUnitType: supportedUnit?.type.fullName,
        prompt: 'Select support destination (or same for hold)',
        validTargets: targets,
      );
    } else if (state.orderType == 'convoy') {
      // Convoy: BFS through sea zones with friendly fleets to find reachable coastal destinations.
      final destinations = _convoyDestinations(auxLoc);
      state = state.copyWith(
        phase: OrderPhase.awaitingAuxTarget,
        auxLoc: auxLoc,
        prompt: 'Select convoy destination',
        validTargets: destinations,
      );
    }
  }

  void _selectAuxTarget(String auxTarget) {
    if (!state.validTargets.contains(auxTarget)) return;

    final unit = state.selectedUnit!;
    final unitCoast = unit.coast.isNotEmpty ? unit.coast : null;

    if (state.orderType == 'support') {
      if (auxTarget == state.auxLoc) {
        // Support hold
        addOrder(OrderInput(
          unitType: unit.type.fullName,
          location: state.selectedProvince!,
          coast: unitCoast,
          orderType: 'support',
          auxLoc: state.auxLoc,
          auxUnitType: state.auxUnitType,
        ));
      } else {
        // Support move
        addOrder(OrderInput(
          unitType: unit.type.fullName,
          location: state.selectedProvince!,
          coast: unitCoast,
          orderType: 'support',
          target: auxTarget,
          auxLoc: state.auxLoc,
          auxTarget: auxTarget,
          auxUnitType: state.auxUnitType,
        ));
      }
    } else if (state.orderType == 'convoy') {
      addOrder(OrderInput(
        unitType: unit.type.fullName,
        location: state.selectedProvince!,
        coast: unitCoast,
        orderType: 'convoy',
        auxLoc: state.auxLoc,
        auxTarget: auxTarget,
        auxUnitType: 'army',
      ));
    }

    state = state.reset();
  }

  /// BFS through sea zones containing fleets to find coastal provinces reachable by convoy.
  Set<String> _convoyDestinations(String armyLoc) {
    if (gameState == null) return {};

    // Collect sea zones that have a fleet.
    final fleetSeas = <String>{};
    for (final u in gameState!.units) {
      if (u.type == UnitType.fleet) {
        final prov = provinces[u.province];
        if (prov != null && prov.isSea) {
          fleetSeas.add(u.province);
        }
      }
    }

    // BFS from fleet-occupied sea zones adjacent to the army.
    final armyAdj = allAdjacent(armyLoc);
    final reachableSeas = <String>{};
    final queue = <String>[];
    for (final sea in armyAdj) {
      if (fleetSeas.contains(sea)) {
        reachableSeas.add(sea);
        queue.add(sea);
      }
    }

    while (queue.isNotEmpty) {
      final current = queue.removeLast();
      for (final neighbor in fleetTargets(current)) {
        if (fleetSeas.contains(neighbor) && reachableSeas.add(neighbor)) {
          queue.add(neighbor);
        }
      }
    }

    // Destinations: land/coastal provinces adjacent to any reachable sea, excluding the army's origin.
    final destinations = <String>{};
    for (final sea in reachableSeas) {
      for (final neighbor in allAdjacent(sea)) {
        final prov = provinces[neighbor];
        if (prov != null && prov.isLand && neighbor != armyLoc) {
          destinations.add(neighbor);
        }
      }
    }
    return destinations;
  }

  void addOrder(OrderInput order) {
    // Replace any existing order for the same location.
    final orders = state.pendingOrders
        .where((o) => o.location != order.location)
        .toList()
      ..add(order);
    state = state.copyWith(pendingOrders: orders);
  }

  /// Remove a pending order at the given index.
  void removeOrder(int index) {
    if (index < 0 || index >= state.pendingOrders.length) return;
    final orders = [...state.pendingOrders]..removeAt(index);
    state = state.copyWith(pendingOrders: orders);
  }

  /// Cancel current input flow.
  void cancel() {
    state = state.reset();
  }

  void markSubmitted() {
    state = state.copyWith(submitted: true);
  }

  /// Revert to editing mode so orders can be changed and re-submitted.
  void unsubmit() {
    state = state.copyWith(submitted: false);
  }

  void markReady() {
    state = state.copyWith(ready: true);
  }

  /// Reset everything for a new phase.
  void resetForNewPhase() {
    state = const OrderInputState();
  }
}
