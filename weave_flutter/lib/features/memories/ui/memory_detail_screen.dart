import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../data/memory_api.dart';
import '../domain/memory_notifier.dart';
import 'memory_list_screen.dart' show kPrimaryTypeLabels;

/// 记忆详情：按 captureId 加载，支持修正 / 续写 / 置顶 / 归档 / 删除，
/// 展示修订历史与音频 / 转写入口。
class MemoryDetailScreen extends ConsumerStatefulWidget {
  const MemoryDetailScreen({super.key, required this.captureId});

  final String captureId;

  @override
  ConsumerState<MemoryDetailScreen> createState() =>
      _MemoryDetailScreenState();
}

class _MemoryDetailScreenState extends ConsumerState<MemoryDetailScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      ref.read(memoryDetailNotifierProvider.notifier).load(widget.captureId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(memoryDetailNotifierProvider);

    ref.listen<MemoryDetailState>(memoryDetailNotifierProvider, (prev, next) {
      if (next is MemoryDetailLoaded) {
        final message = next.message;
        if (message != null && message != (prev is MemoryDetailLoaded ? prev.message : null)) {
          ScaffoldMessenger.of(context)
            ..clearSnackBars()
            ..showSnackBar(SnackBar(content: Text(message)));
        }
      }
    });

    return Scaffold(
      appBar: AppBar(title: const Text('记忆详情')),
      body: switch (state) {
        MemoryDetailLoading() =>
          const Center(child: CircularProgressIndicator()),
        MemoryDetailError(:final message) => _buildError(context, message),
        MemoryDetailLoaded(:final detail, :final saving) => _buildLoaded(
          context,
          detail,
          saving,
        ),
      },
    );
  }

  Widget _buildError(BuildContext context, String message) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(message),
          const SizedBox(height: 16),
          OutlinedButton(
            onPressed: () => ref
                .read(memoryDetailNotifierProvider.notifier)
                .load(widget.captureId),
            child: const Text('重试'),
          ),
        ],
      ),
    );
  }

  Widget _buildLoaded(BuildContext context, MemoryDetail detail, bool saving) {
    final notifier = ref.read(memoryDetailNotifierProvider.notifier);

    return Stack(
      children: [
        SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (saving) const LinearProgressIndicator(),
              _header(context, detail, notifier),
              const SizedBox(height: 16),
              _originalBlock(context, detail),
              const SizedBox(height: 16),
              _fieldsBlock(context, detail),
              const SizedBox(height: 16),
              _noteButton(context, notifier),
              const SizedBox(height: 16),
              _revisionsBlock(context, detail),
              const SizedBox(height: 24),
              _actions(context, detail, notifier),
            ],
          ),
        ),
      ],
    );
  }

  // ---- 头部：类型 / 状态 / 置顶 ----

  Widget _header(
    BuildContext context,
    MemoryDetail detail,
    MemoryDetailNotifier notifier,
  ) {
    final card = detail.memoryCard;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _typeChip(context, card.primaryType),
                const SizedBox(width: 8),
                Text(
                  _lifecycleLabel(captureLifecycle(detail)),
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: cardIsArchived(detail) ? Colors.grey : Colors.green,
                  ),
                ),
                const Spacer(),
                IconButton(
                  icon: Icon(
                    card.isPinned ? Icons.push_pin : Icons.push_pin_outlined,
                    color: card.isPinned ? Colors.orange : null,
                  ),
                  tooltip: card.isPinned ? '取消置顶' : '置顶',
                  onPressed: () => notifier.setPinned(!card.isPinned),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              card.title,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 4),
            Text(
              _formatFullTime(detail.capture.capturedAt ?? detail.capture.createdAt),
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: Colors.grey),
            ),
          ],
        ),
      ),
    );
  }

  // ---- 原始记忆 ----

  Widget _originalBlock(BuildContext context, MemoryDetail detail) {
    final text = detail.transcript?.text ?? detail.capture.originalText;
    final displayText = (text != null && text.isNotEmpty) ? text : '（无文本）';
    final audio = detail.audio;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('原始记忆', style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  displayText,
                  style: Theme.of(context).textTheme.bodyLarge,
                ),
                if (audio != null) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      const Icon(Icons.graphic_eq, size: 16, color: Colors.grey),
                      const SizedBox(width: 6),
                      Text(
                        _audioMeta(audio),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      const Spacer(),
                      TextButton(
                        onPressed: () => context.push(
                          '/captures/${detail.capture.id}/transcript',
                        ),
                        child: const Text('查看/修正转写'),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }

  // ---- AI / fallback 字段 ----

  Widget _fieldsBlock(BuildContext context, MemoryDetail detail) {
    final card = detail.memoryCard;
    final summary = card.summary;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('整理字段', style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (summary != null && summary.isNotEmpty) ...[
                  Text('摘要', style: Theme.of(context).textTheme.labelMedium),
                  const SizedBox(height: 4),
                  Text(summary, style: Theme.of(context).textTheme.bodyMedium),
                  const SizedBox(height: 12),
                ],
                if (card.keyPoints.isNotEmpty) ...[
                  Text('要点', style: Theme.of(context).textTheme.labelMedium),
                  const SizedBox(height: 4),
                  for (final point in card.keyPoints)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('· '),
                          Expanded(
                            child: Text(
                              point,
                              style: Theme.of(context).textTheme.bodyMedium,
                            ),
                          ),
                        ],
                      ),
                    ),
                  const SizedBox(height: 12),
                ],
                if (card.tags.isNotEmpty) ...[
                  Text('标签', style: Theme.of(context).textTheme.labelMedium),
                  const SizedBox(height: 4),
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: [
                      for (final tag in card.tags)
                        Chip(
                          label: Text(tag),
                          visualDensity: VisualDensity.compact,
                          materialTapTargetSize:
                              MaterialTapTargetSize.shrinkWrap,
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }

  // ---- 续写 ----

  Widget _noteButton(BuildContext context, MemoryDetailNotifier notifier) {
    return SizedBox(
      width: double.infinity,
      child: FilledButton.icon(
        icon: const Icon(Icons.edit_outlined),
        label: const Text('继续思考 / 续写'),
        onPressed: () => _openNoteDialog(context, notifier),
      ),
    );
  }

  Future<void> _openNoteDialog(
    BuildContext context,
    MemoryDetailNotifier notifier,
  ) async {
    final text = await showDialog<String>(
      context: context,
      builder: (context) => const _NoteDialog(),
    );
    if (text == null || text.trim().isEmpty) return;
    await notifier.addNote(text.trim());
  }

  // ---- 修订历史 ----

  Widget _revisionsBlock(BuildContext context, MemoryDetail detail) {
    final revisions = detail.revisions;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('修订历史', style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 8),
        if (revisions.isEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                '暂无修订记录',
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium?.copyWith(color: Colors.grey),
              ),
            ),
          )
        else
          for (final revision in revisions) _revisionTile(context, revision),
      ],
    );
  }

  Widget _revisionTile(BuildContext context, MemoryRevision revision) {
    final source = _sourceLabel(revision.source);
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        dense: true,
        leading: CircleAvatar(
          radius: 14,
          child: Text('${revision.revision}', style: const TextStyle(fontSize: 12)),
        ),
        title: Text('$source 修订 #${revision.revision}'),
        subtitle: Text(
          _changesText(revision),
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
        ),
        trailing: Text(
          _formatTime(revision.createdAt),
          style: Theme.of(
            context,
          ).textTheme.bodySmall?.copyWith(color: Colors.grey),
        ),
      ),
    );
  }

  // ---- 操作：修正 / 归档 / 删除 ----

  Widget _actions(
    BuildContext context,
    MemoryDetail detail,
    MemoryDetailNotifier notifier,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        OutlinedButton.icon(
          icon: const Icon(Icons.edit_note_outlined),
          label: const Text('修正字段'),
          onPressed: () => _openCorrectDialog(context, detail, notifier),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          icon: const Icon(Icons.archive_outlined),
          label: const Text('归档'),
          onPressed: () => notifier.archive(),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          icon: const Icon(Icons.delete_outline),
          label: const Text('删除'),
          style: OutlinedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.error,
          ),
          onPressed: () => _confirmDelete(context, notifier),
        ),
      ],
    );
  }

  Future<void> _openCorrectDialog(
    BuildContext context,
    MemoryDetail detail,
    MemoryDetailNotifier notifier,
  ) async {
    final card = detail.memoryCard;
    final result = await showDialog<_CorrectFormResult>(
      context: context,
      builder: (context) => _CorrectDialog(
        title: card.title,
        summary: card.summary ?? '',
        primaryType: card.primaryType,
        tags: card.tags,
        keyPoints: card.keyPoints,
      ),
    );
    if (result == null) return;

    await notifier.correct(
      title: result.title != card.title ? result.title : null,
      summary: result.summary != (card.summary ?? '') ? result.summary : null,
      primaryType: result.primaryType != card.primaryType
          ? result.primaryType
          : null,
      tags: _listEquals(result.tags, card.tags) ? null : result.tags,
      keyPoints: _listEquals(result.keyPoints, card.keyPoints)
          ? null
          : result.keyPoints,
    );
  }

  Future<void> _confirmDelete(
    BuildContext context,
    MemoryDetailNotifier notifier,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => const _DeleteConfirmDialog(),
    );
    if (confirmed != true) return;
    final ok = await notifier.delete();
    if (ok && context.mounted) {
      context.pop();
    }
  }

  // ---- 小部件 ----

  Widget _typeChip(BuildContext context, String primaryType) {
    final label = kPrimaryTypeLabels[primaryType] ?? '未分类';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(label, style: Theme.of(context).textTheme.labelSmall),
    );
  }

  String _lifecycleLabel(String lifecycleStatus) {
    return lifecycleStatus == 'archived' ? '已归档' : '进行中';
  }

  String _audioMeta(AudioAssetModel audio) {
    final duration = audio.durationMs;
    if (duration != null) {
      final seconds = (duration / 1000).round();
      return '音频 · ${audio.mimeType} · ${seconds}s';
    }
    return '音频 · ${audio.mimeType}';
  }

  String _sourceLabel(String source) {
    return switch (source) {
      'user' => '用户',
      'ai' => 'AI',
      _ => '本地整理',
    };
  }

  String _changesText(MemoryRevision revision) {
    final note = revision.changes['note'];
    if (note is String) {
      return note;
    }
    final keys = revision.changes.keys.toList();
    if (keys.isEmpty) {
      return '（无字段变更）';
    }
    return '更新字段: ${keys.join(', ')}';
  }

  String _formatTime(DateTime dt) {
    return DateFormat('MM-dd HH:mm').format(dt.toLocal());
  }

  String _formatFullTime(DateTime dt) {
    return DateFormat('yyyy-MM-dd HH:mm').format(dt.toLocal());
  }

  bool _listEquals(List<String> a, List<String> b) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}

