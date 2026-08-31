import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart';

class AudioFileMetadata {
  const AudioFileMetadata({required this.sizeBytes, required this.sha256});

  final int sizeBytes;
  final String sha256;
}

/// Returns the app-private directory used to store recorded audio captures.
Future<String> getAudioCaptureDirectory() async {
  final base = await getApplicationDocumentsDirectory();
  final directory = Directory(path.join(base.path, 'audio_captures'));
  await directory.create(recursive: true);
  return directory.path;
}

/// Reads the size and full-file SHA-256 of a recorded audio file.
Future<AudioFileMetadata> readAudioFileMetadata(String filePath) async {
  final file = File(filePath);
  final length = await file.length();
  final Digest digest = await sha256.bind(file.openRead()).first;
  return AudioFileMetadata(sizeBytes: length, sha256: digest.toString());
}

/// Deletes a recorded audio file (e.g. when the user cancels recording).
Future<void> deleteAudioFile(String filePath) async {
  final file = File(filePath);
  if (await file.exists()) {
    await file.delete();
  }
}
