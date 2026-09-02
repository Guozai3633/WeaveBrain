import 'package:shared_preferences/shared_preferences.dart';

/// 回响提醒的客户端本地偏好（投递时刻 / 静默时段）。
///
/// 与 [EchoSettings]（服务端 enabled+cadence，revision CAS）不同：这些字段
/// 只决定「本地通知何时邀请」，服务端读取时仍以 cadence 为准判断回响是否到期，
/// 因此不上送到 `/users/me/echo-settings`。
class EchoReminderPrefs {
  const EchoReminderPrefs({
    this.deliveryHour = 20,
    this.deliveryMinute = 0,
    this.silentEnabled = false,
    this.silentStartMinutes = 22 * 60,
    this.silentEndMinutes = 8 * 60,
  });

  /// 每日投递本地时刻（默认 20:00）。
  final int deliveryHour;
  final int deliveryMinute;

  /// 静默窗口（默认 22:00–08:00，跨午夜），命中该窗口的投递会被跳过。
  final bool silentEnabled;
  final int silentStartMinutes;
  final int silentEndMinutes;

  int get deliveryMinutesOfDay => deliveryHour * 60 + deliveryMinute;

  bool isInSilentWindow(int minuteOfDay) {
    if (!silentEnabled) return false;
    if (silentStartMinutes <= silentEndMinutes) {
      return minuteOfDay >= silentStartMinutes && minuteOfDay < silentEndMinutes;
    }
    // 跨午夜：22:00–08:00。
    return minuteOfDay >= silentStartMinutes || minuteOfDay < silentEndMinutes;
  }

  EchoReminderPrefs copyWith({
    int? deliveryHour,
    int? deliveryMinute,
    bool? silentEnabled,
    int? silentStartMinutes,
    int? silentEndMinutes,
  }) {
    return EchoReminderPrefs(
      deliveryHour: deliveryHour ?? this.deliveryHour,
      deliveryMinute: deliveryMinute ?? this.deliveryMinute,
      silentEnabled: silentEnabled ?? this.silentEnabled,
      silentStartMinutes: silentStartMinutes ?? this.silentStartMinutes,
      silentEndMinutes: silentEndMinutes ?? this.silentEndMinutes,
    );
  }

  static const _hourKey = 'echo_reminder_delivery_hour';
  static const _minuteKey = 'echo_reminder_delivery_minute';
  static const _silentEnabledKey = 'echo_reminder_silent_enabled';
  static const _silentStartKey = 'echo_reminder_silent_start_minutes';
  static const _silentEndKey = 'echo_reminder_silent_end_minutes';

  static Future<EchoReminderPrefs> load() async {
    final prefs = await SharedPreferences.getInstance();
    return EchoReminderPrefs(
      deliveryHour: prefs.getInt(_hourKey) ?? 20,
      deliveryMinute: prefs.getInt(_minuteKey) ?? 0,
      silentEnabled: prefs.getBool(_silentEnabledKey) ?? false,
      silentStartMinutes: prefs.getInt(_silentStartKey) ?? 22 * 60,
      silentEndMinutes: prefs.getInt(_silentEndKey) ?? 8 * 60,
    );
  }

  Future<void> save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(_hourKey, deliveryHour);
    await prefs.setInt(_minuteKey, deliveryMinute);
    await prefs.setBool(_silentEnabledKey, silentEnabled);
    await prefs.setInt(_silentStartKey, silentStartMinutes);
    await prefs.setInt(_silentEndKey, silentEndMinutes);
  }
}
