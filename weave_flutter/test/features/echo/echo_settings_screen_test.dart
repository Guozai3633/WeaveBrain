import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/echo/domain/echo_scheduler.dart';
import 'package:weave_flutter/features/echo/ui/echo_settings_screen.dart';
import 'package:weave_flutter/shared/native/native_services.dart';

import '../../support/echo_fakes.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  Widget buildScreen({
    required FakeEchoSettingsGateway gateway,
    FakeEchoScheduler? scheduler,
    FakeLocalNotificationService? notifications,
    Map<String, Object> prefsValues = const {},
  }) {
    SharedPreferences.setMockInitialValues(prefsValues);
    return ProviderScope(
      overrides: [
        echoSettingsGatewayProvider.overrideWithValue(gateway),
        echoSchedulerProvider.overrideWithValue(scheduler ?? FakeEchoScheduler()),
        localNotificationServiceProvider.overrideWithValue(
          notifications ?? FakeLocalNotificationService(),
        ),
      ],
      child: const MaterialApp(home: EchoSettingsScreen()),
    );
  }

  Future<void> pumpAndSettleSettings(WidgetTester tester) async {
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    await tester.pump(const Duration(milliseconds: 50));
  }

  SwitchListTile enabledSwitch(WidgetTester tester) {
    return tester.widget<SwitchListTile>(
      find.byKey(const Key('echo_enabled_switch')),
    );
  }

  testWidgets('renders enabled switch, cadence options and revision', (
    tester,
  ) async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: true, cadence: echoCadenceDaily, revision: 3),
    );
    await tester.pumpWidget(buildScreen(gateway: gateway));
    await pumpAndSettleSettings(tester);

    expect(enabledSwitch(tester).value, isTrue);
    expect(find.text('每天'), findsOneWidget);
    expect(find.text('隔天'), findsOneWidget);
    expect(find.text('每周'), findsOneWidget);
    expect(find.text('revision 3'), findsOneWidget);
  });

  testWidgets('turning on requests notification permission and saves', (
    tester,
  ) async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: false, cadence: echoCadenceDaily, revision: 1),
    );
    final scheduler = FakeEchoScheduler();
    final notifications = FakeLocalNotificationService();
    await tester.pumpWidget(
      buildScreen(
        gateway: gateway,
        scheduler: scheduler,
        notifications: notifications,
      ),
    );
    await pumpAndSettleSettings(tester);
    expect(enabledSwitch(tester).value, isFalse);

    await tester.tap(find.byKey(const Key('echo_enabled_switch')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(notifications.permissionRequests, 1);
    expect(gateway.lastEnabled, isTrue);
    expect(enabledSwitch(tester).value, isTrue);
    expect(gateway.lastExpectedRevision, 1);
    expect(scheduler.refreshes, hasLength(1));
    expect(scheduler.refreshes.single.enabled, isTrue);
  });

  testWidgets('selecting a cadence saves only the cadence field', (
    tester,
  ) async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: true, cadence: echoCadenceDaily, revision: 1),
    );
    await tester.pumpWidget(buildScreen(gateway: gateway));
    await pumpAndSettleSettings(tester);

    await tester.tap(find.text('每周'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(gateway.lastCadence, echoCadenceWeekly);
    expect(gateway.lastEnabled, isNull);
  });

  testWidgets('shows a conflict warning when delivery falls in the silent window', (
    tester,
  ) async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: true, cadence: echoCadenceDaily, revision: 1),
    );
    // 投递 23:00，静默 22:00–08:00（跨午夜）→ 冲突。
    await tester.pumpWidget(
      buildScreen(
        gateway: gateway,
        prefsValues: {
          'echo_reminder_delivery_hour': 23,
          'echo_reminder_silent_enabled': true,
        },
      ),
    );
    await pumpAndSettleSettings(tester);

    expect(find.textContaining('落在静默时段内'), findsOneWidget);
  });
}
