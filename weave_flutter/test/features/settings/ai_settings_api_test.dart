import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/settings/data/ai_settings_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient()
    : super(baseUrl: 'http://test', authRepository: AuthRepository());

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
  group('AISettings.fromJson', () {
    test('parses switches and revision', () {
      final settings = AISettings.fromJson(const {
        'user_id': 'u1',
        'ai_memory_enabled': true,
        'ai_completion_enabled': false,
        'speech_to_text_enabled': true,
        'cloud_text_allowed': false,
        'cloud_audio_allowed': true,
        'revision': 3,
      });
      expect(settings.userId, 'u1');
      expect(settings.aiMemoryEnabled, isTrue);
      expect(settings.aiCompletionEnabled, isFalse);
      expect(settings.speechToTextEnabled, isTrue);
      expect(settings.cloudTextAllowed, isFalse);
      expect(settings.cloudAudioAllowed, isTrue);
      expect(settings.revision, 3);
    });

    test('defaults to all-false and revision 0 when fields are missing', () {
      final settings = AISettings.fromJson(const <String, dynamic>{});
      expect(settings.aiMemoryEnabled, isFalse);
      expect(settings.aiCompletionEnabled, isFalse);
      expect(settings.speechToTextEnabled, isFalse);
      expect(settings.cloudTextAllowed, isFalse);
      expect(settings.cloudAudioAllowed, isFalse);
      expect(settings.revision, 0);
      expect(settings.userId, isNull);
    });
  });

  group('AISettingsResult.fromJson', () {
    test('parses the GET envelope', () {
      final result = AISettingsResult.fromJson(const {
        'settings': {'ai_memory_enabled': true, 'revision': 2},
        'pending_reorganize': 5,
        'request_id': 'req-1',
      });
      expect(result.settings.aiMemoryEnabled, isTrue);
      expect(result.settings.revision, 2);
      expect(result.pendingReorganize, 5);
    });

    test('defaults pending count to 0 and settings to empty', () {
      final result = AISettingsResult.fromJson(const <String, dynamic>{});
      expect(result.pendingReorganize, 0);
      expect(result.settings.aiMemoryEnabled, isFalse);
    });
  });

  group('AISettingsApi', () {
    test('get parses settings and pending count', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'settings': {'ai_memory_enabled': true, 'revision': 4},
          'pending_reorganize': 9,
          'request_id': 'r',
        });
      final api = AISettingsApi(client);

      final result = await api.get();

      expect(client.lastPath, '/users/me/ai-settings');
      expect(result.settings.revision, 4);
      expect(result.pendingReorganize, 9);
    });

    test('update sends expected_revision and only present fields', () async {
      final client = _FakeApiClient()
        ..patchResponse = ApiResponse(data: {
          'settings': {
            'ai_memory_enabled': true,
            'ai_completion_enabled': false,
            'revision': 1,
          },
          'request_id': 'r',
        });
      final api = AISettingsApi(client);

      final updated = await api.update(
        expectedRevision: 0,
        aiMemoryEnabled: true,
      );

      expect(client.lastPath, '/users/me/ai-settings');
      expect(client.lastBody, {
        'expected_revision': 0,
        'ai_memory_enabled': true,
      });
      expect(updated.revision, 1);
      expect(updated.aiMemoryEnabled, isTrue);
      expect(updated.aiCompletionEnabled, isFalse);
    });

    test('reorganize returns the reorganized count', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'reorganized': 12,
          'request_id': 'r',
        });
      final api = AISettingsApi(client);

      final count = await api.reorganize();

      expect(client.lastPath, '/users/me/ai-settings/reorganize');
      expect(count, 12);
    });

    test('get throws FormatException when data is not a map', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: <dynamic>['nope']);
      final api = AISettingsApi(client);

      expect(() => api.get(), throwsA(isA<FormatException>()));
    });
  });
}
