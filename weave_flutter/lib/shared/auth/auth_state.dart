import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_client.dart';
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
    final resp = await _apiClient.post('/auth/login', body: {
      'provider': provider,
      'provider_id': providerId,
    });
    final data = resp.data as Map<String, dynamic>;
    final token = data['token'] as String;
    final user = User.fromJson(data['user'] as Map<String, dynamic>);
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
    final data = resp.data as Map<String, dynamic>;
    final token = data['token'] as String;
    final user = User.fromJson(data['user'] as Map<String, dynamic>);
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
  return ApiClient(
    baseUrl: 'http://10.0.2.2:8080/api/v1',
    authRepository: authRepo,
  );
});

final authNotifierProvider =
    NotifierProvider<AuthNotifier, AuthState>(() => AuthNotifier());
