import 'package:flutter/foundation.dart';
import 'package:flutter_timezone/flutter_timezone.dart';

/// Resolves the device's IANA timezone name (e.g. `Asia/Shanghai`) so echo
/// reminder occurrences can be anchored to the user's local wall clock across
/// DST boundaries. Web/unsupported hosts fall back to UTC.
abstract class TimezoneService {
  Future<String> deviceTimezoneName();
}

class NoopTimezoneService implements TimezoneService {
  const NoopTimezoneService();

  @override
  Future<String> deviceTimezoneName() async => 'UTC';
}

class DeviceTimezoneService implements TimezoneService {
  @override
  Future<String> deviceTimezoneName() async {
    if (kIsWeb) return 'UTC';
    final info = await FlutterTimezone.getLocalTimezone();
    return info.identifier;
  }
}
