import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/timezone.dart' as tz;

/// Schedules echo reminder notifications on the device. Echo correctness is
/// server-authoritative (`GET /echoes/current` decides whether an echo is
/// actually due when the app opens); these notifications are only an
/// invitation, so scheduling is best-effort and payloads carry no capture id.
abstract class LocalNotificationService {
  /// One-time plugin setup (channel + tap callback registration).
  Future<void> initialize();

  /// Asks the OS for notification permission when the user first enables echo
  /// (Android 13+ runtime permission / iOS alert).
  Future<void> requestPermission();

  /// Drops every scheduled and already-shown notification.
  Future<void> cancelAll();

  /// Schedules one notification to fire at the given UTC instant.
  Future<void> schedule({
    required int id,
    required String title,
    required String body,
    required DateTime whenUtc,
    required String payload,
  });

  /// Payload of the notification that launched the app (null when the app was
  /// not started by a notification tap). Used once at cold start.
  Future<String?> launchPayload();

  /// Warm-start taps: fires with the payload each time the user taps a
  /// notification while the app is alive.
  Stream<String> get onTap;
}

/// No-op used on web, where local scheduled notifications are unavailable.
class NoopLocalNotificationService implements LocalNotificationService {
  const NoopLocalNotificationService();

  @override
  Future<void> initialize() async {}

  @override
  Future<void> requestPermission() async {}

  @override
  Future<void> cancelAll() async {}

  @override
  Future<void> schedule({
    required int id,
    required String title,
    required String body,
    required DateTime whenUtc,
    required String payload,
  }) async {}

  @override
  Future<String?> launchPayload() async => null;

  @override
  Stream<String> get onTap => const Stream<String>.empty();
}

/// Real device implementation (Android + iOS). On Android the schedule mode is
/// inexact-while-idle so no SCHEDULE_EXACT_ALARM permission is required.
class DeviceLocalNotificationService implements LocalNotificationService {
  DeviceLocalNotificationService({FlutterLocalNotificationsPlugin? plugin})
    : _plugin = plugin ?? FlutterLocalNotificationsPlugin();

  final FlutterLocalNotificationsPlugin _plugin;
  final StreamController<String> _onTap = StreamController<String>.broadcast();

  @override
  Stream<String> get onTap => _onTap.stream;

  @override
  Future<void> initialize() async {
    if (kIsWeb) return;
    const settings = InitializationSettings(
      android: AndroidInitializationSettings('@mipmap/ic_launcher'),
      iOS: DarwinInitializationSettings(),
    );
    await _plugin.initialize(
      settings: settings,
      onDidReceiveNotificationResponse: (response) {
        final payload = response.payload;
        if (payload != null && payload.isNotEmpty) {
          _onTap.add(payload);
        }
      },
    );
  }

  @override
  Future<void> requestPermission() async {
    if (kIsWeb) return;
    if (defaultTargetPlatform == TargetPlatform.android) {
      await _plugin
          .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin
          >()
          ?.requestNotificationsPermission();
    } else if (defaultTargetPlatform == TargetPlatform.iOS) {
      await _plugin
          .resolvePlatformSpecificImplementation<
            IOSFlutterLocalNotificationsPlugin
          >()
          ?.requestPermissions(alert: true, badge: true, sound: true);
    }
  }

  @override
  Future<void> cancelAll() async {
    if (kIsWeb) return;
    await _plugin.cancelAll();
  }

  @override
  Future<void> schedule({
    required int id,
    required String title,
    required String body,
    required DateTime whenUtc,
    required String payload,
  }) async {
    if (kIsWeb) return;
    const details = NotificationDetails(
      android: AndroidNotificationDetails(
        'echo_reminders',
        '回响提醒',
        channelDescription: '到点邀请你回看一条记忆',
        importance: Importance.high,
        priority: Priority.high,
      ),
      iOS: DarwinNotificationDetails(),
    );
    await _plugin.zonedSchedule(
      id: id,
      title: title,
      body: body,
      scheduledDate: tz.TZDateTime.from(whenUtc.toUtc(), tz.UTC),
      notificationDetails: details,
      androidScheduleMode: AndroidScheduleMode.inexactAllowWhileIdle,
      payload: payload,
    );
  }

  @override
  Future<String?> launchPayload() async {
    if (kIsWeb) return null;
    final details = await _plugin.getNotificationAppLaunchDetails();
    if (details == null || !details.didNotificationLaunchApp) return null;
    return details.notificationResponse?.payload;
  }
}
