import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../shared/models/timeline_event.dart';
import '../data/timeline_api.dart';

final timelineNotifierProvider =
    StateNotifierProvider<TimelineNotifier, AsyncValue<List<TimelineEvent>>>((
      ref,
    ) {
      final api = ref.watch(timelineApiProvider);
      return TimelineNotifier(api);
    });

class TimelineNotifier extends StateNotifier<AsyncValue<List<TimelineEvent>>> {
  final TimelineApi _api;
  int _currentPage = 1;
  bool _hasMore = true;

  TimelineNotifier(this._api) : super(const AsyncValue.loading()) {
    refresh();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    try {
      _currentPage = 1;
      final events = await _api.getTimeline(page: _currentPage);
      _hasMore = events.length == 20; // limit is 20
      state = AsyncValue.data(events);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  Future<void> loadMore() async {
    if (!_hasMore || state is AsyncLoading) return;

    final currentData = state.value ?? [];
    try {
      _currentPage++;
      final newEvents = await _api.getTimeline(page: _currentPage);
      if (newEvents.isEmpty) {
        _hasMore = false;
        return;
      }
      _hasMore = newEvents.length == 20;
      state = AsyncValue.data([...currentData, ...newEvents]);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }
}
