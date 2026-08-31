import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import '../../../shared/api/api_client.dart';

/// Parsed transcript revision returned by the backend.
class AudioTranscriptResult {
  const AudioTranscriptResult({
    required this.text,
    required this.source,
    required this.version,
  });

  final String text;

  /// 'stt' for automatic transcription, 'user' for a user-corrected revision.
  final String source;
  final int version;

  factory AudioTranscriptResult.fromJson(Map<String, dynamic> json) {
    final transcript = json['transcript'];
    if (transcript is! Map<String, dynamic>) {
      throw const FormatException('Invalid transcript response');
    }
    return AudioTranscriptResult(
      text: (transcript['text'] as String?) ?? '',
      source: (transcript['source'] as String?) ?? 'stt',
      version: (transcript['revision'] as num?)?.toInt() ?? 1,
    );
  }
}

class AudioAssetInitiateParams {
  const AudioAssetInitiateParams({
    required this.assetId,
    required this.captureId,
    required this.mimeType,
    required this.sizeBytes,
    required this.sha256,
    required this.totalChunks,
    this.durationMs,
    this.sttEnabled = true,
  });

  final String assetId;
  final String captureId;
  final String mimeType;
  final int sizeBytes;
  final String sha256;
  final int totalChunks;
  final int? durationMs;
  final bool sttEnabled;
}

abstract interface class AudioRemoteGateway {
  /// POST /audio-assets — idempotent; safe to call repeatedly for the same
  /// asset id.
  Future<void> initiateAudioAsset(AudioAssetInitiateParams params);

  /// PUT /audio-assets/:id/chunks/:index — raw bytes with per-chunk SHA-256.
  Future<void> uploadAudioChunk({
    required String assetId,
    required int index,
    required Uint8List bytes,
  });

  /// POST /audio-assets/:id/complete — finalizes the asset after all chunks.
  Future<void> completeAudioAsset(String assetId);

  /// POST /captures/:captureId/transcribe — runs full-file STT.
  Future<AudioTranscriptResult> transcribe(String captureId);

  /// PATCH /captures/:captureId/transcript — records a user-corrected revision.
  Future<AudioTranscriptResult> correctTranscript(
    String captureId,
    String text,
  );

  /// GET /captures/:captureId/transcript — fetches the latest revision.
  Future<AudioTranscriptResult> getLatestTranscript(String captureId);
}

class AudioApi implements AudioRemoteGateway {
  const AudioApi(this._client);

  final ApiClient _client;

  @override
  Future<void> initiateAudioAsset(AudioAssetInitiateParams params) async {
    final response = await _client.post(
      '/audio-assets',
      body: {
        'asset_id': params.assetId,
        'capture_id': params.captureId,
        'mime_type': params.mimeType,
        'duration_ms': params.durationMs,
        'size_bytes': params.sizeBytes,
        'sha256': params.sha256,
        'total_chunks': params.totalChunks,
        'stt_enabled': params.sttEnabled,
      },
    );
    if (response.data is! Map<String, dynamic>) {
      throw const FormatException('Invalid audio initiate response');
    }
  }

  @override
  Future<void> uploadAudioChunk({
    required String assetId,
    required int index,
    required Uint8List bytes,
  }) async {
    final chunkSha = sha256.convert(bytes).toString();
    await _client.putBytes(
      '/audio-assets/$assetId/chunks/$index',
      body: bytes,
      headers: {'X-Chunk-SHA256': chunkSha},
    );
  }

  @override
  Future<void> completeAudioAsset(String assetId) async {
    final response = await _client.post('/audio-assets/$assetId/complete');
    if (response.data is! Map<String, dynamic>) {
      throw const FormatException('Invalid audio complete response');
    }
  }

  @override
  Future<AudioTranscriptResult> transcribe(String captureId) async {
    final response = await _client.post('/captures/$captureId/transcribe');
    return AudioTranscriptResult.fromJson(
      response.data as Map<String, dynamic>,
    );
  }

  @override
  Future<AudioTranscriptResult> correctTranscript(
    String captureId,
    String text,
  ) async {
    final response = await _client.patch(
      '/captures/$captureId/transcript',
      body: {'text': text},
    );
    return AudioTranscriptResult.fromJson(
      response.data as Map<String, dynamic>,
    );
  }

  @override
  Future<AudioTranscriptResult> getLatestTranscript(String captureId) async {
    final response = await _client.get('/captures/$captureId/transcript');
    return AudioTranscriptResult.fromJson(
      response.data as Map<String, dynamic>,
    );
  }
}
