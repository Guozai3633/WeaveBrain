import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/imports/data/import_api.dart';
import 'package:weave_flutter/features/imports/domain/import_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class _FakeImportGateway implements ImportGateway {
  ApiException? createError;
  ApiException? previewError;
  ApiException? completionError;
  ApiException? commitError;
  ApiException? errorReportError;

  int createCalls = 0;
  int previewCalls = 0;
  int completionCalls = 0;
  int applyCalls = 0;
  int commitCalls = 0;
  int errorReportCalls = 0;
  int singleCalls = 0;
  List<ImportRowSelection>? lastSelections;
  String? lastDuplicateAction;
  bool duplicateSingle = false;
  ApiException? singleError;

  ImportJobModel job({String status = 'draft'}) => ImportJobModel(
        id: 'job-1',
        userId: 'u1',
        sourceName: '旧备忘录',
        format: 'plain_text',
        rawText: 'a\n---\nb',
        columnMapping: const {},
        separator: '---',
        totalRows: 2,
        validRows: 2,
        invalidRows: 0,
        duplicateRows: 0,
        needsInputRows: 0,
        importedRows: 0,
        skippedRows: 0,
        failedRows: 0,
        status: status,
        createdAt: DateTime.utc(2026, 8, 29),
        updatedAt: DateTime.utc(2026, 8, 29),
      );

  ImportRowModel row(int rowNumber, {String dedupeStatus = 'none'}) {
    return ImportRowModel(
      id: 'row-$rowNumber',
      importJobId: 'job-1',
      userId: 'u1',
      rowNumber: rowNumber,
      rawPayload: const {},
      normalizedPayload: const {},
      content: '第$rowNumber条',
      validationErrors: const [],
      dedupeStatus: dedupeStatus,
      status: 'pending',
      completionProposals: const [],
      createdAt: DateTime.utc(2026, 8, 29),
      updatedAt: DateTime.utc(2026, 8, 29),
    );
  }

  @override
  Future<ImportJobModel> createJob({
    required String format,
    required String sourceName,
    required String content,
    String? separator,
    String? timezone,
    String? originalFilename,
  }) async {
    createCalls++;
    final error = createError;
    if (error != null) throw error;
    return job();
  }

  @override
  Future<ImportJobModel> getJob(String jobId) async => job();

  @override
  Future<ImportPreviewModel> preview(
    String jobId, {
    int limit = 10,
    int offset = 0,
  }) async {
    previewCalls++;
    final error = previewError;
    if (error != null) throw error;
    return ImportPreviewModel(
      job: job(),
      rows: [row(1), row(2, dedupeStatus: 'suggested')],
    );
  }

  @override
  Future<ImportCompletionModel> completionPreview(
    String jobId, {
    List<int>? rowNumbers,
  }) async {
    completionCalls++;
    final error = completionError;
    if (error != null) throw error;
    return ImportCompletionModel(
      rows: [
        ImportRowModel(
          id: 'row-1',
          importJobId: 'job-1',
          userId: 'u1',
          rowNumber: 1,
          rawPayload: const {},
          normalizedPayload: const {},
          content: '第1条',
          validationErrors: const [],
          dedupeStatus: 'none',
          status: 'pending',
          completionProposals: [
            ImportProposalModel(
              id: 'p1',
              fieldName: 'tags',
              proposedValue: '["工作"]',
              provenance: 'ai',
              applyPolicy: 'suggest_only',
              confidence: 0.9,
              status: 'pending',
              evidenceSpans: const [],
            ),
          ],
          createdAt: DateTime.utc(2026, 8, 29),
          updatedAt: DateTime.utc(2026, 8, 29),
        ),
      ],
    );
  }

  @override
  Future<ImportCompletionModel> completionApply(
    String jobId, {
    required List<ImportRowSelection> selections,
  }) async {
    applyCalls++;
    lastSelections = selections;
    final error = completionError;
    if (error != null) throw error;
    return ImportCompletionModel(
      rows: [
        ImportRowModel(
          id: 'row-1',
          importJobId: 'job-1',
          userId: 'u1',
          rowNumber: 1,
          rawPayload: const {},
          normalizedPayload: const {},
          content: '第1条',
          validationErrors: const [],
          dedupeStatus: 'none',
          status: 'pending',
          completionProposals: [
            ImportProposalModel(
              id: 'p1',
              fieldName: 'tags',
              proposedValue: '["工作"]',
              provenance: 'ai',
              applyPolicy: 'suggest_only',
              confidence: 0.9,
              status: 'accepted',
              evidenceSpans: const [],
            ),
          ],
          createdAt: DateTime.utc(2026, 8, 29),
          updatedAt: DateTime.utc(2026, 8, 29),
        ),
      ],
    );
  }

  @override
  Future<ImportCommitResultModel> commit(
    String jobId, {
    required String duplicateContentAction,
    Map<int, String>? rowActions,
  }) async {
    commitCalls++;
    lastDuplicateAction = duplicateContentAction;
    final error = commitError;
    if (error != null) throw error;
    return ImportCommitResultModel(
      imported: 1,
      failed: 0,
      skipped: 1,
      needsInput: 0,
      total: 2,
      jobStatus: 'completed',
      committedAt: DateTime.utc(2026, 8, 29),
    );
  }

  @override
  Future<List<ImportErrorReportEntryModel>> errorReport(String jobId) async {
    errorReportCalls++;
    final error = errorReportError;
    if (error != null) throw error;
    return [
      ImportErrorReportEntryModel(
        rowNumber: 2,
        status: 'failed',
        dedupeStatus: 'none',
        validationErrors: const [],
      ),
    ];
  }

  @override
  Future<SingleImportResult> importSingle(SingleImportDraft draft) async {
    singleCalls++;
    final err = singleError;
    if (err != null) throw err;
    if (duplicateSingle) {
      throw const ImportDuplicateException(existingCaptureId: 'cap-0');
    }
    return SingleImportResult(
      captureId: 'cap-new',
      dedupeStatus: 'suggested',
      dedupeExistingCaptureId: 'cap-old',
    );
  }
}

