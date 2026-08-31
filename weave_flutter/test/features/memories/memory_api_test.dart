import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient()
    : super(baseUrl: 'http://test', authRepository: AuthRepository());

  String? lastPath;
  Map<String, dynamic>? lastQuery;
  dynamic lastBody;
  ApiResponse getResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse patchResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse postResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse deleteResponse = ApiResponse(data: <String, dynamic>{});

  @override
  Future<ApiResponse> get(
    String path, {
    Map<String, dynamic>? queryParams,
  }) async {
    lastPath = path;
    lastQuery = queryParams;
    return getResponse;
  }

  @override
  Future<ApiResponse> patch(String path, {dynamic body}) async {
    lastPath = path;
    lastBody = body;
    return patchResponse;
  }

  @override
  Future<ApiResponse> post(
    String path, {
    dynamic body,
    Map<String, dynamic>? queryParams,
    Map<String, String>? headers,
  }) async {
    lastPath = path;
    lastBody = body;
    return postResponse;
  }

  @override
  Future<ApiResponse> delete(String path) async {
    lastPath = path;
    return deleteResponse;
  }
}

Map<String, dynamic> _cardJson() => {
      'id': 'card-1',
      'user_id': 'u1',
      'capture_id': 'c1',
      'primary_type': 'idea',
      'title': '继续完善记忆流',
      'summary': '支持搜索与筛选',
      'tags': ['记忆', '搜索'],
      'key_points': ['要点一', '要点二'],
      'processing_status': 'completed',
      'version': 2,
      'is_pinned': true,
      'pinned_at': '2026-08-30T10:00:00Z',
      'created_at': '2026-08-29T08:00:00Z',
      'updated_at': '2026-08-30T10:00:00Z',
    };

Map<String, dynamic> _captureJson() => {
      'id': 'c1',
      'user_id': 'u1',
      'kind': 'text',
      'original_text': '原文内容',
      'captured_at': '2026-08-29T08:00:00Z',
      'captured_at_precision': 'full',
      'source': 'capture',
      'privacy_mode': 'private',
      'client_version': 1,
      'version': 1,
      'lifecycle_status': 'active',
      'created_at': '2026-08-29T08:00:00Z',
      'updated_at': '2026-08-29T08:00:00Z',
    };

Map<String, dynamic> _revisionJson() => {
      'id': 1,
      'user_id': 'u1',
      'capture_id': 'c1',
      'revision': 1,
      'card_version': 1,
      'source': 'fallback',
      'source_revision': 1,
      'changes': {'title': 'x'},
      'provenance': {'title': 'fallback'},
      'created_at': '2026-08-29T08:00:00Z',
    };

