import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:quick_actions/quick_actions.dart';

import 'launch_request.dart';

/// The action type registered for the app shortcut that starts a quick record.
const String quickCaptureActionType = 'quick_capture';

/// Surface for the home-screen App Shortcut ("速记"). The shortcut forwards to
/// the minimal record route; the type is normalized to a [LaunchRequest] so the
/// router never depends on the plugin.
abstract class QuickActionsService {
  /// Registers the shortcut items and the warm-start tap stream.
  Future<void> initialize();

  /// Warm-start shortcut taps.
  Stream<LaunchRequest> get launches;
}

class NoopQuickActionsService implements QuickActionsService {
  const NoopQuickActionsService();

  @override
  Future<void> initialize() async {}

  @override
  Stream<LaunchRequest> get launches => const Stream<LaunchRequest>.empty();
}

class DeviceQuickActionsService implements QuickActionsService {
  DeviceQuickActionsService({QuickActions? actions})
    : _actions = actions ?? QuickActions();

  final QuickActions _actions;
  final StreamController<LaunchRequest> _launches =
      StreamController<LaunchRequest>.broadcast();

  @override
  Stream<LaunchRequest> get launches => _launches.stream;

  @override
  Future<void> initialize() async {
    if (kIsWeb) return;
    await _actions.initialize((type) {
      if (type == quickCaptureActionType) {
        _launches.add(const LaunchRequest(LaunchSource.quickCapture));
      }
    });
    await _actions.setShortcutItems(
      const [
        ShortcutItem(type: quickCaptureActionType, localizedTitle: '速记'),
      ],
    );
  }
}
