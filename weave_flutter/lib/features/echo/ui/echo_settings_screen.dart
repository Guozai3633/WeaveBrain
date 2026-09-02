import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/native/native_services.dart';
import '../data/echo_api.dart';
import '../domain/echo_reminder_prefs.dart';
import '../domain/echo_scheduler.dart';
import '../domain/echo_settings_notifier.dart';

/// 回响与提醒设置：服务端 enabled+cadence（revision CAS）+ 本地投递时刻 /
/// 静默时段。本地投递字段只用于本地通知排程，不上送服务端。
class EchoSettingsScreen extends ConsumerStatefulWidget {
  const EchoSettingsScreen({super.key});

  @override
  ConsumerState<EchoSettingsScreen> createState() => _EchoSettingsScreenState();
}

class _EchoSettingsScreenState extends ConsumerState<EchoSettingsScreen> {
  EchoReminderPrefs _prefs = const EchoReminderPrefs();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(echoSettingsNotifierProvider.notifier).load();
      _loadPrefs();
    });
  }

  Future<void> _loadPrefs() async {
    final prefs = await EchoReminderPrefs.load();
    if (mounted) setState(() => _prefs = prefs);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(echoSettingsNotifierProvider);

    ref.listen<EchoSettingsState>(echoSettingsNotifierProvider, (previous, next) {
      if (next is EchoSettingsLoaded && next.message != null) {
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(SnackBar(content: Text(next.message!)));
        ref.read(echoSettingsNotifierProvider.notifier).clearMessage();
      }
    });

    return Scaffold(
      appBar: AppBar(title: const Text('回响与提醒')),
      body: switch (state) {
        EchoSettingsLoading() =>
          const Center(child: CircularProgressIndicator()),
        EchoSettingsError() => _ErrorView(
          message: state.message,
          onRetry: () => ref.read(echoSettingsNotifierProvider.notifier).load(),
        ),
        EchoSettingsLoaded() => _SettingsBody(
          state: state,
          prefs: _prefs,
          onPrefsChanged: _savePrefsAndReschedule,
        ),
      },
    );
  }

  Future<void> _savePrefsAndReschedule(EchoReminderPrefs prefs) async {
    setState(() => _prefs = prefs);
    await prefs.save();
    final current = ref.read(echoSettingsNotifierProvider);
    if (current is! EchoSettingsLoaded) return;
    try {
      await ref
          .read(echoSchedulerProvider)
          .refresh(
            enabled: current.result.settings.enabled,
            cadence: current.result.settings.cadence,
            prefs: prefs,
          );
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('本地提醒重排失败，可稍后重试')),
        );
      }
    }
  }
}

class _SettingsBody extends ConsumerWidget {
  const _SettingsBody({
    required this.state,
    required this.prefs,
    required this.onPrefsChanged,
  });

  final EchoSettingsLoaded state;
  final EchoReminderPrefs prefs;
  final ValueChanged<EchoReminderPrefs> onPrefsChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = state.result.settings;
    final notifier = ref.read(echoSettingsNotifierProvider.notifier);
    final busy = state.saving;
    final deliveryInSilent = prefs.isInSilentWindow(prefs.deliveryMinutesOfDay);

    return ListView(
      children: [
        SwitchListTile(
          key: const Key('echo_enabled_switch'),
          title: const Text('开启回响'),
          subtitle: const Text('隔一段时间挑一条旧记忆回来，提醒你重新看看'),
          value: settings.enabled,
          onChanged: busy
              ? null
              : (value) async {
                  if (value) {
                    unawaited(
                      ref.read(localNotificationServiceProvider).requestPermission(),
                    );
                  }
                  await notifier.setEnabled(value);
                },
        ),
        const Divider(),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('节奏', style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                key: const Key('echo_cadence_segment'),
                segments: const [
                  ButtonSegment(value: echoCadenceDaily, label: Text('每天')),
                  ButtonSegment(
                    value: echoCadenceEveryOtherDay,
                    label: Text('隔天'),
                  ),
                  ButtonSegment(value: echoCadenceWeekly, label: Text('每周')),
                ],
                selected: {settings.cadence},
                onSelectionChanged: busy || !settings.enabled
                    ? null
                    : (selection) =>
                          notifier.setCadence(selection.first),
              ),
            ],
          ),
        ),
        const Divider(),
        SwitchListTile(
          key: const Key('echo_silent_switch'),
          title: const Text('静默时段'),
          subtitle: const Text('落在该时段的提醒会被跳过，不会打扰你'),
          value: prefs.silentEnabled,
          onChanged: (value) {
            onPrefsChanged(
              prefs.copyWith(
                silentEnabled: value,
                silentStartMinutes: prefs.silentStartMinutes,
                silentEndMinutes: prefs.silentEndMinutes,
              ),
            );
          },
        ),
        ListTile(
          key: const Key('echo_delivery_time_tile'),
          title: const Text('提醒时间'),
          trailing: Text(_formatMinutes(prefs.deliveryMinutesOfDay)),
          onTap: () async {
            final picked = await _pickTime(context, prefs.deliveryMinutesOfDay);
            if (picked != null) {
              onPrefsChanged(
                prefs.copyWith(
                  deliveryHour: picked.hour,
                  deliveryMinute: picked.minute,
                ),
              );
            }
          },
        ),
        if (prefs.silentEnabled) ...[
          ListTile(
            title: const Text('静默开始'),
            trailing: Text(_formatMinutes(prefs.silentStartMinutes)),
            onTap: () async {
              final picked = await _pickTime(
                context,
                prefs.silentStartMinutes,
              );
              if (picked != null) {
                onPrefsChanged(
                  prefs.copyWith(
                    silentStartMinutes: picked.hour * 60 + picked.minute,
                  ),
                );
              }
            },
          ),
          ListTile(
            title: const Text('静默结束'),
            trailing: Text(_formatMinutes(prefs.silentEndMinutes)),
            onTap: () async {
              final picked = await _pickTime(context, prefs.silentEndMinutes);
              if (picked != null) {
                onPrefsChanged(
                  prefs.copyWith(
                    silentEndMinutes: picked.hour * 60 + picked.minute,
                  ),
                );
              }
            },
          ),
        ],
        if (deliveryInSilent)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: Text(
              '提醒时间落在静默时段内，回响提醒会被跳过。',
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        const Divider(),
        ListTile(
          title: const Text('版本'),
          trailing: Text('revision ${settings.revision}'),
        ),
      ],
    );
  }
}

Future<TimeOfDay?> _pickTime(BuildContext context, int minutesOfDay) {
  return showTimePicker(
    context: context,
    initialTime: TimeOfDay(hour: minutesOfDay ~/ 60, minute: minutesOfDay % 60),
  );
}

String _formatMinutes(int minutesOfDay) {
  final h = (minutesOfDay ~/ 60).toString().padLeft(2, '0');
  final m = (minutesOfDay % 60).toString().padLeft(2, '0');
  return '$h:$m';
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48),
            const SizedBox(height: 16),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
