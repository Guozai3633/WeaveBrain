// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reminder.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Reminder _$ReminderFromJson(Map<String, dynamic> json) => Reminder(
  id: (json['id'] as num).toInt(),
  userId: json['user_id'] as String,
  projectId: (json['project_id'] as num?)?.toInt(),
  triggerTime: DateTime.parse(json['trigger_time'] as String),
  message: json['message'] as String,
  status: json['status'] as String? ?? 'pending',
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
);

Map<String, dynamic> _$ReminderToJson(Reminder instance) => <String, dynamic>{
  'id': instance.id,
  'user_id': instance.userId,
  'project_id': instance.projectId,
  'trigger_time': instance.triggerTime.toIso8601String(),
  'message': instance.message,
  'status': instance.status,
  'created_at': instance.createdAt?.toIso8601String(),
};
