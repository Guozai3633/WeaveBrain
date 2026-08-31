import 'dart:io' show Platform;

String get apiHost {
  if (Platform.isAndroid) return '10.0.2.2:8083';
  return 'localhost:8083';
}
