import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';

/// Order input state machine phases.
enum OrderPhase {
  idle,
  unitSelected,
  awaitingTarget,
  awaitingCoast,
  awaitingAuxLoc,
  awaitingAuxTarget,
  complete,
}

/// State of the order input flow.
class OrderInputState {
  final OrderPhase phase;
  final String? selectedProvince;
  final GameUnit? selectedUnit;
  final String? orderType;
  final String? target;
  final String? coast;
  final String? auxLoc;
  final String? auxTarget;
  final String? auxUnitType;
  final List<OrderInput> pendingOrders;
  final String prompt;
  final Set<String> validTargets;
  final bool submitted;
  final bool ready;

  const OrderInputState({
    this.phase = OrderPhase.idle,
    this.selectedProvince,
    this.selectedUnit,
    this.orderType,
    this.target,
    this.coast,
    this.auxLoc,
    this.auxTarget,
    this.auxUnitType,
    this.pendingOrders = const [],
    this.prompt = 'Select a unit',
    this.validTargets = const {},
    this.submitted = false,
    this.ready = false,
  });

  OrderInputState copyWith({
    OrderPhase? phase,
    String? selectedProvince,
    GameUnit? selectedUnit,
    String? orderType,
    String? target,
    String? coast,
    String? auxLoc,
    String? auxTarget,
    String? auxUnitType,
    List<OrderInput>? pendingOrders,
    String? prompt,
    Set<String>? validTargets,
    bool? submitted,
    bool? ready,
  }) {
    return OrderInputState(
      phase: phase ?? this.phase,
      selectedProvince: selectedProvince ?? this.selectedProvince,
      selectedUnit: selectedUnit ?? this.selectedUnit,
      orderType: orderType ?? this.orderType,
      target: target ?? this.target,
      coast: coast ?? this.coast,
      auxLoc: auxLoc ?? this.auxLoc,
      auxTarget: auxTarget ?? this.auxTarget,
      auxUnitType: auxUnitType ?? this.auxUnitType,
      pendingOrders: pendingOrders ?? this.pendingOrders,
      prompt: prompt ?? this.prompt,
      validTargets: validTargets ?? this.validTargets,
      submitted: submitted ?? this.submitted,
      ready: ready ?? this.ready,
    );
  }

  OrderInputState reset() {
    return OrderInputState(
      pendingOrders: pendingOrders,
      submitted: submitted,
      ready: ready,
    );
  }
}
