import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:record/record.dart';

import '../../../shared/api/api_host.dart';
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
  StreamSubscription? _audioSub;
  String _accumulatedText = '';

  @override
  VoiceState build() => const VoiceIdle();

  String get _host => apiHost;

  Future<void> startRecording() async {
    final authState = ref.read(authNotifierProvider);
    if (authState is! Authenticated) {
      state = const VoiceError(message: '未登录');
      return;
    }

    state = const VoiceConnecting();
    _accumulatedText = '';

    // Track resources for cleanup on failure
    VoiceRepository? repo;
    AudioRecorder? recorder;
    StreamSubscription? resultSub;

    try {
      // Check permission first (before any network connections)
      recorder = AudioRecorder();
      final hasPermission = await recorder.hasPermission();
      if (!hasPermission) {
        recorder.dispose();
        state = const VoiceError(message: '没有麦克风权限');
        return;
      }

      // Connect WebSocket
      repo = VoiceRepository();
      repo.connect(_host, authState.token);

      resultSub = repo.results.listen(
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

      repo.startRecording();

      // Use startStream() for cross-platform audio streaming (works on web + native)
      final audioStream = await recorder.startStream(
        const RecordConfig(
          encoder: AudioEncoder.pcm16bits,
          sampleRate: 16000,
          numChannels: 1,
        ),
      );

      _audioSub = audioStream.listen((chunk) {
        repo?.sendAudio(chunk);
      }, onError: (_) {});

      // All succeeded — assign to instance fields
      _recorder = recorder;
      _repo = repo;
      _resultSub = resultSub;
      state = const VoiceRecording();
    } catch (e) {
      // Clean up all partially-created resources
      resultSub?.cancel();
      repo?.disconnect();
      recorder?.dispose();
      state = VoiceError(message: '无法连接: $e');
    }
  }

  Future<void> stopRecording() async {
    if (_repo == null) return;

    state = VoiceProcessing(finalText: _accumulatedText);

    // Stop audio stream
    _audioSub?.cancel();
    _audioSub = null;

    if (_recorder != null) {
      await _recorder!.stop();
      _recorder!.dispose();
      _recorder = null;
    }

    _repo!.stopRecording();

    // Wait a moment for final results, then disconnect
    await Future.delayed(const Duration(seconds: 1));
    _resultSub?.cancel();
    _resultSub = null;
    _repo?.disconnect();
    _repo = null;

    if (_accumulatedText.isNotEmpty) {
      // Send to Agent for processing
      try {
        final apiClient = ref.read(apiClientProvider);
        final resp = await apiClient.post(
          '/agent/process',
          body: {'input': _accumulatedText},
        );
        final data = resp.data;
        if (data is Map<String, dynamic> && !data.containsKey('error')) {
          state = VoiceResult(
            text: _accumulatedText,
            agentResponse: AgentResponse.fromJson(data),
          );
        } else {
          state = VoiceResult(text: _accumulatedText);
        }
      } catch (e) {
        // Agent processing failed, still show the transcription
        state = VoiceResult(text: _accumulatedText);
      }
    } else {
      state = const VoiceIdle();
    }
  }

  void reset() {
    _audioSub?.cancel();
    _audioSub = null;
    _resultSub?.cancel();
    _resultSub = null;
    _repo?.disconnect();
    _repo = null;
    _recorder?.dispose();
    _recorder = null;
    _accumulatedText = '';
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
        if (ar.tags.isNotEmpty) {
          structuredData['tags'] = ar.tags;
        }
        if (ar.feasibility != null) {
          structuredData['feasibility'] = ar.feasibility;
        }
        if (ar.suggestions.isNotEmpty) {
          structuredData['suggestions'] = ar.suggestions;
        }
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

final voiceNotifierProvider = NotifierProvider<VoiceNotifier, VoiceState>(
  () => VoiceNotifier(),
);
