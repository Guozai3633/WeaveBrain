import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/imports/data/import_api.dart';
import 'package:weave_flutter/shared/api/api_client.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';
import 'package:weave_flutter/shared/auth/auth_repository.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient() : super(baseUrl: 'http://test', authRepository: AuthRepository());

  String? lastPath;
  Map<String, dynamic>? lastBody;
  Map<String, String>? lastHeaders;
  ApiResponse getResponse = ApiResponse(data: <String, dynamic>{});
  ApiResponse postResponse = ApiResponse(data: <String, dynamic>{});
  ApiException? error;

  @override
  Future<ApiResponse> get(
    String path, {
    Map<String, dynamic>? queryParams,
  }) async {
    lastPath = path;
    lastBody = null;
    final err = error;
    if (err != null) throw err;
    return getResponse;
  }

  @override
  Future<ApiResponse> post(
    String path, {
    dynamic body,
    Map<String, dynamic>? queryParams,
    Map<String, String>? headers,
  }) async {
    lastPath = path;
    lastBody = body as Map<String, dynamic>?;
    lastHeaders = headers;
    final err = error;
    if (err != null) throw err;
    return postResponse;
  }
}

Map<String, dynamic> _jobJson({
  String status = 'draft',
  int totalRows = 2,
}) {
  return {
    'id': 'job-1',
    'user_id': 'u1',
    'source_name': '旧备忘录',
    'format': 'csv',
    'original_filename': 'notes.csv',
    'raw_text': 'external_id,content\nt1,第一条',
    'column_mapping': {'content': 'content', 'tags': 'tags'},
    'separator': '---',
    'timezone': 'Asia/Shanghai',
    'total_rows': totalRows,
    'valid_rows': 2,
    'invalid_rows': 0,
    'duplicate_rows': 0,
    'needs_input_rows': 0,
    'imported_rows': 0,
    'skipped_rows': 0,
    'failed_rows': 0,
    'status': status,
    'created_at': '2026-08-29T00:00:00Z',
    'updated_at': '2026-08-29T00:00:00Z',
  };
}

Map<String, dynamic> _proposalJson({
  String id = 'p1',
  String field = 'tags',
  String value = '["工作","灵感"]',
  String status = 'pending',
}) {
  return {
    'id': id,
    'field_name': field,
    'proposed_value': value,
    'provenance': 'ai',
    'apply_policy': 'suggest_only',
    'confidence': 0.9,
    'evidence_spans': [
      {'start': 0, 'end': 4, 'quote': '工作'},
    ],
    'status': status,
  };
}

Map<String, dynamic> _rowJson({
  int rowNumber = 1,
  String? externalId = 't1',
  String dedupeStatus = 'none',
  String status = 'pending',
  List<Map<String, dynamic>>? errors,
  List<Map<String, dynamic>>? proposals,
}) {
  return {
    'id': 'row-$rowNumber',
    'import_job_id': 'job-1',
    'user_id': 'u1',
    'row_number': rowNumber,
    'external_id': externalId,
    'raw_payload': {'content': '第一条'},
    'normalized_payload': {'content': '第一条'},
    'content': '第一条',
    'content_hash': 'abc',
    'validation_errors': errors ?? const <Map<String, dynamic>>[],
    'dedupe_status': dedupeStatus,
    'status': status,
    'completion_proposals': proposals ?? const <Map<String, dynamic>>[],
    'created_at': '2026-08-29T00:00:00Z',
    'updated_at': '2026-08-29T00:00:00Z',
  };
}

