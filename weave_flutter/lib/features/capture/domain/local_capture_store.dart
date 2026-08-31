import 'local_capture.dart';

abstract interface class LocalCaptureStore {
  Future<void> saveDraft(LocalCapture capture);

  Future<void> saveAudioAsset(LocalCaptureAsset asset);
  Future<LocalCapture> claimOwner(String id, String userId);

  Future<void> markPendingSync(String id);

  Future<void> markSyncing(String id);

  Future<List<LocalCapture>> listPending({
    int limit = 50,
    bool includeDeferred = false,
  });

  Future<void> applyServerRevision(
    String id, {
    required int serverVersion,
    required ServerProcessingStatus processingStatus,
  });

  Future<void> markRetryableError(
    String id, {
    required String errorCode,
    required DateTime nextRetryAt,
  });

  Future<void> markConflict(String id, {required String errorCode});
  Future<void> markRejected(String id, {required String errorCode});

  /// Persists the set of audio chunk indices already uploaded so a retry can
  /// resume without re-sending chunks (idempotent chunk upload).
  Future<void> updateAudioUploadedChunks(String id, List<int> uploadedChunks);

  /// Persists the latest transcript (STT or user-corrected) for an audio
  /// capture so it survives restarts and can be re-fetched.
  Future<void> saveTranscript(
    String id, {
    required String text,
    required String source,
    required int version,
  });

  Future<LocalCapture?> getById(String id);

  Future<List<LocalCapture>> listAll();

  Stream<List<LocalCapture>> watchAll();

  Future<void> recoverInterruptedSync();

  Future<void> trash(String id);

  Future<void> close();
}
