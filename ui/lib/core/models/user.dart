class User {
  final String id;
  final String provider;
  final String providerId;
  final String displayName;
  final String avatarUrl;
  final DateTime createdAt;
  final DateTime updatedAt;

  const User({
    required this.id,
    required this.provider,
    required this.providerId,
    required this.displayName,
    this.avatarUrl = '',
    required this.createdAt,
    required this.updatedAt,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as String,
      provider: json['provider'] as String,
      providerId: json['provider_id'] as String,
      displayName: json['display_name'] as String,
      avatarUrl: json['avatar_url'] as String? ?? '',
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'provider': provider,
        'provider_id': providerId,
        'display_name': displayName,
        'avatar_url': avatarUrl,
        'created_at': createdAt.toIso8601String(),
        'updated_at': updatedAt.toIso8601String(),
      };
}
