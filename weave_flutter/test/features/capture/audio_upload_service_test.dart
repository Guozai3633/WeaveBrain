import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:sembast/sembast_memory.dart';
import 'package:weave_flutter/features/capture/data/audio_remote_gateway.dart';
import 'package:weave_flutter/features/capture/data/audio_upload_service.dart';
import 'package:weave_flutter/features/capture/data/sembast_local_capture_store.dart';
import 'package:weave_flutter/features/capture/domain/local_capture.dart';

class FakeAudioRemote implements AudioRemoteGateway {
  final List<AudioAssetInitiateParams> initiates = [];
  final List<({String assetId, int index, Uint8List bytes})> uploadedChunks =
      [];
  final List<String> completed = [];
  final List<String> transcribed = [];
  final List<({String captureId, String text})> corrections = [];

  AudioTranscriptResult Function()? onTranscribe;

  @override
  Future<void> initiateAudioAsset(AudioAssetInitiateParams params) async {
    initiates.add(params);
  }

  @override
  Future<void> uploadAudioChunk({
    required String assetId,
    required int index,
    required Uint8List bytes,
  }) async {
    uploadedChunks.add((assetId: assetId, index: index, bytes: bytes));
  }

  @override
  Future<void> completeAudioAsset(String assetId) async {
    completed.add(assetId);
  }

  @override
  Future<AudioTranscriptResult> transcribe(String captureId) async {
    transcribed.add(captureId);
    return onTranscribe?.call() ??
        const AudioTranscriptResult(text: 'hello world', source: 'stt', version: 1);
  }

  @override
  Future<AudioTranscriptResult> correctTranscript(
    String captureId,
    String text,
  ) async {
    corrections.add((captureId: captureId, text: text));
    return AudioTranscriptResult(text: text, source: 'user', version: 2);
  }

  @override
  Future<AudioTranscriptResult> getLatestTranscript(String captureId) async {
    return const AudioTranscriptResult(text: 'hello world', source: 'stt', version: 1);
  }
}

Future<SembastLocalCaptureStore> createStore() async {
  final database = await databaseFactoryMemory.openDatabase(
    'audio_upload_${DateTime.now().microsecondsSinceEpoch}',
  );
  return SembastLocalCaptureStore(database);
}

LocalCapture audioCapture({
  String? assetId,
  int sizeBytes = 100,
  int totalChunks = 1,
  List<int> uploadedChunks = const [],
  bool sttEnabled = true,
  String? localPath = '/tmp/test.webm',
}) {
  final now = DateTime.utc(2026, 8, 16, 12);
  return LocalCapture(
    id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
    kind: 'audio',
    text: '',
    capturedAt: now,
    source: 'mobile_android',
    syncState: LocalSyncState.savedLocal,
    createdAt: now,
    updatedAt: now,
    audioAssetId: assetId,
    audioMimeType: 'audio/webm',
    audioSizeBytes: sizeBytes,
    audioSha256: 'deadbeef',
    audioTotalChunks: totalChunks,
    audioUploadedChunks: uploadedChunks,
    audioLocalPath: localPath,
    audioDurationMs: 5000,
    sttEnabled: sttEnabled,
  );
}

/// Returns a chunk reader backed by an in-memory buffer.
AudioChunkReader bufferReader(List<int> data) {
  return ({
    required LocalCapture capture,
    required int offset,
    required int length,
  }) async {
    if (offset >= data.length) return Uint8List(0);
    final end = (offset + length) > data.length ? data.length : offset + length;
    return Uint8List.fromList(data.sublist(offset, end));
  };
}

