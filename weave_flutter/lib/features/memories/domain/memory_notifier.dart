import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../data/memory_api.dart';

/// 记忆流筛选条件（不可变）。
class MemoryListFilter {
  const MemoryListFilter({
    this.q = '',
    this.kind,
    this.primaryType,
    this.lifecycleStatus = 'active',
    this.pinnedOnly = false,
  });

  final String q;
  final String? kind;
  final String? primaryType;
  final String lifecycleStatus;
  final bool pinnedOnly;

  MemoryListFilter copyWith({
    String? q,
    String? kind,
    String? primaryType,
    String? lifecycleStatus,
    bool? pinnedOnly,
  }) {
    return MemoryListFilter(
      q: q ?? this.q,
      kind: kind ?? this.kind,
      primaryType: primaryType ?? this.primaryType,
      lifecycleStatus: lifecycleStatus ?? this.lifecycleStatus,
      pinnedOnly: pinnedOnly ?? this.pinnedOnly,
    );
  }
}

sealed class MemoryListState {
  const MemoryListState();
}

class MemoryListLoading extends MemoryListState {
  const MemoryListLoading();
}

class MemoryListLoaded extends MemoryListState {
  const MemoryListLoaded({
    required this.items,
    required this.hasMore,
    this.nextCursor,
    this.saving = false,
    this.message,
  });

  final List<MemoryListEntry> items;
  final String? nextCursor;
  final bool hasMore;
  final bool saving;
  final String? message;

