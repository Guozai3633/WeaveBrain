// coverage:ignore-file
// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'timeline_event.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

T _$identity<T>(T value) => value;

final _privateConstructorUsedError = UnsupportedError(
  'It seems like you constructed your class using `MyClass._()`. This constructor is only meant to be used by freezed and you are not supposed to need it nor use it.\nPlease check the documentation here for more information: https://github.com/rrousselGit/freezed#adding-getters-and-methods-to-our-models',
);

TimelineEvent _$TimelineEventFromJson(Map<String, dynamic> json) {
  return _TimelineEvent.fromJson(json);
}

/// @nodoc
mixin _$TimelineEvent {
  String get id => throw _privateConstructorUsedError;
  String get type => throw _privateConstructorUsedError;
  @JsonKey(name: 'user_id')
  String get userId => throw _privateConstructorUsedError;
  @JsonKey(name: 'source_id')
  String get sourceId => throw _privateConstructorUsedError;
  @JsonKey(name: 'workflow_id')
  String? get workflowId => throw _privateConstructorUsedError;
  @JsonKey(name: 'correlation_id')
  String? get correlationId => throw _privateConstructorUsedError;
  String get title => throw _privateConstructorUsedError;
  String get content => throw _privateConstructorUsedError;
  String get status => throw _privateConstructorUsedError;
  @JsonKey(name: 'icon_type')
  String get iconType => throw _privateConstructorUsedError;
  DateTime get timestamp => throw _privateConstructorUsedError;
  Map<String, dynamic>? get metadata => throw _privateConstructorUsedError;

  /// Serializes this TimelineEvent to a JSON map.
  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;

  /// Create a copy of TimelineEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  $TimelineEventCopyWith<TimelineEvent> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $TimelineEventCopyWith<$Res> {
  factory $TimelineEventCopyWith(
    TimelineEvent value,
    $Res Function(TimelineEvent) then,
  ) = _$TimelineEventCopyWithImpl<$Res, TimelineEvent>;
  @useResult
  $Res call({
    String id,
    String type,
    @JsonKey(name: 'user_id') String userId,
    @JsonKey(name: 'source_id') String sourceId,
    @JsonKey(name: 'workflow_id') String? workflowId,
    @JsonKey(name: 'correlation_id') String? correlationId,
    String title,
    String content,
    String status,
    @JsonKey(name: 'icon_type') String iconType,
    DateTime timestamp,
    Map<String, dynamic>? metadata,
  });
}

/// @nodoc
class _$TimelineEventCopyWithImpl<$Res, $Val extends TimelineEvent>
    implements $TimelineEventCopyWith<$Res> {
  _$TimelineEventCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  /// Create a copy of TimelineEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? id = null,
    Object? type = null,
    Object? userId = null,
    Object? sourceId = null,
    Object? workflowId = freezed,
    Object? correlationId = freezed,
    Object? title = null,
    Object? content = null,
    Object? status = null,
    Object? iconType = null,
    Object? timestamp = null,
    Object? metadata = freezed,
  }) {
    return _then(
      _value.copyWith(
            id: null == id
                ? _value.id
                : id // ignore: cast_nullable_to_non_nullable
                      as String,
            type: null == type
                ? _value.type
                : type // ignore: cast_nullable_to_non_nullable
                      as String,
            userId: null == userId
                ? _value.userId
                : userId // ignore: cast_nullable_to_non_nullable
                      as String,
            sourceId: null == sourceId
                ? _value.sourceId
                : sourceId // ignore: cast_nullable_to_non_nullable
                      as String,
            workflowId: freezed == workflowId
                ? _value.workflowId
                : workflowId // ignore: cast_nullable_to_non_nullable
                      as String?,
            correlationId: freezed == correlationId
                ? _value.correlationId
                : correlationId // ignore: cast_nullable_to_non_nullable
                      as String?,
            title: null == title
                ? _value.title
                : title // ignore: cast_nullable_to_non_nullable
                      as String,
            content: null == content
                ? _value.content
                : content // ignore: cast_nullable_to_non_nullable
                      as String,
            status: null == status
                ? _value.status
                : status // ignore: cast_nullable_to_non_nullable
                      as String,
            iconType: null == iconType
                ? _value.iconType
                : iconType // ignore: cast_nullable_to_non_nullable
                      as String,
            timestamp: null == timestamp
                ? _value.timestamp
                : timestamp // ignore: cast_nullable_to_non_nullable
                      as DateTime,
            metadata: freezed == metadata
                ? _value.metadata
                : metadata // ignore: cast_nullable_to_non_nullable
                      as Map<String, dynamic>?,
          )
          as $Val,
    );
  }
}

