import 'package:json_annotation/json_annotation.dart';

part 'idea.g.dart';

@JsonSerializable()
class Idea {
  final int id;
  @JsonKey(name: 'project_id')
  final int projectId;
  @JsonKey(name: 'user_id')
  final String userId;
  @JsonKey(name: 'raw_input')
  final String rawInput;
  @JsonKey(name: 'structured_data')
  final Map<String, dynamic>? structuredData;
  final List<String> tags;
  @JsonKey(name: 'created_at')
  final DateTime? createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime? updatedAt;

  const Idea({
    required this.id,
    required this.projectId,
    required this.userId,
    required this.rawInput,
    this.structuredData,
    this.tags = const [],
    this.createdAt,
    this.updatedAt,
  });

  factory Idea.fromJson(Map<String, dynamic> json) => _$IdeaFromJson(json);

  Map<String, dynamic> toJson() => _$IdeaToJson(this);
}
