class Game {
  final String id;
  final String name;
  final String creatorId;
  final String status;
  final String? winner;
  final String turnDuration;
  final String retreatDuration;
  final String buildDuration;
  final String powerAssignment;
  final DateTime createdAt;
  final DateTime? startedAt;
  final DateTime? finishedAt;
  final List<GamePlayer> players;
  final int readyCount;
  final int drawVoteCount;

  const Game({
    required this.id,
    required this.name,
    required this.creatorId,
    required this.status,
    this.winner,
    required this.turnDuration,
    required this.retreatDuration,
    required this.buildDuration,
    this.powerAssignment = 'random',
    required this.createdAt,
    this.startedAt,
    this.finishedAt,
    this.players = const [],
    this.readyCount = 0,
    this.drawVoteCount = 0,
  });

  /// True when every player in the game is a bot (spectator-only game).
  bool get isBotOnly => players.isNotEmpty && players.every((p) => p.isBot);

  factory Game.fromJson(Map<String, dynamic> json) {
    return Game(
      id: json['id'] as String,
      name: json['name'] as String,
      creatorId: json['creator_id'] as String,
      status: json['status'] as String,
      winner: json['winner'] as String?,
      turnDuration: json['turn_duration'] as String,
      retreatDuration: json['retreat_duration'] as String,
      buildDuration: json['build_duration'] as String,
      powerAssignment: (json['power_assignment'] as String?)?.isNotEmpty == true
          ? json['power_assignment'] as String
          : 'random',
      createdAt: DateTime.parse(json['created_at'] as String),
      startedAt: json['started_at'] != null
          ? DateTime.parse(json['started_at'] as String)
          : null,
      finishedAt: json['finished_at'] != null
          ? DateTime.parse(json['finished_at'] as String)
          : null,
      players: (json['players'] as List<dynamic>?)
              ?.map((e) => GamePlayer.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      readyCount: (json['ready_count'] as num?)?.toInt() ?? 0,
      drawVoteCount: (json['draw_vote_count'] as num?)?.toInt() ?? 0,
    );
  }
}

class GamePlayer {
  final String gameId;
  final String userId;
  final String? power;
  final bool isBot;
  final String botDifficulty;
  final DateTime joinedAt;

  const GamePlayer({
    required this.gameId,
    required this.userId,
    this.power,
    this.isBot = false,
    this.botDifficulty = 'easy',
    required this.joinedAt,
  });

  factory GamePlayer.fromJson(Map<String, dynamic> json) {
    return GamePlayer(
      gameId: json['game_id'] as String,
      userId: json['user_id'] as String,
      power: json['power'] as String?,
      isBot: json['is_bot'] as bool? ?? false,
      botDifficulty: json['bot_difficulty'] as String? ?? 'easy',
      joinedAt: DateTime.parse(json['joined_at'] as String),
    );
  }
}
