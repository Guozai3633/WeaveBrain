import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../data/import_api.dart';
import '../domain/import_notifier.dart';

enum _ImportMode { single, batch }

/// 导入旧记忆：单条（复用 POST /captures kind=import）与批量（三阶段）。
class ImportScreen extends ConsumerStatefulWidget {
  const ImportScreen({super.key});

  @override
  ConsumerState<ImportScreen> createState() => _ImportScreenState();
}

class _ImportScreenState extends ConsumerState<ImportScreen> {
  _ImportMode _mode = _ImportMode.single;

  // 单条导入表单。
  final _singleText = TextEditingController();
  final _singleExternalId = TextEditingController();
  final _singleTitle = TextEditingController();
  final _singleSourceName = TextEditingController();
  final _singleTags = TextEditingController();
  final _singleTimezone = TextEditingController();
  DateTime? _singleCapturedAt;

  // 批量导入表单。
  final _batchText = TextEditingController();
  final _batchSourceName = TextEditingController();
  final _batchSeparator = TextEditingController();
  final _batchTimezone = TextEditingController();
  String _batchFormat = 'plain_text';
  String _batchDuplicateAction = 'skip';

  // completion:apply 的选中提案（按提案 id）。
  final Set<String> _selectedProposalIds = <String>{};

  @override
  void dispose() {
    _singleText.dispose();
    _singleExternalId.dispose();
    _singleTitle.dispose();
    _singleSourceName.dispose();
    _singleTags.dispose();
    _singleTimezone.dispose();
    _batchText.dispose();
    _batchSourceName.dispose();
    _batchSeparator.dispose();
    _batchTimezone.dispose();
    super.dispose();
  }

  Future<void> _submitSingle() async {
    final text = _singleText.text;
    if (text.trim().isEmpty) {
      _toast('请输入内容');
      return;
    }
    final capturedAt = _singleCapturedAt;
    final draft = SingleImportDraft(
      text: text,
      externalId: _nonEmpty(_singleExternalId.text),
      sourceName: _nonEmpty(_singleSourceName.text),
      title: _nonEmpty(_singleTitle.text),
      tags: _splitTags(_singleTags.text),
      timezone: _nonEmpty(_singleTimezone.text),
      capturedAt: capturedAt != null
          ? DateFormat('yyyy-MM-dd').format(capturedAt)
          : null,
    );
    final result = await ref
        .read(importNotifierProvider.notifier)
        .importSingle(draft);
    if (!mounted || result == null) return;
    final message = result.dedupeStatus == 'suggested'
        ? '已导入（内容疑似重复，可稍后处理）'
        : '导入成功';
    _toast(message);
    context.push('/memories/${result.captureId}');
  }

