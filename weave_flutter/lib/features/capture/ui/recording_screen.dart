import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../domain/audio_capture_controller.dart';
import '../domain/capture_providers.dart';

/// 语音录音页。非 auto 模式保持既有显式开始/停止流；auto 模式（App
/// Shortcut / 桌面小组件 / 全局 FAB 进入，`/record?auto=1`）进入即自动开录，
/// 整页轻触一次停止保存，并触发声音/触觉确认——全程最多一次主动操作。
class RecordingScreen extends ConsumerStatefulWidget {
  const RecordingScreen({super.key, this.autoStart = false});

  final bool autoStart;

  @override
  ConsumerState<RecordingScreen> createState() => _RecordingScreenState();
}

class _RecordingScreenState extends ConsumerState<RecordingScreen> {
  static const _barCount = 48;
  Timer? _autoPopTimer;

  @override
  void initState() {
    super.initState();
    if (widget.autoStart) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        ref.read(audioCaptureControllerProvider.notifier).startIfIdle();
      });
    }
  }

  @override
  void dispose() {
    _autoPopTimer?.cancel();
    super.dispose();
  }

  void _scheduleAutoPop() {
    _autoPopTimer?.cancel();
    _autoPopTimer = Timer(const Duration(milliseconds: 600), () {
      if (!mounted) return;
      if (context.canPop()) {
        context.pop();
      } else {
        // 录音页可能以根路由打开（shortcut/widget 冷启动直达）。
        context.go('/capture');
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final controller = ref.read(audioCaptureControllerProvider.notifier);
    final state = ref.watch(audioCaptureControllerProvider);
    final scheme = Theme.of(context).colorScheme;
    final isSupported = controller.isSupported;
    final auto = widget.autoStart;

    ref.listen<AudioCaptureState>(audioCaptureControllerProvider, (previous, next) {
      if (!auto) return;
      final feedback = ref.read(captureFeedbackServiceProvider);
      final wasRecording = previous?.phase == AudioCapturePhase.recording;
      final wasSaved = previous?.phase == AudioCapturePhase.saved;
      if (next.phase == AudioCapturePhase.recording && !wasRecording) {
        feedback.startTapped();
      }
      if (next.phase == AudioCapturePhase.saved && !wasSaved) {
        feedback.saved();
        _scheduleAutoPop();
      }
    });

    final autoRecording = auto && state.phase == AudioCapturePhase.recording;
    final autoSaved = auto && state.phase == AudioCapturePhase.saved;

    final closeAffordance = IconButton(
      tooltip: '取消录音',
      onPressed: state.isRecording || state.isSaving
          ? () => _confirmCancel(context, controller)
          : () => context.pop(),
      icon: const Icon(Icons.close),
    );

    return PopScope(
      canPop: !state.isRecording && !state.isSaving,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          _confirmCancel(context, controller);
        }
      },
      child: Scaffold(
        backgroundColor: const Color(0xFF141218),
        appBar: AppBar(
          backgroundColor: Colors.transparent,
          foregroundColor: Colors.white,
          leading: autoRecording ? null : closeAffordance,
        ),
        body: SafeArea(
          child: autoRecording
              ? GestureDetector(
                  key: const Key('minimal_record_tap_target'),
                  behavior: HitTestBehavior.opaque,
                  onTap: () => controller.stop(),
                  child: _autoRecordingContent(state, isSupported),
                )
              : _content(controller, state, isSupported, scheme, autoSaved: autoSaved),
        ),
      ),
    );
  }

  Widget _autoRecordingContent(AudioCaptureState state, bool isSupported) {
    return Column(
      children: [
        const Spacer(flex: 2),
        _StatusMessage(state: state, isSupported: isSupported),
        const SizedBox(height: 24),
        _TimerText(elapsed: state.elapsed),
        const SizedBox(height: 32),
        _Waveform(amplitude: state.amplitude, active: state.isRecording),
        const Spacer(flex: 3),
        const Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.touch_app, color: Colors.white70, size: 18),
            SizedBox(width: 8),
            Text(
              '点按任意处结束并保存',
              style: TextStyle(color: Colors.white70, fontSize: 15),
            ),
          ],
        ),
        const SizedBox(height: 48),
      ],
    );
  }

  Widget _content(
    AudioCaptureController controller,
    AudioCaptureState state,
    bool isSupported,
    ColorScheme scheme, {
    required bool autoSaved,
  }) {
    if (autoSaved) {
      return Column(
        children: [
          const Spacer(flex: 2),
          const Icon(Icons.check_circle, color: Color(0xFF34D399), size: 56),
          const SizedBox(height: 16),
          const Text(
            '已安全保存',
            style: TextStyle(
              color: Color(0xFF34D399),
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
          const Spacer(flex: 3),
        ],
      );
    }
    return Column(
      children: [
        const Spacer(flex: 2),
        _StatusMessage(state: state, isSupported: isSupported),
        const SizedBox(height: 24),
        _TimerText(elapsed: state.elapsed),
        const SizedBox(height: 32),
        _Waveform(amplitude: state.amplitude, active: state.isRecording),
        const Spacer(flex: 3),
        _BottomControls(
          controller: controller,
          state: state,
          isSupported: isSupported,
          scheme: scheme,
        ),
        const SizedBox(height: 32),
      ],
    );
  }

  Future<void> _confirmCancel(
    BuildContext context,
    AudioCaptureController controller,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('放弃这段录音？'),
        content: const Text('录音尚未保存，放弃后无法恢复。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('继续录音'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('放弃'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await controller.cancel();
      // The record screen can be the root route (e.g. opened directly), in
      // which case there is nothing to pop.
      if (context.mounted && context.canPop()) context.pop();
    }
  }
}

class _StatusMessage extends StatelessWidget {
  const _StatusMessage({required this.state, required this.isSupported});

  final AudioCaptureState state;
  final bool isSupported;

  @override
  Widget build(BuildContext context) {
    final (message, color) = switch (state.phase) {
      AudioCapturePhase.idle => (
        isSupported ? '准备好后开始录音' : '当前浏览器不支持录音落盘',
        Colors.white70,
      ),
      AudioCapturePhase.requestingMic => ('正在请求麦克风权限…', Colors.white70),
      AudioCapturePhase.recording => ('正在录音', const Color(0xFFF59E0B)),
      AudioCapturePhase.stopping => ('正在安全保存…', Colors.white70),
      AudioCapturePhase.saved => ('已安全保存', const Color(0xFF34D399)),
      AudioCapturePhase.permissionDenied => ('无法录音：没有麦克风权限', const Color(0xFFF87171)),
      AudioCapturePhase.micInUse => ('无法录音：麦克风被占用', const Color(0xFFF87171)),
      AudioCapturePhase.saveFailed => ('保存失败', const Color(0xFFF87171)),
      AudioCapturePhase.unsupported => ('不支持录音', const Color(0xFFF87171)),
    };

    final detail = state.message;
    return Column(
      children: [
        Text(
          message,
          style: TextStyle(
            color: color,
            fontSize: 16,
            fontWeight: FontWeight.w600,
          ),
        ),
        if (detail != null && detail != message) ...[
          const SizedBox(height: 6),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Text(
              detail,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.white60, fontSize: 13),
            ),
          ),
        ],
      ],
    );
  }
}

