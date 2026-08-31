import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../shared/auth/auth_state.dart';
import '../domain/capture_providers.dart';
import '../domain/local_capture.dart';

class CaptureScreen extends ConsumerStatefulWidget {
  const CaptureScreen({super.key});

  @override
  ConsumerState<CaptureScreen> createState() => _CaptureScreenState();
}

class _CaptureScreenState extends ConsumerState<CaptureScreen> {
  final _textController = TextEditingController();
  final _focusNode = FocusNode();

  @override
  void dispose() {
    _textController.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final capture = await ref
        .read(captureComposerControllerProvider.notifier)
        .saveText(_textController.text);
    if (capture != null && mounted) {
      _textController.clear();
      _focusNode.requestFocus();
    }
  }

  Future<void> _retry() async {
    final summary = await ref
        .read(captureComposerControllerProvider.notifier)
        .retryAll();
    if (!mounted) return;
    final message = summary.skippedBecauseGuest
        ? '已保存在本机，登录后自动同步'
        : '同步完成 ${summary.synced} 条，失败 ${summary.failed} 条，拒绝 ${summary.rejected} 条';
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  void _openRecording() {
    if (kIsWeb) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('当前浏览器不支持录音落盘，请使用 App 录音，或先用文字速记。'),
        ),
      );
      return;
    }
    context.push('/record');
  }

  @override
  Widget build(BuildContext context) {
    final composer = ref.watch(captureComposerControllerProvider);
    final captures = ref.watch(localCapturesProvider);
    final authState = ref.watch(authNotifierProvider);
    final isGuest = authState is! Authenticated;

    return Scaffold(
      appBar: AppBar(
        title: const Text('快速记录'),
        actions: [
          IconButton(
            tooltip: '重试同步',
            onPressed: _retry,
            icon: const Icon(Icons.sync),
          ),
          if (isGuest)
            TextButton(
              onPressed: () => context.push('/login'),
              child: const Text('登录同步'),
            ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  TextField(
                    key: const Key('capture_text_input'),
                    controller: _textController,
                    focusNode: _focusNode,
                    autofocus: true,
                    minLines: 3,
                    maxLines: 7,
                    textInputAction: TextInputAction.newline,
                    decoration: InputDecoration(
                      hintText: '现在想到什么？先记下来，稍后再整理。',
                      border: const OutlineInputBorder(),
                      helperText: isGuest ? '游客模式：内容只保存在本机' : '先保存到本机，再自动同步',
                    ),
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(
                        child: FilledButton.icon(
                          key: const Key('capture_save_button'),
                          onPressed:
                              composer.status == CaptureComposerStatus.saving
                              ? null
                              : _save,
                          icon: composer.status == CaptureComposerStatus.saving
                              ? const SizedBox.square(
                                  dimension: 18,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : const Icon(Icons.save_outlined),
                          label: const Text('安全保存'),
                        ),
                      ),
                      const SizedBox(width: 10),
                      IconButton.filledTonal(
                        key: const Key('capture_record_button'),
                        tooltip: '语音记录',
                        onPressed: _openRecording,
                        icon: const Icon(Icons.mic),
                      ),
                    ],
                  ),
                  if (composer.message != null)
                    Padding(
                      key: const Key('capture_saved_message'),
                      padding: const EdgeInsets.only(top: 8),
                      child: Row(
                        children: [
                          Icon(
                            composer.status == CaptureComposerStatus.error
                                ? Icons.error_outline
                                : Icons.check_circle_outline,
                            size: 18,
                            color:
                                composer.status == CaptureComposerStatus.error
                                ? Theme.of(context).colorScheme.error
                                : Colors.green,
                          ),
                          const SizedBox(width: 6),
                          Expanded(child: Text(composer.message!)),
                        ],
                      ),
                    ),
                ],
              ),
            ),
            const Divider(height: 1),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
              child: Row(
                children: [
                  Text('本地记忆', style: Theme.of(context).textTheme.titleMedium),
                  const Spacer(),
                  captures.when(
                    data: (items) => Text('${items.length} 条'),
                    loading: () => const SizedBox.shrink(),
                    error: (_, _) => const Text('读取失败'),
                  ),
                ],
              ),
            ),
            Expanded(
              child: captures.when(
                data: (items) {
                  if (items.isEmpty) {
                    return const Center(child: Text('还没有记录。\n第一条会先安全保存在本机。'));
                  }
                  return ListView.separated(
                    key: const Key('local_capture_list'),
                    padding: const EdgeInsets.fromLTRB(12, 0, 12, 24),
                    itemCount: items.length,
                    separatorBuilder: (_, _) => const SizedBox(height: 6),
                    itemBuilder: (context, index) =>
                        _LocalCaptureTile(capture: items[index]),
                  );
                },
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => Center(child: Text('本地记忆读取失败：$error')),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _LocalCaptureTile extends StatelessWidget {
  const _LocalCaptureTile({required this.capture});

  final LocalCapture capture;

  @override
  Widget build(BuildContext context) {
    final titleText = capture.isAudio ? _audioTitle(capture) : capture.text.trim();

    final subtitleParts = <String>[
      DateFormat('MM-dd HH:mm').format(capture.capturedAt.toLocal()),
    ];
    final durationMs = capture.audioDurationMs;
    if (capture.isAudio && durationMs != null) {
      subtitleParts.add(_formatDuration(durationMs));
    }

    return Card(
      child: ListTile(
        leading: capture.isAudio
            ? const Icon(Icons.graphic_eq, color: Color(0xFFF59E0B))
            : null,
        title: Text(
          titleText,
          maxLines: 3,
          overflow: TextOverflow.ellipsis,
        ),
        subtitle: Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Text(subtitleParts.join(' · ')),
        ),
        trailing: _SyncStateChip(state: capture.syncState),
        onTap: capture.isAudio
            ? () => context.push('/captures/${capture.id}/transcript')
            : null,
      ),
    );
  }

  String _audioTitle(LocalCapture capture) {
    final transcript = capture.transcript;
    if (transcript != null && transcript.trim().isNotEmpty) {
      return transcript;
    }
    return '语音记录';
  }

  String _formatDuration(int milliseconds) {
    final seconds = (milliseconds / 1000).round();
    final m = (seconds ~/ 60).toString().padLeft(2, '0');
    final s = (seconds % 60).toString().padLeft(2, '0');
    return '$m:$s';
  }
}

class _SyncStateChip extends StatelessWidget {
  const _SyncStateChip({required this.state});

  final LocalSyncState state;

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (state) {
      LocalSyncState.savedLocal => ('本地已保存', Colors.blueGrey),
      LocalSyncState.pendingSync => ('待同步', Colors.orange),
      LocalSyncState.syncing => ('同步中', Colors.blue),
      LocalSyncState.synced => ('已同步', Colors.green),
      LocalSyncState.retryableError => ('等待重试', Colors.deepOrange),
      LocalSyncState.conflict => ('需处理', Colors.red),
      LocalSyncState.rejected => ('无法同步', Colors.red),
    };

    return Tooltip(
      message: label,
      child: Icon(Icons.circle, size: 12, color: color),
    );
  }
}
