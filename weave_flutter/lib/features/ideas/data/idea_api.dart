import '../../../shared/api/api_client.dart';
import '../../../shared/models/idea.dart';

class IdeaApi {
  final ApiClient _client;

  IdeaApi(this._client);

  Future<({List<Idea> ideas, int total})> listIdeas({
    required int projectId,
    int page = 1,
    int limit = 20,
    String? search,
    List<String>? tags,
  }) async {
    final params = <String, dynamic>{
      'project_id': projectId,
      'page': page,
      'limit': limit,
    };
    if (search != null && search.isNotEmpty) params['search'] = search;
    if (tags != null && tags.isNotEmpty) params['tags'] = tags.join(',');

    final resp = await _client.get('/ideas', queryParams: params);
    final data = resp.data as Map<String, dynamic>;
    final ideas = (data['ideas'] as List<dynamic>)
        .map((e) => Idea.fromJson(e as Map<String, dynamic>))
        .toList();
    final total = data['total'] as int? ?? ideas.length;
    return (ideas: ideas, total: total);
  }

  Future<Idea> createIdea({
    required int projectId,
    required String rawInput,
    Map<String, dynamic>? structuredData,
    List<String>? tags,
  }) async {
    final resp = await _client.post(
      '/ideas',
      queryParams: {'project_id': projectId},
      body: {
        'raw_input': rawInput,
        if (structuredData != null) 'structured_data': structuredData,
        if (tags != null && tags.isNotEmpty) 'tags': tags,
      },
    );
    final data = resp.data as Map<String, dynamic>;
    return Idea.fromJson(data['idea'] as Map<String, dynamic>);
  }

  Future<Idea> getIdea(int id) async {
    final resp = await _client.get('/ideas/$id');
    final data = resp.data as Map<String, dynamic>;
    return Idea.fromJson(data['idea'] as Map<String, dynamic>);
  }
}
