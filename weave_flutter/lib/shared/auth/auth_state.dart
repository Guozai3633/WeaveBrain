import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_client.dart';
import '../api/api_host.dart';
import '../auth/auth_repository.dart';
import '../models/user.dart';

sealed class AuthState {
  const AuthState();
}

class AuthInitial extends AuthState {
  const AuthInitial();
}

class Authenticated extends AuthState {
  final String token;
  final User user;

  const Authenticated({required this.token, required this.user});
}

class Unauthenticated extends AuthState {
  const Unauthenticated();
}

class AuthNotifier extends Notifier<AuthState> {
  late final AuthRepository _authRepo;
  late final ApiClient _apiClient;

  @override
  AuthState build() {
    _authRepo = ref.watch(authRepositoryProvider);
    _apiClient = ref.watch(apiClientProvider);
    return const AuthInitial();
  }

  Future<void> checkAuth() async {
    final token = await _authRepo.getToken();
    final user = await _authRepo.getUser();
    if (token != null && user != null) {
      state = Authenticated(token: token, user: user);
    } else {
      state = const Unauthenticated();
    }
  }

  Future<void> login(String provider, String providerId) async {
    final resp = await _apiClient.post(
      '/auth/login',
      body: {'provider': provider, 'provider_id': providerId},
    );
    final data = resp.data;
    if (data is! Map<String, dynamic>) {
      throw Exception('服务器返回数据格式错误');
    }
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final token = data['token'] as String?;
    final userJson = data['user'] as Map<String, dynamic>?;
    if (token == null || userJson == null) {
      throw Exception('登录失败: 响应数据不完整');
    }
    final user = User.fromJson(userJson);
    await _authRepo.saveToken(token);
    await _authRepo.saveUser(user);
    state = Authenticated(token: token, user: user);
  }

  Future<void> register(
    String provider,
    String providerId, {
    String? displayName,
  }) async {
    final body = <String, dynamic>{
      'provider': provider,
      'provider_id': providerId,
    };
    if (displayName != null) body['display_name'] = displayName;

    final resp = await _apiClient.post('/auth/register', body: body);
    final data = resp.data;
    if (data is! Map<String, dynamic>) {
      throw Exception('服务器返回数据格式错误');
    }
    if (data.containsKey('error')) {
      throw Exception(data['error']);
    }
    final token = data['token'] as String?;
    final userJson = data['user'] as Map<String, dynamic>?;
    if (token == null || userJson == null) {
      throw Exception('注册失败: 响应数据不完整');
    }
    final user = User.fromJson(userJson);
    await _authRepo.saveToken(token);
    await _authRepo.saveUser(user);
    state = Authenticated(token: token, user: user);
  }

  Future<void> updateUser(User updatedUser) async {
    await _authRepo.saveUser(updatedUser);
    final current = state;
    if (current is Authenticated) {
      state = Authenticated(token: current.token, user: updatedUser);
    }
  }

  Future<void> logout() async {
    await _authRepo.clearAll();
    state = const Unauthenticated();
  }
}

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository();
});

final apiClientProvider = Provider<ApiClient>((ref) {
  final authRepo = ref.watch(authRepositoryProvider);
  return ApiClient(baseUrl: 'http://$apiHost/api/v1', authRepository: authRepo);
});

final authNotifierProvider = NotifierProvider<AuthNotifier, AuthState>(
  () => AuthNotifier(),
);
