import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/echo/domain/echo_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';
import 'package:weave_flutter/shared/auth/auth_state.dart';
import 'package:weave_flutter/shared/models/user.dart';

import '../../support/echo_fakes.dart';

/// 可指定初始鉴权态的 Notifier，供 notifier 单测短路鉴权读取。
/// 继承 AuthNotifier 以匹配 overrideWith 的工厂类型约束。
class _AuthFake extends AuthNotifier {
  _AuthFake(this.initial);

  final AuthState initial;

  @override
  AuthState build() => initial;
}

const _authed = Authenticated(
  token: 't',
  user: User(id: 'u1', displayName: '测试'),
);

ProviderContainer container({
  EchoGateway? gateway,
  AuthState auth = _authed,
}) {
  final c = ProviderContainer(
    overrides: [
      if (gateway != null) echoGatewayProvider.overrideWithValue(gateway),
      authNotifierProvider.overrideWith(() => _AuthFake(auth)),
    ],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  test('initial state is loading before load', () {
    final c = container();
    expect(c.read(echoNotifierProvider), isA<EchoLoading>());
  });

  test('guest short-circuits to guest view without a request', () async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded());
    final c = container(gateway: gateway, auth: const Unauthenticated());
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    expect(c.read(echoNotifierProvider), isA<EchoGuest>());
    expect(gateway.getCalls, 0);
  });

  test('disabled settings publish disabled state', () async {
    final c = container(gateway: FakeEchoGateway(current: makeCurrentDisabled()));
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(echoNotifierProvider) as EchoDisabled;
    expect(state.cadence, echoCadenceDaily);
    expect(state.revision, 1);
  });

  test('no open echo publishes empty state', () async {
    final gateway = FakeEchoGateway(
      current: makeCurrentEmpty(
        emptyReason: 'no_candidates',
        nextDueAt: null,
      ),
    );
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(echoNotifierProvider) as EchoEmpty;
    expect(state.noCandidates, isTrue);
  });

  test('open echo publishes loaded card', () async {
    final gateway = FakeEchoGateway(
      current: makeCurrentLoaded(
        echoId: 'e1',
        captureId: 'c7',
        reasonCode: 'pinned',
        reasonText: '这条记忆被你置顶过',
      ),
    );
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(echoNotifierProvider) as EchoLoaded;
    expect(state.card.id, 'e1');
    expect(state.card.memory.captureId, 'c7');
    expect(state.card.reason.code, 'pinned');
    expect(state.feedbackSaving, isFalse);
  });

  test('401 from the gateway maps to guest view', () async {
    final gateway = FakeEchoGateway()
      ..currentError = ApiException(statusCode: 401, message: 'unauthorized');
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    expect(c.read(echoNotifierProvider), isA<EchoGuest>());
  });

  test('other failures publish error state with retryable message', () async {
    final gateway = FakeEchoGateway()
      ..currentError = ApiException(statusCode: 500, message: 'boom');
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(echoNotifierProvider) as EchoError;
    expect(state.message, contains('加载失败'));
  });

  test('feedback sends verdict then reloads into the empty state', () async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded(echoId: 'e1'));
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);
    await notifier.load();
    expect(c.read(echoNotifierProvider), isA<EchoLoaded>());

    await notifier.feedback(echoVerdictDone);

    expect(gateway.feedbackCalls, hasLength(1));
    expect(gateway.feedbackCalls.single.echoId, 'e1');
    expect(gateway.feedbackCalls.single.verdict, echoVerdictDone);
    expect(c.read(echoNotifierProvider), isA<EchoEmpty>());
  });

  test('feedback 409 surfaces a cross-device message on the loaded card', () async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded(echoId: 'e1'))
      ..feedbackError = ApiException(statusCode: 409, message: 'HTTP 409');
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);
    await notifier.load();

    await notifier.feedback(echoVerdictLater);

    final state = c.read(echoNotifierProvider) as EchoLoaded;
    expect(state.message, '这条回响已在其他设备处理');
    expect(state.feedbackSaving, isFalse);
  });

  test('clearMessage clears the message after the SnackBar is consumed', () async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded())
      ..feedbackError = ApiException(statusCode: 409, message: 'HTTP 409');
    final c = container(gateway: gateway);
    final notifier = c.read(echoNotifierProvider.notifier);
    await notifier.load();
    await notifier.feedback(echoVerdictDone);
    expect((c.read(echoNotifierProvider) as EchoLoaded).message, isNotNull);

    notifier.clearMessage();

    expect((c.read(echoNotifierProvider) as EchoLoaded).message, isNull);
  });
}
