import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/features/memories/ui/memory_detail_screen.dart';
import 'package:weave_flutter/features/settings/data/ai_settings_api.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

CaptureLite _capture({String id = 'c1', String lifecycleStatus = 'active'}) {
  return CaptureLite(
    id: id,
    userId: 'u1',
    kind: 'text',
    capturedAtPrecision: 'full',
    source: 'capture',
    privacyMode: 'private',
    clientVersion: 1,
    version: 1,
    lifecycleStatus: lifecycleStatus,
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryCardModel _card({
  String id = 'card-1',
  String captureId = 'c1',
  String title = '原标题',
  String primaryType = 'idea',
  String? summary = '这是一段摘要',
  List<String> tags = const ['标签一', '标签二'],
  bool isPinned = false,
  String processingStatus = 'completed',
}) {
  return MemoryCardModel(
    id: id,
    userId: 'u1',
    captureId: captureId,
    primaryType: primaryType,
    title: title,
    summary: summary,
    tags: tags,
    keyPoints: const ['要点一', '要点二'],
    processingStatus: processingStatus,
    version: 1,
    isPinned: isPinned,
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryRevision _fallbackRevision() {
  return MemoryRevision(
    id: 1,
    userId: 'u1',
    captureId: 'c1',
    revision: 1,
    cardVersion: 1,
    source: 'fallback',
    sourceRevision: 1,
    changes: const {'title': '原标题'},
    provenance: const {'title': 'fallback'},
    createdAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryDetail _detail() {
  return MemoryDetail(
    capture: _capture(),
    memoryCard: _card(),
    revisions: [_fallbackRevision()],
  );
}

MemoryMutation _mutation({
  MemoryCardModel? card,
  CaptureLite? capture,
  MemoryRevision? revision,
}) {
  return MemoryMutation(
    capture: capture ?? _capture(),
    memoryCard: card ?? _card(),
    revision: revision,
  );
}

class _FakeGateway implements MemoryGateway {
  _FakeGateway({MemoryDetail? detail}) : detailResult = detail ?? _detail();

  MemoryDetail detailResult;
  ApiException? detailError;
  ApiException? mutationError;

  final List<String> detailCalls = [];
  String? lastCorrectTitle;
  String? lastCorrectSummary;
  String? lastNoteText;
  bool? lastPinned;
  String? lastArchiveId;
  String? lastDeleteId;

  @override
  Future<MemoryDetail> detail(String captureId) async {
    final error = detailError;
    if (error != null) throw error;
    detailCalls.add(captureId);
    return detailResult;
  }

  @override
  Future<MemoryListResult> list({
    String? cursor,
    int? limit,
    String? q,
    String? kind,
    String? primaryType,
    String? lifecycleStatus,
    bool? pinned,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> correct(
    String captureId, {
    String? title,
    String? summary,
    String? primaryType,
    List<String>? tags,
    List<String>? keyPoints,
  }) async {
    final error = mutationError;
    if (error != null) throw error;
    lastCorrectTitle = title;
    lastCorrectSummary = summary;
    return _mutation(
      card: _card(captureId: captureId, title: title ?? '原标题'),
      revision: MemoryRevision(
        id: 2,
        userId: 'u1',
        captureId: captureId,
        revision: 2,
        cardVersion: 2,
        source: 'user',
        sourceRevision: 1,
        changes: {'title': title},
        provenance: const {'title': 'user'},
        createdAt: DateTime.utc(2026, 8, 30),
      ),
    );
  }

  @override
  Future<MemoryMutation> addNote(String captureId, {required String text}) async {
    final error = mutationError;
    if (error != null) throw error;
    lastNoteText = text;
    return _mutation(
      card: _card(captureId: captureId),
      revision: MemoryRevision(
        id: 2,
        userId: 'u1',
        captureId: captureId,
        revision: 2,
        cardVersion: 2,
        source: 'user',
        sourceRevision: 1,
        changes: {'note': text},
        provenance: const {'note': 'user'},
        createdAt: DateTime.utc(2026, 8, 30),
      ),
    );
  }

  @override
  Future<MemoryMutation> setPinned(String captureId, {required bool pinned}) async {
    final error = mutationError;
    if (error != null) throw error;
    lastPinned = pinned;
    return _mutation(
      card: _card(captureId: captureId, isPinned: pinned),
      capture: _capture(id: captureId),
    );
  }

  @override
  Future<MemoryMutation> archive(String captureId) async {
    final error = mutationError;
    if (error != null) throw error;
    lastArchiveId = captureId;
    return _mutation(
      card: _card(captureId: captureId),
      capture: _capture(id: captureId, lifecycleStatus: 'archived'),
    );
  }

  @override
  Future<MemoryMutation> delete(String captureId) async {
    final error = mutationError;
    if (error != null) throw error;
    lastDeleteId = captureId;
    return _mutation(card: _card(captureId: captureId));
  }
}

class _FakeAiSettingsGateway implements AISettingsGateway {
  _FakeAiSettingsGateway({bool aiCompletionEnabled = false})
    // ignore: prefer_initializing_formals
    : _aiCompletionEnabled = aiCompletionEnabled;

  final bool _aiCompletionEnabled;

  @override
  Future<AISettingsResult> get() async {
    return AISettingsResult(
      settings: AISettings(
        userId: 'u1',
        aiMemoryEnabled: false,
        aiCompletionEnabled: _aiCompletionEnabled,
        speechToTextEnabled: false,
        cloudTextAllowed: false,
        cloudAudioAllowed: false,
        revision: 1,
      ),
      pendingReorganize: 0,
    );
  }

  @override
  Future<AISettings> update({
    required int expectedRevision,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<int> reorganize() => throw UnimplementedError();
}

Widget _buildScreen(
  _FakeGateway gateway, {
  String captureId = 'c1',
  _FakeAiSettingsGateway? aiSettings,
}) {
  final router = GoRouter(
    initialLocation: '/home/memories/$captureId',
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(body: Text('home stub')),
        routes: [
          GoRoute(
            path: 'memories/:captureId',
            builder: (context, state) {
              final id = state.pathParameters['captureId'];
              if (id == null || id.isEmpty) {
                return const Scaffold(body: Text('bad id'));
              }
              return MemoryDetailScreen(captureId: id);
            },
          ),
        ],
      ),
      GoRoute(
        path: '/captures/:captureId/transcript',
        builder: (context, state) =>
            const Scaffold(body: Center(child: Text('transcript stub'))),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      memoryGatewayProvider.overrideWithValue(gateway),
      aiSettingsGatewayProvider.overrideWithValue(
        aiSettings ?? _FakeAiSettingsGateway(),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('loads detail by the capture id from the path', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    expect(gateway.detailCalls, ['c1']);
    expect(find.text('原标题'), findsOneWidget);
    expect(find.text('这是一段摘要'), findsOneWidget);
    expect(find.text('标签一'), findsOneWidget);
    expect(find.text('要点一'), findsOneWidget);
  });

  testWidgets('rebuilding with the same captureId reloads by id again', (
    tester,
  ) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();
    expect(gateway.detailCalls, ['c1']);

    // 用全新的 widget 树重建同一 captureId，证明从路径参数按 ID 恢复。
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    expect(gateway.detailCalls, ['c1', 'c1']);
  });

  testWidgets('correct opens the dialog and calls the gateway with the new title', (
    tester,
  ) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('修正字段'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('修正字段'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('correct_title_input')),
      '新标题',
    );
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();

    expect(gateway.lastCorrectTitle, '新标题');
    expect(find.text('新标题'), findsOneWidget);
    expect(find.text('已修正'), findsOneWidget);
  });

  testWidgets('addNote opens the note dialog and calls the gateway', (
    tester,
  ) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('继续思考 / 续写'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('继续思考 / 续写'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('note_text_input')),
      '后续想法',
    );
    await tester.tap(find.text('追加'));
    await tester.pumpAndSettle();

    expect(gateway.lastNoteText, '后续想法');
    expect(find.text('已续写'), findsOneWidget);
  });

  testWidgets('pin toggle calls the gateway', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.push_pin_outlined));
    await tester.pumpAndSettle();

    expect(gateway.lastPinned, isTrue);
    expect(find.text('已置顶'), findsOneWidget);
  });

  testWidgets('archive calls the gateway and shows the message', (tester) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('归档'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('归档'));
    await tester.pumpAndSettle();

    expect(gateway.lastArchiveId, 'c1');
    // 头部状态与 snackbar 都会显示「已归档」。
    expect(find.text('已归档'), findsWidgets);
  });

  testWidgets('delete confirms then calls the gateway and pops back', (
    tester,
  ) async {
    final gateway = _FakeGateway();
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('删除'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('删除'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, '删除'));
    await tester.pumpAndSettle();

    expect(gateway.lastDeleteId, 'c1');
    expect(find.text('home stub'), findsOneWidget);
  });

  testWidgets('mutation failure surfaces a version-conflict message', (
    tester,
  ) async {
    final gateway = _FakeGateway()
      ..mutationError = ApiException(statusCode: 409, message: 'HTTP 409');
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text('归档'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('归档'));
    await tester.pumpAndSettle();

    expect(find.text('版本冲突，请刷新后重试'), findsOneWidget);
  });

  testWidgets('completion panel is hidden when AI completion is disabled', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      detail: MemoryDetail(
        capture: _capture(),
        memoryCard: _card(processingStatus: 'ready'),
        revisions: const [],
      ),
    );
    await tester.pumpWidget(
      _buildScreen(
        gateway,
        aiSettings: _FakeAiSettingsGateway(aiCompletionEnabled: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_entry_button')), findsNothing);
  });

  testWidgets('completion panel is hidden when the card is not ready', (
    tester,
  ) async {
    final gateway = _FakeGateway(); // 默认 card processingStatus 'completed'
    await tester.pumpWidget(
      _buildScreen(
        gateway,
        aiSettings: _FakeAiSettingsGateway(aiCompletionEnabled: true),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_entry_button')), findsNothing);
  });

  testWidgets('completion panel entry shows when AI completion enabled and ready', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      detail: MemoryDetail(
        capture: _capture(),
        memoryCard: _card(processingStatus: 'ready'),
        revisions: const [],
      ),
    );
    await tester.pumpWidget(
      _buildScreen(
        gateway,
        aiSettings: _FakeAiSettingsGateway(aiCompletionEnabled: true),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('completion_entry_button')), findsOneWidget);
  });
}
