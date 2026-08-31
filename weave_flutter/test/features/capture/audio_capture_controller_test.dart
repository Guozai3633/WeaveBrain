import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage_service.dart';
import 'package:weave_flutter/features/capture/data/audio_recorder.dart';
import 'package:weave_flutter/features/capture/data/capture_api.dart';
import 'package:weave_flutter/features/capture/data/capture_sync_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/audio_capture_controller.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';

class FakeAudioRecorder implements AudioRecorder {
  bool permissionGranted = true;
  bool failStart = false;
  bool failStop = false;
  String? stopResult;
  final List<String> startedPaths = [];
  int stopCalls = 0;
  int disposeCalls = 0;
  final _amplitudeController = StreamController<double>.broadcast();

  @override
  bool get supportsFileRecording => true;

  @override
  Future<bool> hasPermission() async => permissionGranted;

  @override
  Future<void> start({required String path}) async {
    if (failStart) throw Exception('mic in use');
    startedPaths.add(path);
  }

  @override
  Stream<double> get amplitude => _amplitudeController.stream;

  @override
  Future<String?> stop() async {
    stopCalls++;
    if (failStop) throw Exception('stop failed');
    return stopResult;
  }

  @override
  Future<void> dispose() async {
    disposeCalls++;
    await _amplitudeController.close();
  }
}

class FakeAudioFileStorage implements AudioFileStorage {
  String directory = '/tmp/audio';
  int sizeBytes = 441000;
  String sha256 = 'deadbeef';
  bool failRead = false;
  final List<String> deleted = [];

  @override
  Future<String> getDirectory() async => directory;

  @override
  Future<AudioFileMetadata> readMetadata(String filePath) async {
    if (failRead) throw Exception('read failed');
    return AudioFileMetadata(sizeBytes: sizeBytes, sha256: sha256);
  }

  @override
  Future<void> deleteFile(String filePath) async {
    deleted.add(filePath);
  }
}

class NoOpRemote implements CaptureRemoteGateway {
  var calls = 0;

  @override
  Future<CaptureServerRevision> create(LocalCapture capture) async {
    calls++;
    return const CaptureServerRevision(
      version: 1,
      processingStatus: ServerProcessingStatus.ready,
    );
  }
}

Future<SembastLocalCaptureStore> createStore(String name) async {
  final database = await databaseFactoryMemory.openDatabase(name);
  return SembastLocalCaptureStore(database);
}

