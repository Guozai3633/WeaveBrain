import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_io.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';

LocalCapture captureAt(int index, {LocalSyncState? state}) {
  final now = DateTime.utc(2026, 8, 16, 10, 0, index);
  return LocalCapture(
    id: '00000000-0000-4000-8000-${index.toString().padLeft(12, '0')}',
    text: 'offline idea $index',
    capturedAt: now,
    source: 'mobile_android',
    syncState: state ?? LocalSyncState.savedLocal,
    createdAt: now,
    updatedAt: now,
  );
}

void main() {
  test('20 offline captures survive database close and reopen', () async {
    final directory = await Directory.systemTemp.createTemp(
      'weavebrain_capture_test_',
    );
    final databasePath = '${directory.path}/captures.db';

    var database = await databaseFactoryIo.openDatabase(databasePath);
    var store = SembastLocalCaptureStore(database);
    for (var index = 0; index < 20; index++) {
      await store.saveDraft(captureAt(index));
    }
    expect(await store.listAll(), hasLength(20));
    await store.close();

    database = await databaseFactoryIo.openDatabase(databasePath);
    store = SembastLocalCaptureStore(database);
    await store.recoverInterruptedSync();

    final restored = await store.listAll();
    final pending = await store.listPending(includeDeferred: true);
    expect(restored, hasLength(20));
    expect(pending, hasLength(20));
    expect(
      restored.every(
        (capture) => capture.syncState == LocalSyncState.pendingSync,
      ),
      isTrue,
    );
    expect(
      restored.map((capture) => capture.text),
      contains('offline idea 19'),
    );

    await store.close();
    await directory.delete(recursive: true);
  });

  test('same local UUID cannot overwrite different original text', () async {
    final database = await databaseFactoryMemory.openDatabase(
      'capture_conflict_test',
      mode: DatabaseMode.empty,
    );
    final store = SembastLocalCaptureStore(database);
    final original = captureAt(1);
    await store.saveDraft(original);

    final conflicting = LocalCapture(
      id: original.id,
      text: 'different',
      capturedAt: original.capturedAt,
      source: original.source,
      syncState: LocalSyncState.savedLocal,
      createdAt: original.createdAt,
      updatedAt: original.updatedAt,
    );

    await expectLater(
      store.saveDraft(conflicting),
      throwsA(isA<LocalCaptureIdConflict>()),
    );
    expect((await store.getById(original.id))?.text, original.text);
    await store.close();
  });

  test(
    'interrupted syncing state is recovered into persistent queue',
    () async {
      final database = await databaseFactoryMemory.openDatabase(
        'capture_recovery_test',
        mode: DatabaseMode.empty,
      );
      final store = SembastLocalCaptureStore(database);
      final capture = captureAt(2);
      await store.saveDraft(capture);
      await store.markPendingSync(capture.id);
      await store.markSyncing(capture.id);

      await store.recoverInterruptedSync();

      expect(
        (await store.getById(capture.id))?.syncState,
        LocalSyncState.pendingSync,
      );
      expect(await store.listPending(includeDeferred: true), hasLength(1));
      await store.close();
    },
  );

  test('mobile and web share one V3 request mapping', () {
    final capture = captureAt(3);
    final request = capture.toCreateRequest();

    expect(request['capture_id'], capture.id);
    expect(request['kind'], 'text');
    expect(request['text'], capture.text);
    expect(request['captured_at_precision'], 'exact');
    expect(request['client_version'], 1);
    expect(request.containsKey('project_id'), isFalse);
  });

  test('capture owner is immutable and visibility is account scoped', () async {
    final database = await databaseFactoryMemory.openDatabase(
      'capture_owner_test',
      mode: DatabaseMode.empty,
    );
    final store = SembastLocalCaptureStore(database);
    final guest = captureAt(4);
    await store.saveDraft(guest);

    final claimed = await store.claimOwner(guest.id, 'user-a');

    expect(claimed.ownerUserId, 'user-a');
    expect(claimed.isVisibleTo('user-a'), isTrue);
    expect(claimed.isVisibleTo('user-b'), isFalse);
    expect(claimed.isVisibleTo(null), isFalse);
    await expectLater(
      store.claimOwner(guest.id, 'user-b'),
      throwsA(isA<LocalCaptureOwnerConflict>()),
    );
    expect((await store.getById(guest.id))?.ownerUserId, 'user-a');
    await store.close();
  });
}
