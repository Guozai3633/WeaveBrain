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
    final data = _responseData(resp);
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final ideasList = data['ideas'] as List<dynamic>? ?? [];
    final ideas = ideasList
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
        // Keep the existing request shape until the V3 client replaces this API.
        // ignore: use_null_aware_elements
        if (structuredData != null) 'structured_data': structuredData,
        if (tags != null && tags.isNotEmpty) 'tags': tags,
      },
    );
    final data = _responseData(resp);
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final ideaJson = data['idea'] as Map<String, dynamic>?;
    if (ideaJson == null) {
      throw Exception('创建想法失败: 响应数据为空');
    }
    return Idea.fromJson(ideaJson);
  }

  Future<Idea> getIdea(int id) async {
    final resp = await _client.get('/ideas/$id');
    final data = _responseData(resp);
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final ideaJson = data['idea'] as Map<String, dynamic>?;
    if (ideaJson == null) {
      throw Exception('获取想法失败: 响应数据为空');
    }
    return Idea.fromJson(ideaJson);
  }

  Map<String, dynamic> _responseData(dynamic resp) {
    final data = resp.data;
    if (data is Map<String, dynamic>) return data;
    throw Exception('服务器返回数据格式错误');
  }
}
