import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/idea.dart';
import '../data/idea_api.dart';

sealed class IdeasState {
  const IdeasState();
}

class IdeasInitial extends IdeasState {
  const IdeasInitial();
}

class IdeasLoading extends IdeasState {
  const IdeasLoading();
}

class IdeasLoaded extends IdeasState {
  final List<Idea> ideas;
  final int total;
  final bool hasMore;

  const IdeasLoaded({
    required this.ideas,
    required this.total,
    required this.hasMore,
  });
}

class IdeasError extends IdeasState {
  final String message;
  const IdeasError({required this.message});
}

class IdeasNotifier extends Notifier<IdeasState> {
  late IdeaApi _api;
  int _currentPage = 1;
  static const _pageSize = 20;
  int? _projectId;
  String? _searchQuery;
  List<String>? _tags;

  @override
  IdeasState build() {
    final apiClient = ref.watch(apiClientProvider);
    _api = IdeaApi(apiClient);
    return const IdeasInitial();
  }

  Future<void> loadIdeas(int projectId, {String? search, List<String>? tags}) async {
    _projectId = projectId;
    _currentPage = 1;
    _searchQuery = search;
    _tags = tags;
    state = const IdeasLoading();
    try {
      final result = await _api.listIdeas(
        projectId: projectId,
        page: 1,
        limit: _pageSize,
        search: search,
        tags: tags,
      );
      state = IdeasLoaded(
        ideas: result.ideas,
        total: result.total,
        hasMore: result.ideas.length < result.total,
      );
    } catch (e) {
      state = IdeasError(message: '加载失败: $e');
    }
  }

  Future<void> search(String query) async {
    if (_projectId == null) return;
    await loadIdeas(_projectId!, search: query, tags: _tags);
  }

  Future<void> searchByTag(String tag) async {
    if (_projectId == null) return;
    await loadIdeas(_projectId!, search: _searchQuery, tags: [tag]);
  }

  void clearSearch() {
    if (_projectId == null) return;
    _searchQuery = null;
    _tags = null;
    loadIdeas(_projectId!);
  }

  Future<void> loadMore() async {
    final current = state;
    if (current is! IdeasLoaded || !current.hasMore || _projectId == null) {
      return;
    }

    _currentPage++;
    try {
      final result = await _api.listIdeas(
        projectId: _projectId!,
        page: _currentPage,
        limit: _pageSize,
        search: _searchQuery,
        tags: _tags,
      );
      final allIdeas = [...current.ideas, ...result.ideas];
      state = IdeasLoaded(
        ideas: allIdeas,
        total: result.total,
        hasMore: allIdeas.length < result.total,
      );
    } catch (e) {
      // Keep current state on load-more failure
      _currentPage--;
    }
  }

  Future<void> refresh() async {
    if (_projectId != null) {
      await loadIdeas(_projectId!, search: _searchQuery, tags: _tags);
    }
  }
}

final ideasNotifierProvider =
    NotifierProvider<IdeasNotifier, IdeasState>(() => IdeasNotifier());
