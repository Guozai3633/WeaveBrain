import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/shared/native/launch_request.dart';

void main() {
  group('launchRouteFor', () {
    test('notification taps land on the echo hub', () {
      expect(
        launchRouteFor(const LaunchRequest(LaunchSource.notification)),
        '/echo',
      );
    });

    test('quick capture shortcut and home widget go to the minimal record', () {
      expect(
        launchRouteFor(const LaunchRequest(LaunchSource.quickCapture)),
        '/record?auto=1',
      );
      expect(
        launchRouteFor(const LaunchRequest(LaunchSource.widget)),
        '/record?auto=1',
      );
    });
  });

  group('launchMemoryRouteFor', () {
    final notification = const LaunchRequest(LaunchSource.notification);
    final quickCapture = const LaunchRequest(LaunchSource.quickCapture);

    test('notification with an open echo deep-links to its capture', () {
      expect(
        launchMemoryRouteFor(
          request: notification,
          hasOpenEcho: true,
          captureId: 'c1',
        ),
        '/memories/c1',
      );
    });

    test('notification without an open echo falls back to the echo hub', () {
      expect(
        launchMemoryRouteFor(
          request: notification,
          hasOpenEcho: false,
          captureId: null,
        ),
        '/echo',
      );
    });

    test('quick captures always go to the minimal record', () {
      expect(
        launchMemoryRouteFor(
          request: quickCapture,
          hasOpenEcho: true,
          captureId: 'c1',
        ),
        '/record?auto=1',
      );
    });
  });
}
