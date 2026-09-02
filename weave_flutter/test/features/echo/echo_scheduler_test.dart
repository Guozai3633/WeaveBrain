import 'package:flutter_test/flutter_test.dart';
import 'package:timezone/timezone.dart' as tz;

import 'package:weave_flutter/features/echo/domain/echo_reminder_prefs.dart';
import 'package:weave_flutter/features/echo/domain/echo_scheduler.dart';

import '../../support/echo_fakes.dart';

void main() {
  setUpAll(() {
    // EchoScheduler._locationFor 会加载 IANA 时区库；此处预载供纯函数测试使用。
    try {
      tz.setLocalLocation(tz.UTC);
    } catch (_) {}
  });

  final now = DateTime.utc(2026, 9, 1, 12, 0); // 12:00 UTC。

  final prefsNoSilent = EchoReminderPrefs(
    deliveryHour: 20,
    deliveryMinute: 0,
  );

  group('echoReminderInstants occurrence math', () {
    test('steps every stepDays and anchors at local delivery time', () {
      final instants = echoReminderInstants(
        nowUtc: now,
        location: tz.UTC,
        stepDays: 1,
        deliveryMinutesOfDay: prefsNoSilent.deliveryMinutesOfDay,
        prefs: prefsNoSilent,
        count: 3,
      );
      expect(instants, hasLength(3));
      // 20:00 UTC 当天未过（now 12:00）→ 第一个即当天 20:00。
      expect(instants[0], DateTime.utc(2026, 9, 1, 20, 0));
      expect(instants[1], DateTime.utc(2026, 9, 2, 20, 0));
      expect(instants[2], DateTime.utc(2026, 9, 3, 20, 0));
    });

    test('skips a past delivery today when now is after the slot', () {
      final lateNow = DateTime.utc(2026, 9, 1, 21, 0); // 晚于 20:00。
      final instants = echoReminderInstants(
        nowUtc: lateNow,
        location: tz.UTC,
        stepDays: 1,
        deliveryMinutesOfDay: prefsNoSilent.deliveryMinutesOfDay,
        prefs: prefsNoSilent,
        count: 2,
      );
      expect(instants[0], DateTime.utc(2026, 9, 2, 20, 0));
      expect(instants[1], DateTime.utc(2026, 9, 3, 20, 0));
    });

    test('weekly cadence steps by 7 days', () {
      final instants = echoReminderInstants(
        nowUtc: now,
        location: tz.UTC,
        stepDays: 7,
        deliveryMinutesOfDay: prefsNoSilent.deliveryMinutesOfDay,
        prefs: prefsNoSilent,
        count: 2,
      );
      expect(instants[0], DateTime.utc(2026, 9, 1, 20, 0));
      expect(instants[1], DateTime.utc(2026, 9, 8, 20, 0));
    });

    test('delivery inside the silent window is skipped without shifting', () {
      final prefs = EchoReminderPrefs(
        deliveryHour: 23,
        deliveryMinute: 0,
        silentEnabled: true,
        silentStartMinutes: 22 * 60, // 22:00
        silentEndMinutes: 8 * 60, // 08:00 跨午夜
      );
      final instants = echoReminderInstants(
        nowUtc: now,
        location: tz.UTC,
        stepDays: 1,
        deliveryMinutesOfDay: prefs.deliveryMinutesOfDay,
        prefs: prefs,
        count: 2,
      );
      // 23:00 天天落在 22:00–08:00 窗内 → 全部跳过，防御性返回空列表。
      expect(instants, isEmpty);
    });

    test('same-day silent window leaves outside deliveries untouched', () {
      // 静默 12:00–14:00（当日窗口），投递 20:00 在窗外 → 逐日全排，不误伤。
      final prefs = EchoReminderPrefs(
        deliveryHour: 20,
        deliveryMinute: 0,
        silentEnabled: true,
        silentStartMinutes: 12 * 60,
        silentEndMinutes: 14 * 60,
      );
      final instants = echoReminderInstants(
        nowUtc: now,
        location: tz.UTC,
        stepDays: 1,
        deliveryMinutesOfDay: prefs.deliveryMinutesOfDay,
        prefs: prefs,
        count: 2,
      );
      expect(instants[0], DateTime.utc(2026, 9, 1, 20, 0));
      expect(instants[1], DateTime.utc(2026, 9, 2, 20, 0));
    });
  });

  group('EchoScheduler.refresh', () {
    test('cancels all then schedules count occurrences with ascending ids', () async {
      final notifications = FakeLocalNotificationService();
      final scheduler = EchoScheduler(
        notifications,
        () async => 'Asia/Shanghai',
        () => DateTime.utc(2026, 9, 1, 4, 0), // 上海 12:00。
      );
      const prefs = EchoReminderPrefs(deliveryHour: 20, deliveryMinute: 0);

      await scheduler.refresh(
        enabled: true,
        cadence: 'daily',
        prefs: prefs,
      );

      expect(notifications.cancelAllCalls, 1);
      expect(notifications.scheduled, hasLength(8));
      var expectedId = echoNotificationBaseId;
      for (final record in notifications.scheduled) {
        expect(record.id, expectedId);
        expect(record.payload, echoNotificationPayload);
        expectedId++;
      }
      // 上海 20:00 = 12:00 UTC（当日未过 now 04:00 UTC）→ 第一条约 2026-09-01T12:00Z。
      expect(notifications.scheduled.first.whenUtc, DateTime.utc(2026, 9, 1, 12, 0));
    });

    test('disabled cancels all and schedules nothing', () async {
      final notifications = FakeLocalNotificationService();
      final scheduler = EchoScheduler(
        notifications,
        () async => 'UTC',
        () => DateTime.utc(2026, 9, 1),
      );

      await scheduler.refresh(
        enabled: false,
        cadence: 'daily',
        prefs: const EchoReminderPrefs(),
      );

      expect(notifications.cancelAllCalls, 1);
      expect(notifications.scheduled, isEmpty);
    });

    test('unknown timezone falls back to UTC without throwing', () async {
      final notifications = FakeLocalNotificationService();
      final scheduler = EchoScheduler(
        notifications,
        () async => 'Not/AZone',
        () => DateTime.utc(2026, 9, 1, 0, 0),
      );

      await scheduler.refresh(
        enabled: true,
        cadence: 'weekly',
        prefs: const EchoReminderPrefs(deliveryHour: 20, deliveryMinute: 0),
      );

      expect(notifications.scheduled, hasLength(8));
      expect(notifications.scheduled.first.whenUtc, DateTime.utc(2026, 9, 1, 20, 0));
    });
  });

  group('EchoReminderPrefs.isInSilentWindow', () {
    test('cross-midnight window 22:00–08:00', () {
      const prefs = EchoReminderPrefs(
        silentEnabled: true,
        silentStartMinutes: 22 * 60,
        silentEndMinutes: 8 * 60,
      );
      expect(prefs.isInSilentWindow(23 * 60), isTrue);
      expect(prefs.isInSilentWindow(0), isTrue);
      expect(prefs.isInSilentWindow(7 * 60), isTrue);
      expect(prefs.isInSilentWindow(8 * 60), isFalse);
      expect(prefs.isInSilentWindow(12 * 60), isFalse);
    });

    test('disabled never silences', () {
      const prefs = EchoReminderPrefs();
      expect(prefs.isInSilentWindow(23 * 60), isFalse);
    });
  });
}