ProviderContainer _container(_FakeImportGateway gateway) {
  final c = ProviderContainer(
    overrides: [importGatewayProvider.overrideWithValue(gateway)],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  test('createJob publishes ImportCreated and keeps the job', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);

    final job = await notifier.createJob(
      format: 'plain_text',
      sourceName: '旧备忘录',
      content: 'a\n---\nb',
    );

    expect(gateway.createCalls, 1);
    expect(job, isNotNull);
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportCreated>());
    expect((state as ImportCreated).job.id, 'job-1');
  });

  test('createJob failure publishes ImportError', () async {
    final gateway = _FakeImportGateway()
      ..createError = ApiException(
        statusCode: 400,
        message: 'HTTP 400',
        data: '{"code":"INVALID_ARGUMENT"}',
      );
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);

    final job = await notifier.createJob(
      format: 'csv',
      sourceName: 's',
      content: 'x',
    );

    expect(job, isNull);
    expect(c.read(importNotifierProvider), isA<ImportError>());
  });

  test('preview publishes ImportPreviewed with rows', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(
      format: 'plain_text',
      sourceName: 's',
      content: 'a\n---\nb',
    );

    await notifier.preview();

    expect(gateway.previewCalls, 1);
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportPreviewed>());
    final previewed = state as ImportPreviewed;
    expect(previewed.preview.rows, hasLength(2));
    expect(previewed.preview.rows[1].isSuggested, isTrue);
  });

  test('completionPreview stores proposals overlay', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');
    await notifier.preview();

    await notifier.completionPreview();

    expect(gateway.completionCalls, 1);
    final state = c.read(importNotifierProvider) as ImportPreviewed;
    expect(state.completion, isNotNull);
    expect(state.completion!.rows.single.completionProposals.single.id, 'p1');
  });

  test('completionPreview failure keeps preview and surfaces message', () async {
    final gateway = _FakeImportGateway()
      ..completionError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"FEATURE_NOT_ENABLED"}',
      );
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');
    await notifier.preview();

    await notifier.completionPreview();

    final state = c.read(importNotifierProvider) as ImportPreviewed;
    expect(state.completion, isNull);
    expect(state.message, 'AI 补全未开启');
  });

  test('completionApply sends selections and marks message', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');
    await notifier.preview();

    await notifier.completionApply([
      ImportRowSelection(rowNumber: 1, proposalIds: ['p1']),
    ]);

    expect(gateway.applyCalls, 1);
    expect(gateway.lastSelections!.single.rowNumber, 1);
    final state = c.read(importNotifierProvider) as ImportPreviewed;
    expect(state.message, '已采用所选补全');
    expect(state.completion!.rows.single.completionProposals.single.isAccepted, isTrue);
  });

  test('commit publishes ImportCommitted with result counts', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');

    await notifier.commit(duplicateContentAction: 'skip');

    expect(gateway.commitCalls, 1);
    expect(gateway.lastDuplicateAction, 'skip');
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportCommitted>());
    final committed = state as ImportCommitted;
    expect(committed.result.imported, 1);
    expect(committed.result.skipped, 1);
  });

  test('commit is idempotent on the client side (repeat call works)', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');

    await notifier.commit(duplicateContentAction: 'skip');
    await notifier.commit(duplicateContentAction: 'skip');

    expect(gateway.commitCalls, 2);
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportCommitted>());
    expect((state as ImportCommitted).result.total, 2);
  });

  test('commit failure publishes ImportError', () async {
    final gateway = _FakeImportGateway()
      ..commitError = ApiException(
        statusCode: 404,
        message: 'HTTP 404',
        data: '{"code":"NOT_FOUND"}',
      );
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');

    await notifier.commit(duplicateContentAction: 'skip');

    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportError>());
    expect((state as ImportError).message, '导入任务不存在');
  });

  test('loadErrorReport fills the committed error report', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');
    await notifier.commit(duplicateContentAction: 'skip');

    await notifier.loadErrorReport();

    expect(gateway.errorReportCalls, 1);
    final state = c.read(importNotifierProvider) as ImportCommitted;
    expect(state.errorReport, hasLength(1));
    expect(state.errorReport!.single.rowNumber, 2);
  });

  test('importSingle success returns result and resets to Idle', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);

    final result = await notifier.importSingle(
      const SingleImportDraft(text: '第一条', externalId: 't1'),
    );

    expect(gateway.singleCalls, 1);
    expect(result, isNotNull);
    expect(result!.captureId, 'cap-new');
    expect(result.dedupeStatus, 'suggested');
    expect(c.read(importNotifierProvider), isA<ImportIdle>());
  });

  test('importSingle duplicate publishes ImportError with existing id', () async {
    final gateway = _FakeImportGateway()..duplicateSingle = true;
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);

    final result = await notifier.importSingle(
      const SingleImportDraft(text: '第一条', externalId: 't1'),
    );

    expect(result, isNull);
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportError>());
    final error = state as ImportError;
    expect(error.message, '该 external_id 已导入过，请更换后再试');
    expect(error.duplicateExistingCaptureId, 'cap-0');
  });

  test('importSingle network error maps to ImportError', () async {
    final gateway = _FakeImportGateway()
      ..singleError = ApiException(
        statusCode: 500,
        message: 'HTTP 500',
        data: '{"code":"INTERNAL"}',
      );
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);

    final result = await notifier.importSingle(
      const SingleImportDraft(text: 'x'),
    );

    expect(result, isNull);
    final state = c.read(importNotifierProvider);
    expect(state, isA<ImportError>());
    expect((state as ImportError).message, 'AI 生成失败，请稍后重试');
  });

  test('reset clears the job and returns to Idle', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');

    notifier.reset();

    expect(c.read(importNotifierProvider), isA<ImportIdle>());
  });

  test('clearMessage clears the previewed message', () async {
    final gateway = _FakeImportGateway();
    final c = _container(gateway);
    final notifier = c.read(importNotifierProvider.notifier);
    await notifier.createJob(format: 'plain_text', sourceName: 's', content: 'a');
    await notifier.preview();
    await notifier.completionApply([
      ImportRowSelection(rowNumber: 1, proposalIds: ['p1']),
    ]);
    expect(
      (c.read(importNotifierProvider) as ImportPreviewed).message,
      '已采用所选补全',
    );

    notifier.clearMessage();

    expect(
      (c.read(importNotifierProvider) as ImportPreviewed).message,
      isNull,
    );
  });
}
