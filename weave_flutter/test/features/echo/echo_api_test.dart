import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient() : super(baseUrl: 'http://test', authRepository: AuthRepository());

  String? lastPath;
  Map<String, dynamic>? lastBody;
  ApiResponse getResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse patchResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse postResponse = ApiResponse(data: <String, dynamic>{});

  @override
  Future<ApiResponse> get(
    String path, {
    Map<String, dynamic>? queryParams,
  }) async {
    lastPath = path;
    return getResponse;
  }

  @override
  Future<ApiResponse> patch(String path, {dynamic body}) async {
    lastPath = path;
    lastBody = body as Map<String, dynamic>?;
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
    return postResponse;
  }
}

void main() {
  group('echoCadenceStepDays', () {
    test('maps cadence to days', () {
      expect(echoCadenceStepDays('daily'), 1);
      expect(echoCadenceStepDays('every_other_day'), 2);
      expect(echoCadenceStepDays('weekly'), 7);
      expect(echoCadenceStepDays('bogus'), 1); // 未知回退到 daily。
    });
  });

  group('EchoSettings.fromJson', () {
    test('parses enabled/cadence/revision', () {
      final settings = EchoSettings.fromJson(const {
        'user_id': 'u1',
        'enabled': true,
        'cadence': 'weekly',
        'revision': 3,
      });
      expect(settings.userId, 'u1');
      expect(settings.enabled, isTrue);
      expect(settings.cadence, 'weekly');
      expect(settings.revision, 3);
    });

    test('defaults when fields are missing', () {
      final settings = EchoSettings.fromJson(const <String, dynamic>{});
      expect(settings.userId, isNull);
      expect(settings.enabled, isFalse);
      expect(settings.cadence, echoCadenceDaily);
      expect(settings.revision, 0);
    });
  });

  group('EchoMemory.fromJson', () {
    test('parses memory payload with optional fields', () {
      final memory = EchoMemory.fromJson(const {
        'capture_id': 'c1',
        'kind': 'text',
        'title': '标题',
        'summary': '摘要',
        'primary_type': 'idea',
        'captured_at': '2026-08-01T00:00:00Z',
        'is_pinned': true,
      });
      expect(memory.captureId, 'c1');
      expect(memory.title, '标题');
      expect(memory.summary, '摘要');
      expect(memory.primaryType, 'idea');
      expect(memory.isPinned, isTrue);
    });

    test('defaults captureId to empty and primaryType to uncategorized', () {
      final memory = EchoMemory.fromJson(const <String, dynamic>{});
      expect(memory.captureId, isEmpty);
      expect(memory.primaryType, 'uncategorized');
      expect(memory.isPinned, isFalse);
      expect(memory.summary, isNull);
      expect(memory.capturedAt, isNull);
    });
  });

  group('CurrentEchoResult.fromJson', () {
    test('parses a full response with an open echo', () {
      final result = CurrentEchoResult.fromJson(const {
        'enabled': true,
        'cadence': 'daily',
        'revision': 2,
        'echo': {
          'id': 'e1',
          'status': 'open',
          'reason': {'code': 'pinned', 'text': '这条记忆被你置顶过'},
          'memory': {'capture_id': 'c1', 'title': 'T', 'primary_type': 'idea'},
          'created_at': '2026-09-01T00:00:00Z',
        },
        'request_id': 'req-1',
      });
      expect(result.enabled, isTrue);
      expect(result.revision, 2);
      expect(result.echo, isNotNull);
      expect(result.echo!.id, 'e1');
      expect(result.echo!.reason.code, 'pinned');
      expect(result.echo!.memory.captureId, 'c1');
      expect(result.nextDueAt, isNull);
    });

    test('parses an empty response with next_due_at', () {
      final result = CurrentEchoResult.fromJson(const {
        'enabled': true,
        'cadence': 'daily',
        'revision': 2,
        'next_due_at': '2026-09-02T12:00:00Z',
        'request_id': 'req-1',
      });
      expect(result.echo, isNull);
      expect(result.nextDueAt, isNotNull);
      expect(result.emptyReason, isNull);
    });
  });

  group('EchoFeedbackResult.fromJson', () {
    test('parses verdict and next due', () {
      final result = EchoFeedbackResult.fromJson(const {
        'echo_id': 'e1',
        'status': 'done',
        'next_due_at': '2026-09-03T12:00:00Z',
        'request_id': 'req-1',
      });
      expect(result.echoId, 'e1');
      expect(result.status, 'done');
      expect(result.nextDueAt, isNotNull);
    });
  });

  group('EchoApi', () {
    test('getCurrent calls /echoes/current and parses', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'enabled': true,
          'cadence': 'daily',
          'revision': 1,
          'echo': {
            'id': 'e1',
            'status': 'open',
            'reason': {'code': 'oldest', 'text': '较早记下的想法'},
            'memory': {'capture_id': 'c9', 'title': '久远的', 'primary_type': 'idea'},
            'created_at': '2026-09-01T00:00:00Z',
          },
          'request_id': 'r',
        });
      final api = EchoApi(client);

      final result = await api.getCurrent();

      expect(client.lastPath, '/echoes/current');
      expect(result.enabled, isTrue);
      expect(result.echo!.memory.captureId, 'c9');
    });

    test('submitFeedback posts verdict to the echo route', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'echo_id': 'e1',
          'status': 'done',
          'next_due_at': '2026-09-03T12:00:00Z',
          'request_id': 'r',
        });
      final api = EchoApi(client);

      final result = await api.submitFeedback(echoId: 'e1', verdict: 'done');

      expect(client.lastPath, '/echoes/e1/feedback');
      expect(result.status, 'done');
    });
  });

  group('EchoSettingsApi', () {
    test('get calls /users/me/echo-settings', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'settings': {'enabled': true, 'cadence': 'weekly', 'revision': 4},
          'request_id': 'r',
        });
      final api = EchoSettingsApi(client);

      final result = await api.get();

      expect(client.lastPath, '/users/me/echo-settings');
      expect(result.settings.enabled, isTrue);
      expect(result.settings.cadence, 'weekly');
      expect(result.settings.revision, 4);
    });

    test('update sends expected_revision and only present fields', () async {
      final client = _FakeApiClient()
        ..patchResponse = ApiResponse(data: {
          'settings': {'enabled': true, 'cadence': 'daily', 'revision': 1},
          'request_id': 'r',
        });
      final api = EchoSettingsApi(client);

      final updated = await api.update(
        expectedRevision: 0,
        enabled: true,
      );

      expect(client.lastPath, '/users/me/echo-settings');
      expect(client.lastBody, {'expected_revision': 0, 'enabled': true});
      expect(updated.enabled, isTrue);
      expect(updated.revision, 1);
    });

    test('get throws FormatException when data is not a map', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: <dynamic>['nope']);
      final api = EchoSettingsApi(client);

      expect(() => api.get(), throwsA(isA<FormatException>()));
    });
  });
}
