import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';
import '../../../shared/native/capture_feedback_service.dart';
import '../data/audio_file_storage_service.dart';
import '../data/audio_recorder.dart';
import '../data/audio_remote_gateway.dart';
import '../data/audio_upload_service.dart';
import '../data/capture_api.dart';
import '../data/capture_sync_service.dart';
import '../data/local_capture_database.dart';
import '../domain/local_capture.dart';
import '../domain/local_capture_store.dart';
import 'audio_capture_controller.dart';

final localCaptureStoreProvider = FutureProvider<LocalCaptureStore>((
  ref,
) async {
  final store = await openLocalCaptureStore();
  ref.onDispose(() => unawaited(store.close()));
  return store;
});
List<LocalCapture> visibleCapturesForUser(
  List<LocalCapture> captures,
  String? userId,
) => captures
    .where((capture) => capture.isVisibleTo(userId))
    .toList(growable: false);

final localCapturesProvider = StreamProvider<List<LocalCapture>>((ref) async* {
  final authState = ref.watch(authNotifierProvider);
  final userId = authState is Authenticated ? authState.user.id : null;
  final store = await ref.watch(localCaptureStoreProvider.future);
  yield visibleCapturesForUser(await store.listAll(), userId);
  yield* store.watchAll().map(
    (captures) => visibleCapturesForUser(captures, userId),
  );
});

/// 设备上尚未被任何账号认领的游客记录（`ownerUserId == null`）。
///
/// 与按会话过滤的 [localCapturesProvider] 不同：游客记录在本地保存时 owner 为空，
/// 登录后 `capture_sync_service` 才经 `claimOwner` 认领为当前账号——因此在登录会话中
/// 它们不可见于 [localCapturesProvider]，却又真实待合并。本 provider 不做会话过滤，
/// 供设置页「游客记录与同步」只读区块统计待认领条数（不触发任何同步）。
final guestLocalCapturesProvider = StreamProvider<List<LocalCapture>>((
  ref,
) async* {
  final store = await ref.watch(localCaptureStoreProvider.future);
  List<LocalCapture> ownerless(List<LocalCapture> captures) => captures
      .where((capture) => capture.ownerUserId == null)
      .toList(growable: false);
  yield ownerless(await store.listAll());
  yield* store.watchAll().map(ownerless);
});

final captureApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final captureRemoteGatewayProvider = Provider<CaptureRemoteGateway>((ref) {
  return CaptureApi(ref.watch(captureApiClientProvider));
});

final audioRemoteGatewayProvider = Provider<AudioRemoteGateway>((ref) {
  return AudioApi(ref.watch(captureApiClientProvider));
});

final audioUploadServiceProvider = FutureProvider<AudioUploader>((ref) async {
  final store = await ref.watch(localCaptureStoreProvider.future);
  return AudioUploadService(
    store: store,
    remote: ref.watch(audioRemoteGatewayProvider),
  );
});

final captureSyncServiceProvider = FutureProvider<CaptureSyncService>((
  ref,
) async {
  final store = await ref.watch(localCaptureStoreProvider.future);
  final authRepository = ref.watch(authRepositoryProvider);
  final service = CaptureSyncService(
    store: store,
    remote: ref.watch(captureRemoteGatewayProvider),
    audioUploader: await ref.watch(audioUploadServiceProvider.future),
    currentUserId: () async {
      final token = await authRepository.getToken();
      final user = await authRepository.getUser();
      if (token == null || token.isEmpty || user == null) return null;
      return user.id;
    },
  );
  ref.onDispose(service.dispose);
  return service;
});

final captureSyncBootstrapProvider = Provider<void>((ref) {
  ref.watch(authNotifierProvider);
  final syncService = ref.watch(captureSyncServiceProvider);
  StreamSubscription<List<ConnectivityResult>>? subscription;

  syncService.whenData((service) {
    unawaited(service.syncPending(includeDeferred: true));
    subscription = Connectivity().onConnectivityChanged.listen((results) {
      final hasConnection = results.any(
        (result) => result != ConnectivityResult.none,
      );
      if (hasConnection) {
        unawaited(service.syncPending(includeDeferred: true));
      }
    });
  });

  ref.onDispose(() {
    unawaited(subscription?.cancel());
  });
});

