import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/completions/data/completion_api.dart';
import 'package:weave_flutter/features/completions/domain/completion_notifier.dart';
import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class _FakeGateway implements CompletionGateway {
  CompletionPreviewResultModel? previewResult;
  CompletionApplyResultModel applyResult = CompletionApplyResultModel(
    memoryCard: _card(),
    appliedProposalIds: const <String>[],
  );
  ApiException? previewError;
  ApiException? applyError;
  ApiException? undoError;

  final List<String> previewCalls = [];
  final List<String> applyCalls = [];
  final List<String> undoCalls = [];
  List<String>? lastProposalIds;
  int? lastSourceRevision;

  @override
  Future<CompletionPreviewResultModel> preview(String captureId) async {
    previewCalls.add(captureId);
    final error = previewError;
    if (error != null) throw error;
    return previewResult ?? _preview();
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
    return applyResult;
  }

  @override
  Future<CompletionApplyResultModel> undo(String captureId) async {
    undoCalls.add(captureId);
    final error = undoError;
    if (error != null) throw error;
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
}) {
  return CompletionProposalModel(
    id: id,
    captureId: 'c1',
    previewId: 'prev-1',
    sourceRevision: 3,
    fieldName: field,
    originalValue: '[]',
    proposedValue: '["工作","灵感"]',
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

CompletionPreviewResultModel _preview({List<CompletionProposalModel>? proposals}) {
  return CompletionPreviewResultModel(
    captureId: 'c1',
    sourceRevision: 3,
    missingFields: const ['tags', 'key_points'],
    proposals: proposals ?? [_proposal(), _proposal(id: 'p2', field: 'key_points')],
  );
}

void main() {
  ProviderContainer container({CompletionGateway? gateway}) {
    final c = ProviderContainer(
      overrides: [
        completionGatewayProvider.overrideWithValue(gateway ?? _FakeGateway()),
      ],
    );
    addTearDown(c.dispose);
    return c;
  }

  test('loadPreview publishes loaded state with proposals', () async {
    final gateway = _FakeGateway();
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);

    await notifier.loadPreview('c1');

    expect(gateway.previewCalls, ['c1']);
    final state = c.read(completionNotifierProvider);
    expect(state, isA<CompletionLoaded>());
    final loaded = state as CompletionLoaded;
    expect(loaded.preview?.proposals, hasLength(2));
    expect(loaded.message, isNull);
  });

  test('loadPreview failure publishes error state', () async {
    final gateway = _FakeGateway()
      ..previewError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"FEATURE_NOT_ENABLED"}',
      );
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);

    await notifier.loadPreview('c1');

    expect(c.read(completionNotifierProvider), isA<CompletionError>());
  });

  test('applyAllSafe sends only safe_auto pending proposal ids', () async {
    final gateway = _FakeGateway();
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.applyAllSafe('c1');

    expect(gateway.lastProposalIds, ['p1', 'p2']);
    expect(gateway.lastSourceRevision, 3);
    expect(result, isNotNull);
    final loaded = c.read(completionNotifierProvider) as CompletionLoaded;
    expect(loaded.message, '已采用 AI 补全');
    // 采用后清空预览，收起提案列表。
    expect(loaded.preview, isNull);
  });

  test('applyAllSafe skips suggest_only proposals', () async {
    final gateway = _FakeGateway()
      ..previewResult = _preview(proposals: [
        _proposal(policy: 'safe_auto'),
        _proposal(id: 'p2', field: 'key_points', policy: 'suggest_only'),
      ]);
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.applyAllSafe('c1');

    expect(gateway.lastProposalIds, ['p1']);
    expect(result, isNotNull);
  });

  test('applyAllSafe returns null when there is nothing to apply', () async {
    final gateway = _FakeGateway()
      ..previewResult = _preview(
        proposals: [
          _proposal(policy: 'suggest_only'),
          _proposal(id: 'p2', field: 'key_points', policy: 'suggest_only'),
        ],
      );
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.applyAllSafe('c1');

    expect(result, isNull);
    expect(gateway.applyCalls, isEmpty);
  });

  test('applyOne applies the single proposal', () async {
    final gateway = _FakeGateway();
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');
    final proposal = _proposal();

    final result = await notifier.applyOne('c1', proposal);

    expect(gateway.lastProposalIds, ['p1']);
    expect(gateway.lastSourceRevision, 3);
    expect(result, isNotNull);
  });

  test('apply version-conflict 409 surfaces the stale-proposal message', () async {
    final gateway = _FakeGateway()
      ..applyError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"VERSION_CONFLICT"}',
      );
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.applyAllSafe('c1');

    expect(result, isNull);
    final loaded = c.read(completionNotifierProvider) as CompletionLoaded;
    expect(loaded.message, '提案已过期，请重新生成');
    expect(loaded.busy, isFalse);
  });

  test('apply feature-not-enabled 409 surfaces the disabled message', () async {
    final gateway = _FakeGateway()
      ..applyError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"FEATURE_NOT_ENABLED"}',
      );
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    await notifier.applyAllSafe('c1');

    final loaded = c.read(completionNotifierProvider) as CompletionLoaded;
    expect(loaded.message, 'AI 补全未开启');
  });

  test('undo calls the gateway and clears the preview', () async {
    final gateway = _FakeGateway();
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.undo('c1');

    expect(gateway.undoCalls, ['c1']);
    expect(result, isNotNull);
    final loaded = c.read(completionNotifierProvider) as CompletionLoaded;
    expect(loaded.message, '已撤销上次补全');
    expect(loaded.preview, isNull);
  });

  test('undo nothing-to-undo 409 surfaces a message', () async {
    final gateway = _FakeGateway()
      ..undoError = ApiException(
        statusCode: 409,
        message: 'HTTP 409',
        data: '{"code":"PRECONDITION_FAILED"}',
      );
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');

    final result = await notifier.undo('c1');

    expect(result, isNull);
    final loaded = c.read(completionNotifierProvider) as CompletionLoaded;
    expect(loaded.message, '记忆尚未准备好，或没有可撤销的补全');
    expect(loaded.busy, isFalse);
    // 预览保留（撤销失败不销毁提案）。
    expect(loaded.preview, isNotNull);
  });

  test('clearMessage clears the last message', () async {
    final gateway = _FakeGateway();
    final c = container(gateway: gateway);
    final notifier = c.read(completionNotifierProvider.notifier);
    await notifier.loadPreview('c1');
    await notifier.applyAllSafe('c1');
    expect(
      (c.read(completionNotifierProvider) as CompletionLoaded).message,
      '已采用 AI 补全',
    );

    notifier.clearMessage();

    expect(
      (c.read(completionNotifierProvider) as CompletionLoaded).message,
      isNull,
    );
  });
}
