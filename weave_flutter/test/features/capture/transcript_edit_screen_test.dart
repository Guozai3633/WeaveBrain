import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/capture_api.dart';
import 'package:weave_flutter/features/capture/data/capture_sync_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';
import 'package:weave_flutter/features/capture/ui/transcript_edit_screen.dart';

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

LocalCapture audioCapture({String transcript = ''}) {
  final now = DateTime.now().toUtc();
  return LocalCapture(
    id: 'cap-audio-1',
    kind: 'audio',
    text: '',
    capturedAt: now,
    source: 'app',
    syncState: LocalSyncState.pendingSync,
    createdAt: now,
    updatedAt: now,
    audioAssetId: 'asset-1',
    audioMimeType: 'audio/wav',
    audioSizeBytes: 1000,
    audioSha256: 'abc',
    audioTotalChunks: 1,
    audioDurationMs: 5000,
    sttEnabled: true,
    transcript: transcript,
    transcriptSource: transcript.isEmpty ? null : 'stt',
    transcriptVersion: transcript.isEmpty ? null : 1,
  );
}

void main() {
  Future<void> pumpAsync(WidgetTester tester) async {
    for (var frame = 0; frame < 10; frame++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
  }

  testWidgets('loads existing transcript and saves a guest correction', (
    tester,
  ) async {
    final store = await createStore('transcript_widget_guest');
    // Sembast transactions rely on the real event loop; run the write outside
    // the fake-async zone so it completes before the widget is pumped.
    await tester.runAsync(
      () => store.saveDraft(audioCapture(transcript: '今天开会讨论上线计划。')),
    );
    final syncService = CaptureSyncService(
      store: store,
      remote: NoOpRemote(),
      currentUserId: () async => null,
      automaticRetry: false,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localCaptureStoreProvider.overrideWith((ref) async => store),
          captureSyncServiceProvider.overrideWith((ref) async => syncService),
        ],
        child: const MaterialApp(
          home: TranscriptEditScreen(captureId: 'cap-audio-1'),
        ),
      ),
    );
    await pumpAsync(tester);

    expect(find.byKey(const Key('transcript_edit_field')), findsOneWidget);
    expect(find.text('今天开会讨论上线计划。'), findsOneWidget);
    expect(find.text('自动转写'), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('transcript_edit_field')),
      '修正后的转写',
    );
    await tester.tap(find.byKey(const Key('transcript_save_button')));
    await pumpAsync(tester);

    expect(find.text('转写已保存'), findsOneWidget);
    final stored = await tester.runAsync(() => store.getById('cap-audio-1'));
    expect(stored!.transcript, '修正后的转写');
    expect(stored.transcriptSource, 'user');
    expect(stored.transcriptVersion, 1);

    // Let the SnackBar auto-dismiss timer fire and its exit animation finish.
    // Note: pumpAndSettle cannot be used here because the focused TextField
    // blinks its cursor and schedules frames forever.
    await tester.pump(const Duration(seconds: 5));
    await tester.pump(const Duration(seconds: 1));
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });

  testWidgets('empty store shows empty editable field', (tester) async {
    final store = await createStore('transcript_widget_empty');
    final syncService = CaptureSyncService(
      store: store,
      remote: NoOpRemote(),
      currentUserId: () async => null,
      automaticRetry: false,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localCaptureStoreProvider.overrideWith((ref) async => store),
          captureSyncServiceProvider.overrideWith((ref) async => syncService),
        ],
        child: const MaterialApp(
          home: TranscriptEditScreen(captureId: 'missing-capture'),
        ),
      ),
    );
    await pumpAsync(tester);

    expect(find.byKey(const Key('transcript_edit_field')), findsOneWidget);
    expect(find.text('转写生成中，请稍后回来查看…'), findsOneWidget);
    expect(find.text('自动转写'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    await store.close();
  });
}
