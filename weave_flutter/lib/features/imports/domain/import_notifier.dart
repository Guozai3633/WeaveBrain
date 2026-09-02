import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_exception.dart';
import '../data/import_api.dart';

/// 导入页状态机：单条导入与批量导入共用。
sealed class ImportState {
  const ImportState();

  /// 最近一次操作的用户提示；SnackBar 消费后调用 [ImportNotifier.clearMessage]。
  String? get message => null;
}

class ImportIdle extends ImportState {
  const ImportIdle();
}

class ImportCreating extends ImportState {
  const ImportCreating();
}

/// 批量任务已创建（尚未预览）。
class ImportCreated extends ImportState {
  const ImportCreated({required this.job});

  final ImportJobModel job;
}

/// 批量任务已预览；`completion` 为最近一次补全结果（proposals 覆盖层）。
class ImportPreviewed extends ImportState {
  const ImportPreviewed({
    required this.job,
    required this.preview,
    this.completion,
    this.message,
  });

  final ImportJobModel job;
  final ImportPreviewModel preview;
  final ImportCompletionModel? completion;

  @override
  final String? message;

  ImportPreviewed copyWith({
    ImportCompletionModel? completion,
    bool clearCompletion = false,
    String? message,
    bool clearMessage = false,
  }) {
    return ImportPreviewed(
      job: job,
      preview: preview,
      completion: clearCompletion ? null : (completion ?? this.completion),
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class ImportCommitting extends ImportState {
  const ImportCommitting();
}

/// 批量提交完成；`errorReport` 在用户查看错误报告后填充。
class ImportCommitted extends ImportState {
  const ImportCommitted({
    required this.result,
    required this.job,
    this.errorReport,
    this.message,
  });

  final ImportCommitResultModel result;
  final ImportJobModel job;
  final List<ImportErrorReportEntryModel>? errorReport;

  @override
  final String? message;

  ImportCommitted copyWith({
    List<ImportErrorReportEntryModel>? errorReport,
    String? message,
    bool clearMessage = false,
  }) {
    return ImportCommitted(
      result: result,
      job: job,
      errorReport: errorReport ?? this.errorReport,
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

class ImportError extends ImportState {
  const ImportError({required this.message, this.duplicateExistingCaptureId});

  @override
  final String message;

  /// 单条导入 external_id 已被占用时，已存在的 capture id。
  final String? duplicateExistingCaptureId;
}

class ImportNotifier extends Notifier<ImportState> {
  String? _jobId;
  ImportJobModel? _lastJob;

  ImportGateway get _gateway => ref.read(importGatewayProvider);

  @override
  ImportState build() => const ImportIdle();

  /// 批量：创建解析任务。成功发布 Created(job)，随后通常调用 [preview]。
  Future<ImportJobModel?> createJob({
    required String format,
    required String sourceName,
    required String content,
    String? separator,
    String? timezone,
    String? originalFilename,
  }) async {
    state = const ImportCreating();
    try {
      final job = await _gateway.createJob(
        format: format,
        sourceName: sourceName,
        content: content,
        separator: separator,
        timezone: timezone,
        originalFilename: originalFilename,
      );
      _jobId = job.id;
      _lastJob = job;
      state = ImportCreated(job: job);
      return job;
    } catch (e) {
      state = ImportError(message: _importError(e));
      return null;
    }
  }

  /// 批量：加载预览（前 10 行）。
  Future<void> preview() async {
    final jobId = _jobId;
    if (jobId == null) return;
    final current = state;
    if (current is! ImportCreated && current is! ImportPreviewed) return;
    try {
      final preview = await _gateway.preview(jobId);
      _lastJob = preview.job;
      state = ImportPreviewed(job: preview.job, preview: preview);
    } catch (e) {
      state = ImportError(message: _importError(e));
    }
  }

  /// 批量：为缺失字段生成 AI 提案。单行失败不阻塞其他行。
  Future<void> completionPreview({List<int>? rowNumbers}) async {
    final jobId = _jobId;
    final current = state;
    if (jobId == null || current is! ImportPreviewed) return;
    try {
      final completion = await _gateway.completionPreview(
        jobId,
        rowNumbers: rowNumbers,
      );
      state = current.copyWith(completion: completion);
    } catch (e) {
      state = current.copyWith(message: _importError(e));
    }
  }

  /// 批量：采用所选提案。
  Future<void> completionApply(List<ImportRowSelection> selections) async {
    final jobId = _jobId;
    final current = state;
    if (jobId == null || current is! ImportPreviewed) return;
    try {
      final completion = await _gateway.completionApply(
        jobId,
        selections: selections,
      );
      state = current.copyWith(completion: completion, message: '已采用所选补全');
    } catch (e) {
      state = current.copyWith(message: _importError(e));
    }
  }

  /// 批量：提交导入。
  Future<void> commit({
    required String duplicateContentAction,
    Map<int, String>? rowActions,
  }) async {
    final jobId = _jobId;
    final lastJob = _lastJob;
    if (jobId == null) return;
    state = const ImportCommitting();
    try {
      final result = await _gateway.commit(
        jobId,
        duplicateContentAction: duplicateContentAction,
        rowActions: rowActions,
      );
      state = ImportCommitted(
        result: result,
        job: lastJob ?? ImportJobModel.fromJson(const <String, dynamic>{}),
      );
    } catch (e) {
      state = ImportError(message: _importError(e));
    }
  }

  /// 批量：加载错误报告（Committed 后展示）。
  Future<void> loadErrorReport() async {
    final jobId = _jobId;
    final current = state;
    if (jobId == null || current is! ImportCommitted) return;
    try {
      final entries = await _gateway.errorReport(jobId);
      state = current.copyWith(errorReport: entries);
    } catch (e) {
      state = current.copyWith(message: _importError(e));
    }
  }

  /// 单条导入：POST /captures kind=import。成功返回结果供页面跳转；
  /// external_id 重复时发布 Error 并携带既有 capture id。
  Future<SingleImportResult?> importSingle(SingleImportDraft draft) async {
    state = const ImportCreating();
    try {
      final result = await _gateway.importSingle(draft);
      _jobId = null;
      _lastJob = null;
      state = const ImportIdle();
      return result;
    } on ImportDuplicateException catch (e) {
      state = ImportError(
        message: '该 external_id 已导入过，请更换后再试',
        duplicateExistingCaptureId: e.existingCaptureId,
      );
      return null;
    } catch (e) {
      state = ImportError(message: _importError(e));
      return null;
    }
  }

  /// 清空当前任务，回到初始状态。
  void reset() {
    _jobId = null;
    _lastJob = null;
    state = const ImportIdle();
  }

  /// 消费 SnackBar 提示。
  void clearMessage() {
    final current = state;
    if (current is ImportPreviewed) {
      state = current.copyWith(clearMessage: true);
    } else if (current is ImportCommitted) {
      state = current.copyWith(clearMessage: true);
    }
  }
}

String _importError(Object error) {
  if (error is ImportDuplicateException) {
    return '该 external_id 已导入过';
  }
  if (error is ApiException) {
    if (error.statusCode == 409) {
      switch (_errorCode(error)) {
        case 'FEATURE_NOT_ENABLED':
          return 'AI 补全未开启';
        case 'PRECONDITION_FAILED':
          return '任务已完成或存在冲突，请刷新后重试';
        default:
          return '操作冲突，请刷新后重试';
      }
    }
    if (error.statusCode == 404) {
      return '导入任务不存在';
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

final importNotifierProvider =
    NotifierProvider<ImportNotifier, ImportState>(ImportNotifier.new);