  MemoryListLoaded copyWith({
    List<MemoryListEntry>? items,
    String? nextCursor,
    bool? hasMore,
    bool? saving,
    String? message,
    bool clearMessage = false,
  }) {
    return MemoryListLoaded(
      items: items ?? this.items,
      nextCursor: nextCursor ?? this.nextCursor,
      hasMore: hasMore ?? this.hasMore,
      saving: saving ?? this.saving,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class MemoryListError extends MemoryListState {
  const MemoryListError({required this.message});

  final String message;
}

class MemoryListNotifier extends Notifier<MemoryListState> {
  MemoryListFilter _filter = const MemoryListFilter();
  String? _nextCursor;
  static const _pageSize = 20;

  MemoryGateway get _gateway => ref.read(memoryGatewayProvider);

  @override
  MemoryListState build() => const MemoryListLoading();

  /// 加载（或按新筛选重新加载）第一页。已处于 Loaded 时不切到全屏 Loading，
  /// 以便下拉刷新时列表保持可见。
  Future<void> load({MemoryListFilter? filter}) async {
    if (filter != null) _filter = filter;
    _nextCursor = null;
    final previous = state;
    if (previous is! MemoryListLoaded) {
      state = const MemoryListLoading();
    }
    try {
      final result = await _gateway.list(
        limit: _pageSize,
        q: _filter.q,
        kind: _filter.kind,
        primaryType: _filter.primaryType,
        lifecycleStatus: _filter.lifecycleStatus,
        pinned: _filter.pinnedOnly,
      );
      _nextCursor = result.nextCursor;
      state = MemoryListLoaded(
        items: result.items,
        nextCursor: result.nextCursor,
        hasMore: result.nextCursor != null,
        saving: previous is MemoryListLoaded ? previous.saving : false,
        message: previous is MemoryListLoaded ? previous.message : null,
      );
    } catch (e) {
      state = MemoryListError(message: _errorMessage(e));
    }
  }

  /// 加载下一页，追加到列表末尾。失败时保持当前状态。
  Future<void> loadMore() async {
    final current = state;
    if (current is! MemoryListLoaded) return;
    if (!current.hasMore || current.saving) return;
    final cursor = _nextCursor;
    if (cursor == null) return;
    try {
      final result = await _gateway.list(
        cursor: cursor,
        limit: _pageSize,
        q: _filter.q,
        kind: _filter.kind,
        primaryType: _filter.primaryType,
        lifecycleStatus: _filter.lifecycleStatus,
        pinned: _filter.pinnedOnly,
      );
      _nextCursor = result.nextCursor;
      state = current.copyWith(
        items: [...current.items, ...result.items],
        nextCursor: result.nextCursor,
        hasMore: result.nextCursor != null,
      );
    } catch (_) {
      // 加载更多失败：保持当前列表。
    }
  }

  Future<void> search(String q) => load(filter: _filter.copyWith(q: q.trim()));

  Future<void> setFilter(MemoryListFilter filter) => load(filter: filter);

  Future<void> refresh() => load();

  /// 动作（如置顶 / 修正）后本地替换一张卡。
  void updateLocally(MemoryCardModel card) {
    final current = state;
    if (current is! MemoryListLoaded) return;
    final items = <MemoryListEntry>[];
    for (final entry in current.items) {
      if (entry.capture.id == card.captureId) {
        items.add(MemoryListEntry(capture: entry.capture, memoryCard: card));
      } else {
        items.add(entry);
      }
    }
    state = current.copyWith(items: items);
  }

  /// 删除后从列表移除一项。
  void removeLocally(String captureId) {
    final current = state;
    if (current is! MemoryListLoaded) return;
    state = current.copyWith(
      items: [
        for (final entry in current.items)
          if (entry.capture.id != captureId) entry,
      ],
    );
  }

  void setMessage(String? message) {
    final current = state;
    if (current is MemoryListLoaded) {
      state = current.copyWith(message: message);
    }
  }
}

sealed class MemoryDetailState {
  const MemoryDetailState();
}

class MemoryDetailLoading extends MemoryDetailState {
  const MemoryDetailLoading();
}

class MemoryDetailLoaded extends MemoryDetailState {
  const MemoryDetailLoaded({
    required this.detail,
    this.saving = false,
    this.message,
  });

  final MemoryDetail detail;
  final bool saving;
  final String? message;

  MemoryDetailLoaded copyWith({
    MemoryDetail? detail,
    bool? saving,
    String? message,
    bool clearMessage = false,
  }) {
    return MemoryDetailLoaded(
      detail: detail ?? this.detail,
      saving: saving ?? this.saving,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class MemoryDetailError extends MemoryDetailState {
  const MemoryDetailError({required this.message});

  final String message;
}

class MemoryDetailNotifier extends Notifier<MemoryDetailState> {
  String? _captureId;

  MemoryGateway get _gateway => ref.read(memoryGatewayProvider);

  @override
  MemoryDetailState build() => const MemoryDetailLoading();

  /// 按 captureId 加载详情（由路由路径参数驱动）。
  Future<void> load(String captureId) async {
    _captureId = captureId;
    state = const MemoryDetailLoading();
    try {
      final detail = await _gateway.detail(captureId);
      state = MemoryDetailLoaded(detail: detail);
    } catch (e) {
      state = MemoryDetailError(message: _errorMessage(e));
    }
  }

  /// 修正字段。成功返回 true 并更新本地卡片；409/404 等失败返回 false 并提示。
  Future<bool> correct({
    String? title,
    String? summary,
    String? primaryType,
    List<String>? tags,
    List<String>? keyPoints,
  }) {
    return _mutate(
      action: () => _gateway.correct(
        _requireCaptureId(),
        title: title,
        summary: summary,
        primaryType: primaryType,
        tags: tags,
        keyPoints: keyPoints,
      ),
      successMessage: '已修正',
    );
  }

  Future<bool> addNote(String text) {
    return _mutate(
      action: () => _gateway.addNote(_requireCaptureId(), text: text),
      successMessage: '已续写',
    );
  }

  Future<bool> setPinned(bool pinned) {
    return _mutate(
      action: () => _gateway.setPinned(_requireCaptureId(), pinned: pinned),
      successMessage: pinned ? '已置顶' : '已取消置顶',
    );
  }

  Future<bool> archive() {
    return _mutate(
      action: () => _gateway.archive(_requireCaptureId()),
      successMessage: '已归档',
    );
  }

  /// 删除成功后详情页应 pop；返回 true 表示成功。
  Future<bool> delete() async {
    final captureId = _captureId;
    final current = state;
    if (captureId == null || current is! MemoryDetailLoaded) {
      return false;
    }
    state = current.copyWith(saving: true, clearMessage: true);
    try {
      await _gateway.delete(captureId);
      state = current.copyWith(saving: false, message: '已删除');
      return true;
    } catch (e) {
      state = current.copyWith(saving: false, message: _actionError(e));
      return false;
    }
  }

  void setMessage(String? message) {
    final current = state;
    if (current is MemoryDetailLoaded) {
      state = current.copyWith(message: message);
    }
  }

  Future<bool> _mutate({
    required Future<MemoryMutation> Function() action,
    required String successMessage,
  }) async {
    final captureId = _captureId;
    final current = state;
    if (captureId == null || current is! MemoryDetailLoaded) {
      return false;
    }
    state = current.copyWith(saving: true, clearMessage: true);
    try {
      final mutation = await action();
      final revisions = [...current.detail.revisions];
      final revision = mutation.revision;
      if (revision != null) {
        revisions.add(revision);
      }
      state = current.copyWith(
        detail: MemoryDetail(
          capture: mutation.capture,
          memoryCard: mutation.memoryCard,
          audio: current.detail.audio,
          transcript: current.detail.transcript,
          revisions: revisions,
        ),
        saving: false,
        message: successMessage,
      );
      return true;
    } catch (e) {
      state = current.copyWith(saving: false, message: _actionError(e));
      return false;
    }
  }

  String _requireCaptureId() {
    final captureId = _captureId;
    if (captureId == null) {
      throw StateError('MemoryDetailNotifier.load() must be called first');
    }
    return captureId;
  }
}

String _errorMessage(Object error) {
  if (error is ApiException) {
    return error.message;
  }
  return '$error';
}

String _actionError(Object error) {
  final code = error is ApiException ? error.statusCode : null;
  if (code == 409) {
    return '版本冲突，请刷新后重试';
  }
  if (code == 404) {
    return '记忆不存在';
  }
  return _errorMessage(error);
}

final memoryListNotifierProvider =
    NotifierProvider<MemoryListNotifier, MemoryListState>(
      () => MemoryListNotifier(),
    );

final memoryDetailNotifierProvider =
    NotifierProvider<MemoryDetailNotifier, MemoryDetailState>(
      () => MemoryDetailNotifier(),
    );
