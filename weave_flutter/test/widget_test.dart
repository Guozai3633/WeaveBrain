import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:weave_flutter/features/auth/ui/login_screen.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage_service.dart';
import 'package:weave_flutter/features/capture/data/audio_recorder.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';
import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/imports/data/import_api.dart';
import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/features/platform/data/capabilities_api.dart';
import 'package:weave_flutter/features/settings/data/ai_settings_api.dart';
import 'package:weave_flutter/features/settings/ui/settings_screen.dart';
import 'package:weave_flutter/features/workflows/ui/workflow_list_screen.dart';
import 'package:weave_flutter/main.dart';
import 'package:weave_flutter/shared/native/launch_request.dart';
import 'package:weave_flutter/shared/native/native_services.dart';

import 'support/echo_fakes.dart';

// ---- 测试辅助：记忆网关替身 ----

class _FakeAiSettingsGateway implements AISettingsGateway {
  @override
  Future<AISettingsResult> get() async {
    return AISettingsResult(
      settings: AISettings(
        userId: 'u1',
        aiMemoryEnabled: false,
        aiCompletionEnabled: false,
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

class _FakeImportGateway implements ImportGateway {
  @override
  Future<ImportJobModel> createJob({
    required String format,
    required String sourceName,
    required String content,
    String? separator,
    String? timezone,
    String? originalFilename,
  }) async {
    return ImportJobModel(
      id: 'job1',
      userId: 'u1',
      sourceName: sourceName,
      format: format,
      rawText: content,
      columnMapping: const {},
      separator: separator ?? '---',
      totalRows: 0,
      validRows: 0,
      invalidRows: 0,
      duplicateRows: 0,
      needsInputRows: 0,
      importedRows: 0,
      skippedRows: 0,
      failedRows: 0,
      status: 'draft',
      createdAt: DateTime.utc(2026, 8, 29),
      updatedAt: DateTime.utc(2026, 8, 29),
    );
  }

  @override
  Future<ImportJobModel> getJob(String jobId) => throw UnimplementedError();

  @override
  Future<ImportPreviewModel> preview(
    String jobId, {
    int limit = 10,
    int offset = 0,
  }) async {
    return ImportPreviewModel(
      job: await createJob(
        format: 'plain_text',
        sourceName: '旧备忘录',
        content: '',
      ),
      rows: const [],
    );
  }

  @override
  Future<ImportCompletionModel> completionPreview(
    String jobId, {
    List<int>? rowNumbers,
  }) => throw UnimplementedError();

  @override
  Future<ImportCompletionModel> completionApply(
    String jobId, {
    required List<ImportRowSelection> selections,
  }) => throw UnimplementedError();

  @override
  Future<ImportCommitResultModel> commit(
    String jobId, {
    required String duplicateContentAction,
    Map<int, String>? rowActions,
  }) async {
    return ImportCommitResultModel(
      imported: 0,
      failed: 0,
      skipped: 0,
      needsInput: 0,
      total: 0,
      jobStatus: 'completed',
    );
  }

  @override
  Future<List<ImportErrorReportEntryModel>> errorReport(String jobId) async {
    return const [];
  }

  @override
  Future<SingleImportResult> importSingle(SingleImportDraft draft) async {
    return SingleImportResult(captureId: 'imported-c1');
  }
}

class _FakeMemoryGateway implements MemoryGateway {
  final List<String> detailCalls = [];

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
    return MemoryListResult(items: [_entry()]);
  }

  @override
  Future<MemoryDetail> detail(String captureId) async {
    detailCalls.add(captureId);
    return _detail(captureId);
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
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> addNote(
    String captureId, {
    required String text,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> setPinned(
    String captureId, {
    required bool pinned,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> archive(String captureId) async {
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> delete(String captureId) async {
    throw UnimplementedError();
  }
}

CaptureLite _capture(String id) {
  return CaptureLite(
    id: id,
    userId: 'u1',
    kind: 'text',
    capturedAtPrecision: 'full',
    source: 'capture',
    privacyMode: 'private',
    clientVersion: 1,
    version: 1,
    lifecycleStatus: 'active',
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryCardModel _card(String captureId) {
  return MemoryCardModel(
    id: 'card-$captureId',
    userId: 'u1',
    captureId: captureId,
    primaryType: 'idea',
    title: '闪念标题',
    summary: '一段摘要',
    processingStatus: 'completed',
    version: 1,
    isPinned: false,
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryListEntry _entry() {
  return MemoryListEntry(capture: _capture('c1'), memoryCard: _card('c1'));
}

MemoryDetail _detail(String captureId) {
  return MemoryDetail(
    capture: _capture(captureId),
    memoryCard: _card(captureId),
    revisions: const [],
  );
}

class _FakeAudioRecorder implements AudioRecorder {
  _FakeAudioRecorder();

  final List<String> startedPaths = [];

  @override
  bool get supportsFileRecording => true;

  @override
  Future<bool> hasPermission() async => true;

  @override
  Future<void> start({required String path}) async {
    startedPaths.add(path);
  }

  @override
  Stream<double> get amplitude => const Stream<double>.empty();

  @override
  Future<String?> stop() async =>
      startedPaths.isEmpty ? null : startedPaths.last;

  @override
  Future<void> dispose() async {}
}

class _FakeAudioFileStorage implements AudioFileStorage {
  @override
  Future<String> getDirectory() async => '/tmp/widget';

  @override
  Future<AudioFileMetadata> readMetadata(String filePath) async {
    return const AudioFileMetadata(sizeBytes: 441000, sha256: 'feedface');
  }

  @override
  Future<void> deleteFile(String filePath) async {}
}

class _FakeCapabilitiesGateway implements CapabilitiesGateway {
  const _FakeCapabilitiesGateway();

  @override
  Future<PlatformCapabilities> fetch() async =>
      const PlatformCapabilities.disabled();
}

/// 游客记录（owner == null）构造器：登录前在本机保存的捕捉。
LocalCapture _guestCapture(String id) {
  final now = DateTime.utc(2026, 8, 30);
  return LocalCapture(
    id: id,
    text: '游客记录内容',
    capturedAt: now,
    source: 'web',
    syncState: LocalSyncState.savedLocal,
    createdAt: now,
    updatedAt: now,
  );
}

Future<_FakeMemoryGateway> _pumpApp(
  WidgetTester tester, {
  _FakeMemoryGateway? gateway,
  Map<String, String>? storage,
  List<Override> extraOverrides = const [],
}) async {
  FlutterSecureStorage.setMockInitialValues(storage ?? {});
  SharedPreferences.setMockInitialValues({});
  final database = await databaseFactoryMemory.openDatabase('app_widget_test');
  final store = SembastLocalCaptureStore(database);
  final fakeGateway = gateway ?? _FakeMemoryGateway();

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        localCaptureStoreProvider.overrideWith((ref) async => store),
        captureSyncBootstrapProvider.overrideWith((ref) {}),
        memoryGatewayProvider.overrideWithValue(fakeGateway),
        aiSettingsGatewayProvider.overrideWithValue(_FakeAiSettingsGateway()),
        importGatewayProvider.overrideWithValue(_FakeImportGateway()),
        capabilitiesGatewayProvider.overrideWithValue(
          _FakeCapabilitiesGateway(),
        ),
        // R11 注入：native 服务 + 回响网关全部换假，避免任何插件通道/真实 HTTP。
        localNotificationServiceProvider.overrideWithValue(
          FakeLocalNotificationService(),
        ),
        quickActionsServiceProvider.overrideWithValue(FakeQuickActionsService()),
        homeWidgetServiceProvider.overrideWithValue(FakeHomeWidgetService()),
        timezoneServiceProvider.overrideWithValue(FakeTimezoneService()),
        echoGatewayProvider.overrideWithValue(FakeEchoGateway()),
        echoSettingsGatewayProvider.overrideWithValue(FakeEchoSettingsGateway()),
        captureFeedbackServiceProvider.overrideWithValue(
          FakeCaptureFeedbackService(),
        ),
        ...extraOverrides,
      ],
      child: const WeaveBrainApp(),
    ),
  );
  // 等待 checkAuth 解析与首次路由渲染。
  for (var frame = 0; frame < 30; frame++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
  return fakeGateway;
}

void main() {
  testWidgets('guest redirects to local capture and sees 4 destinations', (
    tester,
  ) async {
    await _pumpApp(tester);

    expect(find.text('快速记录'), findsOneWidget);
    expect(find.text('游客模式：内容只保存在本机'), findsOneWidget);
    expect(find.text('登录同步'), findsOneWidget);

    // 底部导航恰好 4 个目的地：记忆 / 捕捉 / 回响 / 我的。
    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationDestination), findsNWidgets(4));
    final nav = find.byType(NavigationBar);
    for (final label in ['记忆', '捕捉', '回响', '我的']) {
      expect(
        find.descendant(of: nav, matching: find.text(label)),
        findsOneWidget,
      );
    }

    // 旧功能路由（想法 / 时间线 / 项目）不在底部导航。
    for (final label in ['想法', '时间线', '项目']) {
      expect(
        find.descendant(of: nav, matching: find.text(label)),
        findsNothing,
      );
    }

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets(
    'authenticated user lands on memory stream; detail loads by capture id',
    (tester) async {
      final gateway = await _pumpApp(
        tester,
        storage: {
          'jwt_token': 'test-token',
          'user_data': jsonEncode({'id': 'u1', 'display_name': '测试用户'}),
        },
      );

      // 登录用户默认落在记忆流，列表渲染一条记忆卡。
      expect(find.text('闪念标题'), findsOneWidget);
      expect(find.text('继续思考'), findsOneWidget); // idea → 主按钮
      expect(find.byType(NavigationDestination), findsNWidgets(4));

      // 点击卡上的主按钮 → 进入 /memories/c1 → 按路径参数 captureId 加载详情。
      await tester.tap(find.text('继续思考'));
      await tester.pumpAndSettle();

      expect(gateway.detailCalls, ['c1']);
      expect(find.text('记忆详情'), findsOneWidget); // 详情页 AppBar
      expect(find.text('闪念标题'), findsWidgets); // 列表卡仍在树中 + 详情头部

      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump();
    },
  );

  testWidgets('guest visiting /workflows is redirected to login', (
    tester,
  ) async {
    await _pumpApp(tester);

    // 游客落在捕捉页；直接访问预留的 /workflows 应被守卫重定向到登录页。
    final router = GoRouter.of(tester.element(find.byType(NavigationBar)));
    router.go('/workflows');
    await tester.pumpAndSettle();

    expect(find.byType(LoginScreen), findsOneWidget);
    expect(find.byType(WorkflowListScreen), findsNothing);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('authenticated settings shows pending guest merge count', (
    tester,
  ) async {
    await _pumpApp(
      tester,
      storage: {
        'jwt_token': 'test-token',
        'user_data': jsonEncode({'id': 'u1', 'display_name': '测试用户'}),
      },
      extraOverrides: [
        guestLocalCapturesProvider.overrideWith(
          (ref) => Stream.value([_guestCapture('g1')]),
        ),
      ],
    );

    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('我的'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('游客记录 1 条将在登录后自动合并到账号，不产生重复。'), findsOneWidget);
    expect(find.text('暂无待合并的游客记录。'), findsNothing);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets(
    'authenticated settings shows no pending and opens /workflows page',
    (tester) async {
      await _pumpApp(
        tester,
        storage: {
          'jwt_token': 'test-token',
          'user_data': jsonEncode({'id': 'u1', 'display_name': '测试用户'}),
        },
      );

      await tester.tap(
        find.descendant(
          of: find.byType(NavigationBar),
          matching: find.text('我的'),
        ),
      );
      await tester.pumpAndSettle();

      // 无游客记录 → 只读区块提示「暂无待合并」。
      expect(find.text('暂无待合并的游客记录。'), findsOneWidget);

      // 「工作流」入口展示「规划中」；点击进入 /workflows 占位页而非编辑器。
      final settingsScrollable = find.descendant(
        of: find.byType(SettingsScreen),
        matching: find.byType(Scrollable),
      );
      await tester.scrollUntilVisible(
        find.widgetWithText(ListTile, '工作流'),
        120,
        scrollable: settingsScrollable,
      );
      await tester.tap(find.widgetWithText(ListTile, '工作流'));
      await tester.pumpAndSettle();

      expect(find.byType(WorkflowListScreen), findsOneWidget);
      expect(find.text('还没有工作流'), findsOneWidget);
      expect(find.textContaining('设计与执行能力均未开放'), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump();
    },
  );

  testWidgets('global capture FAB is present and opens the minimal record', (
    tester,
  ) async {
    final recorder = _FakeAudioRecorder();
    await _pumpApp(
      tester,
      extraOverrides: [
        audioRecorderDeviceProvider.overrideWithValue(recorder),
        audioFileStorageProvider.overrideWithValue(_FakeAudioFileStorage()),
      ],
    );

    // 全局 FAB 由 AppScaffold 提供，所有 tab 均可见。
    expect(find.byKey(const Key('global_capture_fab')), findsOneWidget);

    await tester.tap(find.byKey(const Key('global_capture_fab')));
    await tester.pumpAndSettle();

    // 极简录音：进入即自动开录，无需任何主动操作。
    expect(recorder.startedPaths, hasLength(1));
    expect(find.text('点按任意处结束并保存'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('notification launch resolves an open echo into its memory', (
    tester,
  ) async {
    final gateway = await _pumpApp(
      tester,
      storage: {
        'jwt_token': 'test-token',
        'user_data': jsonEncode({'id': 'u1', 'display_name': '测试用户'}),
      },
      extraOverrides: [
        echoGatewayProvider.overrideWithValue(
          FakeEchoGateway(current: makeCurrentLoaded(captureId: 'c1')),
        ),
      ],
    );

    final container = ProviderScope.containerOf(
      tester.element(find.byType(NavigationBar)),
    );
    container.read(launchRequestProvider.notifier).state = const LaunchRequest(
      LaunchSource.notification,
    );
    await tester.pumpAndSettle();

    // 通知 → 拉 /echoes/current → 深链 /memories/c1 → 按 captureId 加载详情。
    expect(gateway.detailCalls, ['c1']);
    expect(find.text('记忆详情'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('guest notification launch lands on the echo hub', (
    tester,
  ) async {
    await _pumpApp(tester);

    final container = ProviderScope.containerOf(
      tester.element(find.byType(NavigationBar)),
    );
    container.read(launchRequestProvider.notifier).state = const LaunchRequest(
      LaunchSource.notification,
    );
    await tester.pumpAndSettle();

    // 游客通知点击 → 落回响 tab（本页游客态展示登录引导）。
    expect(find.text('登录后开启回响'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('quick capture shortcut launch opens the minimal record', (
    tester,
  ) async {
    final recorder = _FakeAudioRecorder();
    await _pumpApp(
      tester,
      extraOverrides: [
        audioRecorderDeviceProvider.overrideWithValue(recorder),
        audioFileStorageProvider.overrideWithValue(_FakeAudioFileStorage()),
      ],
    );

    final container = ProviderScope.containerOf(
      tester.element(find.byType(NavigationBar)),
    );
    container.read(launchRequestProvider.notifier).state = const LaunchRequest(
      LaunchSource.quickCapture,
    );
    await tester.pumpAndSettle();

    expect(recorder.startedPaths, hasLength(1));
    expect(find.text('点按任意处结束并保存'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });
}