  Future<void> _pickCapturedAt() async {
    final now = DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: now,
      firstDate: DateTime(2000),
      lastDate: now.add(const Duration(days: 1)),
    );
    if (picked != null) {
      setState(() => _singleCapturedAt = picked);
    }
  }

  Future<void> _pickFile() async {
    final reader = ref.read(textFileReaderProvider);
    final content = await reader.pickAndReadUtf8();
    if (!mounted) return;
    if (content == null) {
      _toast('当前平台请直接粘贴文本内容');
      return;
    }
    setState(() => _batchText.text = content);
  }

  Future<void> _parseAndPreview() async {
    final content = _batchText.text;
    if (content.trim().isEmpty) {
      _toast('请输入或选择要导入的内容');
      return;
    }
    final notifier = ref.read(importNotifierProvider.notifier);
    final job = await notifier.createJob(
      format: _batchFormat,
      sourceName: _nonEmpty(_batchSourceName.text) ?? '旧备忘录',
      content: content,
      separator: _nonEmpty(_batchSeparator.text),
      timezone: _nonEmpty(_batchTimezone.text),
    );
    if (job == null) return;
    await notifier.preview();
    if (mounted) {
      setState(() => _selectedProposalIds.clear());
    }
  }

  Future<void> _runCompletion() async {
    await ref.read(importNotifierProvider.notifier).completionPreview();
    if (mounted) {
      setState(() => _selectedProposalIds.clear());
    }
  }

  Future<void> _applySelections() async {
    final state = ref.read(importNotifierProvider);
    if (state is! ImportPreviewed) return;
    if (_selectedProposalIds.isEmpty) {
      _toast('请先勾选要采用的提案');
      return;
    }
    // 提案在 completion 覆盖层中；优先按覆盖层构造选择，避免与预览基座错位。
    final rows = state.completion?.rows.isNotEmpty == true
        ? state.completion!.rows
        : state.preview.rows;
    final selections = <ImportRowSelection>[];
    for (final row in rows) {
      final ids = row.completionProposals
          .where((p) => _selectedProposalIds.contains(p.id))
          .map((p) => p.id)
          .toList();
      if (ids.isNotEmpty) {
        selections.add(
          ImportRowSelection(rowNumber: row.rowNumber, proposalIds: ids),
        );
      }
    }
    await ref
        .read(importNotifierProvider.notifier)
        .completionApply(selections);
    if (mounted) {
      setState(() => _selectedProposalIds.clear());
    }
  }

  Future<void> _commit() async {
    await ref.read(importNotifierProvider.notifier).commit(
      duplicateContentAction: _batchDuplicateAction,
    );
  }

  Future<void> _loadErrorReport() async {
    await ref.read(importNotifierProvider.notifier).loadErrorReport();
  }

  void _reset() {
    ref.read(importNotifierProvider.notifier).reset();
    setState(() => _selectedProposalIds.clear());
  }

  void _toggleProposal(String id, bool checked) {
    setState(() {
      if (checked) {
        _selectedProposalIds.add(id);
      } else {
        _selectedProposalIds.remove(id);
      }
    });
  }

  void _toast(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(importNotifierProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('导入旧记忆')),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
              child: SegmentedButton<_ImportMode>(
                segments: const [
                  ButtonSegment(
                    value: _ImportMode.single,
                    label: Text('单条'),
                    icon: Icon(Icons.edit_note_outlined),
                  ),
                  ButtonSegment(
                    value: _ImportMode.batch,
                    label: Text('批量'),
                    icon: Icon(Icons.file_download_outlined),
                  ),
                ],
                selected: {_mode},
                onSelectionChanged: (selection) {
                  setState(() => _mode = selection.first);
                },
              ),
            ),
            const Divider(height: 1),
            Expanded(
              child: state is ImportCommitted && _mode == _ImportMode.batch
                  ? _CommittedSection(
                      state: state,
                      onReload: _loadErrorReport,
                      onReset: _reset,
                    )
                  : ListView(
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
                      children: [
                        if (state is ImportError) _ErrorBanner(state: state),
                        if (_mode == _ImportMode.single) ..._singleForm(state),
                        if (_mode == _ImportMode.batch) ..._batchForm(state),
                        if (_mode == _ImportMode.batch &&
                            state is ImportPreviewed)
                          _PreviewSection(
                            state: state,
                            selectedProposalIds: _selectedProposalIds,
                            duplicateAction: _batchDuplicateAction,
                            onToggleProposal: _toggleProposal,
                            onCompletion: _runCompletion,
                            onApply: _applySelections,
                            onDuplicateAction: (value) {
                              setState(() => _batchDuplicateAction = value);
                            },
                            onCommit: _commit,
                          ),
                      ],
                    ),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _singleForm(ImportState state) {
    final busy = state is ImportCreating;
    return [
      TextField(
        key: const Key('import_single_text'),
        controller: _singleText,
        minLines: 3,
        maxLines: 6,
        decoration: const InputDecoration(
          labelText: '内容 *',
          hintText: '粘贴一段文字或笔记',
          border: OutlineInputBorder(),
        ),
      ),
      const SizedBox(height: 12),
      TextField(
        key: const Key('import_single_title'),
        controller: _singleTitle,
        decoration: const InputDecoration(
          labelText: '标题（选填）',
          border: OutlineInputBorder(),
        ),
      ),
      const SizedBox(height: 12),
      Row(
        children: [
          Expanded(
            child: TextField(
              key: const Key('import_single_external_id'),
              controller: _singleExternalId,
              decoration: const InputDecoration(
                labelText: '外部 ID（选填，去重依据）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: TextField(
              key: const Key('import_single_source_name'),
              controller: _singleSourceName,
              decoration: const InputDecoration(
                labelText: '来源名（选填）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
        ],
      ),
      const SizedBox(height: 12),
      TextField(
        key: const Key('import_single_tags'),
        controller: _singleTags,
        decoration: const InputDecoration(
          labelText: '标签（选填，逗号分隔）',
          border: OutlineInputBorder(),
        ),
      ),
      const SizedBox(height: 12),
      Row(
        children: [
          Expanded(
            child: TextField(
              key: const Key('import_single_timezone'),
              controller: _singleTimezone,
              decoration: const InputDecoration(
                labelText: '时区（选填，如 Asia/Shanghai）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
          const SizedBox(width: 12),
          OutlinedButton.icon(
            key: const Key('import_single_pick_date'),
            onPressed: _pickCapturedAt,
            icon: const Icon(Icons.calendar_today_outlined),
            label: Text(
              _singleCapturedAt == null
                  ? '记录时间'
                  : DateFormat('yyyy-MM-dd').format(_singleCapturedAt!),
            ),
          ),
        ],
      ),
      const SizedBox(height: 16),
      FilledButton.icon(
        key: const Key('import_single_submit'),
        onPressed: busy ? null : _submitSingle,
        icon: busy
            ? const SizedBox.square(
                dimension: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : const Icon(Icons.publish_outlined),
        label: const Text('导入这一条'),
      ),
    ];
  }

  List<Widget> _batchForm(ImportState state) {
    final busy = state is ImportCreating || state is ImportCommitting;
    return [
      Row(
        children: [
          Expanded(
            child: DropdownButtonFormField<String>(
              key: const Key('import_batch_format'),
              initialValue: _batchFormat,
              decoration: const InputDecoration(
                labelText: '格式',
                border: OutlineInputBorder(),
              ),
              items: const [
                DropdownMenuItem(value: 'plain_text', child: Text('多段文本')),
                DropdownMenuItem(value: 'markdown', child: Text('TXT / Markdown')),
                DropdownMenuItem(value: 'csv', child: Text('CSV')),
                DropdownMenuItem(value: 'jsonl', child: Text('JSONL')),
              ],
              onChanged: (value) {
                if (value != null) setState(() => _batchFormat = value);
              },
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: TextField(
              key: const Key('import_batch_source_name'),
              controller: _batchSourceName,
              decoration: const InputDecoration(
                labelText: '来源名（默认 旧备忘录）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
        ],
      ),
      const SizedBox(height: 12),
      TextField(
        key: const Key('import_batch_text'),
        controller: _batchText,
        minLines: 6,
        maxLines: 12,
        decoration: const InputDecoration(
          labelText: '内容',
          hintText: '粘贴多段文本 / CSV / JSONL，或用下方按钮选择文件',
          border: OutlineInputBorder(),
        ),
      ),
      const SizedBox(height: 8),
      Row(
        children: [
          Expanded(
            child: TextField(
              key: const Key('import_batch_separator'),
              controller: _batchSeparator,
              decoration: const InputDecoration(
                labelText: '分段分隔符（选填，默认 ---）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: TextField(
              key: const Key('import_batch_timezone'),
              controller: _batchTimezone,
              decoration: const InputDecoration(
                labelText: '时区（选填）',
                border: OutlineInputBorder(),
              ),
            ),
          ),
        ],
      ),
      const SizedBox(height: 12),
      Row(
        children: [
          Expanded(
            child: OutlinedButton.icon(
              key: const Key('import_batch_pick_file'),
              onPressed: _pickFile,
              icon: const Icon(Icons.attach_file_outlined),
              label: const Text('选择文件'),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: FilledButton.icon(
              key: const Key('import_batch_parse'),
              onPressed: busy ? null : _parseAndPreview,
              icon: busy
                  ? const SizedBox.square(
                      dimension: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.table_view_outlined),
              label: const Text('解析并预览'),
            ),
          ),
        ],
      ),
    ];
  }
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.state});

  final ImportError state;

  @override
  Widget build(BuildContext context) {
    final existingId = state.duplicateExistingCaptureId;
    return Card(
      color: Theme.of(context).colorScheme.errorContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.error_outline, color: Theme.of(context).colorScheme.error),
                const SizedBox(width: 8),
                Expanded(child: Text(state.message)),
              ],
            ),
            if (existingId != null) ...[
              const SizedBox(height: 8),
              TextButton.icon(
                onPressed: () => context.push('/memories/$existingId'),
                icon: const Icon(Icons.link),
                label: const Text('查看已存在的记忆'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _PreviewSection extends StatelessWidget {
  const _PreviewSection({
    required this.state,
    required this.selectedProposalIds,
    required this.duplicateAction,
    required this.onToggleProposal,
    required this.onCompletion,
    required this.onApply,
    required this.onDuplicateAction,
    required this.onCommit,
  });

  final ImportPreviewed state;
  final Set<String> selectedProposalIds;
  final String duplicateAction;
  final void Function(String id, bool checked) onToggleProposal;
  final VoidCallback onCompletion;
  final VoidCallback onApply;
  final void Function(String value) onDuplicateAction;
  final VoidCallback onCommit;

  @override
  Widget build(BuildContext context) {
    final job = state.job;
    final completion = state.completion;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Divider(height: 24),
        Row(
          children: [
            Text('预览', style: Theme.of(context).textTheme.titleMedium),
            const Spacer(),
            Text(
              '共 ${job.totalRows} 行 · 有效 ${job.validRows} · 待补 ${job.needsInputRows}',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
        ),
        const SizedBox(height: 8),
        for (final row in state.preview.rows)
          _RowCard(
            row: row,
            completion: completion,
            selectedProposalIds: selectedProposalIds,
            onToggleProposal: onToggleProposal,
          ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                key: const Key('import_batch_completion'),
                onPressed: onCompletion,
                icon: const Icon(Icons.auto_awesome_outlined),
                label: const Text('AI 补全缺失项'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: FilledButton.tonalIcon(
                key: const Key('import_batch_apply'),
                onPressed: onApply,
                icon: const Icon(Icons.checklist),
                label: Text('采用所选 (${selectedProposalIds.length})'),
              ),
            ),
          ],
        ),
        const SizedBox(height: 16),
        Text('疑似重复内容处理', style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        SegmentedButton<String>(
          segments: const [
            ButtonSegment(value: 'import', label: Text('仍导入')),
            ButtonSegment(value: 'skip', label: Text('跳过')),
          ],
          selected: {duplicateAction},
          onSelectionChanged: (selection) => onDuplicateAction(selection.first),
        ),
        const SizedBox(height: 16),
        FilledButton.icon(
          key: const Key('import_batch_commit'),
          onPressed: onCommit,
          icon: const Icon(Icons.download_done_outlined),
          label: const Text('开始导入'),
        ),
      ],
    );
  }
}

class _RowCard extends StatelessWidget {
  const _RowCard({
    required this.row,
    this.completion,
    this.selectedProposalIds = const {},
    this.onToggleProposal,
  });

  final ImportRowModel row;
  final ImportCompletionModel? completion;
  final Set<String> selectedProposalIds;
  final void Function(String id, bool checked)? onToggleProposal;

  @override
  Widget build(BuildContext context) {
    final proposals = _proposalsForRow();
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('#${row.rowNumber}', style: Theme.of(context).textTheme.labelLarge),
                if (row.externalId != null && row.externalId!.isNotEmpty) ...[
                  const SizedBox(width: 8),
                  Text(row.externalId!, style: Theme.of(context).textTheme.bodySmall),
                ],
                const Spacer(),
                ..._chips(context),
              ],
            ),
            const SizedBox(height: 6),
            Text(row.contentPreview, style: Theme.of(context).textTheme.bodyMedium),
            if (row.hasErrors)
              Padding(
                padding: const EdgeInsets.only(top: 6),
                child: Text(
                  row.validationErrors.map((e) => e.message).join('；'),
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            if (proposals.isNotEmpty) ...[
              const Divider(height: 16),
              for (final p in proposals)
                _ProposalTile(
                  proposal: p,
                  selected: selectedProposalIds.contains(p.id),
                  onToggle: onToggleProposal,
                ),
            ],
          ],
        ),
      ),
    );
  }

  List<Widget> _chips(BuildContext context) {
    final chips = <Widget>[];
    if (row.isDuplicateExternal) {
      chips.add(const _Chip(label: '重复·跳过', color: Colors.red));
    }
    if (row.isSuggested) {
      chips.add(const _Chip(label: '疑似重复', color: Colors.orange));
    }
    if (row.needsInput) {
      chips.add(const _Chip(label: '待补充', color: Colors.blueGrey));
    }
    if (row.isImported) {
      chips.add(const _Chip(label: '已导入', color: Colors.green));
    }
    return chips;
  }

  List<ImportProposalModel> _proposalsForRow() {
    final overlay = completion;
    if (overlay == null) return const <ImportProposalModel>[];
    for (final r in overlay.rows) {
      if (r.rowNumber == row.rowNumber) {
        return r.completionProposals;
      }
    }
    return const <ImportProposalModel>[];
  }
}

class _ProposalTile extends StatelessWidget {
  const _ProposalTile({
    required this.proposal,
    required this.selected,
    this.onToggle,
  });

  final ImportProposalModel proposal;
  final bool selected;
  final void Function(String id, bool checked)? onToggle;

  @override
  Widget build(BuildContext context) {
    final checked = selected || proposal.isAccepted;
    return CheckboxListTile(
      key: Key('import_proposal_${proposal.id}'),
      dense: true,
      value: checked,
      onChanged: proposal.isAccepted
          ? null
          : (value) {
              final toggle = onToggle;
              if (toggle != null) {
                toggle(proposal.id, value ?? false);
              }
            },
      title: Text(
        proposal.fieldName,
        style: Theme.of(context).textTheme.labelMedium,
      ),
      subtitle: Text(_displayValue(proposal)),
    );
  }

  String _displayValue(ImportProposalModel proposal) {
    final values = proposal.proposedValues;
    if (values.isNotEmpty) return values.join('、');
    return proposal.proposedValue;
  }
}

class _CommittedSection extends StatelessWidget {
  const _CommittedSection({
    required this.state,
    required this.onReload,
    required this.onReset,
  });

  final ImportCommitted state;
  final VoidCallback onReload;
  final VoidCallback onReset;

  @override
  Widget build(BuildContext context) {
    final result = state.result;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        const Icon(Icons.check_circle_outline, size: 48, color: Colors.green),
        const SizedBox(height: 8),
        Text(
          '导入完成',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 16),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              children: [
                _StatRow(label: '成功导入', value: result.imported),
                _StatRow(label: '跳过', value: result.skipped),
                _StatRow(label: '失败', value: result.failed),
                _StatRow(label: '待补充', value: result.needsInput),
                _StatRow(label: '总计', value: result.total),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                key: const Key('import_batch_error_report'),
                onPressed: onReload,
                icon: const Icon(Icons.report_outlined),
                label: const Text('查看错误报告'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: OutlinedButton.icon(
                key: const Key('import_batch_reset'),
                onPressed: onReset,
                icon: const Icon(Icons.replay),
                label: const Text('再次导入'),
              ),
            ),
          ],
        ),
        if (state.errorReport != null) ...[
          const SizedBox(height: 16),
          Text('错误报告', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          for (final entry in state.errorReport!) _ReportRow(entry: entry),
        ],
      ],
    );
  }
}

class _StatRow extends StatelessWidget {
  const _StatRow({required this.label, required this.value});

  final String label;
  final int value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        children: [
          Expanded(child: Text(label)),
          Text('$value', style: Theme.of(context).textTheme.titleMedium),
        ],
      ),
    );
  }
}

class _ReportRow extends StatelessWidget {
  const _ReportRow({required this.entry});

  final ImportErrorReportEntryModel entry;

  @override
  Widget build(BuildContext context) {
    final messages = entry.validationErrors.map((e) => e.message).join('；');
    return Card(
      margin: const EdgeInsets.only(bottom: 6),
      child: ListTile(
        dense: true,
        leading: Text('#${entry.rowNumber}'),
        title: Text(messages.isEmpty ? entry.status : messages),
        subtitle: entry.externalId != null && entry.externalId!.isNotEmpty
            ? Text(entry.externalId!)
            : null,
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(left: 4),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: TextStyle(fontSize: 11, color: color),
      ),
    );
  }
}

String? _nonEmpty(String value) {
  final trimmed = value.trim();
  return trimmed.isEmpty ? null : trimmed;
}

List<String> _splitTags(String raw) {
  return raw
      .split(',')
      .map((s) => s.trim())
      .where((s) => s.isNotEmpty)
      .toList();
}