void main() {
  group('MemoryCardModel.fromJson', () {
    test('parses all fields', () {
      final card = MemoryCardModel.fromJson(_cardJson());
      expect(card.id, 'card-1');
      expect(card.userId, 'u1');
      expect(card.captureId, 'c1');
      expect(card.primaryType, 'idea');
      expect(card.title, '继续完善记忆流');
      expect(card.summary, '支持搜索与筛选');
      expect(card.tags, ['记忆', '搜索']);
      expect(card.keyPoints, ['要点一', '要点二']);
      expect(card.processingStatus, 'completed');
      expect(card.version, 2);
      expect(card.isPinned, isTrue);
      expect(card.pinnedAt, isNotNull);
      expect(card.createdAt, isNotNull);
      expect(card.updatedAt, isNotNull);
    });

    test('defaults missing optional fields', () {
      final card = MemoryCardModel.fromJson(const {
        'id': 'card-2',
        'user_id': 'u1',
        'capture_id': 'c2',
      });
      expect(card.primaryType, 'uncategorized');
      expect(card.title, '');
      expect(card.summary, isNull);
      expect(card.tags, isEmpty);
      expect(card.keyPoints, isEmpty);
      expect(card.isPinned, isFalse);
      expect(card.version, 1);
    });
  });

  group('CaptureLite.fromJson', () {
    test('parses fields', () {
      final capture = CaptureLite.fromJson(_captureJson());
      expect(capture.id, 'c1');
      expect(capture.kind, 'text');
      expect(capture.originalText, '原文内容');
      expect(capture.capturedAt, isNotNull);
      expect(capture.lifecycleStatus, 'active');
      expect(capture.version, 1);
    });

    test('defaults missing fields', () {
      final capture = CaptureLite.fromJson(const {
        'id': 'c2',
        'user_id': 'u1',
      });
      expect(capture.kind, 'text');
      expect(capture.originalText, isNull);
      expect(capture.lifecycleStatus, 'active');
    });
  });

  group('MemoryListResult.fromJson', () {
    test('parses items and nextCursor', () {
      final result = MemoryListResult.fromJson({
        'items': [
          {'capture': _captureJson(), 'memory_card': _cardJson()},
        ],
        'next_cursor': 'abc',
        'request_id': 'r1',
      });
      expect(result.items, hasLength(1));
      expect(result.items.first.capture.id, 'c1');
      expect(result.items.first.memoryCard.id, 'card-1');
      expect(result.nextCursor, 'abc');
    });

    test('defaults empty items and null cursor', () {
      final result = MemoryListResult.fromJson(const {});
      expect(result.items, isEmpty);
      expect(result.nextCursor, isNull);
    });
  });

  group('MemoryRevision.fromJson', () {
    test('parses fields', () {
      final revision = MemoryRevision.fromJson(_revisionJson());
      expect(revision.id, 1);
      expect(revision.revision, 1);
      expect(revision.cardVersion, 1);
      expect(revision.source, 'fallback');
      expect(revision.sourceRevision, 1);
      expect(revision.changes['title'], 'x');
      expect(revision.provenance['title'], 'fallback');
    });

    test('defaults missing fields', () {
      final revision = MemoryRevision.fromJson(const {});
      expect(revision.id, 0);
      expect(revision.source, 'fallback');
      expect(revision.changes, isEmpty);
    });
  });

  group('MemoryDetail.fromJson', () {
    test('parses audio, transcript and revisions', () {
      final detail = MemoryDetail.fromJson({
        'capture': _captureJson(),
        'memory_card': _cardJson(),
        'audio': {
          'id': 'a1',
          'user_id': 'u1',
          'capture_id': 'c1',
          'mime_type': 'audio/mp4',
          'duration_ms': 12000,
          'size_bytes': 1024,
          'upload_state': 'complete',
          'total_chunks': 4,
          'received_chunks': 4,
          'stt_enabled': true,
          'created_at': '2026-08-29T08:00:00Z',
          'updated_at': '2026-08-29T08:00:00Z',
        },
        'transcript': {
          'id': 5,
          'user_id': 'u1',
          'capture_id': 'c1',
          'revision': 2,
          'text': '转写文本',
          'source': 'user',
          'confidence': 0.9,
          'created_at': '2026-08-29T08:00:00Z',
        },
        'revisions': [_revisionJson()],
        'request_id': 'r1',
      });

      final audio = detail.audio;
      final transcript = detail.transcript;
      expect(audio, isNotNull);
      expect(audio?.durationMs, 12000);
      expect(transcript, isNotNull);
      expect(transcript?.text, '转写文本');
      expect(detail.revisions, hasLength(1));
    });

    test('tolerates missing audio/transcript/revisions', () {
      final detail = MemoryDetail.fromJson({
        'capture': _captureJson(),
        'memory_card': _cardJson(),
      });
      expect(detail.audio, isNull);
      expect(detail.transcript, isNull);
      expect(detail.revisions, isEmpty);
    });
  });

  group('MemoryApi', () {
    test('list sends query params and parses the page', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'items': [
            {'capture': _captureJson(), 'memory_card': _cardJson()},
          ],
          'next_cursor': 'next-page',
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      final result = await api.list(
        cursor: 'abc',
        limit: 10,
        q: '你好',
        kind: 'text',
        primaryType: 'idea',
        lifecycleStatus: 'archived',
        pinned: true,
      );

      expect(client.lastPath, '/memories');
      expect(client.lastQuery!['cursor'], 'abc');
      expect(client.lastQuery!['limit'], 10);
      expect(client.lastQuery!['q'], '你好');
      expect(client.lastQuery!['kind'], 'text');
      expect(client.lastQuery!['primary_type'], 'idea');
      expect(client.lastQuery!['lifecycle_status'], 'archived');
      expect(client.lastQuery!['pinned'], isTrue);
      expect(result.items, hasLength(1));
      expect(result.nextCursor, 'next-page');
    });

    test('list sends no query params when all filters are empty', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {'items': [], 'request_id': 'r1'});
      final api = MemoryApi(client);

      await api.list();

      expect(client.lastQuery, isEmpty);
    });

    test('detail hits /memories/:captureId', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'revisions': [_revisionJson()],
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      final detail = await api.detail('c1');

      expect(client.lastPath, '/memories/c1');
      expect(detail.capture.id, 'c1');
      expect(detail.memoryCard.id, 'card-1');
    });

    test('correct sends only provided fields', () async {
      final client = _FakeApiClient()
        ..patchResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'revision': _revisionJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      final mutation = await api.correct('c1', title: '新标题', tags: ['a']);

      expect(client.lastPath, '/memories/c1');
      expect(client.lastBody, {'title': '新标题', 'tags': ['a']});
      expect(mutation.revision, isNotNull);
    });

    test('correct omits unset fields entirely', () async {
      final client = _FakeApiClient()
        ..patchResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      await api.correct('c1');

      expect(client.lastBody, isEmpty);
    });

    test('addNote posts {text}', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'revision': _revisionJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      final mutation = await api.addNote('c1', text: '后续想法');

      expect(client.lastPath, '/memories/c1/notes');
      expect(client.lastBody, {'text': '后续想法'});
      expect(mutation.memoryCard.id, 'card-1');
    });

    test('setPinned posts {pinned}', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      await api.setPinned('c1', pinned: true);

      expect(client.lastPath, '/memories/c1/pin');
      expect(client.lastBody, {'pinned': true});
    });

    test('archive posts the archive path', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      await api.archive('c1');

      expect(client.lastPath, '/memories/c1/archive');
    });

    test('delete hits the delete path', () async {
      final client = _FakeApiClient()
        ..deleteResponse = ApiResponse(data: {
          'capture': _captureJson(),
          'memory_card': _cardJson(),
          'request_id': 'r1',
        });
      final api = MemoryApi(client);

      await api.delete('c1');

      expect(client.lastPath, '/memories/c1');
    });

    test('throws FormatException when response is not a map', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: <dynamic>['nope']);
      final api = MemoryApi(client);

      expect(() => api.list(), throwsA(isA<FormatException>()));
      expect(() => api.detail('c1'), throwsA(isA<FormatException>()));
    });
  });
}
