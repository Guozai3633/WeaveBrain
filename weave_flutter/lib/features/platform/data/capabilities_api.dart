import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';

/// 服务端能力声明，对应后端 `GET /api/v3/capabilities`。
///
/// MVP 中 `workflowDesigner` / `workflowExecution` 恒为 false：客户端据此呈现
/// 「规划中」而非可用的工作流编辑器。取值来自服务器，客户端不自行推断。
class PlatformCapabilities {
  const PlatformCapabilities({
    required this.mobileCapture,
    required this.webReview,
    required this.workflowDesigner,
    required this.workflowExecution,
    required this.supportedCaptureSources,
  });

  /// 服务端不可达或响应异常时的安全回退：全部能力关闭，绝不暗示工作流可用。
  const PlatformCapabilities.disabled()
    : mobileCapture = false,
      webReview = false,
      workflowDesigner = false,
      workflowExecution = false,
      supportedCaptureSources = const [];

  factory PlatformCapabilities.fromJson(Map<String, dynamic> json) {
    final sources = json['supported_capture_sources'];
    return PlatformCapabilities(
      mobileCapture: json['mobile_capture'] as bool? ?? false,
      webReview: json['web_review'] as bool? ?? false,
      workflowDesigner: json['workflow_designer'] as bool? ?? false,
      workflowExecution: json['workflow_execution'] as bool? ?? false,
      supportedCaptureSources: sources is List
          ? sources.whereType<String>().toList(growable: false)
          : const <String>[],
    );
  }

  final bool mobileCapture;
  final bool webReview;
  final bool workflowDesigner;
  final bool workflowExecution;
  final List<String> supportedCaptureSources;

  bool get workflowAvailable => workflowDesigner || workflowExecution;
}

/// 数据访问抽象，便于测试替换。
abstract interface class CapabilitiesGateway {
  Future<PlatformCapabilities> fetch();
}

class CapabilitiesApi implements CapabilitiesGateway {
  const CapabilitiesApi(this._client);

  final ApiClient _client;

  @override
  Future<PlatformCapabilities> fetch() async {
    final response = await _client.get('/capabilities');
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的能力响应');
    }
    return PlatformCapabilities.fromJson(data);
  }
}

/// 与其它 v3 特性共用的公共 /api/v3 host。
final capabilitiesApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final capabilitiesGatewayProvider = Provider<CapabilitiesGateway>((ref) {
  return CapabilitiesApi(ref.watch(capabilitiesApiClientProvider));
});
