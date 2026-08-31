import '../../../shared/api/api_client.dart';
import '../../../shared/models/project.dart';

class ProjectApi {
  final ApiClient _client;

  ProjectApi(this._client);

  Future<List<Project>> listProjects() async {
    final resp = await _client.get('/projects');
    final data = _responseData(resp);
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final items =
        data['data'] as List<dynamic>? ??
        data['projects'] as List<dynamic>? ??
        [];
    return items
        .map((e) => Project.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Project> createProject({
    required String name,
    bool defaultProject = false,
  }) async {
    final resp = await _client.post(
      '/projects',
      body: {'name': name, 'default_project': defaultProject},
    );
    final data = _responseData(resp);
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final projectJson = data['project'] as Map<String, dynamic>?;
    if (projectJson == null) {
      throw Exception('创建项目失败: 响应数据为空');
    }
    return Project.fromJson(projectJson);
  }

  Future<void> updateProject(
    int id, {
    String? name,
    bool? defaultProject,
  }) async {
    final body = <String, dynamic>{};
    if (name != null) body['name'] = name;
    if (defaultProject != null) body['default_project'] = defaultProject;
    await _client.put('/projects/$id', body: body);
  }

  Future<void> deleteProject(int id) async {
    await _client.delete('/projects/$id');
  }

  Map<String, dynamic> _responseData(dynamic resp) {
    final data = resp.data;
    if (data is Map<String, dynamic>) return data;
    throw Exception('服务器返回数据格式错误');
  }
}
