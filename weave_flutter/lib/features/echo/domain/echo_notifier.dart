import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../../../shared/auth/auth_state.dart';
import '../data/echo_api.dart';

/// 回响页的展示状态。
sealed class EchoState {
  const EchoState();
}

class EchoLoading extends EchoState {
  const EchoLoading();
}

/// 游客：回响需要登录，本页只展示登录引导，不发请求。
class EchoGuest extends EchoState {
  const EchoGuest();
}

/// 服务端设置已关闭回响。
class EchoDisabled extends EchoState {
  const EchoDisabled({required this.cadence, required this.revision});

  final String cadence;
  final int revision;
}

/// 已开启但当前无打开回响：`off_period`（下次到点 [nextDueAt]）或
/// `no_candidates`（还没有可回看的记忆）。
class EchoEmpty extends EchoState {
  const EchoEmpty({
    required this.cadence,
    required this.revision,
    this.nextDueAt,
    this.emptyReason,
  });

  final String cadence;
  final int revision;
  final DateTime? nextDueAt;
  final String? emptyReason;

  bool get noCandidates => emptyReason == 'no_candidates';
}

/// 有一张待回应的记忆卡片。
class EchoLoaded extends EchoState {
  const EchoLoaded({
    required this.card,
    required this.cadence,
    required this.revision,
    this.feedbackSaving = false,
    this.message,
  });

  final EchoCard card;
  final String cadence;
  final int revision;
  final bool feedbackSaving;

  /// 最近一次反馈的用户提示；SnackBar 消费后调用 [EchoNotifier.clearMessage]。
  final String? message;

  EchoLoaded copyWith({
    EchoCard? card,
    bool? feedbackSaving,
    String? message,
    bool clearMessage = false,
  }) {
    return EchoLoaded(
      card: card ?? this.card,
      cadence: cadence,
      revision: revision,
      feedbackSaving: feedbackSaving ?? this.feedbackSaving,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class EchoError extends EchoState {
  const EchoError({required this.message});

  final String message;
}

class EchoNotifier extends Notifier<EchoState> {
  // 非 final：build() 在 gateway provider 变化时可能重跑，需要可重赋值。
  late EchoGateway _gateway;

  @override
  EchoState build() {
    _gateway = ref.watch(echoGatewayProvider);
    return const EchoLoading();
  }

  /// 拉取当前回响。游客短路为 [EchoGuest]，不发起任何请求。
  Future<void> load() async {
    final auth = ref.read(authNotifierProvider);
    if (auth is! Authenticated) {
      state = const EchoGuest();
      return;
    }
    state = const EchoLoading();
    try {
      final result = await _gateway.getCurrent();
      if (!result.enabled) {
        state = EchoDisabled(cadence: result.cadence, revision: result.revision);
        return;
      }
      final echo = result.echo;
      if (echo == null) {
        state = EchoEmpty(
          cadence: result.cadence,
          revision: result.revision,
          nextDueAt: result.nextDueAt,
          emptyReason: result.emptyReason,
        );
        return;
      }
      state = EchoLoaded(
        card: echo,
        cadence: result.cadence,
        revision: result.revision,
      );
    } on ApiException catch (e) {
      state = e.statusCode == 401
          ? const EchoGuest()
          : EchoError(message: '加载失败: ${e.message}');
    } catch (e) {
      state = EchoError(message: '加载失败: $e');
    }
  }

  /// 提交 done/later/not_relevant 反馈；成功后重载以反映服务端真值
  /// （通常落在 off-period / no_candidates 或下一张卡片）。
  Future<void> feedback(String verdict) async {
    final current = state;
    if (current is! EchoLoaded || current.feedbackSaving) return;
    state = current.copyWith(feedbackSaving: true);
    try {
      await _gateway.submitFeedback(echoId: current.card.id, verdict: verdict);
      await load();
    } on ApiException catch (e) {
      final message = e.statusCode == 409
          ? '这条回响已在其他设备处理'
          : '提交失败: ${e.message}';
      state = current.copyWith(feedbackSaving: false, message: message);
    } catch (e) {
      state = current.copyWith(feedbackSaving: false, message: '提交失败: $e');
    }
  }

  void clearMessage() {
    final current = state;
    if (current is EchoLoaded) {
      state = current.copyWith(clearMessage: true);
    }
  }
}

final echoNotifierProvider = NotifierProvider<EchoNotifier, EchoState>(
  EchoNotifier.new,
);
