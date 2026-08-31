import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';

import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/main.dart';

// ---- 测试辅助：记忆网关替身 ----

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
  Future<MemoryMutation> addNote(String captureId, {required String text}) async {
    throw UnimplementedError();
  }

  @override
  Future<MemoryMutation> setPinned(String captureId, {required bool pinned}) async {
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

Future<_FakeMemoryGateway> _pumpApp(
  WidgetTester tester, {
  _FakeMemoryGateway? gateway,
  Map<String, String>? storage,
}) async {
  FlutterSecureStorage.setMockInitialValues(storage ?? {});
  final database = await databaseFactoryMemory.openDatabase(
    'app_widget_test',
  );
  final store = SembastLocalCaptureStore(database);
  final fakeGateway = gateway ?? _FakeMemoryGateway();

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        localCaptureStoreProvider.overrideWith((ref) async => store),
        captureSyncBootstrapProvider.overrideWith((ref) {}),
        memoryGatewayProvider.overrideWithValue(fakeGateway),
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
}
