import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../data/ai_settings_api.dart';

/// 设置页可切换的开关字段。
enum AISettingsField { aiMemory, aiCompletion, speechToText, cloudText, cloudAudio }

sealed class AISettingsState {
  const AISettingsState();
}

class AISettingsLoading extends AISettingsState {
  const AISettingsLoading();
}

class AISettingsLoaded extends AISettingsState {
  const AISettingsLoaded({required this.result, this.saving = false, this.message});

  final AISettingsResult result;
  final bool saving;

  /// 最近一次操作的用户提示；SnackBar 消费后调用 [AISettingsNotifier.clearMessage]。
  final String? message;

  AISettingsLoaded copyWith({
    AISettingsResult? result,
    bool? saving,
    String? message,
    bool clearMessage = false,
  }) {
    return AISettingsLoaded(
      result: result ?? this.result,
      saving: saving ?? this.saving,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class AISettingsError extends AISettingsState {
  const AISettingsError({required this.message});

  final String message;
}

class AISettingsNotifier extends Notifier<AISettingsState> {
  late AISettingsGateway _gateway;

  @override
  AISettingsState build() {
    _gateway = ref.watch(aiSettingsGatewayProvider);
    return const AISettingsLoading();
  }

  Future<void> load() async {
    state = const AISettingsLoading();
    try {
      final result = await _gateway.get();
      state = AISettingsLoaded(result: result);
    } on ApiException catch (e) {
      state = AISettingsError(message: '加载失败: ${e.message}');
    } catch (e) {
      state = AISettingsError(message: '加载失败: $e');
    }
  }

  Future<void> setSwitch(AISettingsField field, bool value) async {
    final current = state;
    if (current is! AISettingsLoaded || current.saving) return;

    final settings = current.result.settings;
    state = current.copyWith(saving: true);
    try {
      final updated = await _gateway.update(
        expectedRevision: settings.revision,
        aiMemoryEnabled: field == AISettingsField.aiMemory ? value : null,
        aiCompletionEnabled: field == AISettingsField.aiCompletion ? value : null,
        speechToTextEnabled: field == AISettingsField.speechToText ? value : null,
        cloudTextAllowed: field == AISettingsField.cloudText ? value : null,
        cloudAudioAllowed: field == AISettingsField.cloudAudio ? value : null,
      );
      state = AISettingsLoaded(
        result: current.result.copyWith(settings: updated),
        message: '已保存',
      );
    } on ApiException catch (e) {
      final message = e.statusCode == 409
          ? '设置已在其他设备修改，请刷新'
          : '保存失败: ${e.message}';
      state = current.copyWith(saving: false, message: message);
    } catch (e) {
      state = current.copyWith(saving: false, message: '保存失败: $e');
    }
  }

  Future<void> reorganize() async {
    final current = state;
    if (current is! AISettingsLoaded || current.saving) return;

    state = current.copyWith(saving: true);
    try {
      await _gateway.reorganize();
      final result = await _gateway.get();
      state = AISettingsLoaded(
        result: result,
        message: '已重新加入整理队列',
      );
    } on ApiException catch (e) {
      final message = e.statusCode == 409
          ? '请先开启 AI 记忆整理'
          : '补整理失败: ${e.message}';
      state = current.copyWith(saving: false, message: message);
    } catch (e) {
      state = current.copyWith(saving: false, message: '补整理失败: $e');
    }
  }

  void clearMessage() {
    final current = state;
    if (current is AISettingsLoaded) {
      state = current.copyWith(clearMessage: true);
    }
  }
}

final aiSettingsNotifierProvider = NotifierProvider<AISettingsNotifier, AISettingsState>(
  AISettingsNotifier.new,
);
