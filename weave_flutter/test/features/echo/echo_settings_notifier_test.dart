import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/echo/domain/echo_scheduler.dart';
import 'package:weave_flutter/features/echo/domain/echo_settings_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

import '../../support/echo_fakes.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  ProviderContainer container({
    FakeEchoSettingsGateway? gateway,
    FakeEchoScheduler? scheduler,
  }) {
    final c = ProviderContainer(
      overrides: [
        echoSettingsGatewayProvider.overrideWithValue(
          gateway ??
              FakeEchoSettingsGateway(result: makeEchoSettings(revision: 2)),
        ),
        echoSchedulerProvider.overrideWithValue(scheduler ?? FakeEchoScheduler()),
      ],
    );
    addTearDown(c.dispose);
    return c;
  }

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('load publishes loaded state', () async {
    final c = container();
    final notifier = c.read(echoSettingsNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(echoSettingsNotifierProvider) as EchoSettingsLoaded;
    expect(state.result.settings.enabled, isFalse);
    expect(state.result.settings.revision, 2);
  });

  test('load failure publishes error state', () async {
    final gateway = FakeEchoSettingsGateway()..getError = ApiException(message: 'boom');
    final c = container(gateway: gateway);
    final notifier = c.read(echoSettingsNotifierProvider.notifier);

    await notifier.load();

    expect(c.read(echoSettingsNotifierProvider), isA<EchoSettingsError>());
  });

  test('setEnabled sends revision and refreshes the local scheduler', () async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: false, cadence: echoCadenceDaily, revision: 2),
    );
    final scheduler = FakeEchoScheduler();
    final c = container(gateway: gateway, scheduler: scheduler);
    final notifier = c.read(echoSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.setEnabled(true);

    expect(gateway.lastExpectedRevision, 2);
    expect(gateway.lastEnabled, isTrue);
    expect(gateway.lastCadence, isNull);
    final loaded = c.read(echoSettingsNotifierProvider) as EchoSettingsLoaded;
    expect(loaded.result.settings.enabled, isTrue);
    expect(loaded.result.settings.revision, 3);
    expect(loaded.message, '已保存');
    expect(scheduler.refreshes, hasLength(1));
    expect(scheduler.refreshes.single.enabled, isTrue);
    expect(scheduler.refreshes.single.cadence, echoCadenceDaily);
  });

  test('setCadence sends only the cadence field and refreshes', () async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: true, cadence: echoCadenceDaily, revision: 1),
    );
    final scheduler = FakeEchoScheduler();
    final c = container(gateway: gateway, scheduler: scheduler);
    final notifier = c.read(echoSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.setCadence(echoCadenceWeekly);

    expect(gateway.lastExpectedRevision, 1);
    expect(gateway.lastEnabled, isNull);
    expect(gateway.lastCadence, echoCadenceWeekly);
    expect(scheduler.refreshes.single.cadence, echoCadenceWeekly);
  });

  test('update 409 surfaces a cross-device conflict message', () async {
    final gateway = FakeEchoSettingsGateway(
      result: makeEchoSettings(enabled: false, revision: 2),
    )..updateError = ApiException(statusCode: 409, message: 'HTTP 409');
    final scheduler = FakeEchoScheduler();
    final c = container(gateway: gateway, scheduler: scheduler);
    final notifier = c.read(echoSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.setEnabled(true);

    final loaded = c.read(echoSettingsNotifierProvider) as EchoSettingsLoaded;
    expect(loaded.message, '设置已在其他设备修改，请刷新');
    expect(loaded.saving, isFalse);
    expect(scheduler.refreshes, isEmpty); // 失败不触发本地重排。
  });

  test('clearMessage clears the last operation message', () async {
    final c = container();
    final notifier = c.read(echoSettingsNotifierProvider.notifier);
    await notifier.load();
    await notifier.setEnabled(true);
    expect((c.read(echoSettingsNotifierProvider) as EchoSettingsLoaded).message, '已保存');

    notifier.clearMessage();

    expect(
      (c.read(echoSettingsNotifierProvider) as EchoSettingsLoaded).message,
      isNull,
    );
  });
}
