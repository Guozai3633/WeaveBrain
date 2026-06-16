import 'package:dio/dio.dart';

import '../auth/auth_repository.dart';
import 'api_exception.dart';

class ApiClient {
  late final Dio _dio;
  final AuthRepository _authRepository;

  ApiClient({
    required String baseUrl,
    required AuthRepository authRepository,
  }) : _authRepository = authRepository {
    _dio = Dio(BaseOptions(
      baseUrl: baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
      headers: {'Content-Type': 'application/json'},
    ));

    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: _onRequest,
      onError: _onError,
    ));
  }

  Dio get dio => _dio;

  Future<void> _onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    final token = await _authRepository.getToken();
    if (token != null && token.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  void _onError(DioException err, ErrorInterceptorHandler handler) {
    if (err.response?.statusCode == 401) {
      _authRepository.clearAll();
    }
    handler.next(err);
  }

  ApiException _toApiException(DioException err) {
    final statusCode = err.response?.statusCode;
    String message;
    dynamic data;

    if (err.response?.data is Map<String, dynamic>) {
      final map = err.response!.data as Map<String, dynamic>;
      message = map['error']?.toString() ?? map['message']?.toString() ?? err.message ?? 'Unknown error';
      data = map;
    } else {
      message = err.message ?? 'Network error';
    }

    return ApiException(statusCode: statusCode, message: message, data: data);
  }

  Future<Response> get(String path, {Map<String, dynamic>? queryParams}) {
    return _dio.get(path, queryParameters: queryParams).catchError((e) {
      if (e is DioException) throw _toApiException(e);
      throw ApiException(message: e.toString());
    });
  }

  Future<Response> post(String path, {dynamic body, Map<String, dynamic>? queryParams}) {
    return _dio.post(path, data: body, queryParameters: queryParams).catchError((e) {
      if (e is DioException) throw _toApiException(e);
      throw ApiException(message: e.toString());
    });
  }

  Future<Response> put(String path, {dynamic body}) {
    return _dio.put(path, data: body).catchError((e) {
      if (e is DioException) throw _toApiException(e);
      throw ApiException(message: e.toString());
    });
  }

  Future<Response> delete(String path) {
    return _dio.delete(path).catchError((e) {
      if (e is DioException) throw _toApiException(e);
      throw ApiException(message: e.toString());
    });
  }
}