bool cardIsArchived(MemoryDetail detail) {
  return detail.capture.lifecycleStatus == 'archived';
}

String captureLifecycle(MemoryDetail detail) {
  return detail.capture.lifecycleStatus;
}

/// 修正表单结果。
class _CorrectFormResult {
  const _CorrectFormResult({
    required this.title,
    required this.summary,
    required this.primaryType,
    required this.tags,
    required this.keyPoints,
  });

  final String title;
  final String summary;
  final String primaryType;
  final List<String> tags;
  final List<String> keyPoints;
}

class _CorrectDialog extends StatefulWidget {
  const _CorrectDialog({
    required this.title,
    required this.summary,
    required this.primaryType,
    required this.tags,
    required this.keyPoints,
  });

  final String title;
  final String summary;
  final String primaryType;
  final List<String> tags;
  final List<String> keyPoints;

  @override
  State<_CorrectDialog> createState() => _CorrectDialogState();
}

class _CorrectDialogState extends State<_CorrectDialog> {
  late final TextEditingController _titleController;
  late final TextEditingController _summaryController;
  late final TextEditingController _tagsController;
  late final TextEditingController _keyPointsController;
  late String _primaryType;

  @override
  void initState() {
    super.initState();
    _titleController = TextEditingController(text: widget.title);
    _summaryController = TextEditingController(text: widget.summary);
    _tagsController = TextEditingController(text: widget.tags.join(', '));
    _keyPointsController = TextEditingController(
      text: widget.keyPoints.join('\n'),
    );
    _primaryType = widget.primaryType;
  }

