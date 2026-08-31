const maxLocalCaptureTextRunes = 100000;

enum LocalSyncState {
  savedLocal('saved_local'),
  pendingSync('pending_sync'),
  syncing('syncing'),
  synced('synced'),
  retryableError('retryable_error'),
  rejected('rejected'),
  conflict('conflict');

  const LocalSyncState(this.wireName);
  final String wireName;

  static LocalSyncState fromWire(String value) {
    return values.firstWhere(
      (state) => state.wireName == value,
      orElse: () => LocalSyncState.savedLocal,
    );
  }
}

enum ServerProcessingStatus {
  unknown('unknown'),
  pending('pending'),
  processing('processing'),
  ready('ready'),
  needsInput('needs_input'),
  failed('failed');

  const ServerProcessingStatus(this.wireName);
  final String wireName;

  static ServerProcessingStatus fromWire(String? value) {
    return values.firstWhere(
      (state) => state.wireName == value,
      orElse: () => ServerProcessingStatus.unknown,
    );
  }
}

class LocalCapture {
  const LocalCapture({
    required this.id,
    required this.text,
    required this.capturedAt,
    required this.source,
    required this.syncState,
    required this.createdAt,
    required this.updatedAt,
    this.ownerUserId,
    this.kind = 'text',
    this.timezone,
    this.privacyMode = 'cloud_allowed',
    this.clientVersion = 1,
    this.serverProcessingStatus = ServerProcessingStatus.unknown,
    this.serverVersion,
    this.retryCount = 0,
    this.nextRetryAt,
    this.lastErrorCode,
    this.trashedAt,
    this.audioAssetId,
    this.audioMimeType,
    this.audioSizeBytes,
    this.audioSha256,
    this.audioTotalChunks,
    this.audioUploadedChunks = const [],
    this.audioLocalPath,
    this.audioDurationMs,
    this.sttEnabled = true,
    this.transcript,
    this.transcriptSource,
    this.transcriptVersion,
  });

  final String id;
  final String? ownerUserId;
  final String text;
  final String kind;
  final DateTime capturedAt;
  final String? timezone;
  final String source;
  final String privacyMode;
  final int clientVersion;
  final LocalSyncState syncState;
  final ServerProcessingStatus serverProcessingStatus;
  final int? serverVersion;
  final int retryCount;
  final DateTime? nextRetryAt;
  final String? lastErrorCode;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? trashedAt;

  // Audio capture metadata (kind == 'audio').
  final String? audioAssetId;
  final String? audioMimeType;
  final int? audioSizeBytes;
  final String? audioSha256;
  final int? audioTotalChunks;
  final List<int> audioUploadedChunks;
  final String? audioLocalPath;
  final int? audioDurationMs;
  final bool sttEnabled;

  // Latest transcript for audio captures.
  final String? transcript;
  final String? transcriptSource;
  final int? transcriptVersion;

  bool get isAudio => kind == 'audio';
  bool get hasAudioMetadata => audioAssetId != null && audioSizeBytes != null;

  bool get isPending =>
      syncState == LocalSyncState.savedLocal ||
      syncState == LocalSyncState.pendingSync ||
      syncState == LocalSyncState.syncing ||
      syncState == LocalSyncState.retryableError;

  /// Chunk indices that still need to be uploaded.
  List<int> get missingAudioChunks {
    final total = audioTotalChunks ?? 0;
    if (total <= 0) return const [];
    final uploaded = audioUploadedChunks.toSet();
    return [
      for (var i = 0; i < total; i++)
        if (!uploaded.contains(i)) i,
    ];
  }

  bool isVisibleTo(String? userId) =>
      ownerUserId == null ? userId == null : ownerUserId == userId;

