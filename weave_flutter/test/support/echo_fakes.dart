import 'dart:async';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/echo/domain/echo_reminder_prefs.dart';
import 'package:weave_flutter/features/echo/domain/echo_scheduler.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';
import 'package:weave_flutter/shared/native/capture_feedback_service.dart';
import 'package:weave_flutter/shared/native/home_widget_service.dart';
import 'package:weave_flutter/shared/native/launch_request.dart';
import 'package:weave_flutter/shared/native/local_notification_service.dart';
import 'package:weave_flutter/shared/native/quick_actions_service.dart';
import 'package:weave_flutter/shared/native/timezone_service.dart';

// ---- Echo result builders ----

EchoSettingsResult makeEchoSettings({
  bool enabled = false,
  String cadence = echoCadenceDaily,
  int revision = 1,
}) {
  return EchoSettingsResult(
    settings: EchoSettings(
      userId: 'u1',
      enabled: enabled,
      cadence: cadence,
      revision: revision,
    ),
  );
}

CurrentEchoResult makeCurrentDisabled({
  String cadence = echoCadenceDaily,
  int revision = 1,
}) {
  return CurrentEchoResult(
    enabled: false,
    cadence: cadence,
    revision: revision,
  );
}

CurrentEchoResult makeCurrentEmpty({
  String cadence = echoCadenceDaily,
  int revision = 1,
  DateTime? nextDueAt,
  String? emptyReason,
}) {
  return CurrentEchoResult(
    enabled: true,
    cadence: cadence,
    revision: revision,
    nextDueAt: nextDueAt,
    emptyReason: emptyReason,
  );
}

CurrentEchoResult makeCurrentLoaded({
  String echoId = 'e1',
  String captureId = 'c1',
  String kind = 'text',
  String title = '闪念标题',
  String? summary = '一段摘要',
  String primaryType = 'idea',
  bool isPinned = false,
  DateTime? capturedAt,
  String reasonCode = 'first_echo',
  String reasonText = '从你较早记下、还没回看过的记忆开始',
  String cadence = echoCadenceDaily,
  int revision = 1,
  DateTime? createdAt,
}) {
  return CurrentEchoResult(
    enabled: true,
    cadence: cadence,
    revision: revision,
    echo: EchoCard(
      id: echoId,
      status: 'open',
      reason: EchoReason(code: reasonCode, text: reasonText),
      memory: EchoMemory(
        captureId: captureId,
        kind: kind,
        title: title,
        summary: summary,
        primaryType: primaryType,
        capturedAt: capturedAt ?? DateTime.utc(2026, 8, 1),
        isPinned: isPinned,
      ),
      createdAt: createdAt ?? DateTime.utc(2026, 9, 1),
    ),
  );
}

// ---- Echo gateways ----

class FakeEchoGateway implements EchoGateway {
  FakeEchoGateway({CurrentEchoResult? current})
    : current = current ?? makeCurrentDisabled();

  CurrentEchoResult current;
  ApiException? currentError;
  ApiException? feedbackError;
  int getCalls = 0;
  final feedbackCalls = <({String echoId, String verdict})>[];

  @override
  Future<CurrentEchoResult> getCurrent() async {
    getCalls++;
    final error = currentError;
    if (error != null) throw error;
    return current;
  }

  @override
  Future<EchoFeedbackResult> submitFeedback({
    required String echoId,
    required String verdict,
  }) async {
    final error = feedbackError;
    if (error != null) throw error;
    feedbackCalls.add((echoId: echoId, verdict: verdict));
    // 模拟服务端：反馈后该回响关闭，下一次读取落在 off-period。
    current = makeCurrentEmpty(
      nextDueAt: DateTime.utc(2026, 9, 3),
      emptyReason: null,
    );
    return EchoFeedbackResult(
      echoId: echoId,
      status: verdict,
      nextDueAt: DateTime.utc(2026, 9, 3),
    );
  }
}

