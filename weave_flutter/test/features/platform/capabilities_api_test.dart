import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/platform/data/capabilities_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient()
    : super(baseUrl: 'http://test', authRepository: AuthRepository());

  String? lastPath;
  ApiResponse getResponse = ApiResponse(data: <String, dynamic>{});
  ApiException? error;

  @override
  Future<ApiResponse> get(
    String path, {
    Map<String, dynamic>? queryParams,
  }) async {
    lastPath = path;
    final err = error;
    if (err != null) throw err;
    return getResponse;
  }
}

void main() {
  group('CapabilitiesApi', () {
    test('GET /capabilities and decodes server response', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(
          data: <String, dynamic>{
            'mobile_capture': true,
            'web_review': true,
            'workflow_designer': false,
            'workflow_execution': false,
            'supported_capture_sources': ['text', 'audio', 'import'],
          },
        );
      final api = CapabilitiesApi(client);

      final capabilities = await api.fetch();

      expect(client.lastPath, '/capabilities');
      expect(capabilities.mobileCapture, isTrue);
      expect(capabilities.webReview, isTrue);
      expect(capabilities.workflowDesigner, isFalse);
      expect(capabilities.workflowExecution, isFalse);
      expect(capabilities.workflowAvailable, isFalse);
      expect(capabilities.supportedCaptureSources, ['text', 'audio', 'import']);
    });

    test('defaults missing keys to safe disabled booleans', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(
          data: <String, dynamic>{
            'mobile_capture': true,
            'supported_capture_sources': null,
          },
        );
      final api = CapabilitiesApi(client);

      final capabilities = await api.fetch();

      expect(capabilities.webReview, isFalse);
      expect(capabilities.workflowDesigner, isFalse);
      expect(capabilities.workflowExecution, isFalse);
      expect(capabilities.supportedCaptureSources, isEmpty);
    });

    test('rethrows transport error', () async {
      final client = _FakeApiClient()
        ..error = ApiException(statusCode: 500, message: 'boom');
      final api = CapabilitiesApi(client);

      await expectLater(api.fetch(), throwsA(isA<ApiException>()));
    });

    test('throws FormatException on non-map payload', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: <String>['not', 'a', 'map']);
      final api = CapabilitiesApi(client);

      await expectLater(api.fetch(), throwsA(isA<FormatException>()));
    });
  });
}
