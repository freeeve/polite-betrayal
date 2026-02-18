class Phase {
  final String id;
  final String gameId;
  final int year;
  final String season;
  final String phaseType;
  final Map<String, dynamic> stateBefore;
  final Map<String, dynamic>? stateAfter;
  final DateTime deadline;
  final DateTime? resolvedAt;
  final DateTime createdAt;

  const Phase({
    required this.id,
    required this.gameId,
    required this.year,
    required this.season,
    required this.phaseType,
    required this.stateBefore,
    this.stateAfter,
    required this.deadline,
    this.resolvedAt,
    required this.createdAt,
  });

  factory Phase.fromJson(Map<String, dynamic> json) {
    return Phase(
      id: json['id'] as String,
      gameId: json['game_id'] as String,
      year: json['year'] as int,
      season: json['season'] as String,
      phaseType: json['phase_type'] as String,
      stateBefore: json['state_before'] as Map<String, dynamic>,
      stateAfter: json['state_after'] as Map<String, dynamic>?,
      deadline: DateTime.parse(json['deadline'] as String),
      resolvedAt: json['resolved_at'] != null
          ? DateTime.parse(json['resolved_at'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
