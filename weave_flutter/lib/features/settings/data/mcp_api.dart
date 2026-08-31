import '../../../shared/api/api_client.dart';

class McpConfig {
  final String namespace;
  final bool enabled;
  final bool isConfigured;

  McpConfig({
    required this.namespace,
    required this.enabled,
    required this.isConfigured,
  });

  factory McpConfig.fromJson(Map<String, dynamic> json) {
    return McpConfig(
      namespace: json['namespace'] as String,
      enabled: json['enabled'] as bool,
      isConfigured: json['is_configured'] as bool,
    );
  }
}

class McpApi {
  final ApiClient _client;

  McpApi(this._client);

  Future<List<McpConfig>> getConfigs() async {
    final resp = await _client.get('/users/me/mcp-configs');
    final data = resp.data;
    if (data is! List) {
      throw Exception('服务器返回数据格式错误');
    }
    return data
        .map((e) => McpConfig.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<void> upsertConfig(
    String namespace,
    String credentials,
    bool enabled,
  ) async {
    final body = {'credentials': credentials, 'enabled': enabled};
    final resp = await _client.put(
      '/users/me/mcp-configs/$namespace',
      body: body,
    );
    if (resp.statusCode != 200) {
      throw Exception('保存配置失败');
    }
  }

  Future<void> deleteConfig(String namespace) async {
    final resp = await _client.delete('/users/me/mcp-configs/$namespace');
    if (resp.statusCode != 200) {
      throw Exception('删除配置失败');
    }
  }
}
