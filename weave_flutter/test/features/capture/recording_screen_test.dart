import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage.dart';
import 'package:weave_flutter/features/capture/data/audio_file_storage_service.dart';
import 'package:weave_flutter/features/capture/data/audio_recorder.dart';
import 'package:weave_flutter/features/capture/data/capture_api.dart';
import 'package:weave_flutter/features/capture/data/capture_sync_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';
import 'package:weave_flutter/features/capture/ui/recording_screen.dart';

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
  @override
  Future<CaptureServerRevision> create(LocalCapture capture) async {
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

void main() {
  Future<void> pumpAsync(WidgetTester tester) async {
    for (var frame = 0; frame < 10; frame++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
  }

  Future<Widget> buildApp({
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
    final router = GoRouter(
      initialLocation: '/record',
      routes: [
        GoRoute(path: '/record', builder: (_, _) => const RecordingScreen()),
        GoRoute(
          path: '/captures/:captureId/transcript',
          builder: (_, _) =>
              const Scaffold(body: Center(child: Text('transcript stub'))),
        ),
      ],
    );
    return ProviderScope(
      overrides: [
        localCaptureStoreProvider.overrideWith((ref) async => store),
        audioRecorderDeviceProvider.overrideWithValue(recorder),
        audioFileStorageProvider.overrideWithValue(storage),
        captureSyncServiceProvider.overrideWith((ref) async => syncService),
      ],
      child: MaterialApp.router(routerConfig: router),
    );
  }

  testWidgets('start recording transitions idle → recording', (tester) async {
    final store = await createStore('rec_widget_start');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage();
    final app = await buildApp(
      store: store,
      recorder: recorder,
      storage: storage,
    );

    await tester.pumpWidget(app);
    await pumpAsync(tester);

    expect(find.text('准备好后开始录音'), findsOneWidget);
    expect(find.byKey(const Key('record_start_button')), findsOneWidget);

    await tester.tap(find.byKey(const Key('record_start_button')));
    await pumpAsync(tester);

    expect(find.text('正在录音'), findsOneWidget);
    expect(find.byKey(const Key('record_stop_button')), findsOneWidget);
    expect(find.text('00:00'), findsOneWidget);
    expect(recorder.startedPaths, hasLength(1));
    expect(recorder.startedPaths.single, contains('/tmp/audio/'));

    // Tearing down while recording disposes the controller and cancels the
    // ticker via ref.onDispose.
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('permission denied surfaces retry without starting', (
    tester,
  ) async {
    final store = await createStore('rec_widget_perm');
    final recorder = FakeAudioRecorder()..permissionGranted = false;
    final storage = FakeAudioFileStorage();
    final app = await buildApp(
      store: store,
      recorder: recorder,
      storage: storage,
    );

    await tester.pumpWidget(app);
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_start_button')));
    await pumpAsync(tester);

    expect(find.text('无法录音：没有麦克风权限'), findsOneWidget);
    expect(find.byKey(const Key('record_retry_button')), findsOneWidget);
    expect(recorder.startedPaths, isEmpty);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('stop saves an audio capture and offers transcript correction', (
    tester,
  ) async {
    final store = await createStore('rec_widget_stop');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage()..sizeBytes = 882000;
    final app = await buildApp(
      store: store,
      recorder: recorder,
      storage: storage,
    );

    await tester.pumpWidget(app);
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_start_button')));
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_stop_button')));
    await pumpAsync(tester);

    expect(find.text('已安全保存'), findsOneWidget);
    expect(find.byKey(const Key('record_view_transcript')), findsOneWidget);
    expect(find.text('完成'), findsOneWidget);

    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, hasLength(1));
    expect(captures!.single.isAudio, isTrue);
    expect(captures.single.syncState, LocalSyncState.pendingSync);
    expect(captures.single.audioSizeBytes, 882000);
    expect(captures.single.audioSha256, 'deadbeef');
    expect(captures.single.audioTotalChunks, 1);

    // The transcript button routes to the correction page.
    await tester.tap(find.byKey(const Key('record_view_transcript')));
    await pumpAsync(tester);
    expect(find.text('transcript stub'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('stop failure surfaces save failed with retry', (tester) async {
    final store = await createStore('rec_widget_stopfail');
    final recorder = FakeAudioRecorder()..failStop = true;
    final storage = FakeAudioFileStorage();
    final app = await buildApp(
      store: store,
      recorder: recorder,
      storage: storage,
    );

    await tester.pumpWidget(app);
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_start_button')));
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_stop_button')));
    await pumpAsync(tester);

    expect(find.text('保存失败'), findsOneWidget);
    expect(find.byKey(const Key('record_retry_button')), findsOneWidget);

    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, isEmpty);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('back while recording confirms cancel and cleans up', (
    tester,
  ) async {
    final store = await createStore('rec_widget_cancel');
    final recorder = FakeAudioRecorder();
    final storage = FakeAudioFileStorage();
    final app = await buildApp(
      store: store,
      recorder: recorder,
      storage: storage,
    );

    await tester.pumpWidget(app);
    await pumpAsync(tester);
    await tester.tap(find.byKey(const Key('record_start_button')));
    await pumpAsync(tester);
    expect(find.text('正在录音'), findsOneWidget);

    await tester.tap(find.byTooltip('取消录音'));
    await pumpAsync(tester);
    expect(find.text('放弃这段录音？'), findsOneWidget);

    await tester.tap(find.text('放弃'));
    await pumpAsync(tester);

    expect(find.text('准备好后开始录音'), findsOneWidget);
    expect(recorder.stopCalls, 1);
    expect(storage.deleted, recorder.startedPaths);

    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, isEmpty);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });
}
