// ignore_for_file: prefer_initializing_formals

import 'dart:typed_data';

import '../domain/local_capture.dart';
import '../domain/local_capture_store.dart';
import 'audio_chunk_source.dart';
import 'audio_remote_gateway.dart';

/// Thrown when an audio upload cannot proceed because the local capture is
/// missing audio metadata or its local file is unreadable. Distinct from
/// [ApiException], which represents a transport or server-side failure.
class AudioUploadException implements Exception {
  const AudioUploadException(this.code);

  final String code;

  @override
  String toString() => 'AudioUploadException($code)';
}

/// Signature for reading one audio chunk from the capture's local file.
/// Injectable so tests can back it with an in-memory buffer instead of a real
/// file.
typedef AudioChunkReader = Future<Uint8List> Function({
  required LocalCapture capture,
  required int offset,
  required int length,
});

/// Uploads a locally-recorded audio capture to the server and (optionally)
/// runs full-file STT.
abstract interface class AudioUploader {
  /// Performs the full pipeline for an audio [capture]:
  /// initiate → upload missing chunks → complete → transcribe.
  ///
  /// Returns the transcript when STT is enabled, otherwise null. Throws on
  /// failure so the caller can mark the capture retryable; a later attempt
  /// resumes from the persisted chunk progress.
  Future<AudioTranscriptResult?> uploadAudio(LocalCapture capture);
}

class AudioUploadService implements AudioUploader {
  AudioUploadService({
    required LocalCaptureStore store,
    required AudioRemoteGateway remote,
    this.chunkBytes = defaultAudioChunkBytes,
    AudioChunkReader? chunkReader,
  }) : _store = store,
       _remote = remote,
       _chunkReader = chunkReader ?? readAudioChunk;

  /// Must match the backend's `MaxAudioChunkBytes` (5 MiB).
  static const int defaultAudioChunkBytes = 5 << 20;

  final LocalCaptureStore _store;
  final AudioRemoteGateway _remote;
  final int chunkBytes;
  final AudioChunkReader _chunkReader;

  @override
  Future<AudioTranscriptResult?> uploadAudio(LocalCapture capture) async {
    if (!capture.isAudio) return null;

    final sizeBytes = capture.audioSizeBytes;
    final localPath = capture.audioLocalPath;
    final totalChunksValue = capture.audioTotalChunks;
    if (sizeBytes == null ||
        sizeBytes <= 0 ||
        localPath == null ||
        localPath.isEmpty ||
        totalChunksValue == null ||
        totalChunksValue <= 0) {
      throw const AudioUploadException('AUDIO_METADATA_MISSING');
    }
    final totalChunks = totalChunksValue;

    final assetId = capture.audioAssetId ?? capture.id;

    // 1. Initiate — idempotent per asset id, safe to re-run on retry.
    await _remote.initiateAudioAsset(
      AudioAssetInitiateParams(
        assetId: assetId,
        captureId: capture.id,
        mimeType: capture.audioMimeType ?? 'audio/webm',
        sizeBytes: sizeBytes,
        sha256: capture.audioSha256 ?? '',
        totalChunks: totalChunks,
        durationMs: capture.audioDurationMs,
        sttEnabled: capture.sttEnabled,
      ),
    );

    // 2. Upload missing chunks, persisting progress after each chunk so a
    //    retry resumes instead of re-sending bytes.
    final uploadedChunks = <int>[...capture.audioUploadedChunks];
    for (final index in capture.missingAudioChunks) {
      final offset = index * chunkBytes;
      final length = index == totalChunks - 1 ? sizeBytes - offset : chunkBytes;
      if (length <= 0) continue;

      final bytes = await _chunkReader(
        capture: capture,
        offset: offset,
        length: length,
      );
      if (bytes.isEmpty) {
        throw const AudioUploadException('AUDIO_FILE_UNREADABLE');
      }

      await _remote.uploadAudioChunk(
        assetId: assetId,
        index: index,
        bytes: bytes,
      );
      uploadedChunks.add(index);
      await _store.updateAudioUploadedChunks(capture.id, uploadedChunks);
    }

    // 3. Complete — the backend verifies the full-file SHA-256 and size.
    await _remote.completeAudioAsset(assetId);

    // 4. Transcribe when requested, persisting the result locally so the
    //    transcript survives restarts.
    AudioTranscriptResult? transcript;
    if (capture.sttEnabled) {
      transcript = await _remote.transcribe(capture.id);
      await _store.saveTranscript(
        capture.id,
        text: transcript.text,
        source: transcript.source,
        version: transcript.version,
      );
    }
    return transcript;
  }
}
