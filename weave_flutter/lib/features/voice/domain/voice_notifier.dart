import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';

import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/agent_response.dart';
import '../../../shared/models/idea.dart';
import '../../ideas/data/idea_api.dart';
import '../data/voice_repository.dart';

sealed class VoiceState {
  const VoiceState();
}

class VoiceIdle extends VoiceState {
  const VoiceIdle();
}

class VoiceConnecting extends VoiceState {
  const VoiceConnecting();
}

class VoiceRecording extends VoiceState {
  final String partialText;
  const VoiceRecording({this.partialText = ''});
}

class VoiceProcessing extends VoiceState {
  final String finalText;
  const VoiceProcessing({required this.finalText});
}

class VoiceResult extends VoiceState {
  final String text;
  final AgentResponse? agentResponse;
  const VoiceResult({required this.text, this.agentResponse});
}

class VoiceError extends VoiceState {
  final String message;
  const VoiceError({required this.message});
}

class VoiceSaving extends VoiceState {
  const VoiceSaving();
}

class VoiceSaved extends VoiceState {
  final Idea idea;
  const VoiceSaved({required this.idea});
}

class VoiceNotifier extends Notifier<VoiceState> {
  VoiceRepository? _repo;
  AudioRecorder? _recorder;
  StreamSubscription? _resultSub;
  Timer? _audioTimer;
  String _accumulatedText = '';
  String? _tempPath;
  int _lastSentOffset = 0;

  @override
  VoiceState build() => const VoiceIdle();

  String get _host {
    return '10.0.2.2:8080';
  }

  Future<void> startRecording() async {
    final authState = ref.read(authNotifierProvider);
    if (authState is! Authenticated) {
      state = const VoiceError(message: '未登录');
      return;
    }

    state = const VoiceConnecting();
    _accumulatedText = '';

    try {
      _recorder = AudioRecorder();

      // Check permission
      final hasPermission = await _recorder!.hasPermission();
      if (!hasPermission) {
        state = const VoiceError(message: '没有麦克风权限');
        return;
      }

      // Connect WebSocket
      _repo = VoiceRepository();
      _repo!.connect(_host, authState.token);

      _resultSub = _repo!.results.listen(
        (result) {
          if (result.type == 'partial') {
            state = VoiceRecording(
              partialText: '$_accumulatedText${result.text}',
            );
          } else if (result.type == 'final') {
            _accumulatedText += result.text;
            state = VoiceRecording(partialText: _accumulatedText);
          }
        },
        onError: (error) {
          state = VoiceError(message: '连接错误: $error');
        },
      );

      _repo!.startRecording();

      // Start audio recording to temp file (raw PCM, no WAV header)
      final dir = await getTemporaryDirectory();
      _tempPath = '${dir.path}/weave_audio_${DateTime.now().millisecondsSinceEpoch}.pcm';
      _lastSentOffset = 0;
      await _recorder!.start(
        const RecordConfig(
          encoder: AudioEncoder.pcm16bits,
          sampleRate: 16000,
          numChannels: 1,
        ),
        path: _tempPath!,
      );

      // Periodically send audio chunks
      _audioTimer = Timer.periodic(const Duration(milliseconds: 500), (_) {
        _sendAudioChunk();
      });

      state = const VoiceRecording();
    } catch (e) {
      state = VoiceError(message: '无法连接: $e');
    }
  }

  Future<void> _sendAudioChunk() async {
    if (_tempPath == null || _repo == null) return;
    try {
      final file = File(_tempPath!);
      if (await file.exists()) {
        final bytes = await file.readAsBytes();
        final newBytes = bytes.length - _lastSentOffset;
        if (newBytes > 0) {
          final chunk = bytes.sublist(_lastSentOffset, bytes.length);
          _lastSentOffset = bytes.length;
          _repo!.sendAudio(chunk);
        }
      }
    } catch (_) {
      // Ignore chunk send errors during recording
    }
  }

  Future<void> stopRecording() async {
    if (_repo == null) return;

    state = VoiceProcessing(finalText: _accumulatedText);

    // Stop recording
    _audioTimer?.cancel();
    _audioTimer = null;

    if (_recorder != null) {
      await _recorder!.stop();
      _recorder!.dispose();
      _recorder = null;
    }

    // Send final audio chunk
    await _sendAudioChunk();

    _repo!.stopRecording();

    // Wait a moment for final results, then disconnect
    await Future.delayed(const Duration(seconds: 1));
    _resultSub?.cancel();
    _repo?.disconnect();
    _repo = null;

    // Clean up temp file
    if (_tempPath != null) {
      try {
        final file = File(_tempPath!);
        if (await file.exists()) {
          await file.delete();
        }
      } catch (_) {}
      _tempPath = null;
    }

    if (_accumulatedText.isNotEmpty) {
      // Send to Agent for processing
      try {
        final apiClient = ref.read(apiClientProvider);
        final resp = await apiClient.post('/agent/process', body: {
          'input': _accumulatedText,
        });
        final data = resp.data as Map<String, dynamic>;
        state = VoiceResult(
          text: _accumulatedText,
          agentResponse: AgentResponse.fromJson(data),
        );
      } catch (e) {
        // Agent processing failed, still show the transcription
        state = VoiceResult(text: _accumulatedText);
      }
    } else {
      state = const VoiceIdle();
    }
  }

  void reset() {
    _audioTimer?.cancel();
    _audioTimer = null;
    _resultSub?.cancel();
    _repo?.disconnect();
    _repo = null;
    _recorder?.dispose();
    _recorder = null;
    _accumulatedText = '';
    _lastSentOffset = 0;
    if (_tempPath != null) {
      try {
        File(_tempPath!).delete();
      } catch (_) {}
      _tempPath = null;
    }
    state = const VoiceIdle();
  }

  Future<void> saveIdea(int projectId) async {
    final current = state;
    if (current is! VoiceResult) return;

    state = const VoiceSaving();

    try {
      final apiClient = ref.read(apiClientProvider);
      final ideaApi = IdeaApi(apiClient);

      final structuredData = <String, dynamic>{};
      if (current.agentResponse != null) {
        final ar = current.agentResponse!;
        if (ar.tags.isNotEmpty) structuredData['tags'] = ar.tags;
        if (ar.feasibility != null) structuredData['feasibility'] = ar.feasibility;
        if (ar.suggestions.isNotEmpty) structuredData['suggestions'] = ar.suggestions;
        if (ar.baseInput != null) structuredData['base_input'] = ar.baseInput;
      }

      final idea = await ideaApi.createIdea(
        projectId: projectId,
        rawInput: current.text,
        structuredData: structuredData.isNotEmpty ? structuredData : null,
        tags: current.agentResponse?.tags,
      );

      state = VoiceSaved(idea: idea);
    } catch (e) {
      state = VoiceError(message: '保存失败: $e');
    }
  }
}

final voiceNotifierProvider =
    NotifierProvider<VoiceNotifier, VoiceState>(() => VoiceNotifier());