/// @nodoc
abstract class _$$TimelineEventImplCopyWith<$Res>
    implements $TimelineEventCopyWith<$Res> {
  factory _$$TimelineEventImplCopyWith(
    _$TimelineEventImpl value,
    $Res Function(_$TimelineEventImpl) then,
  ) = __$$TimelineEventImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({
    String id,
    String type,
    @JsonKey(name: 'user_id') String userId,
    @JsonKey(name: 'source_id') String sourceId,
    @JsonKey(name: 'workflow_id') String? workflowId,
    @JsonKey(name: 'correlation_id') String? correlationId,
    String title,
    String content,
    String status,
    @JsonKey(name: 'icon_type') String iconType,
    DateTime timestamp,
    Map<String, dynamic>? metadata,
  });
}

/// @nodoc
class __$$TimelineEventImplCopyWithImpl<$Res>
    extends _$TimelineEventCopyWithImpl<$Res, _$TimelineEventImpl>
    implements _$$TimelineEventImplCopyWith<$Res> {
  __$$TimelineEventImplCopyWithImpl(
    _$TimelineEventImpl _value,
    $Res Function(_$TimelineEventImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of TimelineEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? id = null,
    Object? type = null,
    Object? userId = null,
    Object? sourceId = null,
    Object? workflowId = freezed,
    Object? correlationId = freezed,
    Object? title = null,
    Object? content = null,
    Object? status = null,
    Object? iconType = null,
    Object? timestamp = null,
    Object? metadata = freezed,
  }) {
    return _then(
      _$TimelineEventImpl(
        id: null == id
            ? _value.id
            : id // ignore: cast_nullable_to_non_nullable
                  as String,
        type: null == type
            ? _value.type
            : type // ignore: cast_nullable_to_non_nullable
                  as String,
        userId: null == userId
            ? _value.userId
            : userId // ignore: cast_nullable_to_non_nullable
                  as String,
        sourceId: null == sourceId
            ? _value.sourceId
            : sourceId // ignore: cast_nullable_to_non_nullable
                  as String,
        workflowId: freezed == workflowId
            ? _value.workflowId
            : workflowId // ignore: cast_nullable_to_non_nullable
                  as String?,
        correlationId: freezed == correlationId
            ? _value.correlationId
            : correlationId // ignore: cast_nullable_to_non_nullable
                  as String?,
        title: null == title
            ? _value.title
            : title // ignore: cast_nullable_to_non_nullable
                  as String,
        content: null == content
            ? _value.content
            : content // ignore: cast_nullable_to_non_nullable
                  as String,
        status: null == status
            ? _value.status
            : status // ignore: cast_nullable_to_non_nullable
                  as String,
        iconType: null == iconType
            ? _value.iconType
            : iconType // ignore: cast_nullable_to_non_nullable
                  as String,
        timestamp: null == timestamp
            ? _value.timestamp
            : timestamp // ignore: cast_nullable_to_non_nullable
                  as DateTime,
        metadata: freezed == metadata
            ? _value._metadata
            : metadata // ignore: cast_nullable_to_non_nullable
                  as Map<String, dynamic>?,
      ),
    );
  }
}

/// @nodoc
@JsonSerializable()
class _$TimelineEventImpl implements _TimelineEvent {
  const _$TimelineEventImpl({
    required this.id,
    required this.type,
    @JsonKey(name: 'user_id') required this.userId,
    @JsonKey(name: 'source_id') required this.sourceId,
    @JsonKey(name: 'workflow_id') this.workflowId,
    @JsonKey(name: 'correlation_id') this.correlationId,
    required this.title,
    required this.content,
    required this.status,
    @JsonKey(name: 'icon_type') required this.iconType,
    required this.timestamp,
    final Map<String, dynamic>? metadata,
  }) : _metadata = metadata;

