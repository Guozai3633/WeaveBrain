import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/auth/auth_state.dart';
import '../data/audio_upload_service.dart';
import 'capture_providers.dart';
import 'local_capture.dart';

/// Phase machine for a single voice capture.
enum AudioCapturePhase {
  idle,
  requestingMic,
  recording,
  stopping,
  saved,
  permissionDenied,
  micInUse,
  saveFailed,
  unsupported,
}

class AudioCaptureState {
  const AudioCaptureState({
    this.phase = AudioCapturePhase.idle,
    this.captureId,
    this.elapsed = Duration.zero,
    this.amplitude = 0,
    this.message,
  });

  final AudioCapturePhase phase;
  final String? captureId;
  final Duration elapsed;
  final double amplitude;
  final String? message;

  bool get isRecording => phase == AudioCapturePhase.recording;
  bool get isSaving => phase == AudioCapturePhase.stopping;
  bool get isBusy =>
      phase == AudioCapturePhase.requestingMic ||
      phase == AudioCapturePhase.stopping;

  AudioCaptureState copyWith({
    AudioCapturePhase? phase,
    String? captureId,
    Duration? elapsed,
    double? amplitude,
    String? message,
  }) {
    return AudioCaptureState(
      phase: phase ?? this.phase,
      captureId: captureId ?? this.captureId,
      elapsed: elapsed ?? this.elapsed,
      amplitude: amplitude ?? this.amplitude,
      message: message ?? this.message,
    );
  }
}

class AudioCaptureController extends Notifier<AudioCaptureState> {
  Timer? _ticker;
  StreamSubscription<double>? _amplitudeSub;
  String? _recordingPath;

  @override
  AudioCaptureState build() {
    ref.onDispose(() {
      _ticker?.cancel();
      _amplitudeSub?.cancel();
    });
    return const AudioCaptureState();
  }

  /// Web cannot persist audio to a file in this round.
  bool get isSupported => !kIsWeb;

  Future<void> start() async {
    if (!isSupported) {
      state = const AudioCaptureState(
        phase: AudioCapturePhase.unsupported,
        message: '当前浏览器不支持音频落盘，请使用 App 录音，或先用文字速记。',
      );
      return;
    }

    final recorder = ref.read(audioRecorderDeviceProvider);
    state = const AudioCaptureState(phase: AudioCapturePhase.requestingMic);
    final granted = await recorder.hasPermission();
    if (!granted) {
      state = const AudioCaptureState(
        phase: AudioCapturePhase.permissionDenied,
        message: '需要麦克风权限才能录音，请在系统设置中允许后重试。',
      );
      return;
    }

    final captureId = ref.read(captureUuidProvider).v4();
    try {
      final storage = ref.read(audioFileStorageProvider);
      final directory = await storage.getDirectory();
      final recordingPath = '$directory/$captureId.wav';
      _recordingPath = recordingPath;
      await recorder.start(path: recordingPath);
    } catch (_) {
      _recordingPath = null;
      state = const AudioCaptureState(
        phase: AudioCapturePhase.micInUse,
        message: '麦克风正被其他应用占用，或录音初始化失败，请重试。',
      );
      return;
    }

    _amplitudeSub?.cancel();
    _amplitudeSub = recorder.amplitude.listen((value) {
      state = state.copyWith(amplitude: value);
    });
    _startTicker();
    state = AudioCaptureState(
      phase: AudioCapturePhase.recording,
      captureId: captureId,
    );
  }

  Future<LocalCapture?> stop() async {
    final recorder = ref.read(audioRecorderDeviceProvider);
    _ticker?.cancel();
    _amplitudeSub?.cancel();
    state = state.copyWith(phase: AudioCapturePhase.stopping);

    final recordingPath = _recordingPath;
    if (recordingPath == null) {
      state = const AudioCaptureState(
        phase: AudioCapturePhase.saveFailed,
        message: '录音尚未开始。',
      );
      return null;
    }

    try {
      final stoppedPath = await recorder.stop();
      final actualPath = stoppedPath ?? recordingPath;
      final storage = ref.read(audioFileStorageProvider);
      final metadata = await storage.readMetadata(actualPath);
      final now = ref.read(captureClockProvider)().toUtc();
      final authState = ref.read(authNotifierProvider);
      final captureId = state.captureId ?? ref.read(captureUuidProvider).v4();

      final capture = LocalCapture(
        id: captureId,
        kind: 'audio',
        text: '',
        ownerUserId: authState is Authenticated ? authState.user.id : null,
        capturedAt: now,
        timezone: DateTime.now().timeZoneName,
        source: ref.read(captureSourceProvider),
        syncState: LocalSyncState.savedLocal,
        createdAt: now,
        updatedAt: now,
        audioAssetId: ref.read(captureUuidProvider).v4(),
        audioMimeType: 'audio/wav',
        audioSizeBytes: metadata.sizeBytes,
        audioSha256: metadata.sha256,
        audioTotalChunks: _totalChunks(metadata.sizeBytes),
        audioUploadedChunks: const [],
        audioLocalPath: actualPath,
        audioDurationMs: state.elapsed.inMilliseconds,
        sttEnabled: true,
      );

      final store = await ref.read(localCaptureStoreProvider.future);
      await store.saveDraft(capture);
      try {
        await store.markPendingSync(capture.id);
      } catch (_) {
        // Already local; a later sync pass will pick it up.
      }
      // The capture is saved; a later cancel() must not delete this file.
      _recordingPath = null;

      state = AudioCaptureState(
        phase: AudioCapturePhase.saved,
        captureId: capture.id,
        elapsed: state.elapsed,
        message: '已安全保存',
      );

      try {
        final syncService = await ref.read(captureSyncServiceProvider.future);
        unawaited(syncService.syncPending());
      } catch (_) {
        // Sync is best-effort; the capture is safely local.
      }
      return capture;
    } catch (_) {
      state = const AudioCaptureState(
        phase: AudioCapturePhase.saveFailed,
        message: '录音保存失败，请重试。',
      );
      return null;
    }
  }

  Future<void> cancel() async {
    _ticker?.cancel();
    _amplitudeSub?.cancel();
    final recordingPath = _recordingPath;
    if (recordingPath != null) {
      final recorder = ref.read(audioRecorderDeviceProvider);
      try {
        await recorder.stop();
      } catch (_) {
        // Recorder may already be stopped.
      }
      try {
        await ref.read(audioFileStorageProvider).deleteFile(recordingPath);
      } catch (_) {
        // Leftover file is harmless.
      }
    }
    _recordingPath = null;
    state = const AudioCaptureState();
  }

  void _startTicker() {
    _ticker?.cancel();
    final startedAt = DateTime.now();
    _ticker = Timer.periodic(const Duration(milliseconds: 250), (_) {
      state = state.copyWith(elapsed: DateTime.now().difference(startedAt));
    });
  }

  int _totalChunks(int sizeBytes) {
    final chunk = AudioUploadService.defaultAudioChunkBytes;
    if (sizeBytes <= 0) return 0;
    return (sizeBytes / chunk).ceil();
  }
}
