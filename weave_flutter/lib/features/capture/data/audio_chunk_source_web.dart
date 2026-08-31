import 'dart:typed_data';

import '../domain/local_capture.dart';

/// Web does not persist recorded audio to a real file in this round, so
/// chunked upload from local disk is not supported. The recording UI disables
/// audio capture on web and falls back to text capture.
Future<Uint8List> readAudioChunk({
  required LocalCapture capture,
  required int offset,
  required int length,
}) {
  throw UnsupportedError(
    'Audio chunk upload is not supported on web in this round',
  );
}
