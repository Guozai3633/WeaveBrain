import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/ai_settings_notifier.dart';

class AISettingsScreen extends ConsumerStatefulWidget {
  const AISettingsScreen({super.key});

  @override
  ConsumerState<AISettingsScreen> createState() => _AISettingsScreenState();
}

class _AISettingsScreenState extends ConsumerState<AISettingsScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(aiSettingsNotifierProvider.notifier).load();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(aiSettingsNotifierProvider);
    ref.listen<AISettingsState>(aiSettingsNotifierProvider, (previous, next) {
      if (next is AISettingsLoaded) {
        final message = next.message;
        if (message != null) {
          ScaffoldMessenger.of(context)
            ..hideCurrentSnackBar()
            ..showSnackBar(SnackBar(content: Text(message)));
          ref.read(aiSettingsNotifierProvider.notifier).clearMessage();
        }
      }
    });

    return Scaffold(
      appBar: AppBar(title: const Text('AI 与自动化')),
      body: switch (state) {
        AISettingsLoading() => const Center(child: CircularProgressIndicator()),
        AISettingsError() => _ErrorView(
          message: state.message,
          onRetry: () => ref.read(aiSettingsNotifierProvider.notifier).load(),
        ),
        AISettingsLoaded() => _SettingsBody(state: state),
      },
    );
  }
}

class _SettingsBody extends ConsumerWidget {
  const _SettingsBody({required this.state});

  final AISettingsLoaded state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = state.result.settings;
    final aiEnabled = settings.aiMemoryEnabled;
    final pending = state.result.pendingReorganize;
    final notifier = ref.read(aiSettingsNotifierProvider.notifier);
    final busy = state.saving;

    return ListView(
      children: [
        _SettingsSwitchTile(
          title: 'AI 记忆整理',
          subtitle: '默认关闭，开启后新记录会由 AI 整理',
          value: settings.aiMemoryEnabled,
          onChanged: busy
              ? null
              : (value) => notifier.setSwitch(AISettingsField.aiMemory, value),
        ),
        const Divider(),
        _SettingsSwitchTile(
          title: '语音转写',
          subtitle: '关闭整理仍可转写语音',
          value: settings.speechToTextEnabled,
          onChanged: busy
              ? null
              : (value) =>
                    notifier.setSwitch(AISettingsField.speechToText, value),
        ),
        const Divider(),
        _SettingsSwitchTile(
          title: 'AI 补全',
          subtitle: '整理开启时可用',
          value: settings.aiCompletionEnabled,
          onChanged: busy || !aiEnabled
              ? null
              : (value) =>
                    notifier.setSwitch(AISettingsField.aiCompletion, value),
        ),
        const Divider(),
        _SettingsSwitchTile(
          title: '云端文本处理',
          subtitle: '整理开启时可用',
          value: settings.cloudTextAllowed,
          onChanged: busy || !aiEnabled
              ? null
              : (value) => notifier.setSwitch(AISettingsField.cloudText, value),
        ),
        const Divider(),
        _SettingsSwitchTile(
          title: '云端音频处理',
          subtitle: '整理开启时可用',
          value: settings.cloudAudioAllowed,
          onChanged: busy || !aiEnabled
              ? null
              : (value) =>
                    notifier.setSwitch(AISettingsField.cloudAudio, value),
        ),
        const Divider(),
        if (aiEnabled && pending > 0)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
            child: FilledButton.icon(
              onPressed: busy
                  ? null
                  : () => ref.read(aiSettingsNotifierProvider.notifier).reorganize(),
              icon: busy
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.replay),
              label: Text('补整理未处理记忆 ($pending 条)'),
            ),
          ),
        ListTile(
          title: const Text('版本'),
          trailing: Text('revision ${settings.revision}'),
        ),
      ],
    );
  }
}

class _SettingsSwitchTile extends StatelessWidget {
  const _SettingsSwitchTile({
    required this.title,
    required this.subtitle,
    required this.value,
    required this.onChanged,
  });

  final String title;
  final String subtitle;
  final bool value;
  final ValueChanged<bool>? onChanged;

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      title: Text(title),
      subtitle: Text(subtitle),
      value: value,
      onChanged: onChanged,
    );
  }
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
