import 'package:json_annotation/json_annotation.dart';

part 'reminder.g.dart';

@JsonSerializable()
class Reminder {
  final int id;
  @JsonKey(name: 'user_id')
  final String userId;
  @JsonKey(name: 'project_id')
  final int? projectId;
  @JsonKey(name: 'trigger_time')
  final DateTime triggerTime;
  final String message;
  final String status;
  @JsonKey(name: 'created_at')
  final DateTime? createdAt;

  const Reminder({
    required this.id,
    required this.userId,
    this.projectId,
    required this.triggerTime,
    required this.message,
    this.status = 'pending',
    this.createdAt,
  });

  factory Reminder.fromJson(Map<String, dynamic> json) =>
      _$ReminderFromJson(json);

  Map<String, dynamic> toJson() => _$ReminderToJson(this);
}