class FakeEchoSettingsGateway implements EchoSettingsGateway {
  FakeEchoSettingsGateway({EchoSettingsResult? result})
    : result = result ?? makeEchoSettings();

  EchoSettingsResult result;
  ApiException? getError;
  ApiException? updateError;
  int getCalls = 0;
  int? lastExpectedRevision;
  bool? lastEnabled;
  String? lastCadence;

  @override
  Future<EchoSettingsResult> get() async {
    getCalls++;
    final error = getError;
    if (error != null) throw error;
    return result;
  }

  @override
  Future<EchoSettings> update({
    required int expectedRevision,
    bool? enabled,
    String? cadence,
  }) async {
    final error = updateError;
    if (error != null) throw error;
    lastExpectedRevision = expectedRevision;
    lastEnabled = enabled;
    lastCadence = cadence;
    final updated = result.settings.copyWith(
      enabled: enabled,
      cadence: cadence,
      revision: expectedRevision + 1,
    );
    result = EchoSettingsResult(settings: updated);
    return updated;
  }
}

// ---- Native service fakes ----

class FakeLocalNotificationService implements LocalNotificationService {
  int initializeCalls = 0;
  int permissionRequests = 0;
  int cancelAllCalls = 0;
  String? launchPayloadValue;
  final List<({int id, DateTime whenUtc, String payload})> scheduled = [];
  final StreamController<String> _onTap = StreamController<String>.broadcast();

  @override
  Future<void> initialize() async {
    initializeCalls++;
  }

  @override
  Future<void> requestPermission() async {
    permissionRequests++;
  }

  @override
  Future<void> cancelAll() async {
    cancelAllCalls++;
  }

  @override
  Future<void> schedule({
    required int id,
    required String title,
    required String body,
    required DateTime whenUtc,
    required String payload,
  }) async {
    scheduled.add((id: id, whenUtc: whenUtc, payload: payload));
  }

  @override
  Future<String?> launchPayload() async => launchPayloadValue;

  @override
  Stream<String> get onTap => _onTap.stream;

  void pushTap(String payload) {
    _onTap.add(payload);
  }
}

class FakeQuickActionsService implements QuickActionsService {
  int initializeCalls = 0;
  final StreamController<LaunchRequest> _launches =
      StreamController<LaunchRequest>.broadcast();

  @override
  Future<void> initialize() async {
    initializeCalls++;
  }

  @override
  Stream<LaunchRequest> get launches => _launches.stream;

  void push(LaunchRequest request) {
    _launches.add(request);
  }
}

class FakeHomeWidgetService implements HomeWidgetService {
  LaunchRequest? initiallyValue;
  final StreamController<LaunchRequest> _launches =
      StreamController<LaunchRequest>.broadcast();

  @override
  Future<LaunchRequest?> initiallyLaunched() async => initiallyValue;

  @override
  Stream<LaunchRequest> get launches => _launches.stream;

  void push(LaunchRequest request) {
    _launches.add(request);
  }
}

class FakeTimezoneService implements TimezoneService {
  FakeTimezoneService({this.name = 'UTC'});

  String name;

  @override
  Future<String> deviceTimezoneName() async => name;
}

class FakeCaptureFeedbackService implements CaptureFeedbackService {
  int startTappedCalls = 0;
  int savedCalls = 0;

  @override
  void startTapped() {
    startTappedCalls++;
  }

  @override
  void saved() {
    savedCalls++;
  }
}

/// 记录 refresh 调用的排程器替身（免去真实时区/通知插件依赖）。
class FakeEchoScheduler extends EchoScheduler {
  FakeEchoScheduler()
    : super(
        const NoopLocalNotificationService(),
        () async => 'UTC',
        () => DateTime.utc(2026, 9, 1),
      );

  final refreshes = <({bool enabled, String cadence, EchoReminderPrefs prefs})>[
  ];

  @override
  Future<void> refresh({
    required bool enabled,
    required String cadence,
    required EchoReminderPrefs prefs,
  }) async {
    refreshes.add((enabled: enabled, cadence: cadence, prefs: prefs));
  }
}
