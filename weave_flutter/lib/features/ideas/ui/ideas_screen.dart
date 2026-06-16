import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/widgets/project_selector.dart';
import '../domain/ideas_notifier.dart';
import 'idea_card.dart';

class IdeasScreen extends ConsumerStatefulWidget {
  const IdeasScreen({super.key});

  @override
  ConsumerState<IdeasScreen> createState() => _IdeasScreenState();
}

class _IdeasScreenState extends ConsumerState<IdeasScreen> {
  int? _selectedProjectId;
  final _searchController = TextEditingController();
  Timer? _debounce;
  String? _activeTag;

  @override
  void initState() {
    super.initState();
    Future.microtask(() => _loadIdeas());
  }

  @override
  void dispose() {
    _searchController.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  void _loadIdeas() {
    ref.read(ideasNotifierProvider.notifier).loadIdeas(_selectedProjectId ?? 1);
  }

  void _onSearchChanged(String query) {
    _debounce?.cancel();
    if (query.isEmpty) {
      _activeTag = null;
      ref.read(ideasNotifierProvider.notifier).clearSearch();
      return;
    }
    _debounce = Timer(const Duration(milliseconds: 300), () {
      _activeTag = null;
      ref.read(ideasNotifierProvider.notifier).search(query);
    });
  }

  void _onTagTapped(String tag) {
    _searchController.clear();
    setState(() => _activeTag = tag);
    ref.read(ideasNotifierProvider.notifier).searchByTag(tag);
  }

  void _clearTagFilter() {
    setState(() => _activeTag = null);
    _searchController.clear();
    ref.read(ideasNotifierProvider.notifier).clearSearch();
  }

  @override
  Widget build(BuildContext context) {
    final ideasState = ref.watch(ideasNotifierProvider);

    return Column(
      children: [
        // Project filter
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
          child: ProjectSelector(
            selectedProjectId: _selectedProjectId,
            onChanged: (id) {
              if (id != null) {
                setState(() => _selectedProjectId = id);
                _searchController.clear();
                _activeTag = null;
                ref.read(ideasNotifierProvider.notifier).loadIdeas(id);
              }
            },
          ),
        ),

        // Search bar
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: SearchBar(
            controller: _searchController,
            hintText: '搜索想法...',
            leading: const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Icon(Icons.search),
            ),
            trailing: [
              if (_searchController.text.isNotEmpty || _activeTag != null)
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed: () {
                    _searchController.clear();
                    _clearTagFilter();
                  },
                ),
            ],
            onChanged: _onSearchChanged,
            elevation: WidgetStateProperty.all(1),
            padding: WidgetStateProperty.all(
              const EdgeInsets.symmetric(horizontal: 4),
            ),
          ),
        ),

        // Active tag chip
        if (_activeTag != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: Row(
              children: [
                const Text('标签筛选: '),
                Chip(
                  label: Text(_activeTag!),
                  onDeleted: _clearTagFilter,
                  visualDensity: VisualDensity.compact,
                ),
              ],
            ),
          ),

        // Header row
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              const Icon(Icons.lightbulb_outline, size: 20),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  '想法列表',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              IconButton(
                icon: const Icon(Icons.refresh),
                onPressed: () =>
                    ref.read(ideasNotifierProvider.notifier).refresh(),
              ),
            ],
          ),
        ),

        // Ideas list
        Expanded(
          child: switch (ideasState) {
            IdeasInitial() || IdeasLoading() =>
              const Center(child: CircularProgressIndicator()),
            IdeasError(:final message) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(message),
                    const SizedBox(height: 16),
                    OutlinedButton(
                      onPressed: () => _loadIdeas(),
                      child: const Text('重试'),
                    ),
                  ],
                ),
              ),
            IdeasLoaded(:final ideas, :final hasMore) => ideas.isEmpty
                ? _buildEmptyState(context)
                : RefreshIndicator(
                    onRefresh: () =>
                        ref.read(ideasNotifierProvider.notifier).refresh(),
                    child: ListView.builder(
                      itemCount: ideas.length + (hasMore ? 1 : 0),
                      itemBuilder: (context, index) {
                        if (index >= ideas.length) {
                          ref
                              .read(ideasNotifierProvider.notifier)
                              .loadMore();
                          return const Padding(
                            padding: EdgeInsets.all(16),
                            child:
                                Center(child: CircularProgressIndicator()),
                          );
                        }
                        return IdeaCard(
                          idea: ideas[index],
                          onTagTapped: _onTagTapped,
                        );
                      },
                    ),
                  ),
          },
        ),
      ],
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.lightbulb_outline, size: 64, color: Colors.grey),
          const SizedBox(height: 16),
          Text(
            '还没有想法',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  color: Colors.grey,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            '去语音录入吧',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                ),
          ),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: () => context.go('/'),
            icon: const Icon(Icons.mic),
            label: const Text('开始录音'),
          ),
        ],
      ),
    );
  }
}
