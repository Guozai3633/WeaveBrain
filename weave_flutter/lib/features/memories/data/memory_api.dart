import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';

/// 记忆卡模型，对应后端 `entity.MemoryCard`。
class MemoryCardModel {
  const MemoryCardModel({
    required this.id,
    required this.userId,
    required this.captureId,
    required this.primaryType,
    required this.title,
    required this.processingStatus,
    required this.version,
    required this.isPinned,
    required this.createdAt,
    required this.updatedAt,
    this.summary,
    this.tags = const [],
    this.keyPoints = const [],
    this.pinnedAt,
  });

  factory MemoryCardModel.fromJson(Map<String, dynamic> json) {
    return MemoryCardModel(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      captureId: json['capture_id'] as String,
      primaryType: json['primary_type'] as String? ?? 'uncategorized',
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String?,
      tags: _stringList(json['tags']),
      keyPoints: _stringList(json['key_points']),
      processingStatus: json['processing_status'] as String? ?? 'pending',
      version: (json['version'] as num?)?.toInt() ?? 1,
      isPinned: json['is_pinned'] as bool? ?? false,
      pinnedAt: _parseDateTime(json['pinned_at']),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseDateTime(json['updated_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String userId;
  final String captureId;
  final String primaryType;
  final String title;
  final String? summary;
  final List<String> tags;
  final List<String> keyPoints;
  final String processingStatus;
  final int version;
  final bool isPinned;
  final DateTime? pinnedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  MemoryCardModel copyWith({
    String? primaryType,
    String? title,
    String? summary,
    List<String>? tags,
    List<String>? keyPoints,
    int? version,
    bool? isPinned,
    DateTime? pinnedAt,
  }) {
    return MemoryCardModel(
      id: id,
      userId: userId,
      captureId: captureId,
      primaryType: primaryType ?? this.primaryType,
      title: title ?? this.title,
      summary: summary ?? this.summary,
      tags: tags ?? this.tags,
      keyPoints: keyPoints ?? this.keyPoints,
      processingStatus: processingStatus,
      version: version ?? this.version,
      isPinned: isPinned ?? this.isPinned,
      pinnedAt: pinnedAt ?? this.pinnedAt,
      createdAt: createdAt,
      updatedAt: updatedAt,
    );
  }
}

/// 捕获记录（记忆流列表与详情中的 capture 字段），对应后端 `entity.Capture`。
class CaptureLite {
  const CaptureLite({
    required this.id,
    required this.userId,
    required this.kind,
    required this.capturedAtPrecision,
    required this.source,
    required this.privacyMode,
    required this.clientVersion,
    required this.version,
    required this.lifecycleStatus,
    required this.createdAt,
    required this.updatedAt,
    this.originalText,
    this.capturedAt,
    this.timezone,
    this.collectionId,
  });

  factory CaptureLite.fromJson(Map<String, dynamic> json) {
    return CaptureLite(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      kind: json['kind'] as String? ?? 'text',
      originalText: json['original_text'] as String?,
      capturedAt: _parseDateTime(json['captured_at']),
      capturedAtPrecision: json['captured_at_precision'] as String? ?? 'full',
      timezone: json['timezone'] as String?,
      source: json['source'] as String? ?? 'capture',
      collectionId: (json['collection_id'] as num?)?.toInt(),
      privacyMode: json['privacy_mode'] as String? ?? 'private',
      clientVersion: (json['client_version'] as num?)?.toInt() ?? 1,
      version: (json['version'] as num?)?.toInt() ?? 1,
      lifecycleStatus: json['lifecycle_status'] as String? ?? 'active',
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseDateTime(json['updated_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String userId;
  final String kind;
  final String? originalText;
  final DateTime? capturedAt;
  final String capturedAtPrecision;
  final String? timezone;
  final String source;
  final int? collectionId;
  final String privacyMode;
  final int clientVersion;
  final int version;
  final String lifecycleStatus;
  final DateTime createdAt;
  final DateTime updatedAt;
}

/// 记忆流列表行：一条捕获 + 其记忆卡。
class MemoryListEntry {
  const MemoryListEntry({required this.capture, required this.memoryCard});

  factory MemoryListEntry.fromJson(Map<String, dynamic> json) {
    return MemoryListEntry(
      capture: CaptureLite.fromJson(_asMap(json['capture'])),
      memoryCard: MemoryCardModel.fromJson(_asMap(json['memory_card'])),
    );
  }

  final CaptureLite capture;
  final MemoryCardModel memoryCard;
}

/// 记忆流分页结果。
class MemoryListResult {
  const MemoryListResult({required this.items, this.nextCursor});

  factory MemoryListResult.fromJson(Map<String, dynamic> json) {
    final rawItems = json['items'];
    final items = rawItems is List
        ? rawItems
              .whereType<Map<String, dynamic>>()
              .map(MemoryListEntry.fromJson)
              .toList()
        : const <MemoryListEntry>[];
    return MemoryListResult(
      items: items,
      nextCursor: json['next_cursor'] as String?,
    );
  }

  final List<MemoryListEntry> items;
  final String? nextCursor;
}

/// 记忆卡修订记录，对应后端 `entity.EnrichmentRevision`。
class MemoryRevision {
  const MemoryRevision({
    required this.id,
    required this.userId,
    required this.captureId,
    required this.revision,
    required this.cardVersion,
    required this.source,
    required this.sourceRevision,
    required this.createdAt,
    this.changes = const {},
    this.provenance = const {},
  });

  factory MemoryRevision.fromJson(Map<String, dynamic> json) {
    return MemoryRevision(
      id: (json['id'] as num?)?.toInt() ?? 0,
      userId: json['user_id'] as String? ?? '',
      captureId: json['capture_id'] as String? ?? '',
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      cardVersion: (json['card_version'] as num?)?.toInt() ?? 0,
      source: json['source'] as String? ?? 'fallback',
      sourceRevision: (json['source_revision'] as num?)?.toInt() ?? 0,
      changes: _asMap(json['changes']),
      provenance: _asMap(json['provenance']),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
    );
  }

  final int id;
  final String userId;
  final String captureId;
  final int revision;
  final int cardVersion;
  final String source;
  final int sourceRevision;
  final Map<String, dynamic> changes;
  final Map<String, dynamic> provenance;
  final DateTime createdAt;
}

/// 音频资产模型，对应后端 `entity.AudioAsset`。
class AudioAssetModel {
  const AudioAssetModel({
    required this.id,
    required this.userId,
    required this.captureId,
    required this.mimeType,
    required this.sizeBytes,
    required this.uploadState,
    required this.totalChunks,
    required this.receivedChunks,
    required this.sttEnabled,
    required this.createdAt,
    required this.updatedAt,
    this.durationMs,
    this.sha256,
  });

  factory AudioAssetModel.fromJson(Map<String, dynamic> json) {
    return AudioAssetModel(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      captureId: json['capture_id'] as String,
      mimeType: json['mime_type'] as String? ?? '',
      durationMs: (json['duration_ms'] as num?)?.toInt(),
      sizeBytes: (json['size_bytes'] as num?)?.toInt() ?? 0,
      sha256: json['sha256'] as String?,
      uploadState: json['upload_state'] as String? ?? 'complete',
      totalChunks: (json['total_chunks'] as num?)?.toInt() ?? 0,
      receivedChunks: (json['received_chunks'] as num?)?.toInt() ?? 0,
      sttEnabled: json['stt_enabled'] as bool? ?? false,
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseDateTime(json['updated_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String userId;
  final String captureId;
  final String mimeType;
  final int? durationMs;
  final int sizeBytes;
  final String? sha256;
  final String uploadState;
  final int totalChunks;
  final int receivedChunks;
  final bool sttEnabled;
  final DateTime createdAt;
  final DateTime updatedAt;
}

/// 转写修订模型，对应后端 `entity.TranscriptRevision`。
class TranscriptRevisionModel {
  const TranscriptRevisionModel({
    required this.id,
    required this.userId,
    required this.captureId,
    required this.revision,
    required this.text,
    required this.source,
    required this.createdAt,
    this.confidence,
  });

  factory TranscriptRevisionModel.fromJson(Map<String, dynamic> json) {
    return TranscriptRevisionModel(
      id: (json['id'] as num?)?.toInt() ?? 0,
      userId: json['user_id'] as String? ?? '',
      captureId: json['capture_id'] as String? ?? '',
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      text: json['text'] as String? ?? '',
      source: json['source'] as String? ?? 'stt',
      confidence: (json['confidence'] as num?)?.toDouble(),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
    );
  }

  final int id;
  final String userId;
  final String captureId;
  final int revision;
  final String text;
  final String source;
  final double? confidence;
  final DateTime createdAt;
}

/// 记忆详情聚合视图。
class MemoryDetail {
  const MemoryDetail({
    required this.capture,
    required this.memoryCard,
    required this.revisions,
    this.audio,
    this.transcript,
  });

  factory MemoryDetail.fromJson(Map<String, dynamic> json) {
    final rawAudio = json['audio'];
    final rawTranscript = json['transcript'];
    final rawRevisions = json['revisions'];
    return MemoryDetail(
      capture: CaptureLite.fromJson(_asMap(json['capture'])),
      memoryCard: MemoryCardModel.fromJson(_asMap(json['memory_card'])),
      audio: rawAudio is Map<String, dynamic>
          ? AudioAssetModel.fromJson(rawAudio)
          : null,
      transcript: rawTranscript is Map<String, dynamic>
          ? TranscriptRevisionModel.fromJson(rawTranscript)
          : null,
      revisions: rawRevisions is List
          ? rawRevisions
                .whereType<Map<String, dynamic>>()
                .map(MemoryRevision.fromJson)
                .toList()
          : const <MemoryRevision>[],
    );
  }

  final CaptureLite capture;
  final MemoryCardModel memoryCard;
  final AudioAssetModel? audio;
  final TranscriptRevisionModel? transcript;
  final List<MemoryRevision> revisions;

  MemoryDetail copyWith({
    MemoryCardModel? memoryCard,
    List<MemoryRevision>? revisions,
  }) {
    return MemoryDetail(
      capture: capture,
      memoryCard: memoryCard ?? this.memoryCard,
      audio: audio,
      transcript: transcript,
      revisions: revisions ?? this.revisions,
    );
  }
}

/// 变更操作（修正 / 续写 / 置顶 / 归档 / 删除）的统一响应。
class MemoryMutation {
  const MemoryMutation({
    required this.capture,
    required this.memoryCard,
    this.revision,
  });

  factory MemoryMutation.fromJson(Map<String, dynamic> json) {
    final rawRevision = json['revision'];
    return MemoryMutation(
      capture: CaptureLite.fromJson(_asMap(json['capture'])),
      memoryCard: MemoryCardModel.fromJson(_asMap(json['memory_card'])),
      revision: rawRevision is Map<String, dynamic>
          ? MemoryRevision.fromJson(rawRevision)
          : null,
    );
  }

  final CaptureLite capture;
  final MemoryCardModel memoryCard;
  final MemoryRevision? revision;
}

/// 记忆流数据访问抽象，便于测试替换。
abstract interface class MemoryGateway {
  /// 记忆流列表，支持分页游标、搜索与筛选。
  Future<MemoryListResult> list({
    String? cursor,
    int? limit,
    String? q,
    String? kind,
    String? primaryType,
    String? lifecycleStatus,
    bool? pinned,
  });

  /// 按 captureId 加载记忆详情。
  Future<MemoryDetail> detail(String captureId);

  /// 修正记忆字段（只发送非 null 字段）。
  Future<MemoryMutation> correct(
    String captureId, {
    String? title,
    String? summary,
    String? primaryType,
    List<String>? tags,
    List<String>? keyPoints,
  });

  /// 续写：追加一条笔记，生成新的 user 来源修订。
  Future<MemoryMutation> addNote(String captureId, {required String text});

  /// 置顶 / 取消置顶。
  Future<MemoryMutation> setPinned(String captureId, {required bool pinned});

  /// 归档（幂等）。
  Future<MemoryMutation> archive(String captureId);

  /// 软删除（移动到 trash）。
  Future<MemoryMutation> delete(String captureId);
}

class MemoryApi implements MemoryGateway {
  const MemoryApi(this._client);

  final ApiClient _client;

  @override
  Future<MemoryListResult> list({
    String? cursor,
    int? limit,
    String? q,
    String? kind,
    String? primaryType,
    String? lifecycleStatus,
    bool? pinned,
  }) async {
    final queryParams = <String, dynamic>{};
    if (cursor != null) queryParams['cursor'] = cursor;
    if (limit != null) queryParams['limit'] = limit;
    if (q != null && q.isNotEmpty) queryParams['q'] = q;
    if (kind != null && kind.isNotEmpty) queryParams['kind'] = kind;
    if (primaryType != null && primaryType.isNotEmpty) {
      queryParams['primary_type'] = primaryType;
    }
    if (lifecycleStatus != null && lifecycleStatus.isNotEmpty) {
      queryParams['lifecycle_status'] = lifecycleStatus;
    }
    if (pinned != null) queryParams['pinned'] = pinned;

    final response = await _client.get('/memories', queryParams: queryParams);
    final data = _requireMap(response.data);
    return MemoryListResult.fromJson(data);
  }

  @override
  Future<MemoryDetail> detail(String captureId) async {
    final response = await _client.get('/memories/$captureId');
    final data = _requireMap(response.data);
    return MemoryDetail.fromJson(data);
  }

  @override
  Future<MemoryMutation> correct(
    String captureId, {
    String? title,
    String? summary,
    String? primaryType,
    List<String>? tags,
    List<String>? keyPoints,
  }) async {
    final body = <String, dynamic>{};
    if (title != null) body['title'] = title;
    if (summary != null) body['summary'] = summary;
    if (primaryType != null) body['primary_type'] = primaryType;
    if (tags != null) body['tags'] = tags;
    if (keyPoints != null) body['key_points'] = keyPoints;

    final response = await _client.patch(
      '/memories/$captureId',
      body: body,
    );
    final data = _requireMap(response.data);
    return MemoryMutation.fromJson(data);
  }

  @override
  Future<MemoryMutation> addNote(String captureId, {required String text}) async {
    final response = await _client.post(
      '/memories/$captureId/notes',
      body: <String, dynamic>{'text': text},
    );
    final data = _requireMap(response.data);
    return MemoryMutation.fromJson(data);
  }

  @override
  Future<MemoryMutation> setPinned(String captureId, {required bool pinned}) async {
    final response = await _client.post(
      '/memories/$captureId/pin',
      body: <String, dynamic>{'pinned': pinned},
    );
    final data = _requireMap(response.data);
    return MemoryMutation.fromJson(data);
  }

  @override
  Future<MemoryMutation> archive(String captureId) async {
    final response = await _client.post('/memories/$captureId/archive');
    final data = _requireMap(response.data);
    return MemoryMutation.fromJson(data);
  }

  @override
  Future<MemoryMutation> delete(String captureId) async {
    final response = await _client.delete('/memories/$captureId');
    final data = _requireMap(response.data);
    return MemoryMutation.fromJson(data);
  }

  static Map<String, dynamic> _requireMap(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data;
    }
    throw const FormatException('无效的记忆响应');
  }
}

/// v3 ApiClient（与 ai-settings / capture 特性共用同一 host）。
final memoryApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final memoryGatewayProvider = Provider<MemoryGateway>((ref) {
  return MemoryApi(ref.watch(memoryApiClientProvider));
});

// ---- JSON 辅助函数 ----

DateTime? _parseDateTime(dynamic value) {
  if (value is String) {
    return DateTime.tryParse(value);
  }
  return null;
}

List<String> _stringList(dynamic value) {
  if (value is List) {
    return value.whereType<String>().toList();
  }
  return const <String>[];
}

Map<String, dynamic> _asMap(dynamic value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  return const <String, dynamic>{};
}