Future<ProviderContainer> makeContainer({
  required SembastLocalCaptureStore store,
  required AudioRecorder recorder,
  required AudioFileStorage storage,
}) async {
  final syncService = CaptureSyncService(
    store: store,
    remote: NoOpRemote(),
    currentUserId: () async => null,
    automaticRetry: false,
  );
  final container = ProviderContainer(
    overrides: [
      localCaptureStoreProvider.overrideWith((ref) async => store),
      audioRecorderDeviceProvider.overrideWithValue(recorder),
      audioFileStorageProvider.overrideWithValue(storage),
      captureSyncServiceProvider.overrideWith((ref) async => syncService),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

void main() {
  test('permission denied stops before recording', () async {
    final store = await createStore('ctrl_perm');
    final recorder = FakeAudioRecorder()..permissionGranted = false;
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: FakeAudioFileStorage(),
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();

    final state = container.read(audioCaptureControllerProvider);
    expect(state.phase, AudioCapturePhase.permissionDenied);
    expect(state.message, isNotNull);
    expect(recorder.startedPaths, isEmpty);
    await store.close();
  });

  test('recorder failure surfaces as mic-in-use', () async {
    final store = await createStore('ctrl_mic');
    final recorder = FakeAudioRecorder()..failStart = true;
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: FakeAudioFileStorage(),
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();

    final state = container.read(audioCaptureControllerProvider);
    expect(state.phase, AudioCapturePhase.micInUse);
    expect(state.message, isNotNull);
    await store.close();
  });

  test('start transitions to recording with capture id and file path', () async {
    final store = await createStore('ctrl_start');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage()..directory = '/data/captures';
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: storage,
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();

    final state = container.read(audioCaptureControllerProvider);
    expect(state.phase, AudioCapturePhase.recording);
    expect(state.captureId, isNotNull);
    expect(recorder.startedPaths, hasLength(1));
    expect(recorder.startedPaths.single, contains('/data/captures/'));
    expect(recorder.startedPaths.single, endsWith('.wav'));
    await store.close();
  });

  test('stop persists an audio capture with metadata and marks pending',
      () async {
    final store = await createStore('ctrl_stop');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage()..sizeBytes = 882000;
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: storage,
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();
    final stateBefore = container.read(audioCaptureControllerProvider);
    final capture = await controller.stop();

    final state = container.read(audioCaptureControllerProvider);
    expect(state.phase, AudioCapturePhase.saved);
    expect(state.message, '已安全保存');
    expect(capture, isNotNull);
    expect(capture!.id, stateBefore.captureId);
    expect(capture.kind, 'audio');
    expect(capture.audioSizeBytes, 882000);
    expect(capture.audioSha256, 'deadbeef');
    expect(capture.audioTotalChunks, 1);
    expect(capture.audioAssetId, isNotNull);
    expect(capture.audioLocalPath, recorder.startedPaths.single);
    expect(capture.sttEnabled, isTrue);
    // The returned capture is the in-memory draft (savedLocal); the persisted
    // record is the one marked pending for sync.
    expect(capture.syncState, LocalSyncState.savedLocal);

    final stored = await store.getById(capture.id);
    expect(stored, isNotNull);
    expect(stored!.isAudio, isTrue);
    expect(stored.syncState, LocalSyncState.pendingSync);
    expect(stored.audioDurationMs, state.elapsed.inMilliseconds);
    await store.close();
  });

  test('stop without a started recording reports save failed', () async {
    final store = await createStore('ctrl_nostart');
    final recorder = FakeAudioRecorder();
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: FakeAudioFileStorage(),
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    final capture = await controller.stop();

    expect(capture, isNull);
    expect(
      container.read(audioCaptureControllerProvider).phase,
      AudioCapturePhase.saveFailed,
    );
    expect(recorder.stopCalls, 0);
    await store.close();
  });

  test('recorder stop failure reports save failed without losing the file',
      () async {
    final store = await createStore('ctrl_stopfail');
    final recorder = FakeAudioRecorder()..failStop = true;
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: FakeAudioFileStorage(),
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();
    final capture = await controller.stop();

    expect(capture, isNull);
    expect(
      container.read(audioCaptureControllerProvider).phase,
      AudioCapturePhase.saveFailed,
    );
    expect(await store.listAll(), isEmpty);
    await store.close();
  });

  test('metadata read failure reports save failed', () async {
    final store = await createStore('ctrl_readfail');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage()..failRead = true;
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: storage,
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();
    final capture = await controller.stop();

    expect(capture, isNull);
    expect(
      container.read(audioCaptureControllerProvider).phase,
      AudioCapturePhase.saveFailed,
    );
    expect(await store.listAll(), isEmpty);
    await store.close();
  });

  test('cancel stops the recorder, deletes the file and returns to idle',
      () async {
    final store = await createStore('ctrl_cancel');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage();
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: storage,
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.start();
    await controller.cancel();

    expect(container.read(audioCaptureControllerProvider).phase, AudioCapturePhase.idle);
    expect(recorder.stopCalls, 1);
    expect(storage.deleted, recorder.startedPaths);
    expect(await store.listAll(), isEmpty);
    await store.close();
  });

  test('cancel while idle does not delete anything', () async {
    final store = await createStore('ctrl_cancel_idle');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage();
    final container = await makeContainer(
      store: store,
      recorder: recorder,
      storage: storage,
    );
    final controller = container.read(audioCaptureControllerProvider.notifier);

    await controller.cancel();

    expect(container.read(audioCaptureControllerProvider).phase, AudioCapturePhase.idle);
    expect(recorder.stopCalls, 0);
    expect(storage.deleted, isEmpty);
    await store.close();
  });
}