  factory _$TimelineEventImpl.fromJson(Map<String, dynamic> json) =>
      _$$TimelineEventImplFromJson(json);

  @override
  final String id;
  @override
  final String type;
  @override
  @JsonKey(name: 'user_id')
  final String userId;
  @override
  @JsonKey(name: 'source_id')
  final String sourceId;
  @override
  @JsonKey(name: 'workflow_id')
  final String? workflowId;
  @override
  @JsonKey(name: 'correlation_id')
  final String? correlationId;
  @override
  final String title;
  @override
  final String content;
  @override
  final String status;
  @override
  @JsonKey(name: 'icon_type')
  final String iconType;
  @override
  final DateTime timestamp;
  final Map<String, dynamic>? _metadata;
  @override
  Map<String, dynamic>? get metadata {
    final value = _metadata;
    if (value == null) return null;
    if (_metadata is EqualUnmodifiableMapView) return _metadata;
    // ignore: implicit_dynamic_type
    return EqualUnmodifiableMapView(value);
  }

  @override
  String toString() {
    return 'TimelineEvent(id: $id, type: $type, userId: $userId, sourceId: $sourceId, workflowId: $workflowId, correlationId: $correlationId, title: $title, content: $content, status: $status, iconType: $iconType, timestamp: $timestamp, metadata: $metadata)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$TimelineEventImpl &&
            (identical(other.id, id) || other.id == id) &&
            (identical(other.type, type) || other.type == type) &&
            (identical(other.userId, userId) || other.userId == userId) &&
            (identical(other.sourceId, sourceId) ||
                other.sourceId == sourceId) &&
            (identical(other.workflowId, workflowId) ||
                other.workflowId == workflowId) &&
            (identical(other.correlationId, correlationId) ||
                other.correlationId == correlationId) &&
            (identical(other.title, title) || other.title == title) &&
            (identical(other.content, content) || other.content == content) &&
            (identical(other.status, status) || other.status == status) &&
            (identical(other.iconType, iconType) ||
                other.iconType == iconType) &&
            (identical(other.timestamp, timestamp) ||
                other.timestamp == timestamp) &&
            const DeepCollectionEquality().equals(other._metadata, _metadata));
  }

  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  int get hashCode => Object.hash(
    runtimeType,
    id,
    type,
    userId,
    sourceId,
    workflowId,
    correlationId,
    title,
    content,
    status,
    iconType,
    timestamp,
    const DeepCollectionEquality().hash(_metadata),
  );

  /// Create a copy of TimelineEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$TimelineEventImplCopyWith<_$TimelineEventImpl> get copyWith =>
      __$$TimelineEventImplCopyWithImpl<_$TimelineEventImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$TimelineEventImplToJson(this);
  }
}

abstract class _TimelineEvent implements TimelineEvent {
  const factory _TimelineEvent({
    required final String id,
    required final String type,
    @JsonKey(name: 'user_id') required final String userId,
    @JsonKey(name: 'source_id') required final String sourceId,
    @JsonKey(name: 'workflow_id') final String? workflowId,
    @JsonKey(name: 'correlation_id') final String? correlationId,
    required final String title,
    required final String content,
    required final String status,
    @JsonKey(name: 'icon_type') required final String iconType,
    required final DateTime timestamp,
    final Map<String, dynamic>? metadata,
  }) = _$TimelineEventImpl;

  factory _TimelineEvent.fromJson(Map<String, dynamic> json) =
      _$TimelineEventImpl.fromJson;

  @override
  String get id;
  @override
  String get type;
  @override
  @JsonKey(name: 'user_id')
  String get userId;
  @override
  @JsonKey(name: 'source_id')
  String get sourceId;
  @override
  @JsonKey(name: 'workflow_id')
  String? get workflowId;
  @override
  @JsonKey(name: 'correlation_id')
  String? get correlationId;
  @override
  String get title;
  @override
  String get content;
  @override
  String get status;
  @override
  @JsonKey(name: 'icon_type')
  String get iconType;
  @override
  DateTime get timestamp;
  @override
  Map<String, dynamic>? get metadata;

  /// Create a copy of TimelineEvent
  /// with the given fields replaced by the non-null parameter values.
  @override
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$TimelineEventImplCopyWith<_$TimelineEventImpl> get copyWith =>
      throw _privateConstructorUsedError;
}
