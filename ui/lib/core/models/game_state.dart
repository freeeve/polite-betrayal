/// Matches Go's GameState struct which serializes with PascalCase keys
/// and UnitType as int (0=Army, 1=Fleet).
class GameState {
  final int year;
  final String season;
  final String phase;
  final List<GameUnit> units;
  final Map<String, String> supplyCenters;
  final List<DislodgedUnit> dislodged;

  const GameState({
    required this.year,
    required this.season,
    required this.phase,
    required this.units,
    required this.supplyCenters,
    this.dislodged = const [],
  });

  factory GameState.fromJson(Map<String, dynamic> json) {
    return GameState(
      year: json['Year'] as int,
      season: json['Season'] as String,
      phase: json['Phase'] as String,
      units: (json['Units'] as List<dynamic>?)
              ?.map((e) => GameUnit.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      supplyCenters: (json['SupplyCenters'] as Map<String, dynamic>?)
              ?.map((k, v) => MapEntry(k, v as String)) ??
          {},
      dislodged: (json['Dislodged'] as List<dynamic>?)
              ?.map((e) => DislodgedUnit.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  /// Returns units belonging to the given power.
  List<GameUnit> unitsOf(String power) =>
      units.where((u) => u.power == power).toList();

  /// Returns number of supply centers owned by the given power.
  int supplyCenterCount(String power) =>
      supplyCenters.values.where((v) => v == power).length;
}

enum UnitType {
  army,
  fleet;

  static UnitType fromInt(int value) => value == 0 ? army : fleet;

  String get label => this == army ? 'A' : 'F';
  String get fullName => this == army ? 'army' : 'fleet';
}

class GameUnit {
  final UnitType type;
  final String power;
  final String province;
  final String coast;

  const GameUnit({
    required this.type,
    required this.power,
    required this.province,
    this.coast = '',
  });

  factory GameUnit.fromJson(Map<String, dynamic> json) {
    return GameUnit(
      type: UnitType.fromInt(json['Type'] as int),
      power: json['Power'] as String,
      province: json['Province'] as String,
      coast: json['Coast'] as String? ?? '',
    );
  }
}

class DislodgedUnit {
  final GameUnit unit;
  final String dislodgedFrom;
  final String attackerFrom;

  const DislodgedUnit({
    required this.unit,
    required this.dislodgedFrom,
    required this.attackerFrom,
  });

  factory DislodgedUnit.fromJson(Map<String, dynamic> json) {
    return DislodgedUnit(
      unit: GameUnit.fromJson(json['Unit'] as Map<String, dynamic>),
      dislodgedFrom: json['DislodgedFrom'] as String,
      attackerFrom: json['AttackerFrom'] as String,
    );
  }
}
