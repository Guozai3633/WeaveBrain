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

import '../../support/echo_fakes.dart';

class _FakeRecorder implements AudioRecorder {
  bool permissionGranted = true;
  final List<String> startedPaths = [];
  int stopCalls = 0;
  final _amplitude = StreamController<double>.broadcast();

  @override
  bool get supportsFileRecording => true;

  @override
  Future<bool> hasPermission() async => permissionGranted;

  @override
  Future<void> start({required String path}) async {
    startedPaths.add(path);
  }

  @override
  Stream<double> get amplitude => _amplitude.stream;

  @override
  Future<String?> stop() async {
    stopCalls++;
    return startedPaths.isEmpty ? null : startedPaths.last;
  }

  @override
  Future<void> dispose() async {
    await _amplitude.close();
  }
}

class _FakeStorage implements AudioFileStorage {
  @override
  Future<String> getDirectory() async => '/tmp/min';

  @override
  Future<AudioFileMetadata> readMetadata(String filePath) async {
    return const AudioFileMetadata(sizeBytes: 441000, sha256: 'aabbccdd');
  }

  @override
  Future<void> deleteFile(String filePath) async {}
}

class _NoOpRemote implements CaptureRemoteGateway {
  @override
  Future<CaptureServerRevision> create(LocalCapture capture) async {
    return const CaptureServerRevision(
      version: 1,
      processingStatus: ServerProcessingStatus.ready,
    );
  }
}

Future<SembastLocalCaptureStore> _createStore(String name) async {
  final database = await databaseFactoryMemory.openDatabase(name);
  return SembastLocalCaptureStore(database);
}

Widget _buildApp({
  required SembastLocalCaptureStore store,
  required _FakeRecorder recorder,
  required FakeCaptureFeedbackService feedback,
}) {
  final syncService = CaptureSyncService(
    store: store,
    remote: _NoOpRemote(),
    currentUserId: () async => null,
    automaticRetry: false,
  );
  final router = GoRouter(
    initialLocation: '/record?auto=1',
    routes: [
      GoRoute(
        path: '/record',
        builder: (_, _) => const RecordingScreen(autoStart: true),
      ),
      GoRoute(
        path: '/capture',
        builder: (_, _) =>
            const Scaffold(body: Center(child: Text('capture stub'))),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      localCaptureStoreProvider.overrideWith((ref) async => store),
      audioRecorderDeviceProvider.overrideWithValue(recorder),
      audioFileStorageProvider.overrideWithValue(_FakeStorage()),
      captureSyncServiceProvider.overrideWith((ref) async => syncService),
      captureFeedbackServiceProvider.overrideWithValue(feedback),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  Future<void> pumpAWhile(WidgetTester tester) async {
    for (var frame = 0; frame < 10; frame++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
  }

  testWidgets('auto entry starts recording without any user tap', (tester) async {
    final store = await _createStore('min_auto_start');
    final recorder = _FakeRecorder();
    final app = _buildApp(
      store: store,
      recorder: recorder,
      feedback: FakeCaptureFeedbackService(),
    );

    await tester.pumpWidget(app);
    await pumpAWhile(tester);

    // 进入即自动开录：无任何主动操作即已 recording。
    expect(recorder.startedPaths, hasLength(1));
    expect(find.text('正在录音'), findsOneWidget);
    expect(find.text('点按任意处结束并保存'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('a single tap stops, saves, gives feedback and auto-pops', (
    tester,
  ) async {
    final store = await _createStore('min_one_tap');
    final recorder = _FakeRecorder();
    final feedback = FakeCaptureFeedbackService();
    final app = _buildApp(
      store: store,
      recorder: recorder,
      feedback: feedback,
    );

    await tester.pumpWidget(app);
    await pumpAWhile(tester);
    expect(recorder.startedPaths, hasLength(1));
    // 记录起始的触觉反馈在 recording 首帧触发一次。
    expect(feedback.startTappedCalls, 1);

    // 整页轻触一次 → 停止并保存。
    await tester.tap(find.byKey(const Key('minimal_record_tap_target')));
    await pumpAWhile(tester);

    expect(recorder.stopCalls, 1);
    expect(find.text('已安全保存'), findsOneWidget);
    expect(feedback.savedCalls, 1);

    // 保存成功后约 600ms 自动返回（根路由回退到 /capture）。
    await tester.pump(const Duration(milliseconds: 700));
    await tester.pumpAndSettle();
    expect(find.text('capture stub'), findsOneWidget);

    final captures = await tester.runAsync<List<dynamic>>(store.listAll);
    expect(captures, isNotEmpty);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });
}
