import 'dart:io';
import 'dart:typed_data';

import '../domain/local_capture.dart';

/// Reads up to [length] bytes of the capture's local audio file starting at
/// [offset].
///
/// Returns an empty list when the capture has no local file or the requested
/// range falls outside the file, so the caller can treat it as unreadable.
Future<Uint8List> readAudioChunk({
  required LocalCapture capture,
  required int offset,
  required int length,
}) async {
  final path = capture.audioLocalPath;
  if (path == null || path.isEmpty) return Uint8List(0);
  if (offset < 0 || length <= 0) return Uint8List(0);

  final File file = File(path);
  if (!await file.exists()) return Uint8List(0);

  final RandomAccessFile raf = await file.open();
  try {
    final fileLength = await raf.length();
    if (offset >= fileLength) return Uint8List(0);
    await raf.setPosition(offset);
    final remaining = fileLength - offset;
    final toRead = length > remaining ? remaining : length;
    final List<int> bytes = await raf.read(toRead);
    return Uint8List.fromList(bytes);
  } finally {
    await raf.close();
  }
}
