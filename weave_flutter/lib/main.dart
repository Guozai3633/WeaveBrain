import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'features/capture/domain/capture_providers.dart';
import 'features/echo/data/echo_api.dart';
import 'features/echo/domain/echo_reminder_prefs.dart';
import 'features/echo/domain/echo_scheduler.dart';
import 'shared/auth/auth_state.dart';
import 'shared/native/launch_request.dart';
import 'shared/native/native_services.dart';

void main() {
  runApp(const ProviderScope(child: WeaveBrainApp()));
}

/// 待处理的系统入口请求（App Shortcut / 桌面小组件 / 通知点击）。设置后由
/// [_WeaveBrainAppState] 在鉴权就绪时导航到目标路由并清空。
final launchRequestProvider = StateProvider<LaunchRequest?>((ref) => null);

class WeaveBrainApp extends ConsumerStatefulWidget {
  const WeaveBrainApp({super.key});

  @override
  ConsumerState<WeaveBrainApp> createState() => _WeaveBrainAppState();
}

class _WeaveBrainAppState extends ConsumerState<WeaveBrainApp>
    with WidgetsBindingObserver {
  final List<StreamSubscription<dynamic>> _launchSubscriptions = [];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    Future.microtask(() {
      ref.read(authNotifierProvider.notifier).checkAuth();
    });
    unawaited(_initLaunchListeners());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    for (final subscription in _launchSubscriptions) {
      unawaited(subscription.cancel());
    }
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) return;
    unawaited(
      ref
          .read(captureSyncServiceProvider.future)
          .then((service) => service.syncPending(includeDeferred: true)),
    );
  }

  /// 接线三路系统入口：通知点击 / App Shortcut / 桌面小组件。热启动走流，
  /// 冷启动读各自的 initial getter（quick actions 冷启动经 initialize 回调
  /// 落入流，因此需要先订阅再初始化）。
  Future<void> _initLaunchListeners() async {
    final notifications = ref.read(localNotificationServiceProvider);
    final quickActions = ref.read(quickActionsServiceProvider);
    final homeWidget = ref.read(homeWidgetServiceProvider);

    _launchSubscriptions.add(
      quickActions.launches.listen(_enqueueLaunch),
    );
    _launchSubscriptions.add(homeWidget.launches.listen(_enqueueLaunch));
    _launchSubscriptions.add(
      notifications.onTap
          .map((_) => const LaunchRequest(LaunchSource.notification))
          .listen(_enqueueLaunch),
    );

    try {
      await notifications.initialize();
    } catch (_) {
      // 插件在部分宿主缺失时静默降级。
    }
    try {
      await quickActions.initialize();
    } catch (_) {
      // 同上。
    }

    String? payload;
    try {
      payload = await notifications.launchPayload();
    } catch (_) {
      // 忽略读取失败。
    }
    if (payload != null && payload.isNotEmpty) {
      _enqueueLaunch(const LaunchRequest(LaunchSource.notification));
      return;
    }
    LaunchRequest? cold;
    try {
      cold = await homeWidget.initiallyLaunched();
    } catch (_) {
      // 忽略。
    }
    if (cold != null) _enqueueLaunch(cold);
  }

  void _enqueueLaunch(LaunchRequest request) {
    ref.read(launchRequestProvider.notifier).state = request;
  }

  /// 把系统入口请求转成目标路由。通知点击无推送设施：先拉 `/echoes/current`
  /// 用服务端真值还原目标记忆（游客或已跨设备消费则落回响 tab）。
  Future<void> _applyLaunch(LaunchRequest request) async {
    final auth = ref.read(authNotifierProvider);
    if (auth is AuthInitial) return; // 等待鉴权解析后由 auth 监听重试。
    final router = ref.read(routerProvider);

    if (request.source == LaunchSource.notification) {
      if (auth is Authenticated) {
        try {
          final gateway = ref.read(echoGatewayProvider);
          final current = await gateway.getCurrent();
          final captureId = current.echo?.memory.captureId;
          if (captureId != null && captureId.isNotEmpty) {
            router.go('/memories/$captureId');
            ref.read(launchRequestProvider.notifier).state = null;
            return;
          }
        } catch (_) {
          // 拉取失败时落到 /echo 展示错误与重试。
        }
      }
      router.go('/echo');
      ref.read(launchRequestProvider.notifier).state = null;
      return;
    }

    // App Shortcut / 桌面小组件 → 极简录音（自动开录）。
    router.go('/record?auto=1');
    ref.read(launchRequestProvider.notifier).state = null;
  }

  /// 回响本地通知重排：登录后按服务端 enabled+cadence + 本地投递时刻刷新。
  Future<void> _refreshEchoSchedule() async {
    try {
      final gateway = ref.read(echoSettingsGatewayProvider);
      final result = await gateway.get();
      final prefs = await EchoReminderPrefs.load();
      await ref
          .read(echoSchedulerProvider)
          .refresh(
            enabled: result.settings.enabled,
            cadence: result.settings.cadence,
            prefs: prefs,
          );
    } catch (_) {
      // 本地提醒是尽力而为；不影响使用。
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(captureSyncBootstrapProvider);
    ref.listen<AuthState>(authNotifierProvider, (previous, next) {
      if (previous is! Authenticated && next is Authenticated) {
        unawaited(_refreshEchoSchedule());
      }
      if (previous is AuthInitial && next is! AuthInitial) {
        final pending = ref.read(launchRequestProvider);
        if (pending != null) unawaited(_applyLaunch(pending));
      }
    });
    ref.listen<LaunchRequest?>(launchRequestProvider, (previous, next) {
      if (next != null) unawaited(_applyLaunch(next));
    });

    final router = ref.watch(routerProvider);

    return MaterialApp.router(
      title: '织脑 WeaveBrain',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: Colors.blueGrey,
          brightness: Brightness.light,
        ),
        useMaterial3: true,
      ),
      darkTheme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: Colors.blueGrey,
          brightness: Brightness.dark,
        ),
        useMaterial3: true,
      ),
      routerConfig: router,
    );
  }
}
