import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../shared/api/api_client.dart';
import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/timeline_event.dart';

final timelineApiProvider = Provider<TimelineApi>((ref) {
  final client = ref.watch(apiClientProvider);
  return TimelineApi(client);
});

class TimelineApi {
  final ApiClient _client;

  TimelineApi(this._client);

  Future<List<TimelineEvent>> getTimeline({
    int page = 1,
    int limit = 20,
  }) async {
    final response = await _client.get(
      '/users/me/timeline',
      queryParams: {'page': page, 'limit': limit},
    );

    final data = response.data['data'] as List;
    return data
        .map((e) => TimelineEvent.fromJson(e as Map<String, dynamic>))
        .toList();
  }
}
