import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// Confirmation feedback for the minimal voice capture flow. The screen drives
/// feedback through this injected service so every platform and test can stub
/// it (web and widget tests use [NoopCaptureFeedbackService] / a fake).
abstract class CaptureFeedbackService {
  /// Fired the moment recording actually begins (haptic tick).
  void startTapped();

  /// Fired once a capture is safely saved (haptic bump + alert sound).
  void saved();
}

/// No-op provider used on web and in tests where audio is unsupported.
class NoopCaptureFeedbackService implements CaptureFeedbackService {
  const NoopCaptureFeedbackService();

  @override
  void startTapped() {}

  @override
  void saved() {}
}

/// Real feedback backed by haptics + system sound on device. Web is a no-op
/// because the browser cannot guarantee tactile feedback.
class DeviceCaptureFeedbackService implements CaptureFeedbackService {
  const DeviceCaptureFeedbackService();

  @override
  void startTapped() {
    if (kIsWeb) return;
    unawaited(HapticFeedback.lightImpact());
  }

  @override
  void saved() {
    if (kIsWeb) return;
    unawaited(HapticFeedback.mediumImpact());
    unawaited(SystemSound.play(SystemSoundType.alert));
  }
}
