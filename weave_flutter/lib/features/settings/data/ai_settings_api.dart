import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';

/// 用户 AI 设置开关，对应后端 `entity.UserAISettings`。
///
/// 所有开关默认关闭（隐私优先）。`revision` 随每次更新递增，用于乐观并发
/// （跨设备检测过期写入）。
class AISettings {
  const AISettings({
    this.userId,
    required this.aiMemoryEnabled,
    required this.aiCompletionEnabled,
    required this.speechToTextEnabled,
    required this.cloudTextAllowed,
    required this.cloudAudioAllowed,
    required this.revision,
  });

  factory AISettings.fromJson(Map<String, dynamic> json) {
    return AISettings(
      userId: json['user_id'] as String?,
      aiMemoryEnabled: json['ai_memory_enabled'] as bool? ?? false,
      aiCompletionEnabled: json['ai_completion_enabled'] as bool? ?? false,
      speechToTextEnabled: json['speech_to_text_enabled'] as bool? ?? false,
      cloudTextAllowed: json['cloud_text_allowed'] as bool? ?? false,
      cloudAudioAllowed: json['cloud_audio_allowed'] as bool? ?? false,
      revision: (json['revision'] as num?)?.toInt() ?? 0,
    );
  }

  final String? userId;
  final bool aiMemoryEnabled;
  final bool aiCompletionEnabled;
  final bool speechToTextEnabled;
  final bool cloudTextAllowed;
  final bool cloudAudioAllowed;
  final int revision;

  AISettings copyWith({
    String? userId,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
    int? revision,
  }) {
    return AISettings(
      userId: userId ?? this.userId,
      aiMemoryEnabled: aiMemoryEnabled ?? this.aiMemoryEnabled,
      aiCompletionEnabled: aiCompletionEnabled ?? this.aiCompletionEnabled,
      speechToTextEnabled: speechToTextEnabled ?? this.speechToTextEnabled,
      cloudTextAllowed: cloudTextAllowed ?? this.cloudTextAllowed,
      cloudAudioAllowed: cloudAudioAllowed ?? this.cloudAudioAllowed,
      revision: revision ?? this.revision,
    );
  }
}

/// GET `/users/me/ai-settings` 响应信封。
class AISettingsResult {
  const AISettingsResult({
    required this.settings,
    required this.pendingReorganize,
  });

  factory AISettingsResult.fromJson(Map<String, dynamic> json) {
    final settingsJson = json['settings'];
    return AISettingsResult(
      settings: settingsJson is Map<String, dynamic>
          ? AISettings.fromJson(settingsJson)
          : AISettings.fromJson(const <String, dynamic>{}),
      pendingReorganize: (json['pending_reorganize'] as num?)?.toInt() ?? 0,
    );
  }

  final AISettings settings;
  final int pendingReorganize;

  AISettingsResult copyWith({AISettings? settings, int? pendingReorganize}) {
    return AISettingsResult(
      settings: settings ?? this.settings,
      pendingReorganize: pendingReorganize ?? this.pendingReorganize,
    );
  }
}

/// 数据访问抽象，便于测试替换。
abstract interface class AISettingsGateway {
  Future<AISettingsResult> get();

  Future<AISettings> update({
    required int expectedRevision,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
  });

  /// 显式补整理 AI 关闭期间创建的记忆，返回重新入队数量。
  Future<int> reorganize();
}

class AISettingsApi implements AISettingsGateway {
  const AISettingsApi(this._client);

  final ApiClient _client;

  @override
  Future<AISettingsResult> get() async {
    final response = await _client.get('/users/me/ai-settings');
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的 AI 设置响应');
    }
    return AISettingsResult.fromJson(data);
  }

  @override
  Future<AISettings> update({
    required int expectedRevision,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
  }) async {
    final body = <String, dynamic>{'expected_revision': expectedRevision};
    if (aiMemoryEnabled != null) {
      body['ai_memory_enabled'] = aiMemoryEnabled;
    }
    if (aiCompletionEnabled != null) {
      body['ai_completion_enabled'] = aiCompletionEnabled;
    }
    if (speechToTextEnabled != null) {
      body['speech_to_text_enabled'] = speechToTextEnabled;
    }
    if (cloudTextAllowed != null) {
      body['cloud_text_allowed'] = cloudTextAllowed;
    }
    if (cloudAudioAllowed != null) {
      body['cloud_audio_allowed'] = cloudAudioAllowed;
    }

    final response = await _client.patch(
      '/users/me/ai-settings',
      body: body,
    );
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的 AI 设置响应');
    }
    final settingsJson = data['settings'];
    if (settingsJson is! Map<String, dynamic>) {
      throw const FormatException('AI 设置响应缺少 settings');
    }
    return AISettings.fromJson(settingsJson);
  }

  @override
  Future<int> reorganize() async {
    final response = await _client.post('/users/me/ai-settings/reorganize');
    final data = response.data;
    if (data is! Map<String, dynamic>) {
      throw const FormatException('无效的补整理响应');
    }
    return (data['reorganized'] as num?)?.toInt() ?? 0;
  }
}

/// v3 ApiClient（与 capture 特性共用同一 host）。
final aiSettingsApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final aiSettingsGatewayProvider = Provider<AISettingsGateway>((ref) {
  return AISettingsApi(ref.watch(aiSettingsApiClientProvider));
});