class _TimerText extends StatelessWidget {
  const _TimerText({required this.elapsed});

  final Duration elapsed;

  String get _formatted {
    final minutes = elapsed.inMinutes.toString().padLeft(2, '0');
    final seconds = (elapsed.inSeconds % 60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  @override
  Widget build(BuildContext context) {
    return Text(
      _formatted,
      style: const TextStyle(
        color: Colors.white,
        fontSize: 56,
        fontWeight: FontWeight.w300,
        fontFeatures: [FontFeature.tabularFigures()],
      ),
    );
  }
}

class _Waveform extends StatelessWidget {
  const _Waveform({required this.amplitude, required this.active});

  final double amplitude;
  final bool active;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 72,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          for (var i = 0; i < _RecordingScreenState._barCount; i++) _bar(i),
        ],
      ),
    );
  }

  Widget _bar(int index) {
    // Deterministic pseudo-variation so bars differ even at a flat input.
    final variance = ((index * 37) % 11) / 20;
    final target = active ? (amplitude * 0.8 + variance).clamp(0.06, 1.0) : 0.06;
    final height = 8 + target * 56;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 1.5),
      child: Container(
        width: 4,
        height: height,
        decoration: BoxDecoration(
          color: active
              ? const Color(0xFFF59E0B)
              : Colors.white.withValues(alpha: 0.25),
          borderRadius: BorderRadius.circular(2),
        ),
      ),
    );
  }
}

