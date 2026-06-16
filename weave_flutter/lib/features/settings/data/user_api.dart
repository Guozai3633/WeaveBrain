import '../../../shared/api/api_client.dart';
import '../../../shared/models/user.dart';

class UserApi {
  final ApiClient _client;

  UserApi(this._client);

  Future<User> getMe() async {
    final resp = await _client.get('/users/me');
    final data = resp.data as Map<String, dynamic>;
    return User.fromJson(data['user'] as Map<String, dynamic>);
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
    final data = resp.data as Map<String, dynamic>;
    return User.fromJson(data['user'] as Map<String, dynamic>);
  }
}
