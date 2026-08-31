import '../../../shared/api/api_client.dart';
import '../../../shared/models/user.dart';

class UserApi {
  final ApiClient _client;

  UserApi(this._client);

  Future<User> getMe() async {
    final resp = await _client.get('/users/me');
    final data = resp.data;
    if (data is! Map<String, dynamic>) {
      throw Exception('服务器返回数据格式错误');
    }
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final userJson = data['user'] as Map<String, dynamic>?;
    if (userJson == null) {
      throw Exception('获取用户信息失败');
    }
    return User.fromJson(userJson);
  }

  Future<User> updateProfile(
    String userId, {
    String? displayName,
    String? avatarUrl,
  }) async {
    final body = <String, dynamic>{};
    if (displayName != null) body['display_name'] = displayName;
    if (avatarUrl != null) body['avatar_url'] = avatarUrl;

    final resp = await _client.put('/users/$userId/profile', body: body);
    final data = resp.data;
    if (data is! Map<String, dynamic>) {
      throw Exception('服务器返回数据格式错误');
    }
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final userJson = data['user'] as Map<String, dynamic>?;
    if (userJson == null) {
      throw Exception('更新用户信息失败');
    }
    return User.fromJson(userJson);
  }
}
