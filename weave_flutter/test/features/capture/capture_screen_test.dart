import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/capture_api.dart';
import 'package:weave_flutter/features/capture/data/capture_sync_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/capture_providers.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';
import 'package:weave_flutter/features/capture/ui/capture_screen.dart';

class NeverCalledRemote implements CaptureRemoteGateway {
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

void main() {
  Future<void> pumpAsync(WidgetTester tester) async {
    for (var frame = 0; frame < 10; frame++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
  }

  testWidgets('text is locally safe before guest network sync', (tester) async {
    final database = await databaseFactoryMemory.openDatabase(
      'capture_widget_test',
    );
    final store = SembastLocalCaptureStore(database);
    final remote = NeverCalledRemote();
    final syncService = CaptureSyncService(
      store: store,
      remote: remote,
      currentUserId: () async => null,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localCaptureStoreProvider.overrideWith((ref) async => store),
          captureSyncServiceProvider.overrideWith((ref) async => syncService),
          captureSourceProvider.overrideWithValue('web'),
        ],
        child: const MaterialApp(home: CaptureScreen()),
      ),
    );
    await pumpAsync(tester);

    await tester.enterText(
      find.byKey(const Key('capture_text_input')),
      '  preserve my original idea  ',
    );
    await tester.tap(find.byKey(const Key('capture_save_button')));
    await pumpAsync(tester);

    expect(find.text('已安全保存'), findsOneWidget);
    expect(find.text('preserve my original idea'), findsOneWidget);
    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, hasLength(1));
    expect(captures!.single.text, '  preserve my original idea  ');
    expect(captures.single.syncState, LocalSyncState.pendingSync);
    expect(remote.calls, 0);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('blank capture is rejected without local write', (tester) async {
    final database = await databaseFactoryMemory.openDatabase(
      'capture_widget_blank_test',
    );
    final store = SembastLocalCaptureStore(database);
    final syncService = CaptureSyncService(
      store: store,
      remote: NeverCalledRemote(),
      currentUserId: () async => null,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localCaptureStoreProvider.overrideWith((ref) async => store),
          captureSyncServiceProvider.overrideWith((ref) async => syncService),
        ],
        child: const MaterialApp(home: CaptureScreen()),
      ),
    );
    await pumpAsync(tester);

    await tester.enterText(find.byKey(const Key('capture_text_input')), '   ');
    await tester.tap(find.byKey(const Key('capture_save_button')));
    await pumpAsync(tester);

    expect(find.text('请输入要记录的内容'), findsOneWidget);
    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, isEmpty);
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });

  testWidgets('oversized capture is rejected without local write', (
    tester,
  ) async {
    final database = await databaseFactoryMemory.openDatabase(
      'capture_widget_oversized_test',
    );
    final store = SembastLocalCaptureStore(database);
    final syncService = CaptureSyncService(
      store: store,
      remote: NeverCalledRemote(),
      currentUserId: () async => null,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localCaptureStoreProvider.overrideWith((ref) async => store),
          captureSyncServiceProvider.overrideWith((ref) async => syncService),
        ],
        child: const MaterialApp(home: CaptureScreen()),
      ),
    );
    await pumpAsync(tester);

    final oversized = List.filled(maxLocalCaptureTextRunes + 1, 'x').join();
    await tester.enterText(
      find.byKey(const Key('capture_text_input')),
      oversized,
    );
    await tester.tap(find.byKey(const Key('capture_save_button')));
    await pumpAsync(tester);

    expect(find.text('内容过长，请控制在 10 万字以内'), findsOneWidget);
    final captures = await tester.runAsync<List<LocalCapture>>(store.listAll);
    expect(captures, isEmpty);
    syncService.dispose();
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
  });
}
