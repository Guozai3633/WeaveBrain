import 'package:sembast/sembast.dart';

import '../domain/local_capture.dart';
import '../domain/local_capture_store.dart';

class LocalCaptureIdConflict implements Exception {
  const LocalCaptureIdConflict(this.id);

  final String id;

  @override
  String toString() => 'LocalCaptureIdConflict($id)';
}

class LocalCaptureOwnerConflict implements Exception {
  const LocalCaptureOwnerConflict(this.id);

  final String id;

  @override
  String toString() => 'LocalCaptureOwnerConflict($id)';
}

class SembastLocalCaptureStore implements LocalCaptureStore {
  SembastLocalCaptureStore(this._database);

  final Database _database;
  final StoreRef<String, Map<String, Object?>> _captureStore =
      stringMapStoreFactory.store('captures');
  final StoreRef<String, Map<String, Object?>> _syncQueueStore =
      stringMapStoreFactory.store('capture_sync_queue');
  final StoreRef<String, Map<String, Object?>> _assetStore =
      stringMapStoreFactory.store('capture_assets');

  @override
  Future<void> saveDraft(LocalCapture capture) async {
    await _database.transaction((transaction) async {
      final record = _captureStore.record(capture.id);
      final existing = await record.get(transaction);
      if (existing != null) {
        final stored = LocalCapture.fromMap(existing);
        if (stored.text != capture.text) {
          throw LocalCaptureIdConflict(capture.id);
        }
        return;
      }
      await record.put(transaction, capture.toMap());
    });
  }

  @override
  Future<void> saveAudioAsset(LocalCaptureAsset asset) {
    return _assetStore.record(asset.id).put(_database, asset.toMap());
  }

  @override
  Future<LocalCapture> claimOwner(String id, String userId) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final owner = capture.ownerUserId;
      if (owner != null && owner != userId) {
        throw LocalCaptureOwnerConflict(id);
      }
      if (owner == userId) return capture;