  Map<String, Object?> toMap() => {
    'id': id,
    'owner_user_id': ownerUserId,
    'kind': kind,
    'text': text,
    'captured_at': capturedAt.toUtc().toIso8601String(),
    'captured_at_precision': 'exact',
    'timezone': timezone,
    'source': source,
    'privacy_mode': privacyMode,
    'client_version': clientVersion,
    'sync_state': syncState.wireName,
    'server_processing_status': serverProcessingStatus.wireName,
    'server_version': serverVersion,
    'retry_count': retryCount,
    'next_retry_at': nextRetryAt?.toUtc().toIso8601String(),
    'last_error_code': lastErrorCode,
    'created_at': createdAt.toUtc().toIso8601String(),
    'updated_at': updatedAt.toUtc().toIso8601String(),
    'trashed_at': trashedAt?.toUtc().toIso8601String(),
    'audio_asset_id': audioAssetId,
    'audio_mime_type': audioMimeType,
    'audio_size_bytes': audioSizeBytes,
    'audio_sha256': audioSha256,
    'audio_total_chunks': audioTotalChunks,
    'audio_uploaded_chunks': audioUploadedChunks.join(','),
    'audio_local_path': audioLocalPath,
    'audio_duration_ms': audioDurationMs,
    'stt_enabled': sttEnabled,
    'transcript': transcript,
    'transcript_source': transcriptSource,
    'transcript_version': transcriptVersion,
  };

  Map<String, Object?> toCreateRequest() => {
    'capture_id': id,
    'kind': kind,
    'text': text,
    'captured_at': capturedAt.toUtc().toIso8601String(),
    'captured_at_precision': 'exact',
    'timezone': timezone,
    'source': source,
    'privacy_mode': privacyMode,
    'client_version': clientVersion,
  };

  factory LocalCapture.fromMap(Map<String, Object?> map) {
    DateTime parseRequired(String key) {
      final value = map[key] as String?;
      if (value == null) throw FormatException('Missing $key');
      return DateTime.parse(value).toUtc();
    }

    DateTime? parseOptional(String key) {
      final value = map[key] as String?;
      return value == null ? null : DateTime.parse(value).toUtc();
    }

    List<int> parseUploadedChunks() {
      final raw = map['audio_uploaded_chunks'] as String?;
      if (raw == null || raw.isEmpty) return const [];
      return [
        for (final part in raw.split(','))
          if (int.tryParse(part) != null) int.parse(part),
      ];
    }

    return LocalCapture(
      id: map['id']! as String,
      ownerUserId: map['owner_user_id'] as String?,
      kind: (map['kind'] as String?) ?? 'text',
      text: map['text']! as String,
      capturedAt: parseRequired('captured_at'),
      timezone: map['timezone'] as String?,
      source: map['source']! as String,
      privacyMode: (map['privacy_mode'] as String?) ?? 'cloud_allowed',
      clientVersion: (map['client_version'] as num?)?.toInt() ?? 1,
      syncState: LocalSyncState.fromWire(
        (map['sync_state'] as String?) ?? 'saved_local',
      ),
      serverProcessingStatus: ServerProcessingStatus.fromWire(
        map['server_processing_status'] as String?,
      ),
      serverVersion: (map['server_version'] as num?)?.toInt(),
      retryCount: (map['retry_count'] as num?)?.toInt() ?? 0,
      nextRetryAt: parseOptional('next_retry_at'),
      lastErrorCode: map['last_error_code'] as String?,
      createdAt: parseRequired('created_at'),
      updatedAt: parseRequired('updated_at'),
      trashedAt: parseOptional('trashed_at'),
      audioAssetId: map['audio_asset_id'] as String?,
      audioMimeType: map['audio_mime_type'] as String?,
      audioSizeBytes: (map['audio_size_bytes'] as num?)?.toInt(),
      audioSha256: map['audio_sha256'] as String?,
      audioTotalChunks: (map['audio_total_chunks'] as num?)?.toInt(),
      audioUploadedChunks: parseUploadedChunks(),
      audioLocalPath: map['audio_local_path'] as String?,
      audioDurationMs: (map['audio_duration_ms'] as num?)?.toInt(),
      sttEnabled: (map['stt_enabled'] as bool?) ?? true,
      transcript: map['transcript'] as String?,
      transcriptSource: map['transcript_source'] as String?,
      transcriptVersion: (map['transcript_version'] as num?)?.toInt(),
    );
  }

