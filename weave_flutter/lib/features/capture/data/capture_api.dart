import '../../../shared/api/api_client.dart';
import '../domain/local_capture.dart';

class CaptureServerRevision {
  const CaptureServerRevision({
    required this.version,
    required this.processingStatus,
  });

  final int version;
  final ServerProcessingStatus processingStatus;
}

abstract interface class CaptureRemoteGateway {
  Future<CaptureServerRevision> create(LocalCapture capture);
}

class CaptureApi implements CaptureRemoteGateway {
  const CaptureApi(this._client);

  final ApiClient _client;

  @override
  Future<CaptureServerRevision> create(LocalCapture capture) async {
    final response = await _client.post(
      '/captures',
      body: capture.toCreateRequest(),
      headers: {'Idempotency-Key': capture.id},
    );
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('Invalid Capture response');
    }

    final captureJson = data['capture'];
    final cardJson = data['memory_card'];
    if (captureJson is! Map<String, dynamic> ||
        cardJson is! Map<String, dynamic>) {
      throw const FormatException('Capture response is incomplete');
    }

    return CaptureServerRevision(
      version: (captureJson['version'] as num?)?.toInt() ?? 1,
      processingStatus: ServerProcessingStatus.fromWire(
        cardJson['processing_status'] as String?,
      ),
    );
  }
}
