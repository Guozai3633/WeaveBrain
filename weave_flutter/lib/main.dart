import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'features/capture/domain/capture_providers.dart';
import 'shared/auth/auth_state.dart';

void main() {
  runApp(const ProviderScope(child: WeaveBrainApp()));
}

class WeaveBrainApp extends ConsumerStatefulWidget {
  const WeaveBrainApp({super.key});

  @override
  ConsumerState<WeaveBrainApp> createState() => _WeaveBrainAppState();
}

class _WeaveBrainAppState extends ConsumerState<WeaveBrainApp>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    // Check auth state on startup
    WidgetsBinding.instance.addObserver(this);
    Future.microtask(() {
      ref.read(authNotifierProvider.notifier).checkAuth();
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
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

  @override
  Widget build(BuildContext context) {
    ref.watch(captureSyncBootstrapProvider);
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