final captureUuidProvider = Provider<Uuid>((ref) => const Uuid());
final captureClockProvider = Provider<DateTime Function()>(
  (ref) => DateTime.now,
);
final captureSourceProvider = Provider<String>((ref) => localCaptureSource);

final audioFileStorageProvider = Provider<AudioFileStorage>(
  (ref) => const DefaultAudioFileStorage(),
);

final audioRecorderDeviceProvider = Provider<AudioRecorder>(
  (ref) => AudioRecorderDevice(),
);

final audioCaptureControllerProvider =
    NotifierProvider<AudioCaptureController, AudioCaptureState>(
      AudioCaptureController.new,
    );

/// Confirmation feedback for the minimal record flow. Injected so tests and the
/// web build can substitute a no-op/fake; real devices get haptics + sound.
final captureFeedbackServiceProvider = Provider<CaptureFeedbackService>((ref) {
  return kIsWeb
      ? const NoopCaptureFeedbackService()
      : const DeviceCaptureFeedbackService();
});

enum CaptureComposerStatus { idle, saving, saved, error }

class CaptureComposerState {
  const CaptureComposerState({
    this.status = CaptureComposerStatus.idle,
    this.lastSavedId,
    this.message,
  });

  final CaptureComposerStatus status;
  final String? lastSavedId;
  final String? message;
}

class CaptureComposerController extends Notifier<CaptureComposerState> {
  @override
  CaptureComposerState build() => const CaptureComposerState();

  Future<LocalCapture?> saveText(String text) async {
    if (text.trim().isEmpty) {
      state = const CaptureComposerState(
        status: CaptureComposerStatus.error,
        message: '请输入要记录的内容',
      );
      return null;
    }
    if (text.runes.length > maxLocalCaptureTextRunes) {
      state = const CaptureComposerState(
        status: CaptureComposerStatus.error,
        message: '内容过长，请控制在 10 万字以内',
      );
      return null;
    }

    state = const CaptureComposerState(status: CaptureComposerStatus.saving);
    final now = ref.read(captureClockProvider)().toUtc();
    final authState = ref.read(authNotifierProvider);
    final capture = LocalCapture(
      id: ref.read(captureUuidProvider).v4(),
      text: text,
      ownerUserId: authState is Authenticated ? authState.user.id : null,
      capturedAt: now,
      timezone: DateTime.now().timeZoneName,
      source: ref.read(captureSourceProvider),
      syncState: LocalSyncState.savedLocal,
      createdAt: now,
      updatedAt: now,
    );

    try {
      final store = await ref.read(localCaptureStoreProvider.future);
      await store.saveDraft(capture);
      state = CaptureComposerState(
        status: CaptureComposerStatus.saved,
        lastSavedId: capture.id,
        message: '已安全保存',
      );

      try {
        await store.markPendingSync(capture.id);
      } catch (_) {
        return capture;
      }

      final syncService = await ref.read(captureSyncServiceProvider.future);
      unawaited(syncService.syncPending());
      return capture;
    } catch (_) {
      state = const CaptureComposerState(
        status: CaptureComposerStatus.error,
        message: '本地保存失败，请勿退出并重试',
      );
      return null;
    }
  }

  Future<CaptureSyncSummary> retryAll() async {
    final service = await ref.read(captureSyncServiceProvider.future);
    return service.syncPending(includeDeferred: true);
  }

  void resetMessage() {
    state = const CaptureComposerState();
  }
}

final captureComposerControllerProvider =
    NotifierProvider<CaptureComposerController, CaptureComposerState>(
      CaptureComposerController.new,
    );
