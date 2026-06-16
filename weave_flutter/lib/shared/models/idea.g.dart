// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'idea.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Idea _$IdeaFromJson(Map<String, dynamic> json) => Idea(
  id: (json['id'] as num).toInt(),
  projectId: (json['project_id'] as num).toInt(),
  userId: json['user_id'] as String,
  rawInput: json['raw_input'] as String,
  structuredData: json['structured_data'] as Map<String, dynamic>?,
  tags:
      (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const [],
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
  updatedAt: json['updated_at'] == null
      ? null
      : DateTime.parse(json['updated_at'] as String),
);

Map<String, dynamic> _$IdeaToJson(Idea instance) => <String, dynamic>{
  'id': instance.id,
  'project_id': instance.projectId,
  'user_id': instance.userId,
  'raw_input': instance.rawInput,
  'structured_data': instance.structuredData,
  'tags': instance.tags,
  'created_at': instance.createdAt?.toIso8601String(),
  'updated_at': instance.updatedAt?.toIso8601String(),
};
