import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/completions/data/completion_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient()
    : super(baseUrl: 'http://test', authRepository: AuthRepository());

  String? lastPath;
  Map<String, dynamic>? lastBody;
  ApiResponse postResponse = ApiResponse(data: <String, dynamic>{});
  ApiException? postError;

  @override
  Future<ApiResponse> post(
    String path, {
    dynamic body,
    Map<String, dynamic>? queryParams,
    Map<String, String>? headers,
  }) async {
    lastPath = path;
    lastBody = body as Map<String, dynamic>?;
    final error = postError;
    if (error != null) throw error;
    return postResponse;
  }
}

List<Map<String, dynamic>> _evidenceSpans() {
  return const [
    {'start': 0, 'end': 4, 'quote': '工作'},
  ];
}

Map<String, dynamic> _proposalJson({
  String id = 'p1',
  String field = 'tags',
  String value = '["工作","灵感"]',
  String policy = 'safe_auto',
  double? confidence = 0.9,
  String status = 'pending',
}) {
  return {
    'id': id,
    'capture_id': 'c1',
    'preview_id': 'prev-1',
    'source_revision': 3,
    'field_name': field,
    'original_value': '[]',
    'proposed_value': value,
    'provenance': 'ai',
    'apply_policy': policy,
    'confidence': confidence,
    'risk_level': 'low',
    'evidence_spans': _evidenceSpans(),
    'status': status,
    'provider': 'ollama',
    'model': 'qwen2.5:7b',
    'config_version': 'completion-prompt-v1',
  };
}

Map<String, dynamic> _cardJson() {
  return {
    'id': 'card-1',
    'user_id': 'u1',
    'capture_id': 'c1',
    'primary_type': 'idea',
    'title': '补全标题',
    'summary': '摘要',
    'tags': ['工作'],
    'key_points': ['要点'],
    'processing_status': 'ready',
    'version': 2,
    'is_pinned': false,
    'created_at': '2026-08-30T00:00:00Z',
    'updated_at': '2026-08-30T00:00:00Z',
  };
}

void main() {
  group('CompletionProposalModel', () {
    test('parses scalar and array fields', () {
      final scalar = CompletionProposalModel.fromJson(
        _proposalJson(field: 'title', value: '好标题', confidence: null),
      );
      expect(scalar.fieldName, 'title');
      expect(scalar.proposedValue, '好标题');
      expect(scalar.proposedValues, isEmpty);
      expect(scalar.confidence, isNull);

      final array = CompletionProposalModel.fromJson(_proposalJson());
      expect(array.proposedValues, ['工作', '灵感']);
      expect(array.canAutoApply, isTrue);
      expect(array.hasEvidence, isTrue);
      expect(array.evidenceSpans.first.quote, '工作');
    });

    test('suggest_only and accepted are not auto-appliable', () {
      final suggestOnly = CompletionProposalModel.fromJson(
        _proposalJson(policy: 'suggest_only'),
      );
      expect(suggestOnly.canAutoApply, isFalse);

      final accepted = CompletionProposalModel.fromJson(
        _proposalJson(status: 'accepted'),
      );
      expect(accepted.canAutoApply, isFalse);
      expect(accepted.isPending, isFalse);
    });
  });

  group('CompletionPreviewResultModel', () {
    test('parses the preview envelope body', () {
      final result = CompletionPreviewResultModel.fromJson({
        'capture_id': 'c1',
        'source_revision': 3,
        'missing_fields': ['tags', 'key_points'],
        'proposals': [_proposalJson(), _proposalJson(id: 'p2', field: 'key_points')],
      });
      expect(result.captureId, 'c1');
      expect(result.sourceRevision, 3);
      expect(result.missingFields, ['tags', 'key_points']);
      expect(result.proposals, hasLength(2));
    });
  });

  group('CompletionApplyResultModel', () {
    test('parses memory_card and applied_proposal_ids', () {
      final result = CompletionApplyResultModel.fromJson({
        'memory_card': _cardJson(),
        'applied_proposal_ids': ['p1'],
      });
      expect(result.memoryCard.title, '补全标题');
      expect(result.memoryCard.version, 2);
      expect(result.appliedProposalIds, ['p1']);
    });
  });

  group('CompletionApi', () {
    test('preview posts to the endpoint and unwraps the preview key', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'preview': {
            'capture_id': 'c1',
            'source_revision': 3,
            'missing_fields': ['tags'],
            'proposals': [_proposalJson()],
          },
          'request_id': 'req-1',
        });
      final api = CompletionApi(client);

      final result = await api.preview('c1');

      expect(client.lastPath, '/captures/c1/completion/preview');
      expect(client.lastBody, isNull);
      expect(result.sourceRevision, 3);
      expect(result.proposals, hasLength(1));
    });

    test('apply posts proposal_ids and source_revision and unwraps apply', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'apply': {
            'memory_card': _cardJson(),
            'applied_proposal_ids': ['p1'],
          },
          'request_id': 'req-2',
        });
      final api = CompletionApi(client);

      final result = await api.apply(
        'c1',
        proposalIds: ['p1'],
        sourceRevision: 3,
      );

      expect(client.lastPath, '/captures/c1/completion/apply');
      expect(client.lastBody, {
        'proposal_ids': ['p1'],
        'source_revision': 3,
      });
      expect(result.appliedProposalIds, ['p1']);
      expect(result.memoryCard.title, '补全标题');
    });

    test('undo posts to the endpoint and unwraps the undo key', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'undo': {
            'memory_card': _cardJson(),
            'applied_proposal_ids': <String>[],
          },
          'request_id': 'req-3',
        });
      final api = CompletionApi(client);

      final result = await api.undo('c1');

      expect(client.lastPath, '/captures/c1/completion/undo');
      expect(result.memoryCard.title, '补全标题');
    });

    test('throws FormatException when the envelope is malformed', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: <dynamic>['nope']);
      final api = CompletionApi(client);

      expect(() => api.preview('c1'), throwsA(isA<FormatException>()));
    });

    test('rethrows ApiException on non-2xx', () async {
      final client = _FakeApiClient()
        ..postError = ApiException(
          statusCode: 409,
          message: 'HTTP 409',
          data: '{"code":"VERSION_CONFLICT"}',
        );
      final api = CompletionApi(client);

      expect(
        () => api.preview('c1'),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 409)),
      );
    });
  });
}
