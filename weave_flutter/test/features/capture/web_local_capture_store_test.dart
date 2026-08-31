@TestOn('browser')
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:sembast_web/sembast_web.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';

void main() {
  const databaseName = 'weavebrain_r3_browser_acceptance';

  setUp(() async {
    await databaseFactoryWeb.deleteDatabase(databaseName);
  });

  tearDown(() async {
    await databaseFactoryWeb.deleteDatabase(databaseName);
  });

  test('Chrome IndexedDB keeps 20 captures across database reopen', () async {
    var database = await databaseFactoryWeb.openDatabase(databaseName);
    var store = SembastLocalCaptureStore(database);
    final startedAt = DateTime.utc(2026, 8, 18, 3);

    for (var index = 0; index < 20; index++) {
      final now = startedAt.add(Duration(seconds: index));
      final capture = LocalCapture(
        id: '00000000-0000-4000-8000-${index.toString().padLeft(12, '0')}',
        text: 'Chrome offline memory $index',
        capturedAt: now,
        source: 'web',
        syncState: LocalSyncState.savedLocal,
        createdAt: now,
        updatedAt: now,
      );
      await store.saveDraft(capture);
    }

    await store.close();

    database = await databaseFactoryWeb.openDatabase(databaseName);
    store = SembastLocalCaptureStore(database);
    await store.recoverInterruptedSync();
    final restored = await store.listAll();

    expect(restored, hasLength(20));
    expect(restored.map((capture) => capture.text).toSet(), {
      for (var index = 0; index < 20; index++) 'Chrome offline memory $index',
    });
    expect(
      restored.every(
        (capture) => capture.syncState == LocalSyncState.pendingSync,
      ),
      isTrue,
    );
    await store.close();
  });
}
