class AudioFileMetadata {
  const AudioFileMetadata({required this.sizeBytes, required this.sha256});

  final int sizeBytes;
  final String sha256;
}

/// Web does not persist recorded audio to a real file in this round; the
/// recording UI shows a capability notice and falls back to text capture.
Future<String> getAudioCaptureDirectory() {
  throw UnsupportedError('Audio file storage is not supported on web');
}

Future<AudioFileMetadata> readAudioFileMetadata(String filePath) {
  throw UnsupportedError('Audio file storage is not supported on web');
}

Future<void> deleteAudioFile(String filePath) {
  throw UnsupportedError('Audio file storage is not supported on web');
}