      final updated = capture.copyWith(
        ownerUserId: userId,
        updatedAt: DateTime.now().toUtc(),
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      return updated;
    });
  }

  @override
  Future<void> markPendingSync(String id) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final now = DateTime.now().toUtc();
      final updated = capture.copyWith(
        syncState: LocalSyncState.pendingSync,
        updatedAt: now,
        clearLastErrorCode: true,
        clearNextRetryAt: true,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      await _syncQueueStore.record(id).put(transaction, {
        'capture_id': id,
        'next_retry_at': null,
        'updated_at': now.toIso8601String(),
      });
    });
  }

  @override
  Future<void> markSyncing(String id) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        syncState: LocalSyncState.syncing,
        updatedAt: DateTime.now().toUtc(),
        clearLastErrorCode: true,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
    });
  }

  @override
  Future<List<LocalCapture>> listPending({
    int limit = 50,
    bool includeDeferred = false,
  }) async {
    final queueRecords = await _syncQueueStore.find(_database);
    final now = DateTime.now().toUtc();
    final eligible = <({String id, DateTime? nextRetryAt})>[];

    for (final record in queueRecords) {
      final value = record.value;
      final retryText = value['next_retry_at'] as String?;
      final retryAt = retryText == null
          ? null
          : DateTime.parse(retryText).toUtc();
      if (includeDeferred || retryAt == null || !retryAt.isAfter(now)) {
        eligible.add((id: record.key, nextRetryAt: retryAt));
      }
    }

    eligible.sort((left, right) {
      final leftTime = left.nextRetryAt;
      final rightTime = right.nextRetryAt;
      if (leftTime == null && rightTime == null) return 0;
      if (leftTime == null) return -1;
      if (rightTime == null) return 1;
      return leftTime.compareTo(rightTime);
    });

    final captures = <LocalCapture>[];
    for (final item in eligible.take(limit)) {
      final map = await _captureStore.record(item.id).get(_database);
      if (map == null) {
        await _syncQueueStore.record(item.id).delete(_database);
        continue;
      }
      final capture = LocalCapture.fromMap(map);
      if (capture.trashedAt == null && capture.isPending) {
        captures.add(capture);
      }
    }
    return captures;
  }

  @override
  Future<void> applyServerRevision(
    String id, {
    required int serverVersion,
    required ServerProcessingStatus processingStatus,
  }) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        syncState: LocalSyncState.synced,
        serverVersion: serverVersion,
        serverProcessingStatus: processingStatus,
        updatedAt: DateTime.now().toUtc(),
        clearLastErrorCode: true,
        clearNextRetryAt: true,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      await _syncQueueStore.record(id).delete(transaction);
    });
  }

  @override
  Future<void> markRetryableError(
    String id, {
    required String errorCode,
    required DateTime nextRetryAt,
  }) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final now = DateTime.now().toUtc();
      final updated = capture.copyWith(
        syncState: LocalSyncState.retryableError,
        retryCount: capture.retryCount + 1,
        nextRetryAt: nextRetryAt.toUtc(),
        lastErrorCode: errorCode,
        updatedAt: now,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      await _syncQueueStore.record(id).put(transaction, {
        'capture_id': id,
        'next_retry_at': nextRetryAt.toUtc().toIso8601String(),
        'updated_at': now.toIso8601String(),
      });
    });
  }

  @override
  Future<void> markConflict(String id, {required String errorCode}) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        syncState: LocalSyncState.conflict,
        lastErrorCode: errorCode,
        updatedAt: DateTime.now().toUtc(),
        clearNextRetryAt: true,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      await _syncQueueStore.record(id).delete(transaction);
    });
  }

  @override
  Future<void> markRejected(String id, {required String errorCode}) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        syncState: LocalSyncState.rejected,
        lastErrorCode: errorCode,
        updatedAt: DateTime.now().toUtc(),
        clearNextRetryAt: true,
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
      await _syncQueueStore.record(id).delete(transaction);
    });
  }

  @override
  Future<void> updateAudioUploadedChunks(String id, List<int> uploadedChunks) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        audioUploadedChunks: [...uploadedChunks],
        updatedAt: DateTime.now().toUtc(),
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
    });
  }

  @override
  Future<void> saveTranscript(
    String id, {
    required String text,
    required String source,
    required int version,
  }) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final updated = capture.copyWith(
        transcript: text,
        transcriptSource: source,
        transcriptVersion: version,
        updatedAt: DateTime.now().toUtc(),
      );
      await _captureStore.record(id).put(transaction, updated.toMap());
    });
  }

  @override
  Future<LocalCapture?> getById(String id) async {
    final map = await _captureStore.record(id).get(_database);
    return map == null ? null : LocalCapture.fromMap(map);
  }

  @override
  Future<List<LocalCapture>> listAll() async {
    final snapshots = await _captureStore.find(
      _database,
      finder: Finder(
        filter: Filter.isNull('trashed_at'),
        sortOrders: [SortOrder('created_at', false)],
      ),
    );
    return snapshots
        .map((snapshot) => LocalCapture.fromMap(snapshot.value))
        .toList(growable: false);
  }

  @override
  Stream<List<LocalCapture>> watchAll() {
    final query = _captureStore.query(
      finder: Finder(
        filter: Filter.isNull('trashed_at'),
        sortOrders: [SortOrder('created_at', false)],
      ),
    );
    return query
        .onSnapshots(_database)
        .map(
          (snapshots) => snapshots
              .map((snapshot) => LocalCapture.fromMap(snapshot.value))
              .toList(growable: false),
        );
  }

  @override
  Future<void> recoverInterruptedSync() {
    return _database.transaction((transaction) async {
      final snapshots = await _captureStore.find(transaction);
      final now = DateTime.now().toUtc();
      for (final snapshot in snapshots) {
        final capture = LocalCapture.fromMap(snapshot.value);
        if (capture.trashedAt != null) continue;
        if (capture.syncState != LocalSyncState.savedLocal &&
            capture.syncState != LocalSyncState.syncing) {
          continue;
        }

        final updated = capture.copyWith(
          syncState: LocalSyncState.pendingSync,
          updatedAt: now,
        );
        await _captureStore
            .record(capture.id)
            .put(transaction, updated.toMap());
        await _syncQueueStore.record(capture.id).put(transaction, {
          'capture_id': capture.id,
          'next_retry_at': null,
          'updated_at': now.toIso8601String(),
        });
      }
    });
  }

  @override
  Future<void> trash(String id) {
    return _database.transaction((transaction) async {
      final capture = await _required(transaction, id);
      final now = DateTime.now().toUtc();
      await _captureStore
          .record(id)
          .put(
            transaction,
            capture.copyWith(trashedAt: now, updatedAt: now).toMap(),
          );
      await _syncQueueStore.record(id).delete(transaction);
    });
  }

  Future<LocalCapture> _required(DatabaseClient client, String id) async {
    final map = await _captureStore.record(id).get(client);
    if (map == null) throw StateError('Local capture $id not found');
    return LocalCapture.fromMap(map);
  }

  @override
  Future<void> close() => _database.close();
}
