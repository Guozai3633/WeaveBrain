import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';

/// 服务端 cadence 取值（镜像后端 entity.EchoCadence）。
const String echoCadenceDaily = 'daily';
const String echoCadenceEveryOtherDay = 'every_other_day';
const String echoCadenceWeekly = 'weekly';

/// 反馈判定取值（镜像后端 entity.EchoFeedbackVerdict）。
const String echoVerdictDone = 'done';
const String echoVerdictLater = 'later';
const String echoVerdictNotRelevant = 'not_relevant';

/// cadence → 相隔天数，用于客户端本地通知排程（服务端读取时仍强制校验）。
int echoCadenceStepDays(String cadence) {
  switch (cadence) {
    case echoCadenceEveryOtherDay:
      return 2;
    case echoCadenceWeekly:
      return 7;
    default:
      return 1;
  }
}

/// 用户回响设置，对应后端 `entity.UserEchoSettings`。默认关闭、每日节奏。
class EchoSettings {
  const EchoSettings({
    this.userId,
    required this.enabled,
    required this.cadence,
    required this.revision,
  });

  factory EchoSettings.fromJson(Map<String, dynamic> json) {
    return EchoSettings(
      userId: json['user_id'] as String?,
      enabled: json['enabled'] as bool? ?? false,
      cadence: json['cadence'] as String? ?? echoCadenceDaily,
      revision: (json['revision'] as num?)?.toInt() ?? 0,
    );
  }

  final String? userId;
  final bool enabled;
  final String cadence;
  final int revision;

  EchoSettings copyWith({bool? enabled, String? cadence, int? revision}) {
    return EchoSettings(
      userId: userId,
      enabled: enabled ?? this.enabled,
      cadence: cadence ?? this.cadence,
      revision: revision ?? this.revision,
    );
  }
}

/// `GET /users/me/echo-settings` 响应信封。
class EchoSettingsResult {
  const EchoSettingsResult({required this.settings});

  factory EchoSettingsResult.fromJson(Map<String, dynamic> json) {
    final settingsJson = json['settings'];
    return EchoSettingsResult(
      settings: settingsJson is Map<String, dynamic>
          ? EchoSettings.fromJson(settingsJson)
          : EchoSettings.fromJson(const <String, dynamic>{}),
    );
  }

  final EchoSettings settings;
}

/// 回响出现的原因（code + 服务端实时渲染的中文说明）。
class EchoReason {
  const EchoReason({required this.code, required this.text});

  factory EchoReason.fromJson(Map<String, dynamic> json) {
    return EchoReason(
      code: json['code'] as String? ?? '',
      text: json['text'] as String? ?? '',
    );
  }

  final String code;
  final String text;
}

/// 回响中的记忆卡片负载；[captureId] 即深链目标 `/memories/:captureId`。
class EchoMemory {
  const EchoMemory({
    required this.captureId,
    required this.kind,
    required this.title,
    required this.primaryType,
    required this.isPinned,
    this.summary,
    this.capturedAt,
  });

  factory EchoMemory.fromJson(Map<String, dynamic> json) {
    return EchoMemory(
      captureId: json['capture_id'] as String? ?? '',
      kind: json['kind'] as String? ?? 'text',
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String?,
      primaryType: json['primary_type'] as String? ?? 'uncategorized',
      capturedAt: _parseDateTime(json['captured_at']),
      isPinned: json['is_pinned'] as bool? ?? false,
    );
  }

  final String captureId;
  final String kind;
  final String title;
  final String? summary;
  final String primaryType;
  final DateTime? capturedAt;
  final bool isPinned;
}

/// 当前回响卡片（echo 子负载）。
class EchoCard {
  const EchoCard({
    required this.id,
    required this.status,
    required this.reason,
    required this.memory,
    required this.createdAt,
  });

