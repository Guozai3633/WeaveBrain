import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:timezone/data/latest.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

import '../../../shared/native/local_notification_service.dart';
import '../../../shared/native/native_services.dart';
import '../data/echo_api.dart';
import 'echo_reminder_prefs.dart';

/// 本地提醒通知载荷（无推送设施；点通知后先拉 `/echoes/current` 判定目标）。
const String echoNotificationPayload = 'echo_reminder';

/// 回响本地提醒的起始通知 id（后续逐条递增）。
const int echoNotificationBaseId = 9000;

/// 未来 8 次回响邀请。节奏（1/2/7 天）与静默窗跳过都在此确定；服务端读取
/// `/echoes/current` 时才真正判定回响是否到期，因此本排程只是「邀请」。
List<DateTime> echoReminderInstants({
  required DateTime nowUtc,
  required tz.Location location,
  required int stepDays,
  required int deliveryMinutesOfDay,
  required EchoReminderPrefs prefs,
  int count = 8,
}) {
  final deliveryHour = deliveryMinutesOfDay ~/ 60;
  final deliveryMinute = deliveryMinutesOfDay % 60;
  final nowLocal = tz.TZDateTime.from(nowUtc, location);

  final results = <DateTime>[];
  // 防御：静默窗吞掉全部投递时避免死循环。
  final maxIterations = count * 20 + 60;
  final base = DateTime(nowLocal.year, nowLocal.month, nowLocal.day);
  var dayOffset = 0;
  var iterations = 0;
  while (results.length < count && iterations < maxIterations) {
    iterations++;
    final day = base.add(Duration(days: dayOffset));
    final localCandidate = tz.TZDateTime(
      location,
      day.year,
      day.month,
      day.day,
      deliveryHour,
      deliveryMinute,
    );
    final candidateUtc = localCandidate.toUtc();
    if (!candidateUtc.isAfter(nowUtc)) {
      dayOffset += stepDays;
      continue;
    }
    if (prefs.isInSilentWindow(deliveryMinutesOfDay)) {
      dayOffset += stepDays; // 落在静默窗内则跳过本次投递，不后移。
      continue;
    }
    results.add(candidateUtc);
    dayOffset += stepDays;
  }
  return results;
}

/// 回响本地通知排程器：把服务端 enabled+cadence 与本地投递时刻/静默时段合成
/// 未来若干条本地通知。取消/重排均由通知服务幂等处理（同 id 覆盖）。
class EchoScheduler {
  EchoScheduler(this._notifications, this._timezoneName, this._clock);

  final LocalNotificationService _notifications;
  final Future<String> Function() _timezoneName;
  final DateTime Function() _clock;

  /// 依服务端设置刷新本地提醒。禁用时取消全部已排通知。
  Future<void> refresh({
    required bool enabled,
    required String cadence,
    required EchoReminderPrefs prefs,
  }) async {
    await _notifications.cancelAll();
    if (!enabled) return;

    final stepDays = echoCadenceStepDays(cadence);
    if (stepDays <= 0) return;

    final location = await _locationFor(await _timezoneName());
    final instants = echoReminderInstants(
      nowUtc: _clock().toUtc(),
      location: location,
      stepDays: stepDays,
      deliveryMinutesOfDay: prefs.deliveryMinutesOfDay,
      prefs: prefs,
    );

    var id = echoNotificationBaseId;
    for (final instant in instants) {
      await _notifications.schedule(
        id: id,
        title: '织脑 · 回响',
        body: '有一条旧记忆想让你再看看',
        whenUtc: instant,
        payload: echoNotificationPayload,
      );
      id++;
    }
  }

  Future<tz.Location> _locationFor(String name) async {
    if (name == 'UTC' || name == 'Etc/UTC' || name.isEmpty) return tz.UTC;
    // 首次调用时加载时区数据库（数据随 timezone 包内置，离线可用）。
    tz_data.initializeTimeZones();
    try {
      return tz.getLocation(name);
    } catch (_) {
      return tz.UTC;
    }
  }
}

/// 默认排程器：注入本机通知服务 + 设备时区名。
final echoSchedulerProvider = Provider<EchoScheduler>((ref) {
  final timezone = ref.watch(timezoneServiceProvider);
  return EchoScheduler(
    ref.watch(localNotificationServiceProvider),
    timezone.deviceTimezoneName,
    DateTime.now,
  );
});
