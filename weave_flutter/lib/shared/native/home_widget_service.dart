import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:home_widget/home_widget.dart';

import 'launch_request.dart';

/// Surface for the Android home-screen capture widget. The widget's single
/// action is "quick capture", so any real widget launch maps to a quick-capture
/// request; the plugin types never leak into routing.
abstract class HomeWidgetService {
  /// Cold start: returns a request when the app was launched by the widget.
  Future<LaunchRequest?> initiallyLaunched();

  /// Warm start: emits a request each time the widget is tapped while running.
  Stream<LaunchRequest> get launches;
}

class NoopHomeWidgetService implements HomeWidgetService {
  const NoopHomeWidgetService();

  @override
  Future<LaunchRequest?> initiallyLaunched() async => null;

  @override
  Stream<LaunchRequest> get launches => const Stream<LaunchRequest>.empty();
}

class DeviceHomeWidgetService implements HomeWidgetService {
  @override
  Future<LaunchRequest?> initiallyLaunched() async {
    if (kIsWeb) return null;
    final uri = await HomeWidget.initiallyLaunchedFromHomeWidget();
    if (uri == null) return null;
    return const LaunchRequest(LaunchSource.widget);
  }

  @override
  Stream<LaunchRequest> get launches {
    if (kIsWeb) return const Stream<LaunchRequest>.empty();
    return HomeWidget.widgetClicked
        .where((uri) => uri != null)
        .map((_) => const LaunchRequest(LaunchSource.widget));
  }
}