void main() {
  test('single-chunk upload runs initiate, complete and transcribe', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(assetId: 'asset-1');
    await store.saveDraft(capture);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(List<int>.filled(100, 7)),
    );

    final transcript = await service.uploadAudio(capture);

    expect(remote.initiates, hasLength(1));
    expect(remote.initiates.single.assetId, 'asset-1');
    expect(remote.initiates.single.captureId, capture.id);
    expect(remote.initiates.single.mimeType, 'audio/webm');
    expect(remote.initiates.single.sizeBytes, 100);
    expect(remote.initiates.single.sha256, 'deadbeef');
    expect(remote.initiates.single.totalChunks, 1);
    expect(remote.initiates.single.durationMs, 5000);
    expect(remote.initiates.single.sttEnabled, isTrue);

    expect(remote.uploadedChunks, hasLength(1));
    expect(remote.uploadedChunks.single.index, 0);
    expect(remote.uploadedChunks.single.bytes, hasLength(100));

    expect(remote.completed, ['asset-1']);
    expect(remote.transcribed, [capture.id]);
    expect(transcript?.text, 'hello world');

    final stored = await store.getById(capture.id);
    expect(stored?.transcript, 'hello world');
    expect(stored?.transcriptSource, 'stt');
    expect(stored?.transcriptVersion, 1);
    expect(stored?.audioUploadedChunks, [0]);

    await store.close();
  });

  test('resume skips already-uploaded chunks and persists progress', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    // 10 MiB split into 2 chunks of 5 MiB; chunk 0 already uploaded.
    const chunkBytes = 5 << 20;
    final size = chunkBytes * 2;
    final capture = audioCapture(
      assetId: 'asset-2',
      sizeBytes: size,
      totalChunks: 2,
      uploadedChunks: [0],
    );
    await store.saveDraft(capture);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: chunkBytes,
      chunkReader: bufferReader(List<int>.filled(size, 1)),
    );

    final transcript = await service.uploadAudio(capture);

    expect(remote.uploadedChunks, hasLength(1));
    expect(remote.uploadedChunks.single.index, 1);
    expect(remote.uploadedChunks.single.bytes, hasLength(chunkBytes));
    expect(remote.completed, ['asset-2']);
    expect(transcript, isNotNull);

    final stored = await store.getById(capture.id);
    expect(stored?.audioUploadedChunks, [0, 1]);
    await store.close();
  });

  test('last chunk is truncated to the remaining file size', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    const chunkBytes = 1000;
    final size = chunkBytes + 42; // 1 full chunk + a 42-byte tail
    final capture = audioCapture(
      assetId: 'asset-3',
      sizeBytes: size,
      totalChunks: 2,
    );
    await store.saveDraft(capture);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: chunkBytes,
      chunkReader: bufferReader(List<int>.filled(size, 2)),
    );

    await service.uploadAudio(capture);

    expect(remote.uploadedChunks, hasLength(2));
    expect(remote.uploadedChunks[0].bytes, hasLength(chunkBytes));
    expect(remote.uploadedChunks[1].bytes, hasLength(42));
    await store.close();
  });

  test('stt disabled uploads the asset but does not transcribe', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(assetId: 'asset-4', sttEnabled: false);
    await store.saveDraft(capture);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(List<int>.filled(100, 3)),
    );

    final transcript = await service.uploadAudio(capture);

    expect(remote.completed, ['asset-4']);
    expect(remote.transcribed, isEmpty);
    expect(transcript, isNull);
    expect((await store.getById(capture.id))?.transcript, isNull);
    await store.close();
  });

  test('missing audio metadata raises AUDIO_METADATA_MISSING', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(
      assetId: 'asset-5',
      sizeBytes: 0,
    );
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(const []),
    );

    expect(
      () => service.uploadAudio(capture),
      throwsA(
        isA<AudioUploadException>().having(
          (e) => e.code,
          'code',
          'AUDIO_METADATA_MISSING',
        ),
      ),
    );
    expect(remote.initiates, isEmpty);
    await store.close();
  });

  test('unreadable file raises AUDIO_FILE_UNREADABLE and stops the pipeline',
      () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(assetId: 'asset-6');
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(const []),
    );

    await expectLater(
      service.uploadAudio(capture),
      throwsA(
        isA<AudioUploadException>().having(
          (e) => e.code,
          'code',
          'AUDIO_FILE_UNREADABLE',
        ),
      ),
    );
    // Initiate happened but the pipeline halted before complete/transcribe.
    expect(remote.initiates, hasLength(1));
    expect(remote.completed, isEmpty);
    expect(remote.transcribed, isEmpty);
    await store.close();
  });

  test('missing local path raises AUDIO_METADATA_MISSING before any call',
      () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(assetId: 'asset-7', localPath: null);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(List<int>.filled(100, 1)),
    );

    await expectLater(
      service.uploadAudio(capture),
      throwsA(
        isA<AudioUploadException>().having(
          (e) => e.code,
          'code',
          'AUDIO_METADATA_MISSING',
        ),
      ),
    );
    expect(remote.initiates, isEmpty);
    await store.close();
  });

  test('non-audio capture is a no-op', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final now = DateTime.utc(2026, 8, 16, 12);
    final capture = LocalCapture(
      id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      text: 'plain text',
      capturedAt: now,
      source: 'web',
      syncState: LocalSyncState.savedLocal,
      createdAt: now,
      updatedAt: now,
    );
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(const []),
    );

    final transcript = await service.uploadAudio(capture);

    expect(transcript, isNull);
    expect(remote.initiates, isEmpty);
    expect(remote.uploadedChunks, isEmpty);
    expect(remote.completed, isEmpty);
    expect(remote.transcribed, isEmpty);
    await store.close();
  });

  test('asset id falls back to the capture id when not persisted', () async {
    final store = await createStore();
    final remote = FakeAudioRemote();
    final capture = audioCapture(assetId: null);
    await store.saveDraft(capture);
    final service = AudioUploadService(
      store: store,
      remote: remote,
      chunkBytes: 100,
      chunkReader: bufferReader(List<int>.filled(100, 5)),
    );

    await service.uploadAudio(capture);

    expect(remote.initiates.single.assetId, capture.id);
    expect(remote.uploadedChunks.single.assetId, capture.id);
    expect(remote.completed, [capture.id]);
    await store.close();
  });
}