  @override
  void dispose() {
    _titleController.dispose();
    _summaryController.dispose();
    _tagsController.dispose();
    _keyPointsController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('修正字段'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            TextField(
              key: const Key('correct_title_input'),
              controller: _titleController,
              decoration: const InputDecoration(labelText: '标题'),
              maxLength: 300,
            ),
            TextField(
              key: const Key('correct_summary_input'),
              controller: _summaryController,
              decoration: const InputDecoration(labelText: '摘要'),
              maxLines: 3,
              maxLength: 2000,
            ),
            DropdownButtonFormField<String>(
              initialValue: _primaryType,
              decoration: const InputDecoration(labelText: '类型'),
              items: [
                for (final entry in kPrimaryTypeLabels.entries)
                  DropdownMenuItem(
                    value: entry.key,
                    child: Text(entry.value),
                  ),
              ],
              onChanged: (value) {
                if (value != null) {
                  setState(() => _primaryType = value);
                }
              },
            ),
            const SizedBox(height: 8),
            TextField(
              key: const Key('correct_tags_input'),
              controller: _tagsController,
              decoration: const InputDecoration(labelText: '标签（逗号分隔）'),
            ),
            TextField(
              key: const Key('correct_key_points_input'),
              controller: _keyPointsController,
              decoration: const InputDecoration(labelText: '要点（每行一个）'),
              maxLines: 3,
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: () {
            final title = _titleController.text.trim();
            if (title.isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('标题不能为空')),
              );
              return;
            }
            final tags = _tagsController.text
                .split(',')
                .map((e) => e.trim())
                .where((e) => e.isNotEmpty)
                .toList();
            final keyPoints = _keyPointsController.text
                .split('\n')
                .map((e) => e.trim())
                .where((e) => e.isNotEmpty)
                .toList();
            Navigator.of(context).pop(
              _CorrectFormResult(
                title: title,
                summary: _summaryController.text.trim(),
                primaryType: _primaryType,
                tags: tags,
                keyPoints: keyPoints,
              ),
            );
          },
          child: const Text('保存'),
        ),
      ],
    );
  }
}

class _NoteDialog extends StatefulWidget {
  const _NoteDialog();

  @override
  State<_NoteDialog> createState() => _NoteDialogState();
}

class _NoteDialogState extends State<_NoteDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('继续思考'),
      content: TextField(
        key: const Key('note_text_input'),
        controller: _controller,
        autofocus: true,
        maxLines: 4,
        maxLength: 10000,
        decoration: const InputDecoration(hintText: '写下后续想法…'),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_controller.text.trim()),
          child: const Text('追加'),
        ),
      ],
    );
  }
}

class _DeleteConfirmDialog extends StatelessWidget {
  const _DeleteConfirmDialog();

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('删除记忆'),
      content: const Text('删除后此记忆将被移入回收站，确定删除吗？'),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('取消'),
        ),
        FilledButton(
          style: FilledButton.styleFrom(
            backgroundColor: Theme.of(context).colorScheme.error,
          ),
          onPressed: () => Navigator.of(context).pop(true),
          child: const Text('删除'),
        ),
      ],
    );
  }
}
