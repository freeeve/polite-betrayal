class Message {
  final String id;
  final String gameId;
  final String senderId;
  final String? recipientId;
  final String content;
  final String? phaseId;
  final DateTime createdAt;

  const Message({
    required this.id,
    required this.gameId,
    required this.senderId,
    this.recipientId,
    required this.content,
    this.phaseId,
    required this.createdAt,
  });

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      gameId: json['game_id'] as String,
      senderId: json['sender_id'] as String,
      recipientId: json['recipient_id'] as String?,
      content: json['content'] as String,
      phaseId: json['phase_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
        'content': content,
        if (recipientId != null) 'recipient_id': recipientId,
      };

  bool get isPublic => recipientId == null || recipientId!.isEmpty;
}
