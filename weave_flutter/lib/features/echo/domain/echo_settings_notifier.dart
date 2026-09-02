import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../data/echo_api.dart';
import 'echo_reminder_prefs.dart';
import 'echo_scheduler.dart';

sealed class EchoSettingsState {
  const EchoSettingsState();
}

class EchoSettingsLoading extends EchoSettingsState {
  const EchoSettingsLoading();
}

class EchoSettingsLoaded extends EchoSettingsState {
  const EchoSettingsLoaded({
    required this.result,
    this.saving = false,
    this.message,
  });

  final EchoSettingsResult result;
  final bool saving;

  /// 最近一次操作的用户提示；SnackBar 消费后调用
  /// [EchoSettingsNotifier.clearMessage]。
  final String? message;

  EchoSettingsLoaded copyWith({
    EchoSettingsResult? result,
    bool? saving,
    String? message,
    bool clearMessage = false,
  }) {
    return EchoSettingsLoaded(
      result: result ?? this.result,
      saving: saving ?? this.saving,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class EchoSettingsError extends EchoSettingsState {
  const EchoSettingsError({required this.message});

  final String message;
}

/// 回响设置：服务端 enabled+cadence（revision CAS）。任何成功变更都会重排
/// 本地提醒通知（[echoSchedulerProvider]）。
class EchoSettingsNotifier extends Notifier<EchoSettingsState> {
  // 非 final：build() 在 gateway provider 变化时可能重跑，需要可重赋值。
  late EchoSettingsGateway _gateway;

  @override
  EchoSettingsState build() {
    _gateway = ref.watch(echoSettingsGatewayProvider);
    return const EchoSettingsLoading();
  }

  Future<void> load() async {
    state = const EchoSettingsLoading();
    try {
      final result = await _gateway.get();
      state = EchoSettingsLoaded(result: result);
    } on ApiException catch (e) {
      state = EchoSettingsError(message: '加载失败: ${e.message}');
    } catch (e) {
      state = EchoSettingsError(message: '加载失败: $e');
    }
  }

  Future<void> setEnabled(bool value) async {
    await _update(enabled: value);
  }

  Future<void> setCadence(String cadence) async {
    await _update(cadence: cadence);
  }

  Future<void> _update({bool? enabled, String? cadence}) async {
    final current = state;
    if (current is! EchoSettingsLoaded || current.saving) return;
    final settings = current.result.settings;
    state = current.copyWith(saving: true);
    try {
      final updated = await _gateway.update(
        expectedRevision: settings.revision,
        enabled: enabled,
        cadence: cadence,
      );
      final loaded = EchoSettingsResult(settings: updated);
      state = EchoSettingsLoaded(result: loaded, message: '已保存');
      await _reschedule(loaded);
    } on ApiException catch (e) {
      final message = e.statusCode == 409
          ? '设置已在其他设备修改，请刷新'
          : '保存失败: ${e.message}';
      state = current.copyWith(saving: false, message: message);
    } catch (e) {
      state = current.copyWith(saving: false, message: '保存失败: $e');
    }
  }

  Future<void> _reschedule(EchoSettingsResult result) async {
    try {
      final prefs = await EchoReminderPrefs.load();
      await ref
          .read(echoSchedulerProvider)
          .refresh(enabled: result.settings.enabled, cadence: result.settings.cadence, prefs: prefs);
    } catch (_) {
      // 本地通知重排是尽力而为；失败不影响服务端设置已保存。
    }
  }

  void clearMessage() {
    final current = state;
    if (current is EchoSettingsLoaded) {
      state = current.copyWith(clearMessage: true);
    }
  }
}

final echoSettingsNotifierProvider = NotifierProvider<EchoSettingsNotifier, EchoSettingsState>(
  EchoSettingsNotifier.new,
);
