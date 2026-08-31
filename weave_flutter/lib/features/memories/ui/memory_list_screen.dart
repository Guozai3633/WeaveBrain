import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../data/memory_api.dart';
import '../domain/memory_notifier.dart';

/// 主类型 → 中文标签。
const Map<String, String> kPrimaryTypeLabels = {
  'idea': '闪念',
  'question': '问题',
  'action': '行动',
  'reflection': '反思',
  'reference': '资料',
  'uncategorized': '未分类',
};

/// 捕获来源类型 → 中文标签。
const Map<String, String> kKindLabels = {
  'text': '文本',
  'audio': '语音',
  'import': '导入',
  'share': '分享',
};

const List<String> kPrimaryTypeOrder = [
  'idea',
  'question',
  'action',
  'reflection',
  'reference',
];

/// 记忆流：搜索、筛选、分页加载与空/错态。
class MemoryListScreen extends ConsumerStatefulWidget {
  const MemoryListScreen({super.key});

  @override
  ConsumerState<MemoryListScreen> createState() => _MemoryListScreenState();
}

class _MemoryListScreenState extends ConsumerState<MemoryListScreen> {
  final _searchController = TextEditingController();
  Timer? _debounce;

  String _lifecycle = 'active';
  bool _pinnedOnly = false;
  String? _kind;
  String? _primaryType;

  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      ref.read(memoryListNotifierProvider.notifier).load();
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  void _applyFilter() {
    ref.read(memoryListNotifierProvider.notifier).setFilter(
      MemoryListFilter(
        q: _searchController.text,
        kind: _kind,
        primaryType: _primaryType,
        lifecycleStatus: _lifecycle,
        pinnedOnly: _pinnedOnly,
      ),
    );
  }

  void _onSearchChanged(String query) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), _applyFilter);
  }

  void _clearSearch() {
    _searchController.clear();
    _applyFilter();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(memoryListNotifierProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('记忆')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
            child: SearchBar(
              controller: _searchController,
              hintText: '搜索记忆…',
              leading: const Padding(
                padding: EdgeInsets.only(left: 8),
                child: Icon(Icons.search),
              ),
              trailing: [
                if (_searchController.text.isNotEmpty)
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: _clearSearch,
                  ),
              ],
              onChanged: _onSearchChanged,
              elevation: WidgetStateProperty.all(1),
              padding: WidgetStateProperty.all(
                const EdgeInsets.symmetric(horizontal: 4),
              ),
            ),
          ),
          _buildFilterRow(),
          Expanded(
            child: switch (state) {
              MemoryListLoading() =>
                const Center(child: CircularProgressIndicator()),
              MemoryListError(:final message) => _buildError(context, message),
              MemoryListLoaded(
                :final items,
                :final hasMore,
                :final saving,
                :final message,
              ) => _buildLoaded(
                context,
                items,
                hasMore,
                saving,
                message,
              ),
            },
          ),
        ],
      ),
    );
  }

  Widget _buildFilterRow() {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: Row(
        children: [
          _chip(
            label: '进行中',
            selected: _lifecycle == 'active',
            onSelected: (_) => setState(() {
              _lifecycle = 'active';
              _applyFilter();
            }),
          ),
          const SizedBox(width: 8),
          _chip(
            label: '已归档',
            selected: _lifecycle == 'archived',
            onSelected: (_) => setState(() {
              _lifecycle = 'archived';
              _applyFilter();
            }),
          ),
          const SizedBox(width: 8),
          _chip(
            label: '仅看置顶',
            selected: _pinnedOnly,
            onSelected: (selected) => setState(() {
              _pinnedOnly = selected;
              _applyFilter();
            }),
          ),
          const SizedBox(width: 8),
          for (final entry in kKindLabels.entries) ...[
            _chip(
              label: entry.value,
              selected: _kind == entry.key,
              onSelected: (_) => setState(() {
                _kind = _kind == entry.key ? null : entry.key;
                _applyFilter();
              }),
            ),
            const SizedBox(width: 8),
          ],
          for (final type in kPrimaryTypeOrder) ...[
            _chip(
              label: kPrimaryTypeLabels[type] ?? type,
              selected: _primaryType == type,
              onSelected: (_) => setState(() {
                _primaryType = _primaryType == type ? null : type;
                _applyFilter();
              }),
            ),
            const SizedBox(width: 8),
          ],
        ],
      ),
    );
  }

  Widget _chip({
    required String label,
    required bool selected,
    required ValueChanged<bool> onSelected,
  }) {
    return FilterChip(
      label: Text(label),
      selected: selected,
      onSelected: onSelected,
      visualDensity: VisualDensity.compact,
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
    );
  }

  Widget _buildLoaded(
    BuildContext context,
    List<MemoryListEntry> items,
    bool hasMore,
    bool saving,
    String? message,
  ) {
    if (items.isEmpty) {
      return _buildEmpty(context);
    }
    return RefreshIndicator(
      onRefresh: () => ref.read(memoryListNotifierProvider.notifier).refresh(),
      child: ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        itemCount: items.length + (hasMore ? 1 : 0),
        itemBuilder: (context, index) {
          if (index >= items.length) {
            ref.read(memoryListNotifierProvider.notifier).loadMore();
            return const Padding(
              padding: EdgeInsets.all(16),
              child: Center(child: CircularProgressIndicator()),
            );
          }
          return _MemoryCardTile(entry: items[index]);
        },
      ),
    );
  }

  Widget _buildEmpty(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.library_books_outlined, size: 64, color: Colors.grey),
          const SizedBox(height: 16),
          Text(
            '还没有记忆',
            style: Theme.of(
              context,
            ).textTheme.titleMedium?.copyWith(color: Colors.grey),
          ),
          const SizedBox(height: 8),
          Text(
            '去捕捉一条语音或文字吧',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: Colors.grey),
          ),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: () => context.go('/capture'),
            icon: const Icon(Icons.mic),
            label: const Text('去捕捉'),
          ),
        ],
      ),
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
            onPressed: () =>
                ref.read(memoryListNotifierProvider.notifier).load(),
            child: const Text('重试'),
          ),
        ],
      ),
    );
  }
}

