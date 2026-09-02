import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:weave_flutter/features/echo/data/echo_api.dart';
import 'package:weave_flutter/features/echo/ui/echo_screen.dart';
import 'package:weave_flutter/shared/auth/auth_state.dart';
import 'package:weave_flutter/shared/models/user.dart';

import '../../support/echo_fakes.dart';

class _AuthFake extends AuthNotifier {
  _AuthFake(this.initial);

  final AuthState initial;

  @override
  AuthState build() => initial;
}

const _authed = Authenticated(token: 't', user: User(id: 'u1'));
const _guest = Unauthenticated();

Widget _buildApp({
  required FakeEchoGateway gateway,
  AuthState auth = _authed,
}) {
  final router = GoRouter(
    initialLocation: '/echo',
    routes: [
      GoRoute(path: '/echo', builder: (_, _) => const EchoScreen()),
      GoRoute(
        path: '/login',
        builder: (_, _) => const Scaffold(body: Center(child: Text('登录页'))),
      ),
      GoRoute(
        path: '/settings/echo',
        builder: (_, _) => const Scaffold(body: Center(child: Text('设置页'))),
      ),
      GoRoute(
        path: '/memories/:captureId',
        builder: (_, _) => const Scaffold(body: Center(child: Text('记忆详情页'))),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      echoGatewayProvider.overrideWithValue(gateway),
      authNotifierProvider.overrideWith(() => _AuthFake(auth)),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

Future<void> _pumpAndLoad(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
  await tester.pump(const Duration(milliseconds: 50));
}

void main() {
  testWidgets('guest sees a login CTA and no request is made', (tester) async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded());
    await tester.pumpWidget(_buildApp(gateway: gateway, auth: _guest));
    await _pumpAndLoad(tester);

    expect(find.text('登录后开启回响'), findsOneWidget);
    expect(find.byKey(const Key('echo_login_button')), findsOneWidget);
    expect(gateway.getCalls, 0);

    await tester.tap(find.byKey(const Key('echo_login_button')));
    await tester.pumpAndSettle();
    expect(find.text('登录页'), findsOneWidget);
  });

  testWidgets('disabled settings show the off state and a way to enable', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(current: makeCurrentDisabled());
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    expect(find.text('回响已关闭'), findsOneWidget);
    expect(find.text('去开启回响'), findsOneWidget);

    await tester.tap(find.text('去开启回响'));
    await tester.pumpAndSettle();
    expect(find.text('设置页'), findsOneWidget);
  });

  testWidgets('no open echo with next_due_at shows the off-period empty state', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(
      current: makeCurrentEmpty(nextDueAt: DateTime.utc(2026, 9, 3)),
    );
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    expect(find.text('下次回响还没到'), findsOneWidget);
    expect(find.textContaining('之后再来'), findsOneWidget);
  });

  testWidgets('no candidates empty state invites capturing first', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(
      current: makeCurrentEmpty(emptyReason: 'no_candidates'),
    );
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    expect(find.text('还没有可回看的记忆'), findsOneWidget);
  });

  testWidgets('loaded echo shows reason, card and the three feedback actions', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(
      current: makeCurrentLoaded(
        captureId: 'c7',
        reasonText: '这条记忆被你置顶过',
        primaryType: 'idea',
      ),
    );
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    expect(find.text('这次回响：这条记忆被你置顶过'), findsOneWidget);
    expect(find.text('闪念标题'), findsOneWidget);
    expect(find.text('一段摘要'), findsOneWidget);
    expect(find.byKey(const Key('echo_memory_card')), findsOneWidget);
    expect(find.byKey(const Key('echo_feedback_done')), findsOneWidget);
    expect(find.byKey(const Key('echo_feedback_later')), findsOneWidget);
    expect(find.byKey(const Key('echo_feedback_not_relevant')), findsOneWidget);
    expect(find.text('有用，回顾了'), findsOneWidget);
    expect(find.text('稍后再说'), findsOneWidget);
    expect(find.text('不太相关'), findsOneWidget);
  });

  testWidgets('tapping the memory card deep-links to its detail', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded(captureId: 'c7'));
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    await tester.tap(find.byKey(const Key('echo_memory_card')));
    await tester.pumpAndSettle();
    expect(find.text('记忆详情页'), findsOneWidget);
  });

  testWidgets('done feedback sends verdict and moves to the empty state', (
    tester,
  ) async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded(echoId: 'e1'));
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    await tester.tap(find.byKey(const Key('echo_feedback_done')));
    await _pumpAndLoad(tester);

    expect(gateway.feedbackCalls, hasLength(1));
    expect(gateway.feedbackCalls.single.verdict, echoVerdictDone);
    expect(find.text('下次回响还没到'), findsOneWidget);
  });

  testWidgets('error state offers retry that reloads', (tester) async {
    // 单一挂载：网关先失败 → 错误视图 + 重试；清错后重试 → 成功加载。
    final switching = _SwitchingGateway(current: makeCurrentLoaded())
      ..error = true;
    await tester.pumpWidget(_buildApp(gateway: switching));
    await _pumpAndLoad(tester);
    expect(find.textContaining('加载失败'), findsOneWidget);

    switching.error = false;
    await tester.tap(find.text('重试'));
    await _pumpAndLoad(tester);
    expect(find.text('闪念标题'), findsOneWidget);
  });

  testWidgets('authenticated app bar shows settings entry', (tester) async {
    final gateway = FakeEchoGateway(current: makeCurrentLoaded());
    await tester.pumpWidget(_buildApp(gateway: gateway));
    await _pumpAndLoad(tester);

    await tester.tap(find.byTooltip('回响设置'));
    await tester.pumpAndSettle();
    expect(find.text('设置页'), findsOneWidget);
  });
}

/// 可在测试中途切换错误/结果的网关（模拟先失败、后重试成功）。
class _SwitchingGateway extends FakeEchoGateway {
  _SwitchingGateway({required super.current});

  bool error = false;

  @override
  Future<CurrentEchoResult> getCurrent() async {
    if (error) throw Exception('boom');
    return current;
  }
}
