import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/audio_remote_gateway.dart';
import 'package:weave_flutter/features/capture/data/audio_upload_service.dart';
import 'package:weave_flutter/features/capture/data/capture_api.dart';
import 'package:weave_flutter/features/capture/data/capture_sync_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class FakeCaptureRemote implements CaptureRemoteGateway {
  FakeCaptureRemote(this.handler);

  final Future<CaptureServerRevision> Function(LocalCapture capture) handler;
  final List<String> captureIds = [];

  @override
  Future<CaptureServerRevision> create(LocalCapture capture) {
    captureIds.add(capture.id);
    return handler(capture);
  }
}

LocalCapture pendingCapture() {
  final now = DateTime.utc(2026, 8, 16, 12);
  return LocalCapture(
    id: '11111111-1111-4111-8111-111111111111',
    text: 'keep this exact original',
    capturedAt: now,
    source: 'web',
    syncState: LocalSyncState.savedLocal,
    createdAt: now,
    updatedAt: now,
  );
}

Future<SembastLocalCaptureStore> createStore() async {
  final database = await databaseFactoryMemory.openDatabase(
    'capture_sync_${DateTime.now().microsecondsSinceEpoch}',
  );
  return SembastLocalCaptureStore(database);
}

class FakeAudioUploader implements AudioUploader {
  FakeAudioUploader({this.result});

  final AudioTranscriptResult? result;
  final List<String> uploadedIds = [];
  bool fail = false;
  String failCode = 'AUDIO_UPLOAD_FAILED';

  @override
  Future<AudioTranscriptResult?> uploadAudio(LocalCapture capture) async {
    uploadedIds.add(capture.id);
    if (fail) throw AudioUploadException(failCode);
    return result;
  }
}

LocalCapture audioPendingCapture() {
  final now = DateTime.utc(2026, 8, 16, 12);
  return LocalCapture(
    id: '22222222-2222-4222-8222-222222222222',
    kind: 'audio',
    text: '',
    capturedAt: now,
    source: 'mobile_android',
    syncState: LocalSyncState.savedLocal,
    createdAt: now,
    updatedAt: now,
    audioAssetId: 'asset-x',
    audioMimeType: 'audio/webm',
    audioSizeBytes: 100,
    audioSha256: 'abc',
    audioTotalChunks: 1,
    audioUploadedChunks: const [],
    audioLocalPath: '/tmp/x.webm',
    audioDurationMs: 5000,
    sttEnabled: true,
  );
}

