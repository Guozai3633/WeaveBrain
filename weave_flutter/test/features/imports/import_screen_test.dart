import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:weave_flutter/features/imports/data/import_api.dart';
import 'package:weave_flutter/features/imports/ui/import_screen.dart';

class _FakeImportGateway implements ImportGateway {
  _FakeImportGateway({this.duplicateSingle = false});

  final bool duplicateSingle;
  int createCalls = 0;
  int previewCalls = 0;
  int completionCalls = 0;
  int applyCalls = 0;
  int commitCalls = 0;
  int errorReportCalls = 0;
  int singleCalls = 0;
  String? lastFormat;
  String? lastSourceName;
  String? lastContent;
  List<ImportRowSelection>? lastSelections;
  String? lastDuplicateAction;

  ImportJobModel job() => ImportJobModel(
        id: 'job-1',
        userId: 'u1',
        sourceName: '旧备忘录',
        format: 'csv',
        rawText: '',
        columnMapping: const {},
        separator: '---',
        totalRows: 2,
        validRows: 2,
        invalidRows: 0,
        duplicateRows: 1,
        needsInputRows: 0,
        importedRows: 0,
        skippedRows: 0,
        failedRows: 0,
        status: 'previewed',
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
      content: '第$rowNumber条内容',
      validationErrors: const [],
      dedupeStatus: dedupeStatus,
      status: 'pending',
      completionProposals: const [],
      createdAt: DateTime.utc(2026, 8, 29),
      updatedAt: DateTime.utc(2026, 8, 29),
    );
  }

  ImportRowModel rowWithProposal(int rowNumber) {
    return ImportRowModel(
      id: 'row-$rowNumber',
      importJobId: 'job-1',
      userId: 'u1',
      rowNumber: rowNumber,
      rawPayload: const {},
      normalizedPayload: const {},
      content: '第$rowNumber条内容',
      validationErrors: const [],
      dedupeStatus: 'none',
      status: 'pending',
      completionProposals: [
        ImportProposalModel(
          id: 'p$rowNumber',
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
    lastFormat = format;
    lastSourceName = sourceName;
    lastContent = content;
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
    return ImportPreviewModel(
      job: job(),
      rows: [row(1), row(2, dedupeStatus: 'duplicate_external')],
    );
  }

  @override
  Future<ImportCompletionModel> completionPreview(
    String jobId, {
    List<int>? rowNumbers,
  }) async {
    completionCalls++;
    return ImportCompletionModel(rows: [rowWithProposal(1)]);
  }

  @override
  Future<ImportCompletionModel> completionApply(
    String jobId, {
    required List<ImportRowSelection> selections,
  }) async {
    applyCalls++;
    lastSelections = selections;
    final p = rowWithProposal(1).completionProposals.single;
    return ImportCompletionModel(
      rows: [
        ImportRowModel(
          id: 'row-1',
          importJobId: 'job-1',
          userId: 'u1',
          rowNumber: 1,
          rawPayload: const {},
          normalizedPayload: const {},
          content: '第1条内容',
          validationErrors: const [],
          dedupeStatus: 'none',
          status: 'pending',
          completionProposals: [
            ImportProposalModel(
              id: p.id,
              fieldName: p.fieldName,
              proposedValue: p.proposedValue,
              provenance: p.provenance,
              applyPolicy: p.applyPolicy,
              confidence: p.confidence,
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
    return [
      ImportErrorReportEntryModel(
        rowNumber: 2,
        externalId: 't2',
        status: 'failed',
        dedupeStatus: 'none',
        validationErrors: const [
          ImportValidationErrorModel(code: 'missing_content', message: '缺少内容'),
        ],
      ),
    ];
  }

  @override
  Future<SingleImportResult> importSingle(SingleImportDraft draft) async {
    singleCalls++;
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

class _MemoryStub extends StatelessWidget {
  const _MemoryStub({required this.captureId});

  final String captureId;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('记忆详情')),
      body: Center(child: Text(captureId)),
    );
  }
}

GoRouter _router() {
  return GoRouter(
    initialLocation: '/imports',
    routes: [
      GoRoute(path: '/imports', builder: (_, _) => const ImportScreen()),
      GoRoute(
        path: '/memories/:captureId',
        builder: (_, state) => _MemoryStub(
          captureId: state.pathParameters['captureId'] ?? '',
        ),
      ),
    ],
  );
}

Future<_FakeImportGateway> _pumpImport(
  WidgetTester tester, {
  _FakeImportGateway? gateway,
}) async {
  // 放大视口，保证批量预览区（行卡/补全/导入按钮）都在可视范围内。
  tester.view.physicalSize = const Size(1200, 2600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  final fake = gateway ?? _FakeImportGateway();
  await tester.pumpWidget(
    ProviderScope(
      overrides: [importGatewayProvider.overrideWithValue(fake)],
      child: MaterialApp.router(routerConfig: _router()),
    ),
  );
  await tester.pumpAndSettle();
  return fake;
}

Future<void> _switchToBatch(WidgetTester tester) async {
  await tester.tap(find.text('批量'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('single import submits and navigates to the memory detail', (
    tester,
  ) async {
    final gateway = await _pumpImport(tester);

    await tester.enterText(
      find.byKey(const Key('import_single_text')),
      '第一条笔记',
    );
    await tester.tap(find.byKey(const Key('import_single_submit')));
    await tester.pumpAndSettle();

    expect(gateway.singleCalls, 1);
    // 成功后跳转到 /memories/cap-new 详情页。
    expect(find.text('记忆详情'), findsOneWidget);
    expect(find.text('cap-new'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('single import duplicate shows error banner with existing link', (
    tester,
  ) async {
    await _pumpImport(tester, gateway: _FakeImportGateway(duplicateSingle: true));

    await tester.enterText(
      find.byKey(const Key('import_single_text')),
      '重复内容',
    );
    await tester.tap(find.byKey(const Key('import_single_submit')));
    await tester.pumpAndSettle();

    expect(find.text('该 external_id 已导入过，请更换后再试'), findsOneWidget);
    expect(find.text('查看已存在的记忆'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('batch flow: paste → preview → completion → apply → commit → report', (
    tester,
  ) async {
    final gateway = await _pumpImport(tester);
    await _switchToBatch(tester);

    // 粘贴内容并解析预览。
    await tester.enterText(
      find.byKey(const Key('import_batch_text')),
      'external_id,content\nt1,第一条\nt2,第二条',
    );
    await tester.tap(find.byKey(const Key('import_batch_parse')));
    await tester.pumpAndSettle();

    expect(gateway.createCalls, 1);
    expect(gateway.lastFormat, 'plain_text');
    expect(gateway.previewCalls, 1);
    expect(find.text('预览'), findsOneWidget);
    // 第二行 external_id 精确重复 → 展示「重复·跳过」chip。
    expect(find.text('重复·跳过'), findsOneWidget);

    // AI 补全缺失项 → 提案出现。
    await tester.tap(find.byKey(const Key('import_batch_completion')));
    await tester.pumpAndSettle();
    expect(gateway.completionCalls, 1);
    expect(find.byKey(const Key('import_proposal_p1')), findsOneWidget);

    // 勾选提案并采用。
    await tester.tap(find.byKey(const Key('import_proposal_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('import_batch_apply')));
    await tester.pumpAndSettle();
    expect(gateway.applyCalls, 1);
    expect(gateway.lastSelections!.single.rowNumber, 1);
    expect(gateway.lastSelections!.single.proposalIds, ['p1']);

    // 开始导入 → 完成页统计。
    await tester.tap(find.byKey(const Key('import_batch_commit')));
    await tester.pumpAndSettle();
    expect(gateway.commitCalls, 1);
    expect(gateway.lastDuplicateAction, 'skip');
    expect(find.text('导入完成'), findsOneWidget);
    expect(find.text('成功导入'), findsOneWidget);
    expect(find.text('跳过'), findsOneWidget);

    // 查看错误报告。
    await tester.tap(find.byKey(const Key('import_batch_error_report')));
    await tester.pumpAndSettle();
    expect(gateway.errorReportCalls, 1);
    expect(find.text('缺少内容'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('batch duplicate content action can be set to import', (
    tester,
  ) async {
    final gateway = await _pumpImport(tester);
    await _switchToBatch(tester);

    await tester.enterText(
      find.byKey(const Key('import_batch_text')),
      'a\n---\nb',
    );
    await tester.tap(find.byKey(const Key('import_batch_parse')));
    await tester.pumpAndSettle();

    // 切换「疑似重复内容处理」为「仍导入」。
    await tester.tap(find.text('仍导入'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('import_batch_commit')));
    await tester.pumpAndSettle();

    expect(gateway.lastDuplicateAction, 'import');
    expect(find.text('导入完成'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
