/// System entry points that start (or foreground) the app without the user
/// tapping inside the UI: tapping a scheduled echo notification, an app
/// shortcut, or the home-screen capture widget.
enum LaunchSource { notification, quickCapture, widget }

/// A normalized cold/warm start request. There is no push infrastructure and no
/// server push payload; a notification tap only signals "a reminder fired", so
/// the current echo (and its capture target) is resolved server-side afterwards.
class LaunchRequest {
  const LaunchRequest(this.source);

  final LaunchSource source;
}

/// Pure routing decision for a system entry (testable in isolation):
///
///  * quick-capture shortcut / home widget → the minimal record screen, which
///    auto-starts recording on arrival (`/record?auto=1`);
///  * echo notification tap → the echo hub, whose screen resolves the
///    server-authoritative current echo into a memory deep link when one is
///    open (see [launchMemoryRouteFor]).
String launchRouteFor(LaunchRequest request) {
  switch (request.source) {
    case LaunchSource.notification:
      return '/echo';
    case LaunchSource.quickCapture:
    case LaunchSource.widget:
      return '/record?auto=1';
  }
}

/// Once the current echo has been resolved, decides the memory deep link:
/// an open echo points at its capture; otherwise the user lands on the echo
/// hub (disabled / between windows / no candidates are all explicit empty
/// states there). Only notification launches resolve into a memory; quick
/// captures always go straight to recording.
String launchMemoryRouteFor({
  required LaunchRequest request,
  required bool hasOpenEcho,
  required String? captureId,
}) {
  if (request.source == LaunchSource.notification &&
      hasOpenEcho &&
      captureId != null &&
      captureId.isNotEmpty) {
    return '/memories/$captureId';
  }
  return launchRouteFor(request);
}
