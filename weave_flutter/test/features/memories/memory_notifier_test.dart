import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/memories/data/memory_api.dart';
import 'package:weave_flutter/features/memories/domain/memory_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

// ---- 测试辅助构造 ----

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
  String title = '标题',
  bool isPinned = false,
  String primaryType = 'idea',
  int version = 1,
}) {
  return MemoryCardModel(
    id: id,
    userId: 'u1',
    captureId: captureId,
    primaryType: primaryType,
    title: title,
    processingStatus: 'completed',
    version: version,
    isPinned: isPinned,
    createdAt: DateTime.utc(2026, 8, 29),
    updatedAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryRevision _revision({
  int rev = 1,
  int cardVersion = 1,
  String source = 'fallback',
  Map<String, dynamic> changes = const {},
}) {
  return MemoryRevision(
    id: rev,
    userId: 'u1',
    captureId: 'c1',
    revision: rev,
    cardVersion: cardVersion,
    source: source,
    sourceRevision: 1,
    changes: changes,
    provenance: {},
    createdAt: DateTime.utc(2026, 8, 29),
  );
}

MemoryListEntry _entry(String captureId, {String title = '标题'}) {
  return MemoryListEntry(
    capture: _capture(id: captureId),
    memoryCard: _card(captureId: captureId, title: title),
  );
}

MemoryDetail _detail({MemoryCardModel? card, CaptureLite? capture}) {
  return MemoryDetail(
    capture: capture ?? _capture(),
    memoryCard: card ?? _card(),
    revisions: [_revision()],
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

// ---- 假 Gateway ----

class _FakeGateway implements MemoryGateway {
  _FakeGateway({this.listResult, this.detailResult});

  MemoryListResult? listResult;
  MemoryDetail? detailResult;
  MemoryMutation? mutation;
  ApiException? listError;
  ApiException? detailError;
  ApiException? mutationError;

  final List<Map<String, dynamic>> listCalls = [];
  final List<String> detailCalls = [];
  String? lastCorrectTitle;
  String? lastCorrectSummary;
  String? lastCorrectPrimaryType;
  List<String>? lastCorrectTags;
  List<String>? lastCorrectKeyPoints;
  String? lastNoteText;
  bool? lastPinned;
  String? lastArchiveId;
  String? lastDeleteId;

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
    final error = listError;
    if (error != null) throw error;
    listCalls.add({
      'cursor': cursor,
      'limit': limit,
      'q': q,
      'kind': kind,
      'primaryType': primaryType,
      'lifecycleStatus': lifecycleStatus,
      'pinned': pinned,
    });
    final result = listResult;
    if (result != null) return result;
    return const MemoryListResult(items: []);
  }

  @override
  Future<MemoryDetail> detail(String captureId) async {
    final error = detailError;
    if (error != null) throw error;
    detailCalls.add(captureId);
    final result = detailResult;
    if (result != null) return result;
    return _detail(card: _card(captureId: captureId));
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
    lastCorrectPrimaryType = primaryType;
    lastCorrectTags = tags;
    lastCorrectKeyPoints = keyPoints;
    final m = mutation;
    if (m != null) return m;
    return _mutation(
      card: _card(captureId: captureId, title: title ?? '标题'),
      revision: _revision(
        rev: 2,
        cardVersion: 2,
        source: 'user',
        changes: {'title': title},
      ),
    );
  }

  @override
  Future<MemoryMutation> addNote(String captureId, {required String text}) async {
    final error = mutationError;
    if (error != null) throw error;
    lastNoteText = text;
    final m = mutation;
    if (m != null) return m;
    return _mutation(
      card: _card(captureId: captureId),
      revision: _revision(
        rev: 2,
        cardVersion: 2,
        source: 'user',
        changes: {'note': text},
      ),
    );
  }

  @override
  Future<MemoryMutation> setPinned(String captureId, {required bool pinned}) async {
    final error = mutationError;
    if (error != null) throw error;
    lastPinned = pinned;
    final m = mutation;
    if (m != null) return m;
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
    final m = mutation;
    if (m != null) return m;
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
    final m = mutation;
    if (m != null) return m;
    return _mutation(card: _card(captureId: captureId));
  }
}

ProviderContainer _container(_FakeGateway gateway) {
  final c = ProviderContainer(
    overrides: [
      memoryGatewayProvider.overrideWithValue(gateway),
    ],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  group('MemoryListNotifier', () {
    test('load publishes loaded state and passes filters', () async {
      final gateway = _FakeGateway(
        listResult: MemoryListResult(items: [_entry('c1')], nextCursor: 'pg2'),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryListNotifierProvider.notifier);

      await notifier.load(
        filter: MemoryListFilter(
          q: '你好',
          primaryType: 'idea',
          lifecycleStatus: 'archived',
          pinnedOnly: true,
        ),
      );

      final state = c.read(memoryListNotifierProvider);
      expect(state, isA<MemoryListLoaded>());
      final loaded = state as MemoryListLoaded;
      expect(loaded.items, hasLength(1));
      expect(loaded.hasMore, isTrue);
      final call = gateway.listCalls.single;
      expect(call['q'], '你好');
      expect(call['primaryType'], 'idea');
      expect(call['lifecycleStatus'], 'archived');
      expect(call['pinned'], isTrue);
      expect(call['cursor'], isNull);
    });

    test('load failure publishes error state', () async {
      final gateway = _FakeGateway()..listError = ApiException(message: 'boom');
      final c = _container(gateway);
      final notifier = c.read(memoryListNotifierProvider.notifier);

      await notifier.load();

      expect(c.read(memoryListNotifierProvider), isA<MemoryListError>());
    });

    test('loadMore passes the stored cursor and appends', () async {
      final gateway = _FakeGateway(
        listResult: MemoryListResult(
          items: [_entry('c1')],
          nextCursor: 'pg2',
        ),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryListNotifierProvider.notifier);
      await notifier.load();

      gateway.listResult = MemoryListResult(items: [_entry('c2')]);
      await notifier.loadMore();

      final loaded = c.read(memoryListNotifierProvider) as MemoryListLoaded;
      expect(loaded.items, hasLength(2));
      expect(loaded.hasMore, isFalse);
      expect(gateway.listCalls[1]['cursor'], 'pg2');
    });

    test('search resets the cursor and reloads with the new query', () async {
      final gateway = _FakeGateway(
        listResult: MemoryListResult(items: [_entry('c1')], nextCursor: 'x'),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryListNotifierProvider.notifier);
      await notifier.load();

      gateway.listResult = MemoryListResult(items: [_entry('c2')]);
      await notifier.search(' 新查询 ');

      expect(gateway.listCalls.last['q'], '新查询');
      expect(gateway.listCalls.last['cursor'], isNull);
      final loaded = c.read(memoryListNotifierProvider) as MemoryListLoaded;
      expect(loaded.items.single.capture.id, 'c2');
    });

    test('removeLocally removes an entry by capture id', () async {
      final gateway = _FakeGateway(
        listResult: MemoryListResult(items: [_entry('c1'), _entry('c2')]),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryListNotifierProvider.notifier);
      await notifier.load();

      notifier.removeLocally('c1');

      final loaded = c.read(memoryListNotifierProvider) as MemoryListLoaded;
      expect(loaded.items, hasLength(1));
      expect(loaded.items.single.capture.id, 'c2');
    });
  });

  group('MemoryDetailNotifier', () {
    test('load fetches detail by capture id', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);

      await notifier.load('c1');

      expect(gateway.detailCalls, ['c1']);
      final state = c.read(memoryDetailNotifierProvider);
      expect(state, isA<MemoryDetailLoaded>());
    });

    test('load failure publishes error state', () async {
      final gateway = _FakeGateway()..detailError = ApiException(message: 'boom');
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);

      await notifier.load('c1');

      expect(c.read(memoryDetailNotifierProvider), isA<MemoryDetailError>());
    });

    test('correct updates the card, appends a revision and sets a message', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.correct(title: '新标题');

      expect(ok, isTrue);
      expect(gateway.lastCorrectTitle, '新标题');
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.detail.memoryCard.title, '新标题');
      expect(loaded.detail.revisions, hasLength(2));
      expect(loaded.message, '已修正');
      expect(loaded.saving, isFalse);
    });

    test('addNote appends a user revision and sets a message', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.addNote(' 后续想法 ');

      expect(ok, isTrue);
      expect(gateway.lastNoteText, ' 后续想法 ');
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.detail.revisions, hasLength(2));
      expect(loaded.message, '已续写');
    });

    test('setPinned toggles the card', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.setPinned(true);

      expect(ok, isTrue);
      expect(gateway.lastPinned, isTrue);
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.detail.memoryCard.isPinned, isTrue);
      expect(loaded.message, '已置顶');
    });

    test('archive updates the lifecycle status', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.archive();

      expect(ok, isTrue);
      expect(gateway.lastArchiveId, 'c1');
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.detail.capture.lifecycleStatus, 'archived');
      expect(loaded.message, '已归档');
    });

    test('delete calls the gateway and sets a message', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      );
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.delete();

      expect(ok, isTrue);
      expect(gateway.lastDeleteId, 'c1');
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.message, '已删除');
    });

    test('correct surfaces a version-conflict message on 409', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      )..mutationError = ApiException(statusCode: 409, message: 'HTTP 409');
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.correct(title: 'x');

      expect(ok, isFalse);
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.message, '版本冲突，请刷新后重试');
      expect(loaded.saving, isFalse);
    });

    test('correct surfaces a not-found message on 404', () async {
      final gateway = _FakeGateway(
        detailResult: _detail(card: _card(captureId: 'c1')),
      )..mutationError = ApiException(statusCode: 404, message: 'HTTP 404');
      final c = _container(gateway);
      final notifier = c.read(memoryDetailNotifierProvider.notifier);
      await notifier.load('c1');

      final ok = await notifier.archive();

      expect(ok, isFalse);
      final loaded = c.read(memoryDetailNotifierProvider) as MemoryDetailLoaded;
      expect(loaded.message, '记忆不存在');
    });
  });
}
