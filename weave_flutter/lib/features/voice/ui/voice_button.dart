import 'package:flutter/material.dart';

enum VoiceButtonState { idle, recording, processing }

class VoiceButton extends StatefulWidget {
  final VoiceButtonState state;
  final VoidCallback onPressed;

  const VoiceButton({super.key, required this.state, required this.onPressed});

  @override
  State<VoiceButton> createState() => _VoiceButtonState();
}

class _VoiceButtonState extends State<VoiceButton>
    with SingleTickerProviderStateMixin {
  late AnimationController _pulseController;

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
  }

  @override
  void didUpdateWidget(VoiceButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.state == VoiceButtonState.recording &&
        oldWidget.state != VoiceButtonState.recording) {
      _pulseController.repeat(reverse: true);
    } else if (widget.state != VoiceButtonState.recording) {
      _pulseController.stop();
      _pulseController.value = 0;
    }
  }

  @override
  void dispose() {
    _pulseController.dispose();
    super.dispose();
  }

  Color get _buttonColor {
    return switch (widget.state) {
      VoiceButtonState.idle => Colors.grey.shade400,
      VoiceButtonState.recording => Colors.red,
      VoiceButtonState.processing => Colors.blue,
    };
  }

  IconData get _icon {
    return switch (widget.state) {
      VoiceButtonState.idle => Icons.mic,
      VoiceButtonState.recording => Icons.stop,
      VoiceButtonState.processing => Icons.hourglass_top,
    };
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: widget.state == VoiceButtonState.processing
          ? null
          : widget.onPressed,
      child: AnimatedBuilder(
        animation: _pulseController,
        builder: (context, child) {
          final scale = widget.state == VoiceButtonState.recording
              ? 1.0 + (_pulseController.value * 0.1)
              : 1.0;
          return Transform.scale(scale: scale, child: child);
        },
        child: Container(
          width: 120,
          height: 120,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: _buttonColor,
            boxShadow: [
              BoxShadow(
                color: _buttonColor.withValues(alpha: 0.4),
                blurRadius: 20,
                spreadRadius: widget.state == VoiceButtonState.recording
                    ? 8
                    : 2,
              ),
            ],
          ),
          child: Icon(_icon, size: 48, color: Colors.white),
        ),
      ),
    );
  }
}