class _BottomControls extends StatelessWidget {
  const _BottomControls({
    required this.controller,
    required this.state,
    required this.isSupported,
    required this.scheme,
  });

  final AudioCaptureController controller;
  final AudioCaptureState state;
  final bool isSupported;
  final ColorScheme scheme;

  @override
  Widget build(BuildContext context) {
    switch (state.phase) {
      case AudioCapturePhase.recording:
        return _RecordButton(
          key: const Key('record_stop_button'),
          label: '停止并保存',
          color: const Color(0xFFEF4444),
          onPressed: () => controller.stop(),
        );
      case AudioCapturePhase.requestingMic:
      case AudioCapturePhase.stopping:
        return const SizedBox.square(
          dimension: 72,
          child: CircularProgressIndicator(
            strokeWidth: 4,
            color: Colors.white70,
          ),
        );
      case AudioCapturePhase.saved:
        final captureId = state.captureId;
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.check_circle, color: Color(0xFF34D399), size: 56),
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                OutlinedButton.icon(
                  key: const Key('record_view_transcript'),
                  onPressed: captureId == null
                      ? null
                      : () => context.push('/captures/$captureId/transcript'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: Colors.white,
                    side: const BorderSide(color: Colors.white54),
                  ),
                  icon: const Icon(Icons.edit_note),
                  label: const Text('查看 / 修正转写'),
                ),
                const SizedBox(width: 12),
                FilledButton.icon(
                  onPressed: () => context.pop(),
                  icon: const Icon(Icons.done),
                  label: const Text('完成'),
                ),
              ],
            ),
          ],
        );
      case AudioCapturePhase.permissionDenied:
      case AudioCapturePhase.micInUse:
      case AudioCapturePhase.saveFailed:
      case AudioCapturePhase.unsupported:
        return FilledButton.icon(
          key: const Key('record_retry_button'),
          onPressed: () => controller.start(),
          icon: const Icon(Icons.refresh),
          label: const Text('重试'),
        );
      case AudioCapturePhase.idle:
        return _RecordButton(
          key: const Key('record_start_button'),
          label: isSupported ? '开始录音' : '不支持录音',
          color: const Color(0xFFF59E0B),
          onPressed: isSupported ? () => controller.start() : null,
        );
    }
  }
}

class _RecordButton extends StatelessWidget {
  const _RecordButton({
    super.key,
    required this.label,
    required this.color,
    required this.onPressed,
  });

  final String label;
  final Color color;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        InkWell(
          onTap: onPressed,
          borderRadius: BorderRadius.circular(36),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: color,
              boxShadow: [
                BoxShadow(
                  color: color.withValues(alpha: 0.4),
                  blurRadius: 24,
                  spreadRadius: 2,
                ),
              ],
            ),
            child: onPressed == null
                ? const Icon(Icons.mic_off, color: Colors.white, size: 32)
                : const Icon(Icons.mic, color: Colors.white, size: 32),
          ),
        ),
        const SizedBox(height: 12),
        Text(
          label,
          style: const TextStyle(color: Colors.white70, fontSize: 14),
        ),
      ],
    );
  }
}