/// 记忆流卡片。
class _MemoryCardTile extends StatelessWidget {
  const _MemoryCardTile({required this.entry});

  final MemoryListEntry entry;

  @override
  Widget build(BuildContext context) {
    final capture = entry.capture;
    final card = entry.memoryCard;
    final primaryType = card.primaryType;
    final title = card.title.isNotEmpty
        ? card.title
        : (capture.originalText ?? '未命名记忆');
    final summary = card.summary;

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: InkWell(
        onTap: () => context.push('/memories/${capture.id}'),
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  _typeChip(context, primaryType),
                  const SizedBox(width: 8),
                  _statusText(context, capture.lifecycleStatus),
                  const Spacer(),
                  if (card.isPinned)
                    const Icon(Icons.push_pin, size: 16, color: Colors.orange),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              if (summary != null && summary.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(
                  summary,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(color: Colors.grey[700]),
                ),
              ],
              const SizedBox(height: 8),
              _metaRow(context, capture),
              if (card.tags.isNotEmpty) ...[
                const SizedBox(height: 8),
                _tagsRow(context, card.tags),
              ],
              const SizedBox(height: 8),
              Align(
                alignment: Alignment.centerRight,
                child: FilledButton.tonal(
                  onPressed: () => context.push('/memories/${capture.id}'),
                  child: Text(_primaryActionLabel(primaryType)),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

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

  Widget _statusText(BuildContext context, String lifecycleStatus) {
    final archived = lifecycleStatus == 'archived';
    final color = archived ? Colors.grey : Colors.green;
    return Text(
      archived ? '已归档' : '进行中',
      style: Theme.of(
        context,
      ).textTheme.labelSmall?.copyWith(color: color),
    );
  }

  Widget _metaRow(BuildContext context, CaptureLite capture) {
    final time = capture.capturedAt ?? capture.createdAt;
    final kindLabel = kKindLabels[capture.kind] ?? capture.kind;
    final icon = switch (capture.kind) {
      'audio' => Icons.mic,
      'import' => Icons.file_download_outlined,
      'share' => Icons.share_outlined,
      _ => Icons.text_fields,
    };
    return Row(
      children: [
        Icon(icon, size: 16, color: Colors.grey),
        const SizedBox(width: 4),
        Text(
          _formatTime(time),
          style: Theme.of(
            context,
          ).textTheme.bodySmall?.copyWith(color: Colors.grey),
        ),
        const SizedBox(width: 8),
        Text(
          kindLabel,
          style: Theme.of(
            context,
          ).textTheme.bodySmall?.copyWith(color: Colors.grey),
        ),
      ],
    );
  }

  Widget _tagsRow(BuildContext context, List<String> tags) {
    const maxVisible = 2;
    final visible = tags.take(maxVisible).toList();
    final extra = tags.length - visible.length;
    return Wrap(
      spacing: 6,
      runSpacing: 4,
      children: [
        for (final tag in visible)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(6),
            ),
            child: Text(tag, style: Theme.of(context).textTheme.labelSmall),
          ),
        if (extra > 0)
          Text(
            '+$extra',
            style: Theme.of(
              context,
            ).textTheme.labelSmall?.copyWith(color: Colors.grey),
          ),
      ],
    );
  }

  String _formatTime(DateTime dt) {
    return DateFormat('MM-dd HH:mm').format(dt.toLocal());
  }

  String _primaryActionLabel(String primaryType) {
    return switch (primaryType) {
      'idea' => '继续思考',
      'question' => '去回答',
      'action' => '去完成',
      'reflection' => '稍后回看',
      _ => '打开详情',
    };
  }
}
