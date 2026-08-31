class ApiException implements Exception {
  final int? statusCode;
  final String message;
  final dynamic data;

  ApiException({this.statusCode, required this.message, this.data});

  @override
  String toString() =>
      'ApiException(statusCode: $statusCode, message: $message)';
}
