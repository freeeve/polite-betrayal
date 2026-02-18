/// Order returned from the API (snake_case JSON).
class Order {
  final String id;
  final String phaseId;
  final String power;
  final String unitType;
  final String location;
  final String orderType;
  final String? target;
  final String? auxLoc;
  final String? auxTarget;
  final String? auxUnitType;
  final String? result;
  final DateTime createdAt;

  const Order({
    required this.id,
    required this.phaseId,
    required this.power,
    required this.unitType,
    required this.location,
    required this.orderType,
    this.target,
    this.auxLoc,
    this.auxTarget,
    this.auxUnitType,
    this.result,
    required this.createdAt,
  });

  factory Order.fromJson(Map<String, dynamic> json) {
    return Order(
      id: json['id'] as String,
      phaseId: json['phase_id'] as String,
      power: json['power'] as String,
      unitType: json['unit_type'] as String,
      location: json['location'] as String,
      orderType: json['order_type'] as String,
      target: json['target'] as String?,
      auxLoc: json['aux_loc'] as String?,
      auxTarget: json['aux_target'] as String?,
      auxUnitType: json['aux_unit_type'] as String?,
      result: json['result'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}

/// Order input for submission to backend.
class OrderInput {
  final String unitType;
  final String location;
  final String? coast;
  final String orderType;
  final String? target;
  final String? targetCoast;
  final String? auxLoc;
  final String? auxTarget;
  final String? auxUnitType;

  const OrderInput({
    required this.unitType,
    required this.location,
    this.coast,
    required this.orderType,
    this.target,
    this.targetCoast,
    this.auxLoc,
    this.auxTarget,
    this.auxUnitType,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'unit_type': unitType,
      'location': location,
      'order_type': orderType,
    };
    if (coast != null) json['coast'] = coast;
    if (target != null) json['target'] = target;
    if (targetCoast != null) json['target_coast'] = targetCoast;
    if (auxLoc != null) json['aux_loc'] = auxLoc;
    if (auxTarget != null) json['aux_target'] = auxTarget;
    if (auxUnitType != null) json['aux_unit_type'] = auxUnitType;
    return json;
  }
}