  LocalCapture copyWith({
    String? ownerUserId,
    String? kind,
    LocalSyncState? syncState,
    ServerProcessingStatus? serverProcessingStatus,
    int? serverVersion,
    int? retryCount,
    DateTime? nextRetryAt,
    bool clearNextRetryAt = false,
    String? lastErrorCode,
    bool clearLastErrorCode = false,
    DateTime? updatedAt,
    DateTime? trashedAt,
    String? audioAssetId,
    bool clearAudioAssetId = false,
    String? audioMimeType,
    bool clearAudioMimeType = false,
    int? audioSizeBytes,
    bool clearAudioSizeBytes = false,
    String? audioSha256,
    bool clearAudioSha256 = false,
    int? audioTotalChunks,
    bool clearAudioTotalChunks = false,
    List<int>? audioUploadedChunks,
    String? audioLocalPath,
    bool clearAudioLocalPath = false,
    int? audioDurationMs,
    bool clearAudioDurationMs = false,
    bool? sttEnabled,
    String? transcript,
    bool clearTranscript = false,
    String? transcriptSource,
    int? transcriptVersion,
  }) {
    return LocalCapture(
      id: id,
      ownerUserId: ownerUserId ?? this.ownerUserId,
      kind: kind ?? this.kind,
      text: text,
      capturedAt: capturedAt,
      timezone: timezone,
      source: source,
      privacyMode: privacyMode,
      clientVersion: clientVersion,
      syncState: syncState ?? this.syncState,
      serverProcessingStatus:
          serverProcessingStatus ?? this.serverProcessingStatus,
      serverVersion: serverVersion ?? this.serverVersion,
      retryCount: retryCount ?? this.retryCount,
      nextRetryAt: clearNextRetryAt ? null : (nextRetryAt ?? this.nextRetryAt),
      lastErrorCode: clearLastErrorCode
          ? null
          : (lastErrorCode ?? this.lastErrorCode),
      createdAt: createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      trashedAt: trashedAt ?? this.trashedAt,
      audioAssetId: clearAudioAssetId ? null : (audioAssetId ?? this.audioAssetId),
      audioMimeType: clearAudioMimeType
          ? null
          : (audioMimeType ?? this.audioMimeType),
      audioSizeBytes: clearAudioSizeBytes
          ? null
          : (audioSizeBytes ?? this.audioSizeBytes),
      audioSha256: clearAudioSha256 ? null : (audioSha256 ?? this.audioSha256),
      audioTotalChunks: clearAudioTotalChunks
          ? null
          : (audioTotalChunks ?? this.audioTotalChunks),
      audioUploadedChunks: audioUploadedChunks ?? this.audioUploadedChunks,
      audioLocalPath: clearAudioLocalPath
          ? null
          : (audioLocalPath ?? this.audioLocalPath),
      audioDurationMs: clearAudioDurationMs
          ? null
          : (audioDurationMs ?? this.audioDurationMs),
      sttEnabled: sttEnabled ?? this.sttEnabled,
      transcript: clearTranscript ? null : (transcript ?? this.transcript),
      transcriptSource: transcriptSource ?? this.transcriptSource,
      transcriptVersion: transcriptVersion ?? this.transcriptVersion,
    );
  }
}

class LocalCaptureAsset {
  const LocalCaptureAsset({
    required this.id,
    required this.captureId,
    required this.kind,
    required this.localRef,
    required this.createdAt,
  });

  final String id;
  final String captureId;
  final String kind;
  final String localRef;
  final DateTime createdAt;

  Map<String, Object?> toMap() => {
    'id': id,
    'capture_id': captureId,
    'kind': kind,
    'local_ref': localRef,
    'created_at': createdAt.toUtc().toIso8601String(),
  };
}
