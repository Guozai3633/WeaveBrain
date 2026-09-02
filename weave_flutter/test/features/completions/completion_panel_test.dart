import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/completions/data/completion_api.dart';
import 'package:weave_flutter/features/completions/ui/completion_panel.dart';
import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class _FakeGateway implements CompletionGateway {
  CompletionPreviewResultModel previewResult = _preview();
  CompletionApplyResultModel applyResult = CompletionApplyResultModel(
    memoryCard: _card(),
    appliedProposalIds: const ['p1'],
  );
  ApiException? previewError;
  ApiException? applyError;
  ApiException? undoError;

  final List<String> previewCalls = [];
  final List<String> applyCalls = [];
  final List<String> undoCalls = [];
  List<String>? lastProposalIds;
  int? lastSourceRevision;
  bool applied = false;
  bool undone = false;

  @override
  Future<CompletionPreviewResultModel> preview(String captureId) async {
    previewCalls.add(captureId);
    final error = previewError;
    if (error != null) throw error;
    return previewResult;
  }

  @override
  Future<CompletionApplyResultModel> apply(
    String captureId, {
    required List<String> proposalIds,
    required int sourceRevision,
  }) async {
    applyCalls.add(captureId);
    lastProposalIds = proposalIds;
    lastSourceRevision = sourceRevision;
    final error = applyError;
    if (error != null) throw error;
    applied = true;
    return applyResult;
  }

  @override
  Future<CompletionApplyResultModel> undo(String captureId) async {
    undoCalls.add(captureId);
    final error = undoError;
    if (error != null) throw error;
    undone = true;
    return applyResult;
  }
}

MemoryCardModel _card() {
  return MemoryCardModel(
    id: 'card-1',
    userId: 'u1',
    captureId: 'c1',
    primaryType: 'idea',
    title: '补全标题',
    summary: '摘要',
    tags: const ['工作'],
    keyPoints: const ['要点'],
    processingStatus: 'ready',
    version: 2,
    isPinned: false,
    createdAt: DateTime.utc(2026, 8, 30),
    updatedAt: DateTime.utc(2026, 8, 30),
  );
}

CompletionProposalModel _proposal({
  String id = 'p1',
  String field = 'tags',
  String policy = 'safe_auto',
  String value = '["工作","灵感"]',
}) {
  return CompletionProposalModel(
    id: id,
    captureId: 'c1',
    previewId: 'prev-1',
    sourceRevision: 3,
    fieldName: field,
    originalValue: '[]',
    proposedValue: value,
    provenance: 'ai',
    applyPolicy: policy,
    confidence: 0.9,
    riskLevel: 'low',
    evidenceSpans: const [EvidenceSpanModel(start: 0, end: 2, quote: '工作')],
    status: 'pending',
    provider: 'ollama',
    model: 'qwen2.5:7b',
    configVersion: 'completion-prompt-v1',
  );
}

CompletionPreviewResultModel _preview({
  List<CompletionProposalModel>? proposals,
}) {
  return CompletionPreviewResultModel(
    captureId: 'c1',
    sourceRevision: 3,
    missingFields: const ['tags', 'key_points'],
    proposals: proposals ?? [
      _proposal(id: 'p1', field: 'tags'),
      _proposal(id: 'p2', field: 'key_points', value: '["关键点"]'),
    ],
  );
}

Widget _build({required _FakeGateway gateway, bool enabled = true}) {
  return ProviderScope(
    overrides: [completionGatewayProvider.overrideWithValue(gateway)],
    child: MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: CompletionPanel(captureId: 'c1', enabled: enabled),
        ),
      ),
    ),
  );
}

void main() {
  testWidgets('panel is hidden when disabled', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_build(gateway: gateway, enabled: false));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_entry_button')), findsNothing);
    expect(find.text('AI 补全'), findsNothing);
    expect(gateway.previewCalls, isEmpty);
  });

  testWidgets('entry button loads preview and renders proposals with evidence', (
    tester,
  ) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_entry_button')), findsOneWidget);

    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    expect(gateway.previewCalls, ['c1']);
    // 字段中文标签、建议值 chips、证据引文 chip、置信度。
    expect(find.text('标签'), findsOneWidget);
    expect(find.text('要点'), findsOneWidget);
    expect(find.text('工作'), findsWidgets);
    expect(find.text('灵感'), findsOneWidget);
    expect(find.text('关键点'), findsOneWidget);
    expect(find.text('AI 建议'), findsWidgets);
    expect(find.text('90%'), findsWidgets);
    // 证据引文 chip。
    expect(find.byIcon(Icons.format_quote), findsNWidgets(2));
    // 批量采用 / 撤销按钮。
    expect(find.byKey(const Key('completion_apply_all_safe_button')), findsOneWidget);
    expect(find.byKey(const Key('completion_undo_button')), findsOneWidget);
  });

  testWidgets('apply-all-safe sends safe auto proposal ids', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('completion_apply_all_safe_button')));
    await tester.pumpAndSettle();

    expect(gateway.applyCalls, ['c1']);
    expect(gateway.lastProposalIds, ['p1', 'p2']);
    expect(gateway.lastSourceRevision, 3);
    expect(find.text('已采用 AI 补全'), findsOneWidget);
  });

  testWidgets('per-field apply sends only that proposal id', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    await tester.ensureVisible(
      find.byKey(const Key('completion_apply_field_tags')),
    );
    await tester.tap(find.byKey(const Key('completion_apply_field_tags')));
    await tester.pumpAndSettle();

    expect(gateway.applyCalls, ['c1']);
    expect(gateway.lastProposalIds, ['p1']);
  });

  testWidgets('undo calls the gateway', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(const Key('completion_undo_button')));
    await tester.tap(find.byKey(const Key('completion_undo_button')));
    await tester.pumpAndSettle();

    expect(gateway.undoCalls, ['c1']);
    expect(find.text('已撤销上次补全'), findsOneWidget);
  });

  testWidgets('suggest_only proposal hides the per-field apply button', (
    tester,
  ) async {
    final gateway = _FakeGateway()
      ..previewResult = _preview(
        proposals: [
          _proposal(id: 'p1', field: 'tags'),
          _proposal(
            id: 'p2',
            field: 'summary',
            policy: 'suggest_only',
            value: '一段需要确认的摘要',
          ),
        ],
      );
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_apply_field_tags')), findsOneWidget);
    expect(find.byKey(const Key('completion_apply_field_summary')), findsNothing);
    expect(find.text('需确认'), findsOneWidget);
    // 批量采用只统计 safe_auto → 仅 p1。
    final allSafeButton = tester.widget<FilledButton>(
      find.byKey(const Key('completion_apply_all_safe_button')),
    );
    expect(allSafeButton.onPressed, isNotNull);
  });

  testWidgets('empty preview shows the no-missing-fields note', (tester) async {
    final gateway = _FakeGateway()
      ..previewResult = _preview(proposals: const <CompletionProposalModel>[]);
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    expect(find.text('该记忆暂无缺失字段可补全'), findsOneWidget);
    expect(find.byKey(const Key('completion_apply_all_safe_button')), findsNothing);
  });

  testWidgets('preview error shows the message and a retry entry', (tester) async {
    final gateway = _FakeGateway()
      ..previewError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"FEATURE_NOT_ENABLED"}',
      );
    await tester.pumpWidget(_build(gateway: gateway));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('completion_entry_button')));
    await tester.pumpAndSettle();

    expect(find.text('AI 补全未开启'), findsOneWidget);
    expect(find.byKey(const Key('completion_entry_button')), findsOneWidget);
  });
}