void main() {
  group('ImportJobModel', () {
    test('parses job fields and counts', () {
      final job = ImportJobModel.fromJson(_jobJson());
      expect(job.id, 'job-1');
      expect(job.sourceName, '旧备忘录');
      expect(job.format, 'csv');
      expect(job.originalFilename, 'notes.csv');
      expect(job.separator, '---');
      expect(job.timezone, 'Asia/Shanghai');
      expect(job.totalRows, 2);
      expect(job.validRows, 2);
      expect(job.status, 'draft');
      expect(job.columnMapping['tags'], 'tags');
    });

    test('defaults missing fields', () {
      final job = ImportJobModel.fromJson(const {});
      expect(job.id, '');
      expect(job.sourceName, '');
      expect(job.format, 'plain_text');
      expect(job.separator, '---');
      expect(job.status, 'draft');
    });
  });

  group('ImportRowModel', () {
    test('parses dedupe status and content preview', () {
      final row = ImportRowModel.fromJson(
        _rowJson(dedupeStatus: 'duplicate_external'),
      );
      expect(row.rowNumber, 1);
      expect(row.externalId, 't1');
      expect(row.isDuplicateExternal, isTrue);
      expect(row.isSuggested, isFalse);
      expect(row.contentPreview, '第一条');
    });

    test('exposes suggested / needs_input / imported helpers', () {
      final suggested = ImportRowModel.fromJson(
        _rowJson(dedupeStatus: 'suggested'),
      );
      expect(suggested.isSuggested, isTrue);

      final needsInput = ImportRowModel.fromJson(
        _rowJson(status: 'needs_input'),
      );
      expect(needsInput.needsInput, isTrue);

      final imported = ImportRowModel.fromJson(_rowJson(status: 'imported'));
      expect(imported.isImported, isTrue);
    });

    test('parses validation errors', () {
      final row = ImportRowModel.fromJson(
        _rowJson(errors: [
          {'code': 'missing_content', 'message': '缺少内容'},
        ]),
      );
      expect(row.hasErrors, isTrue);
      expect(row.validationErrors.single.code, 'missing_content');
      expect(row.validationErrors.single.message, '缺少内容');
    });

    test('parses completion proposals', () {
      final row = ImportRowModel.fromJson(
        _rowJson(proposals: [_proposalJson()]),
      );
      expect(row.completionProposals, hasLength(1));
      expect(row.completionProposals.single.fieldName, 'tags');
      expect(row.completionProposals.single.proposedValues, ['工作', '灵感']);
    });
  });

  group('ImportPreviewModel', () {
    test('unwraps job and rows', () {
      final preview = ImportPreviewModel.fromJson({
        'job': _jobJson(),
        'rows': [_rowJson(), _rowJson(rowNumber: 2, externalId: 't2')],
        'next_cursor': '10',
      });
      expect(preview.job.id, 'job-1');
      expect(preview.rows, hasLength(2));
      expect(preview.nextCursor, '10');
    });
  });

  group('ImportCommitResultModel', () {
    test('parses counts and status', () {
      final result = ImportCommitResultModel.fromJson({
        'imported': 1,
        'failed': 1,
        'skipped': 0,
        'needs_input': 0,
        'total': 2,
        'job_status': 'completed',
        'committed_at': '2026-08-29T00:00:00Z',
      });
      expect(result.imported, 1);
      expect(result.failed, 1);
      expect(result.total, 2);
      expect(result.jobStatus, 'completed');
      expect(result.committedAt, isNotNull);
    });
  });

  group('ImportErrorReportEntryModel', () {
    test('parses errors and status', () {
      final entry = ImportErrorReportEntryModel.fromJson({
        'row_number': 3,
        'external_id': 't3',
        'status': 'failed',
        'dedupe_status': 'none',
        'validation_errors': [
          {'code': 'invalid_coordinates', 'message': '经纬度无效'},
        ],
      });
      expect(entry.rowNumber, 3);
      expect(entry.externalId, 't3');
      expect(entry.status, 'failed');
      expect(entry.validationErrors.single.code, 'invalid_coordinates');
    });
  });

  group('SingleImportDraft', () {
    test('toRequest builds kind=import body', () {
      final draft = SingleImportDraft(
        text: '第一条',
        externalId: 't1',
        sourceName: '旧备忘录',
        title: '标题',
        tags: const ['工作'],
        primaryType: 'idea',
        capturedAt: '2026-08-01',
        timezone: 'Asia/Shanghai',
      );
      final request = draft.toRequest(captureId: 'cap-1');
      expect(request['capture_id'], 'cap-1');
      expect(request['kind'], 'import');
      expect(request['text'], '第一条');
      expect(request['external_id'], 't1');
      expect(request['source_name'], '旧备忘录');
      expect(request['tags'], ['工作']);
      expect(request['captured_at'], '2026-08-01');
    });

    test('toRequest omits empty optionals', () {
      final request = const SingleImportDraft(text: 'x').toRequest(
        captureId: 'cap-2',
      );
      expect(request.containsKey('external_id'), isFalse);
      expect(request.containsKey('source_name'), isFalse);
      expect(request.containsKey('tags'), isFalse);
    });
  });

  group('ImportApi', () {
    test('createJob posts to /imports and unwraps the job', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'job': _jobJson(),
          'request_id': 'req-1',
        });
      final api = ImportApi(client);

      final job = await api.createJob(
        format: 'csv',
        sourceName: '旧备忘录',
        content: 'external_id,content\nt1,第一条',
      );

      expect(client.lastPath, '/imports');
      expect(client.lastBody, {
        'format': 'csv',
        'source_name': '旧备忘录',
        'content': 'external_id,content\nt1,第一条',
      });
      expect(job.id, 'job-1');
    });

    test('createJob includes separator / timezone when provided', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {'job': _jobJson()});
      final api = ImportApi(client);

      await api.createJob(
        format: 'plain_text',
        sourceName: 's',
        content: 'a\n---\nb',
        separator: '==',
        timezone: 'Asia/Shanghai',
      );

      expect(client.lastBody!['separator'], '==');
      expect(client.lastBody!['timezone'], 'Asia/Shanghai');
    });

    test('preview gets /imports/:id/preview with query params', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'preview': {
            'job': _jobJson(),
            'rows': [_rowJson()],
          },
          'request_id': 'req-2',
        });
      final api = ImportApi(client);

      final preview = await api.preview('job-1', limit: 10, offset: 20);

      expect(client.lastPath, '/imports/job-1/preview');
      expect(preview.rows, hasLength(1));
    });

    test('completionPreview posts to completion/preview', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'completion': {
            'rows': [_rowJson(proposals: [_proposalJson()])],
          },
          'request_id': 'req-3',
        });
      final api = ImportApi(client);

      final completion = await api.completionPreview('job-1', rowNumbers: [1]);

      expect(client.lastPath, '/imports/job-1/completion/preview');
      expect(client.lastBody, {'row_numbers': [1]});
      expect(completion.rows.single.completionProposals, hasLength(1));
    });

    test('completionApply posts row_selections', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'completion': {
            'rows': [_rowJson(proposals: [_proposalJson(status: 'accepted')])],
          },
          'request_id': 'req-4',
        });
      final api = ImportApi(client);

      final completion = await api.completionApply('job-1', selections: [
        ImportRowSelection(rowNumber: 1, proposalIds: ['p1']),
      ]);

      expect(client.lastPath, '/imports/job-1/completion/apply');
      expect(client.lastBody, {
        'row_selections': [
          {'row_number': 1, 'proposal_ids': ['p1']},
        ],
      });
      expect(completion.rows.single.completionProposals.single.isAccepted, isTrue);
    });

    test('commit posts duplicate_content_action and unwraps commit', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'commit': {
            'imported': 1,
            'failed': 0,
            'skipped': 1,
            'needs_input': 0,
            'total': 2,
            'job_status': 'completed',
          },
          'request_id': 'req-5',
        });
      final api = ImportApi(client);

      final result = await api.commit(
        'job-1',
        duplicateContentAction: 'skip',
        rowActions: {2: 'import'},
      );

      expect(client.lastPath, '/imports/job-1/commit');
      expect(client.lastBody, {
        'duplicate_content_action': 'skip',
        'row_actions': {'2': 'import'},
      });
      expect(result.imported, 1);
      expect(result.skipped, 1);
    });

    test('errorReport gets entries', () async {
      final client = _FakeApiClient()
        ..getResponse = ApiResponse(data: {
          'entries': [
            {
              'row_number': 2,
              'external_id': 't2',
              'status': 'failed',
              'dedupe_status': 'none',
              'validation_errors': [
                {'code': 'missing_content', 'message': '缺少内容'},
              ],
            },
          ],
          'request_id': 'req-6',
        });
      final api = ImportApi(client);

      final entries = await api.errorReport('job-1');

      expect(client.lastPath, '/imports/job-1/error-report');
      expect(entries, hasLength(1));
      expect(entries.single.rowNumber, 2);
    });

    test('importSingle posts to /captures with Idempotency-Key', () async {
      final client = _FakeApiClient()
        ..postResponse = ApiResponse(data: {
          'capture': {'id': 'cap-1'},
          'replayed': false,
          'dedupe': {
            'status': 'suggested',
            'existing_capture_id': 'cap-0',
          },
          'request_id': 'req-7',
        });
      final api = ImportApi(client);

      final result = await api.importSingle(
        const SingleImportDraft(text: '第一条', externalId: 't1'),
      );

      expect(client.lastPath, '/captures');
      expect(client.lastBody!['kind'], 'import');
      expect(client.lastHeaders!['Idempotency-Key'], isNotEmpty);
      expect(result.captureId, 'cap-1');
      expect(result.dedupeStatus, 'suggested');
      expect(result.dedupeExistingCaptureId, 'cap-0');
    });

    test('importSingle throws ImportDuplicateException on 409', () async {
      final client = _FakeApiClient()
        ..error = ApiException(
          statusCode: 409,
          message: 'HTTP 409',
          data: '{"code":"PRECONDITION_FAILED","details":{"existing_capture_id":"cap-0"}}',
        );
      final api = ImportApi(client);

      expect(
        () => api.importSingle(const SingleImportDraft(text: 'x', externalId: 't1')),
        throwsA(
          isA<ImportDuplicateException>()
              .having((e) => e.existingCaptureId, 'existingCaptureId', 'cap-0'),
        ),
      );
    });

    test('rethrows ApiException on non-2xx', () async {
      final client = _FakeApiClient()
        ..error = ApiException(
          statusCode: 500,
          message: 'HTTP 500',
          data: '{"code":"INTERNAL"}',
        );
      final api = ImportApi(client);

      expect(
        () => api.createJob(
          format: 'csv',
          sourceName: 's',
          content: 'x',
        ),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 500)),
      );
    });
  });
}
