// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'timeline_event.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_$TimelineEventImpl _$$TimelineEventImplFromJson(Map<String, dynamic> json) =>
    _$TimelineEventImpl(
      id: json['id'] as String,
      type: json['type'] as String,
      userId: json['user_id'] as String,
      sourceId: json['source_id'] as String,
      workflowId: json['workflow_id'] as String?,
      correlationId: json['correlation_id'] as String?,
      title: json['title'] as String,
      content: json['content'] as String,
      status: json['status'] as String,
      iconType: json['icon_type'] as String,
      timestamp: DateTime.parse(json['timestamp'] as String),
      metadata: json['metadata'] as Map<String, dynamic>?,
    );

Map<String, dynamic> _$$TimelineEventImplToJson(_$TimelineEventImpl instance) =>
    <String, dynamic>{
      'id': instance.id,
      'type': instance.type,
      'user_id': instance.userId,
      'source_id': instance.sourceId,
      'workflow_id': instance.workflowId,
      'correlation_id': instance.correlationId,
      'title': instance.title,
      'content': instance.content,
      'status': instance.status,
      'icon_type': instance.iconType,
      'timestamp': instance.timestamp.toIso8601String(),
      'metadata': instance.metadata,
    };
