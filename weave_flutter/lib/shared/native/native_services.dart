import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'home_widget_service.dart';
import 'local_notification_service.dart';
import 'quick_actions_service.dart';
import 'timezone_service.dart';

/// Default providers for the injected native services. Web has no
/// implementation for any of these plugins, so every provider falls back to a
/// no-op there; widget tests override the provider with fakes so no plugin
/// channel is ever touched under `flutter test`.
final localNotificationServiceProvider =
    Provider<LocalNotificationService>((ref) {
      return kIsWeb
          ? const NoopLocalNotificationService()
          : DeviceLocalNotificationService();
    });

final quickActionsServiceProvider = Provider<QuickActionsService>((ref) {
  return kIsWeb
      ? const NoopQuickActionsService()
      : DeviceQuickActionsService();
});

final homeWidgetServiceProvider = Provider<HomeWidgetService>((ref) {
  return kIsWeb ? const NoopHomeWidgetService() : DeviceHomeWidgetService();
});

final timezoneServiceProvider = Provider<TimezoneService>((ref) {
  return kIsWeb ? const NoopTimezoneService() : DeviceTimezoneService();
});
