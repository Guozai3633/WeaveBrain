import 'audio_file_storage.dart';

/// Injectable audio file storage abstraction used by the capture state
/// machine so tests can substitute a fake without touching the filesystem.
abstract interface class AudioFileStorage {
  Future<String> getDirectory();
  Future<AudioFileMetadata> readMetadata(String filePath);
  Future<void> deleteFile(String filePath);
}

/// Default implementation backed by the platform audio file helpers.
class DefaultAudioFileStorage implements AudioFileStorage {
  const DefaultAudioFileStorage();

  @override
  Future<String> getDirectory() => getAudioCaptureDirectory();

  @override
  Future<AudioFileMetadata> readMetadata(String filePath) =>
      readAudioFileMetadata(filePath);

  @override
  Future<void> deleteFile(String filePath) => deleteAudioFile(filePath);
}
