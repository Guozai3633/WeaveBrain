import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../data/completion_api.dart';

sealed class CompletionState {
  const CompletionState();
}

/// 尚未生成预览（仅显示入口按钮）。
class CompletionInitial extends CompletionState {
  const CompletionInitial();
}

class CompletionLoading extends CompletionState {
  const CompletionLoading();
}

class CompletionLoaded extends CompletionState {
  const CompletionLoaded({this.preview, this.busy = false, this.message});

  /// 最近一次预览；apply/undo 成功后清空以收起提案列表。
  final CompletionPreviewResultModel? preview;
  final bool busy;

  /// 最近一次操作的用户提示；SnackBar 消费后调用 [CompletionNotifier.clearMessage]。
  final String? message;

  CompletionLoaded copyWith({
    CompletionPreviewResultModel? preview,
    bool clearPreview = false,
    bool? busy,
    String? message,
    bool clearMessage = false,
  }) {
    return CompletionLoaded(
      preview: clearPreview ? null : (preview ?? this.preview),
      busy: busy ?? this.busy,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class CompletionError extends CompletionState {
  const CompletionError({required this.message});

  final String message;
}

class CompletionNotifier extends Notifier<CompletionState> {
  CompletionPreviewResultModel? _preview;

  CompletionGateway get _gateway => ref.read(completionGatewayProvider);

  @override
  CompletionState build() => const CompletionInitial();

  /// 生成补全预览（不修改业务对象）。成功发布 Loaded(preview)。
  Future<void> loadPreview(String captureId) async {
    state = const CompletionLoading();
    try {
      final preview = await _gateway.preview(captureId);
      _preview = preview;
      state = CompletionLoaded(preview: preview);
    } catch (e) {
      state = CompletionError(message: _completionError(e));
    }
  }

  /// 全部采用 safe_auto 提案。无 safe_auto 时返回 null；成功返回结果供面板刷新。
  Future<CompletionApplyResultModel?> applyAllSafe(String captureId) async {
    final preview = _preview;
    if (preview == null) return null;
    final ids = <String>[
      for (final p in preview.proposals)
        if (p.canAutoApply) p.id,
    ];
    if (ids.isEmpty) return null;
    return _apply(captureId, ids, preview.sourceRevision);
  }

  /// 逐字段采用一条提案。
  Future<CompletionApplyResultModel?> applyOne(
    String captureId,
    CompletionProposalModel proposal,
  ) async {
    final preview = _preview;
    final sourceRevision = preview?.sourceRevision ?? proposal.sourceRevision;
    return _apply(captureId, [proposal.id], sourceRevision);
  }

  /// 撤销最近一次 AI 补全。
  Future<CompletionApplyResultModel?> undo(String captureId) async {
    final current = state;
    if (current is CompletionLoaded && current.busy) return null;
    state = current is CompletionLoaded
        ? current.copyWith(busy: true, clearMessage: true)
        : const CompletionLoading();
    try {
      final result = await _gateway.undo(captureId);
      _preview = null;
      state = CompletionLoaded(message: '已撤销上次补全');
      return result;
    } catch (e) {
      state = current is CompletionLoaded
          ? current.copyWith(busy: false, message: _completionError(e))
          : CompletionError(message: _completionError(e));
      return null;
    }
  }

  Future<CompletionApplyResultModel?> _apply(
    String captureId,
    List<String> ids,
    int sourceRevision,
  ) async {
    final current = state;
    if (current is CompletionLoaded && current.busy) return null;
    state = current is CompletionLoaded
        ? current.copyWith(busy: true, clearMessage: true)
        : const CompletionLoading();
    try {
      final result = await _gateway.apply(
        captureId,
        proposalIds: ids,
        sourceRevision: sourceRevision,
      );
      // 采用后清空预览，收起提案列表；由调用方触发详情刷新。
      _preview = null;
      state = CompletionLoaded(message: '已采用 AI 补全');
      return result;
    } catch (e) {
      state = current is CompletionLoaded
          ? current.copyWith(busy: false, message: _completionError(e))
          : CompletionError(message: _completionError(e));
      return null;
    }
  }

  void clearMessage() {
    final current = state;
    if (current is CompletionLoaded) {
      state = current.copyWith(clearMessage: true);
    }
  }
}

String _completionError(Object error) {
  if (error is ApiException) {
    if (error.statusCode == 409) {
      switch (_errorCode(error)) {
        case 'FEATURE_NOT_ENABLED':
          return 'AI 补全未开启';
        case 'VERSION_CONFLICT':
          return '提案已过期，请重新生成';
        case 'PRECONDITION_FAILED':
          return '记忆尚未准备好，或没有可撤销的补全';
        default:
          return '操作冲突，请刷新后重试';
      }
    }
    if (error.statusCode == 404) {
      return '记忆不存在';
    }
    if (error.statusCode == 500) {
      return 'AI 生成失败，请稍后重试';
    }
    return error.message;
  }
  return '$error';
}

String? _errorCode(ApiException error) {
  final data = error.data;
  if (data is String && data.isNotEmpty) {
    try {
      final decoded = jsonDecode(data);
      if (decoded is Map<String, dynamic>) {
        final code = decoded['code'];
        if (code is String) return code;
      }
    } catch (_) {
      // 非 JSON 错误体：忽略。
    }
  }
  return null;
}

final completionNotifierProvider = NotifierProvider<CompletionNotifier, CompletionState>(
  CompletionNotifier.new,
);
