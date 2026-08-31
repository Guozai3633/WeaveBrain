// ignore_for_file: invalid_annotation_target

import 'package:freezed_annotation/freezed_annotation.dart';

part 'timeline_event.freezed.dart';
part 'timeline_event.g.dart';

@freezed
class TimelineEvent with _$TimelineEvent {
  const factory TimelineEvent({
    required String id,
    required String type,
    @JsonKey(name: 'user_id') required String userId,
    @JsonKey(name: 'source_id') required String sourceId,
    @JsonKey(name: 'workflow_id') String? workflowId,
    @JsonKey(name: 'correlation_id') String? correlationId,
    required String title,
    required String content,
    required String status,
    @JsonKey(name: 'icon_type') required String iconType,
    required DateTime timestamp,
    Map<String, dynamic>? metadata,
  }) = _TimelineEvent;

  factory TimelineEvent.fromJson(Map<String, dynamic> json) =>
      _$TimelineEventFromJson(json);
}
