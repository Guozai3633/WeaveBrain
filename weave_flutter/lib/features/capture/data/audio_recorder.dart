import 'package:flutter/foundation.dart';
import 'package:record/record.dart' as record;

/// Injectable recorder abstraction used by the capture state machine so tests
/// can substitute a fake without touching the platform plugin.
abstract interface class AudioRecorder {
  bool get supportsFileRecording;
  Future<bool> hasPermission();
  Future<void> start({required String path});
  Stream<double> get amplitude;
  Future<String?> stop();
  Future<void> dispose();
}

/// Thin wrapper around the `record` plugin.
///
/// Records a WAV file to [path] on native platforms. Web can record to a blob
/// but not to a real file in this round, so file-based recording is disabled
/// there and the UI shows a capability notice.
class AudioRecorderDevice implements AudioRecorder {
  final record.AudioRecorder _recorder = record.AudioRecorder();

  @override
  bool get supportsFileRecording => !kIsWeb;

  /// Returns true when the microphone permission is granted.
  @override
  Future<bool> hasPermission() => _recorder.hasPermission();

  @override
  Future<void> start({required String path}) {
    return _recorder.start(
      const record.RecordConfig(
        encoder: record.AudioEncoder.wav,
        sampleRate: 44100,
        numChannels: 1,
      ),
      path: path,
    );
  }

  /// Normalized (0..1) input level stream for the waveform display.
  @override
  Stream<double> get amplitude {
    return _recorder
        .onAmplitudeChanged(const Duration(milliseconds: 100))
        .map((record.Amplitude event) {
      final dbfs = event.current;
      final normalized = ((dbfs + 60) / 60).clamp(0.0, 1.0);
      return normalized;
    });
  }

  /// Stops recording and returns the path of the written file.
  @override
  Future<String?> stop() => _recorder.stop();

  @override
  Future<void> dispose() => _recorder.dispose();
}