void main() {
  test('guest keeps pending capture and never calls remote', () async {
    final store = await createStore();
    final capture = pendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => null,
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);

    expect(summary.skippedBecauseGuest, isTrue);
    expect(remote.captureIds, isEmpty);
    expect(
      (await store.getById(capture.id))?.syncState,
      LocalSyncState.pendingSync,
    );
    service.dispose();
    await store.close();
  });

  test('HTTP 5xx preserves text and enters retryable error', () async {
    final store = await createStore();
    final capture = pendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => throw ApiException(statusCode: 500, message: 'failed'),
    );
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => 'user-a',
      now: () => DateTime.utc(2026, 8, 16, 13),
      retryDelay: (_) => const Duration(seconds: 1),
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);
    final stored = await store.getById(capture.id);

    expect(summary.failed, 1);
    expect(stored?.text, capture.text);
    expect(stored?.syncState, LocalSyncState.retryableError);
    expect(stored?.lastErrorCode, 'SERVER_500');
    expect(stored?.retryCount, 1);
    service.dispose();
    await store.close();
  });

  test('retry reuses UUID and eventually syncs without duplication', () async {
    final store = await createStore();
    final capture = pendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    var calls = 0;
    final remote = FakeCaptureRemote((_) async {
      calls++;
      if (calls == 1) {
        throw ApiException(statusCode: 503, message: 'unavailable');
      }
      return const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      );
    });
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => 'user-a',
      retryDelay: (_) => const Duration(hours: 1),
      automaticRetry: false,
    );

    await service.syncPending(includeDeferred: true);
    await service.syncPending(includeDeferred: true);

    final stored = await store.getById(capture.id);
    expect(remote.captureIds, [capture.id, capture.id]);
    expect(stored?.syncState, LocalSyncState.synced);
    expect(stored?.serverVersion, 1);
    expect(await store.listPending(includeDeferred: true), isEmpty);
    service.dispose();
    await store.close();
  });

  test('409 becomes conflict and does not silently replace UUID', () async {
    final store = await createStore();
    final capture = pendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => throw ApiException(statusCode: 409, message: 'conflict'),
    );
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);
    final stored = await store.getById(capture.id);

    expect(summary.conflicts, 1);
    expect(stored?.id, capture.id);
    expect(stored?.syncState, LocalSyncState.conflict);
    expect(stored?.lastErrorCode, 'IDEMPOTENCY_CONFLICT');
    service.dispose();
    await store.close();
  });

  test(
    'retryable failure is retried automatically at persisted time',
    () async {
      final store = await createStore();
      final capture = pendingCapture();
      await store.saveDraft(capture);
      await store.markPendingSync(capture.id);
      var calls = 0;
      final remote = FakeCaptureRemote((_) async {
        calls++;
        if (calls == 1) {
          throw ApiException(statusCode: 503, message: 'unavailable');
        }
        return const CaptureServerRevision(
          version: 1,
          processingStatus: ServerProcessingStatus.ready,
        );
      });
      final service = CaptureSyncService(
        store: store,
        remote: remote,
        currentUserId: () async => 'user-a',
        retryDelay: (_) => const Duration(milliseconds: 20),
      );

      final first = await service.syncPending(includeDeferred: true);
      expect(first.failed, 1);
      await Future<void>.delayed(const Duration(milliseconds: 100));

      final stored = await store.getById(capture.id);
      expect(remote.captureIds, [capture.id, capture.id]);
      expect(stored?.syncState, LocalSyncState.synced);
      service.dispose();
      await store.close();
    },
  );

  test('account B cannot see or sync account A pending capture', () async {
    final store = await createStore();
    final capture = pendingCapture().copyWith(ownerUserId: 'user-a');
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    var accountBAuthChecks = 0;
    final serviceB = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async {
        accountBAuthChecks++;
        return 'user-b';
      },
    );

    final skipped = await serviceB.syncPending(includeDeferred: true);
    expect(skipped.attempted, 0);
    expect(remote.captureIds, isEmpty);
    expect(
      (await store.getById(capture.id))?.syncState,
      LocalSyncState.pendingSync,
    );
    await Future<void>.delayed(const Duration(milliseconds: 60));
    expect(accountBAuthChecks, 2);

    final serviceA = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );
    final synced = await serviceA.syncPending(includeDeferred: true);

    expect(synced.synced, 1);
    expect(remote.captureIds, [capture.id]);
    expect((await store.getById(capture.id))?.ownerUserId, 'user-a');
    serviceB.dispose();
    serviceA.dispose();
    await store.close();
  });

  test('HTTP 400 is rejected once and removed from retry queue', () async {
    final store = await createStore();
    final capture = pendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => throw ApiException(statusCode: 400, message: 'invalid'),
    );
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);
    final stored = await store.getById(capture.id);

    expect(summary.rejected, 1);
    expect(summary.failed, 0);
    expect(stored?.syncState, LocalSyncState.rejected);
    expect(stored?.lastErrorCode, 'HTTP_400');
    expect(stored?.ownerUserId, 'user-a');
    expect(await store.listPending(includeDeferred: true), isEmpty);
    service.dispose();
    await store.close();
  });

  test('audio capture with successful uploader is synced and uploaded',
      () async {
    final store = await createStore();
    final capture = audioPendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    final audioUploader = FakeAudioUploader(
      result: const AudioTranscriptResult(
        text: 'hello',
        source: 'stt',
        version: 1,
      ),
    );
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      audioUploader: audioUploader,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);

    final stored = await store.getById(capture.id);
    expect(summary.synced, 1);
    expect(audioUploader.uploadedIds, [capture.id]);
    expect(stored?.syncState, LocalSyncState.synced);
    expect(stored?.ownerUserId, 'user-a');
    expect(await store.listPending(includeDeferred: true), isEmpty);
    service.dispose();
    await store.close();
  });

  test('audio capture with failing uploader stays retryable, never synced',
      () async {
    final store = await createStore();
    final capture = audioPendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    final audioUploader = FakeAudioUploader()..fail = true;
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      audioUploader: audioUploader,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
      now: () => DateTime.utc(2026, 8, 16, 13),
      retryDelay: (_) => const Duration(seconds: 1),
    );

    final summary = await service.syncPending(includeDeferred: true);

    final stored = await store.getById(capture.id);
    expect(summary.failed, 1);
    expect(summary.synced, 0);
    expect(audioUploader.uploadedIds, [capture.id]);
    expect(stored?.syncState, LocalSyncState.retryableError);
    expect(stored?.lastErrorCode, 'AUDIO_UPLOAD_FAILED');
    expect(stored?.serverVersion, isNull);
    expect(await store.listPending(includeDeferred: true), isNotEmpty);
    service.dispose();
    await store.close();
  });

  test('audio upload failure recovers on retry and then syncs', () async {
    final store = await createStore();
    final capture = audioPendingCapture();
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    final audioUploader = FakeAudioUploader();
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      audioUploader: audioUploader,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );

    audioUploader.fail = true;
    await service.syncPending(includeDeferred: true);
    audioUploader.fail = false;
    await service.syncPending(includeDeferred: true);

    final stored = await store.getById(capture.id);
    expect(audioUploader.uploadedIds, [capture.id, capture.id]);
    expect(stored?.syncState, LocalSyncState.synced);
    expect(stored?.serverVersion, 1);
    expect(await store.listPending(includeDeferred: true), isEmpty);
    service.dispose();
    await store.close();
  });

  test('text capture ignores the audio uploader entirely', () async {
    final store = await createStore();
    final capture = pendingCapture(); // kind == 'text'
    await store.saveDraft(capture);
    await store.markPendingSync(capture.id);
    final remote = FakeCaptureRemote(
      (_) async => const CaptureServerRevision(
        version: 1,
        processingStatus: ServerProcessingStatus.ready,
      ),
    );
    final audioUploader = FakeAudioUploader();
    final service = CaptureSyncService(
      store: store,
      remote: remote,
      audioUploader: audioUploader,
      currentUserId: () async => 'user-a',
      automaticRetry: false,
    );

    final summary = await service.syncPending(includeDeferred: true);

    expect(summary.synced, 1);
    expect(audioUploader.uploadedIds, isEmpty);
    service.dispose();
    await store.close();
  });
}
