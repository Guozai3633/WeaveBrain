// ignore_for_file: prefer_initializing_formals

import 'dart:convert';
import 'dart:developer' as dev;
import 'dart:io';
import 'dart:typed_data';

import '../auth/auth_repository.dart';
import 'api_exception.dart';

class ApiResponse {
  final dynamic data;
  final int? statusCode;
  ApiResponse({this.data, this.statusCode});
}

class ApiClient {
  final String _baseUrl;
  final AuthRepository _authRepository;
  final HttpClient _httpClient = HttpClient();

  ApiClient({required String baseUrl, required AuthRepository authRepository})
    : _baseUrl = baseUrl,
      _authRepository = authRepository;

  Future<ApiResponse> get(String path, {Map<String, dynamic>? queryParams}) {
    return _request('GET', _buildUri(path, queryParams));
  }

  Future<ApiResponse> post(
    String path, {
    dynamic body,
    Map<String, dynamic>? queryParams,
    Map<String, String>? headers,
  }) {
    return _request(
      'POST',
      _buildUri(path, queryParams),
      body: body,
      headers: headers,
    );
  }

  Future<ApiResponse> put(String path, {dynamic body}) {
    return _request('PUT', _buildUri(path), body: body);
  }

  Future<ApiResponse> patch(String path, {dynamic body}) {
    return _request('PATCH', _buildUri(path), body: body);
  }

  /// Sends raw binary bytes (e.g. an audio chunk) via PUT without JSON
  /// encoding. [headers] may carry chunk checksum headers.
  Future<ApiResponse> putBytes(
    String path, {
    required Uint8List body,
    Map<String, String>? headers,
  }) {
    return _requestRaw('PUT', _buildUri(path), body: body, headers: headers);
  }

  Future<ApiResponse> delete(String path) {
    return _request('DELETE', _buildUri(path));
  }

  String _buildUri(String path, [Map<String, dynamic>? queryParams]) {
    var url = '$_baseUrl$path';
    if (queryParams != null && queryParams.isNotEmpty) {
      final pairs = queryParams.entries
          .map(
            (e) =>
                '${Uri.encodeComponent(e.key)}=${Uri.encodeComponent(e.value.toString())}',
          )
          .join('&');
      url = '$url?$pairs';
    }
    return url;
  }

  Future<ApiResponse> _request(
    String method,
    String url, {
    dynamic body,
    Map<String, String>? headers,
  }) async {
    final token = await _authRepository.getToken();

    dev.log('[API] $method $url');

    try {
      final uri = Uri.parse(url);
      final request = await _httpClient.openUrl(method, uri);

      request.headers.set('Content-Type', 'application/json');
      if (token != null && token.isNotEmpty) {
        request.headers.set('Authorization', 'Bearer $token');
      }

      for (final entry
          in headers?.entries ?? const <MapEntry<String, String>>[]) {
        request.headers.set(entry.key, entry.value);
      }

      if (body != null) {
        request.write(jsonEncode(body));
      }

      final response = await request.close();
      final statusCode = response.statusCode;
      dev.log('[API] $method $url => $statusCode');

      if (statusCode < 200 || statusCode >= 300) {
        if (statusCode == 401) {
          _authRepository.clearAll();
        }
        final responseBody = await response.transform(utf8.decoder).join();
        throw ApiException(
          statusCode: statusCode,
          message: 'HTTP $statusCode',
          data: responseBody,
        );
      }

      final text = await response.transform(utf8.decoder).join();
      if (text.isEmpty) {
        return ApiResponse(data: <String, dynamic>{}, statusCode: statusCode);
      }

      final decoded = jsonDecode(text);
      if (decoded is Map<String, dynamic>) {
        return ApiResponse(data: decoded, statusCode: statusCode);
      }
      return ApiResponse(
        data: <String, dynamic>{'data': decoded},
        statusCode: statusCode,
      );
    } catch (e) {
      if (e is ApiException) rethrow;
      dev.log('[API ERROR] $method $url => $e');
      throw ApiException(message: e.toString());
    }
  }

  /// Raw-bytes variant used by chunked audio upload. Writes [body] verbatim
  /// with an octet-stream content type and does not JSON-encode the payload.
  Future<ApiResponse> _requestRaw(
    String method,
    String url, {
    required Uint8List body,
    Map<String, String>? headers,
  }) async {
    final token = await _authRepository.getToken();

    dev.log('[API] $method $url (raw bytes: ${body.length})');

    try {
      final uri = Uri.parse(url);
      final request = await _httpClient.openUrl(method, uri);

      request.headers.set('Content-Type', 'application/octet-stream');
      if (token != null && token.isNotEmpty) {
        request.headers.set('Authorization', 'Bearer $token');
      }
      for (final entry
          in headers?.entries ?? const <MapEntry<String, String>>[]) {
        request.headers.set(entry.key, entry.value);
      }

      request.add(body);

      final response = await request.close();
      final statusCode = response.statusCode;
      dev.log('[API] $method $url => $statusCode');

      if (statusCode < 200 || statusCode >= 300) {
        if (statusCode == 401) {
          _authRepository.clearAll();
        }
        final responseBody = await response.transform(utf8.decoder).join();
        throw ApiException(
          statusCode: statusCode,
          message: 'HTTP $statusCode',
          data: responseBody,
        );
      }

      final text = await response.transform(utf8.decoder).join();
      if (text.isEmpty) {
        return ApiResponse(data: <String, dynamic>{}, statusCode: statusCode);
      }
      return ApiResponse(
        data: <String, dynamic>{'data': jsonDecode(text)},
        statusCode: statusCode,
      );
    } catch (e) {
      if (e is ApiException) rethrow;
      dev.log('[API ERROR] $method $url => $e');
      throw ApiException(message: e.toString());
    }
  }
}
