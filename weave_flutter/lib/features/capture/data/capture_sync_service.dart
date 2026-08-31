// ignore_for_file: prefer_initializing_formals

import 'dart:async';

import '../../../shared/api/api_exception.dart';
import '../domain/local_capture.dart';
import '../domain/local_capture_store.dart';
import 'audio_upload_service.dart';
import 'capture_api.dart';

class CaptureSyncSummary {
  const CaptureSyncSummary({
    required this.attempted,
    required this.synced,
    required this.conflicts,
    required this.failed,
    required this.rejected,
    this.skippedBecauseGuest = false,
  });

  const CaptureSyncSummary.guest()
    : attempted = 0,
      synced = 0,
      conflicts = 0,
      failed = 0,
      rejected = 0,
      skippedBecauseGuest = true;

  final int attempted;
  final int synced;
  final int conflicts;
  final int failed;
  final int rejected;
  final bool skippedBecauseGuest;
}

class CaptureSyncService {
  CaptureSyncService({
    required LocalCaptureStore store,
    required CaptureRemoteGateway remote,
    required Future<String?> Function() currentUserId,
    DateTime Function()? now,
    Duration Function(int retryCount)? retryDelay,
    this.automaticRetry = true,
    this.audioUploader,
  }) : _store = store,
       _remote = remote,
       _currentUserId = currentUserId,
       _now = now ?? DateTime.now,
       _retryDelay = retryDelay ?? _defaultRetryDelay;

  final LocalCaptureStore _store;
  final CaptureRemoteGateway _remote;
  final Future<String?> Function() _currentUserId;
  final DateTime Function() _now;
  final Duration Function(int retryCount) _retryDelay;
  final bool automaticRetry;

  /// Optional audio uploader used for `kind == 'audio'` captures. When present
  /// and the capture has audio metadata, the pipeline is:
  /// create capture → initiate asset → upload chunks → complete → transcribe.
  /// A failed upload keeps the capture retryable (never marked synced) so the
  /// next attempt resumes from the persisted chunk progress.
  final AudioUploader? audioUploader;

  Future<CaptureSyncSummary>? _activeSync;
  Timer? _retryTimer;
  bool _disposed = false;

  Future<CaptureSyncSummary> syncPending({bool includeDeferred = false}) {
    final active = _activeSync;
    if (active != null) return active;

    _retryTimer?.cancel();
    var operation = _syncPending(includeDeferred: includeDeferred);
    if (automaticRetry) {
      operation = operation.then((summary) async {
        if (!summary.skippedBecauseGuest) {
          await _scheduleNextRetry();
        }
        return summary;
      });
    }
    _activeSync = operation;

    void resetActiveSync() {
      if (identical(_activeSync, operation)) {
        _activeSync = null;
      }
    }

    unawaited(
      operation.then<void>(
        (_) => resetActiveSync(),
        onError: (_, _) => resetActiveSync(),
      ),
    );
    return operation;
  }

  Future<CaptureSyncSummary> _syncPending({
    required bool includeDeferred,
  }) async {
    final userId = await _currentUserId();
    if (userId == null || userId.isEmpty) {
      return const CaptureSyncSummary.guest();
    }

    final allPending = await _store.listPending(
      includeDeferred: includeDeferred,
    );
    final pending = allPending
        .where(
          (capture) =>
              capture.ownerUserId == null || capture.ownerUserId == userId,
        )
        .toList(growable: false);
    var synced = 0;
    var conflicts = 0;
    var failed = 0;
    var rejected = 0;

    for (final capture in pending) {
      final ownedCapture = await _store.claimOwner(capture.id, userId);
      await _store.markSyncing(capture.id);
      try {
        final revision = await _remote.create(ownedCapture);

        // Audio captures also upload the recorded file (chunked, resumable)
        // before being marked synced. A failure keeps them retryable — the
        // capture itself is idempotent on the server, so the next attempt
        // re-runs safely and resumes from persisted chunk progress.
        final uploader = audioUploader;
        if (ownedCapture.isAudio && uploader != null) {
          try {
            await uploader.uploadAudio(ownedCapture);
          } catch (_) {
            await _markRetryable(ownedCapture, 'AUDIO_UPLOAD_FAILED');
            failed++;
            continue;
          }
        }

        await _store.applyServerRevision(
          capture.id,
          serverVersion: revision.version,
          processingStatus: revision.processingStatus,
        );
        synced++;
      } on ApiException catch (error) {
        if (error.statusCode == 409) {
          await _store.markConflict(
            capture.id,
            errorCode: 'IDEMPOTENCY_CONFLICT',
          );
          conflicts++;
          continue;
        }
        if (_isPermanentClientError(error.statusCode)) {
          await _store.markRejected(
            capture.id,
            errorCode: _apiErrorCode(error),
          );
          rejected++;
          continue;
        }

        await _markRetryable(capture, _apiErrorCode(error));
        failed++;
      } catch (_) {
        await _markRetryable(capture, 'NETWORK_OR_FORMAT_ERROR');
        failed++;
      }
    }

    return CaptureSyncSummary(
      attempted: pending.length,
      synced: synced,
      conflicts: conflicts,
      failed: failed,
      rejected: rejected,
    );
  }

  Future<void> _markRetryable(LocalCapture capture, String code) {
    final retryCount = capture.retryCount + 1;
    final retryAt = _now().toUtc().add(_retryDelay(retryCount));
    return _store.markRetryableError(
      capture.id,
      errorCode: code,
      nextRetryAt: retryAt,
    );
  }

  Future<void> _scheduleNextRetry() async {
    if (_disposed) return;
    final userId = await _currentUserId();
    if (userId == null || userId.isEmpty) return;
    final allPending = await _store.listPending(includeDeferred: true);
    final pending = allPending
        .where(
          (capture) =>
              capture.ownerUserId == null || capture.ownerUserId == userId,
        )
        .toList(growable: false);
    if (pending.isEmpty || _disposed) return;

    DateTime? earliest;
    var hasImmediateWork = false;
    for (final capture in pending) {
      final retryAt = capture.nextRetryAt;
      if (retryAt == null) {
        hasImmediateWork = true;
        break;
      }
      if (earliest == null || retryAt.isBefore(earliest)) {
        earliest = retryAt;
      }
    }

    var delay = hasImmediateWork
        ? const Duration(milliseconds: 10)
        : earliest!.difference(_now().toUtc());
    if (delay < const Duration(milliseconds: 10)) {
      delay = const Duration(milliseconds: 10);
    }

    _retryTimer?.cancel();
    _retryTimer = Timer(delay, () {
      if (_disposed) return;
      unawaited(syncPending());
    });
  }

  void dispose() {
    _disposed = true;
    _retryTimer?.cancel();
    _retryTimer = null;
  }

  static bool _isPermanentClientError(int? status) =>
      status != null &&
      status >= 400 &&
      status < 500 &&
      status != 401 &&
      status != 408 &&
      status != 425 &&
      status != 429;

  static String _apiErrorCode(ApiException error) {
    final status = error.statusCode;
    if (status == null) return 'NETWORK_ERROR';
    if (status == 401) return 'AUTH_REQUIRED';
    if (status >= 500) return 'SERVER_$status';
    return 'HTTP_$status';
  }

  static Duration _defaultRetryDelay(int retryCount) {
    final exponent = retryCount.clamp(1, 8);
    return Duration(seconds: 5 * (1 << (exponent - 1)));
  }
}
