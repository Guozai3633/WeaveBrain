import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/completion_api.dart';
import '../domain/completion_notifier.dart';

/// AI 补全面板：入口按钮 → 生成提案 → 逐字段采用 / 全部采用安全字段 / 撤销。
///
/// [enabled] 为 false（AI 补全未开启或记忆未就绪）时渲染为 [SizedBox.shrink]，
/// 不展示入口也不发起调用。apply/undo 成功后回调 [onApplied] 供详情页刷新。
class CompletionPanel extends ConsumerWidget {
  const CompletionPanel({
    super.key,
    required this.captureId,
    required this.enabled,
    this.onApplied,
  });

  final String captureId;
  final bool enabled;
  final VoidCallback? onApplied;

  static const _fieldLabels = <String, String>{
    'title': '标题',
    'primary_type': '类型',
    'summary': '摘要',
    'tags': '标签',
    'key_points': '要点',
  };

  static const _primaryTypeLabels = <String, String>{
    'uncategorized': '未分类',
    'idea': '灵感',
    'question': '疑问',
    'action': '行动',
    'reflection': '反思',
    'reference': '参考',
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!enabled) {
      return const SizedBox.shrink();
    }

    final state = ref.watch(completionNotifierProvider);
    ref.listen<CompletionState>(completionNotifierProvider, (prev, next) {
      if (next is CompletionLoaded) {
        final message = next.message;
        final prevMessage = prev is CompletionLoaded ? prev.message : null;
        if (message != null && message != prevMessage) {
          onApplied?.call();
          ScaffoldMessenger.of(context)
            ..clearSnackBars()
            ..showSnackBar(SnackBar(content: Text(message)));
        }
      }
    });

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('AI 补全', style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: _buildBody(context, ref, state),
          ),
        ),
      ],
    );
  }

  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    CompletionState state,
  ) {
    switch (state) {
      case CompletionError(:final message):
        return _errorBody(context, ref, message);
      case CompletionLoaded(:final preview, :final busy):
        final current = preview != null && preview.captureId == captureId
            ? preview
            : null;
        if (current == null) {
          return _entryBody(context, ref, busy);
        }
        if (current.proposals.isEmpty) {
          return _emptyBody(context, ref, busy);
        }
        return _proposalsBody(context, ref, current, busy);
      case CompletionInitial():
        return _entryBody(context, ref, false);
      case CompletionLoading():
        return _entryBody(context, ref, true);
    }
  }

  Widget _entryBody(BuildContext context, WidgetRef ref, bool busy) {
    return OutlinedButton.icon(
      key: const Key('completion_entry_button'),
      icon: const Icon(Icons.auto_awesome_outlined),
      label: Text(busy ? '生成中…' : 'AI 补全'),
      onPressed: busy
          ? null
          : () => ref
                .read(completionNotifierProvider.notifier)
                .loadPreview(captureId),
    );
  }

  Widget _errorBody(BuildContext context, WidgetRef ref, String message) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          message,
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
            color: Theme.of(context).colorScheme.error,
          ),
        ),
        const SizedBox(height: 8),
        _entryBody(context, ref, false),
      ],
    );
  }

  Widget _emptyBody(BuildContext context, WidgetRef ref, bool busy) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '该记忆暂无缺失字段可补全',
          style: Theme.of(
            context,
          ).textTheme.bodyMedium?.copyWith(color: Colors.grey),
        ),
        const SizedBox(height: 8),
        _entryBody(context, ref, busy),
      ],
    );
  }

  Widget _proposalsBody(
    BuildContext context,
    WidgetRef ref,
    CompletionPreviewResultModel preview,
    bool busy,
  ) {
    final safeCount = preview.proposals.where((p) => p.canAutoApply).length;
    final notifier = ref.read(completionNotifierProvider.notifier);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final proposal in preview.proposals) ...[
          _proposalTile(context, ref, proposal, busy),
          const SizedBox(height: 8),
        ],
        const SizedBox(height: 4),
        SizedBox(
          width: double.infinity,
          child: FilledButton.icon(
            key: const Key('completion_apply_all_safe_button'),
            icon: const Icon(Icons.done_all_outlined),
            label: const Text('全部采用安全字段'),
            onPressed: safeCount > 0 && !busy
                ? () => notifier.applyAllSafe(captureId)
                : null,
          ),
        ),
        const SizedBox(height: 8),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton.icon(
            key: const Key('completion_undo_button'),
            icon: const Icon(Icons.undo_outlined),
            label: const Text('撤销上次补全'),
            onPressed: busy ? null : () => notifier.undo(captureId),
          ),
        ),
      ],
    );
  }

  Widget _proposalTile(
    BuildContext context,
    WidgetRef ref,
    CompletionProposalModel proposal,
    bool busy,
  ) {
    final notifier = ref.read(completionNotifierProvider.notifier);
    final fieldLabel = _fieldLabels[proposal.fieldName] ?? proposal.fieldName;
    final isArray = proposal.fieldName == 'tags' ||
        proposal.fieldName == 'key_points';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              fieldLabel,
              style: Theme.of(context).textTheme.labelMedium,
            ),
            const SizedBox(width: 8),
            _policyChip(context, proposal.applyPolicy),
            const Spacer(),
            if (proposal.confidence != null)
              Text(
                '${(proposal.confidence! * 100).round()}%',
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: Colors.grey,
                ),
              ),
          ],
        ),
        const SizedBox(height: 4),
        if (isArray)
          if (proposal.proposedValues.isNotEmpty)
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: [
                for (final value in proposal.proposedValues)
                  Chip(
                    label: Text(value),
                    visualDensity: VisualDensity.compact,
                    materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
              ],
            )
          else
            Text(
              proposal.proposedValue,
              style: Theme.of(context).textTheme.bodyMedium,
            )
        else
          Text(
            proposal.fieldName == 'primary_type'
                ? (_primaryTypeLabels[proposal.proposedValue] ??
                      proposal.proposedValue)
                : proposal.proposedValue,
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        if (proposal.hasEvidence) ...[
          const SizedBox(height: 4),
          Wrap(
            spacing: 6,
            runSpacing: 4,
            children: [
              for (final span in proposal.evidenceSpans)
                if (span.quote.isNotEmpty)
                  Chip(
                    avatar: const Icon(Icons.format_quote, size: 14),
                    label: Text(
                      span.quote,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                    visualDensity: VisualDensity.compact,
                    materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
            ],
          ),
        ],
        if (proposal.canAutoApply) ...[
          const SizedBox(height: 4),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              key: Key('completion_apply_field_${proposal.fieldName}'),
              onPressed: busy ? null : () => notifier.applyOne(captureId, proposal),
              child: const Text('采用'),
            ),
          ),
        ],
      ],
    );
  }

  Widget _policyChip(BuildContext context, String policy) {
    final isSafe = policy == 'safe_auto';
    final label = isSafe ? 'AI 建议' : '需确认';
    final color = isSafe
        ? Colors.green.shade100
        : Theme.of(context).colorScheme.surfaceContainerHighest;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(label, style: Theme.of(context).textTheme.labelSmall),
    );
  }
}
