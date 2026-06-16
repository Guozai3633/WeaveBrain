import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

class TranscriptionResult {
  final String text;
  final String type; // "partial" or "final"
  final double confidence;

  const TranscriptionResult({
    required this.text,
    required this.type,
    required this.confidence,
  });

  factory TranscriptionResult.fromJson(Map<String, dynamic> json) {
    return TranscriptionResult(
      text: json['text'] as String? ?? '',
      type: json['type'] as String? ?? 'partial',
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
    );
  }
}

class VoiceRepository {
  WebSocketChannel? _channel;
  final _controller = StreamController<TranscriptionResult>.broadcast();

  Stream<TranscriptionResult> get results => _controller.stream;

  void connect(String host, String token) {
    final uri = Uri.parse('ws://$host/ws/voice?token=$token');
    _channel = WebSocketChannel.connect(uri);

    _channel!.stream.listen(
      (message) {
        if (message is String) {
          try {
            final json = jsonDecode(message) as Map<String, dynamic>;
            if (json['type'] == 'partial' || json['type'] == 'final') {
              _controller.add(TranscriptionResult.fromJson(json));
            }
          } catch (_) {
            // Ignore non-JSON messages (e.g. "stopped")
          }
        }
      },
      onError: (error) {
        _controller.addError(error);
      },
      onDone: () {
        _controller.close();
      },
    );
  }

  void startRecording({String language = 'zh-CN'}) {
    _channel?.sink.add(jsonEncode({
      'type': 'start',
      'language': language,
    }));
  }

  void sendAudio(List<int> bytes) {
    _channel?.sink.add(bytes);
  }

  void stopRecording() {
    _channel?.sink.add(jsonEncode({'type': 'stop'}));
  }

  void disconnect() {
    _channel?.sink.close();
    _channel = null;
  }

  bool get isConnected => _channel != null;
}