  factory EchoCard.fromJson(Map<String, dynamic> json) {
    final reasonJson = json['reason'];
    final memoryJson = json['memory'];
    return EchoCard(
      id: json['id'] as String? ?? '',
      status: json['status'] as String? ?? 'open',
      reason: reasonJson is Map<String, dynamic>
          ? EchoReason.fromJson(reasonJson)
          : const EchoReason(code: '', text: ''),
      memory: memoryJson is Map<String, dynamic>
          ? EchoMemory.fromJson(memoryJson)
          : EchoMemory.fromJson(const <String, dynamic>{}),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String status;
  final EchoReason reason;
  final EchoMemory memory;
  final DateTime createdAt;
}

/// `GET /echoes/current` 响应。echo 为空表示当前无打开回响（关闭 / 窗间 /
/// 无候选），对应 [emptyReason] 或 [nextDueAt]。
class CurrentEchoResult {
  const CurrentEchoResult({
    required this.enabled,
    required this.cadence,
    required this.revision,
    this.echo,
    this.nextDueAt,
    this.emptyReason,
  });

  factory CurrentEchoResult.fromJson(Map<String, dynamic> json) {
    final echoJson = json['echo'];
    return CurrentEchoResult(
      enabled: json['enabled'] as bool? ?? false,
      cadence: json['cadence'] as String? ?? echoCadenceDaily,
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      echo: echoJson is Map<String, dynamic>
          ? EchoCard.fromJson(echoJson)
          : null,
      nextDueAt: _parseDateTime(json['next_due_at']),
      emptyReason: json['empty_reason'] as String?,
    );
  }

  final bool enabled;
  final String cadence;
  final int revision;
  final EchoCard? echo;
  final DateTime? nextDueAt;
  final String? emptyReason;
}

/// 反馈提交结果。
class EchoFeedbackResult {
  const EchoFeedbackResult({
    required this.echoId,
    required this.status,
    required this.nextDueAt,
  });

  factory EchoFeedbackResult.fromJson(Map<String, dynamic> json) {
    return EchoFeedbackResult(
      echoId: json['echo_id'] as String? ?? '',
      status: json['status'] as String? ?? '',
      nextDueAt: _parseDateTime(json['next_due_at']) ?? DateTime.now(),
    );
  }

  final String echoId;
  final String status;
  final DateTime nextDueAt;
}

/// 回响读取与反馈的数据访问抽象，便于测试替换。
abstract interface class EchoGateway {
  Future<CurrentEchoResult> getCurrent();

  Future<EchoFeedbackResult> submitFeedback({
    required String echoId,
    required String verdict,
  });
}

/// 回响设置的数据访问抽象，便于测试替换。
abstract interface class EchoSettingsGateway {
  Future<EchoSettingsResult> get();

  Future<EchoSettings> update({
    required int expectedRevision,
    bool? enabled,
    String? cadence,
  });
}

class EchoApi implements EchoGateway {
  const EchoApi(this._client);

  final ApiClient _client;

  @override
  Future<CurrentEchoResult> getCurrent() async {
    final response = await _client.get('/echoes/current');
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的回响响应');
    }
    return CurrentEchoResult.fromJson(data);
  }

  @override
  Future<EchoFeedbackResult> submitFeedback({
    required String echoId,
    required String verdict,
  }) async {
    final response = await _client.post(
      '/echoes/$echoId/feedback',
      body: {'verdict': verdict},
    );
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的反馈响应');
    }
    return EchoFeedbackResult.fromJson(data);
  }
}

class EchoSettingsApi implements EchoSettingsGateway {
  const EchoSettingsApi(this._client);

  final ApiClient _client;

  @override
  Future<EchoSettingsResult> get() async {
    final response = await _client.get('/users/me/echo-settings');
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的回响设置响应');
    }
    return EchoSettingsResult.fromJson(data);
  }

  @override
  Future<EchoSettings> update({
    required int expectedRevision,
    bool? enabled,
    String? cadence,
  }) async {
    final body = <String, dynamic>{'expected_revision': expectedRevision};
    if (enabled != null) body['enabled'] = enabled;
    if (cadence != null) body['cadence'] = cadence;

    final response = await _client.patch(
      '/users/me/echo-settings',
      body: body,
    );
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的回响设置响应');
    }
    final settingsJson = data['settings'];
    if (settingsJson is! Map<String, dynamic>) {
      throw const FormatException('回响设置响应缺少 settings');
    }
    return EchoSettings.fromJson(settingsJson);
  }
}

DateTime? _parseDateTime(dynamic value) {
  if (value is! String || value.isEmpty) return null;
  return DateTime.tryParse(value)?.toLocal();
}

/// v3 ApiClient（与 capture 特性共用同一 host）。
final echoApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final echoGatewayProvider = Provider<EchoGateway>((ref) {
  return EchoApi(ref.watch(echoApiClientProvider));
});

final echoSettingsGatewayProvider = Provider<EchoSettingsGateway>((ref) {
  return EchoSettingsApi(ref.watch(echoApiClientProvider));
});
