import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/features/memories/ui/memory_list_screen.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

CaptureLite _capture({String id = 'c1', String kind = 'text'}) {
  return CaptureLite(
    id: id,
    userId: 'u1',
    kind: kind,
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

MemoryCardModel _card({
  String id = 'card-1',
  String captureId = 'c1',
  String title = '标题',
  String primaryType = 'idea',
  String? summary,
  List<String> tags = const [],
  bool isPinned = false,
}) {
  return MemoryCardModel(
    id: id,
    userId: 'u1',
    captureId: captureId,
    primaryType: primaryType,
    title: title,
    summary: summary,
    tags: tags,
    processingStatus: 'completed',
    version: 1,
    isPinned: isPinned,
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryListEntry _entry({
  String captureId = 'c1',
  String title = '标题',
  String primaryType = 'idea',
  String? summary,
  bool isPinned = false,
}) {
  return MemoryListEntry(
    capture: _capture(id: captureId),
    memoryCard: _card(
      captureId: captureId,
      title: title,
      primaryType: primaryType,
      summary: summary,
      isPinned: isPinned,
    ),
  );
}

class _FakeGateway implements MemoryGateway {
  _FakeGateway({List<MemoryListResult>? pages}) : listPages = pages ?? [];

  final List<MemoryListResult> listPages;
  final List<Map<String, dynamic>> listCalls = [];
  int _pageIndex = 0;

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
    listCalls.add({
      'cursor': cursor,
      'q': q,
      'kind': kind,
      'primaryType': primaryType,
      'lifecycleStatus': lifecycleStatus,
      'pinned': pinned,
    });
    final error = errorMessage;
    if (error != null) {
      throw ApiException(message: error);
    }
    if (_pageIndex < listPages.length) {
      final page = listPages[_pageIndex];
      _pageIndex += 1;
      return page;
    }
    return const MemoryListResult(items: []);
  }

  String? errorMessage;

  @override
  Future<MemoryDetail> detail(String captureId) async {
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

Widget _buildScreen(_FakeGateway gateway) {
  final router = GoRouter(
    initialLocation: '/memories',
    routes: [
      GoRoute(
        path: '/memories',
        builder: (context, state) => const MemoryListScreen(),
      ),
      GoRoute(
        path: '/memories/:captureId',
        builder: (context, state) =>
            const Scaffold(body: Center(child: Text('detail stub'))),
      ),
      GoRoute(
        path: '/capture',
        builder: (context, state) =>
            const Scaffold(body: Center(child: Text('capture stub'))),
      ),
    ],
  );
  return ProviderScope(
    overrides: [memoryGatewayProvider.overrideWithValue(gateway)],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('renders memory cards with type chips and one action button each', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      pages: [
        MemoryListResult(
          items: [
            _entry(captureId: 'c1', title: '闪念标题', primaryType: 'idea'),
            _entry(
              captureId: 'c2',
              title: '行动标题',
              primaryType: 'action',
              summary: '摘要内容',
              isPinned: true,
            ),
          ],
        ),
      ],
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    expect(find.text('闪念标题'), findsOneWidget);
    expect(find.text('行动标题'), findsOneWidget);
    expect(find.text('摘要内容'), findsOneWidget);
    expect(find.text('继续思考'), findsOneWidget);
    expect(find.text('去完成'), findsOneWidget);

    // 每个记忆卡恰好一个主要下一步按钮（SearchBar 内部也有 Card，需按包含按钮的 Card 过滤）。
    final cardButtons = find.byType(FilledButton);
    expect(cardButtons, findsNWidgets(2));

    final memoryCards = find.ancestor(
      of: cardButtons,
      matching: find.byType(Card),
    );
    expect(memoryCards, findsNWidgets(2));

    // 类型 chip 只在卡内出现（筛选行也有 '闪念' / '行动'，需限定在卡内）。
    expect(
      find.descendant(of: memoryCards, matching: find.text('闪念')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: memoryCards, matching: find.text('行动')),
      findsOneWidget,
    );
    for (final element in memoryCards.evaluate()) {
      final buttons = find.descendant(
        of: find.byWidget(element.widget),
        matching: find.byType(FilledButton),
      );
      expect(buttons, findsOneWidget);
    }
  });

  testWidgets('empty state shows a hint and a capture button', (tester) async {
    final gateway = _FakeGateway(
      pages: [const MemoryListResult(items: [])],
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    expect(find.text('还没有记忆'), findsOneWidget);

    await tester.tap(find.text('去捕捉'));
    await tester.pumpAndSettle();

    expect(find.text('capture stub'), findsOneWidget);
  });

  testWidgets('error state shows retry and reloads on tap', (tester) async {
    final gateway = _FakeGateway()..errorMessage = '网络错误';
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    expect(find.text('网络错误'), findsOneWidget);
    final callsBeforeRetry = gateway.listCalls.length;

    gateway.errorMessage = null;
    gateway.listPages.add(
      const MemoryListResult(items: []),
    );

    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();

    expect(gateway.listCalls.length, greaterThan(callsBeforeRetry));
    expect(find.text('还没有记忆'), findsOneWidget);
  });

  testWidgets('typing triggers a debounced search with the query', (tester) async {
    final gateway = _FakeGateway(
      pages: [MemoryListResult(items: [_entry()])],
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField).first, '你好');
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pumpAndSettle();

    expect(gateway.listCalls.last['q'], '你好');
  });

  testWidgets('load-more fetches the next page with the stored cursor', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      pages: [
        MemoryListResult(items: [_entry(captureId: 'c1')], nextCursor: 'pg2'),
        MemoryListResult(items: [_entry(captureId: 'c2')]),
      ],
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pumpAndSettle();

    // 第一页（含尾部的加载项）构建后自动触发 loadMore。
    expect(gateway.listCalls.length, greaterThanOrEqualTo(2));
    expect(
      gateway.listCalls.any((call) => call['cursor'] == 'pg2'),
      isTrue,
    );
    expect(find.text('标题'), findsNWidgets(2)); // 两页都有 '标题'
  });
}
