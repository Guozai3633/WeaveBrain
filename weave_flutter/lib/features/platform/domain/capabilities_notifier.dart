import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/capabilities_api.dart';

/// 服务端能力快照。
///
/// 拉取失败时静默回退到全关闭（`PlatformCapabilities.disabled()`），UI 依此
/// 呈现「规划中」，不让一次网络抖动把工作流占位页变成可用的假入口。
final capabilitiesProvider = FutureProvider<PlatformCapabilities>((ref) async {
  final gateway = ref.watch(capabilitiesGatewayProvider);
  try {
    return await gateway.fetch();
  } catch (_) {
    return const PlatformCapabilities.disabled();
  }
});

/// 便捷：`workflowDesigner/workflowExecution` 是否可用（MVP 恒 false）。
bool workflowPlannedOnly(PlatformCapabilities capabilities) =>
    !capabilities.workflowAvailable;
