import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_exception.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';
import 'text_file_reader.dart';

/// 批量导入任务，对应后端 `entity.ImportJob`。
class ImportJobModel {
  const ImportJobModel({
    required this.id,
    required this.userId,
    required this.sourceName,
    required this.format,
    required this.rawText,
    required this.columnMapping,
    required this.separator,
    required this.totalRows,
    required this.validRows,
    required this.invalidRows,
    required this.duplicateRows,
    required this.needsInputRows,
    required this.importedRows,
    required this.skippedRows,
    required this.failedRows,
    required this.status,
    required this.createdAt,
    required this.updatedAt,
    this.originalFilename,
    this.timezone,
    this.committedAt,
    this.cancelledAt,
  });

  factory ImportJobModel.fromJson(Map<String, dynamic> json) {
    final mapping = json['column_mapping'];
    return ImportJobModel(
      id: json['id'] as String? ?? '',
      userId: json['user_id'] as String? ?? '',
      sourceName: json['source_name'] as String? ?? '',
      format: json['format'] as String? ?? 'plain_text',
      originalFilename: json['original_filename'] as String?,
      rawText: json['raw_text'] as String? ?? '',
      columnMapping: mapping is Map<String, dynamic>
          ? mapping
          : const <String, dynamic>{},
      separator: json['separator'] as String? ?? '---',
      timezone: json['timezone'] as String?,
      totalRows: (json['total_rows'] as num?)?.toInt() ?? 0,
      validRows: (json['valid_rows'] as num?)?.toInt() ?? 0,
      invalidRows: (json['invalid_rows'] as num?)?.toInt() ?? 0,
      duplicateRows: (json['duplicate_rows'] as num?)?.toInt() ?? 0,
      needsInputRows: (json['needs_input_rows'] as num?)?.toInt() ?? 0,
      importedRows: (json['imported_rows'] as num?)?.toInt() ?? 0,
      skippedRows: (json['skipped_rows'] as num?)?.toInt() ?? 0,
      failedRows: (json['failed_rows'] as num?)?.toInt() ?? 0,
      status: json['status'] as String? ?? 'draft',
      committedAt: _parseDateTime(json['committed_at']),
      cancelledAt: _parseDateTime(json['cancelled_at']),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseDateTime(json['updated_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String userId;
  final String sourceName;
  final String format;
  final String? originalFilename;
  final String rawText;
  final Map<String, dynamic> columnMapping;
  final String separator;
  final String? timezone;
  final int totalRows;
  final int validRows;
  final int invalidRows;
  final int duplicateRows;
  final int needsInputRows;
  final int importedRows;
  final int skippedRows;
  final int failedRows;
  final String status;
  final DateTime? committedAt;
  final DateTime? cancelledAt;
  final DateTime createdAt;
  final DateTime updatedAt;
}

/// 一条行级校验错误，对应后端 `entity.ImportValidationError`。
class ImportValidationErrorModel {
  const ImportValidationErrorModel({required this.code, required this.message});

  factory ImportValidationErrorModel.fromJson(Map<String, dynamic> json) {
    return ImportValidationErrorModel(
      code: json['code'] as String? ?? '',
      message: json['message'] as String? ?? '',
    );
  }

  final String code;
  final String message;
}

/// 一条 AI 缺失字段提案，对应后端 `entity.ImportFieldProposal`。
class ImportProposalModel {
  const ImportProposalModel({
    required this.id,
    required this.fieldName,
    required this.proposedValue,
    required this.provenance,
    required this.applyPolicy,
    required this.confidence,
    required this.status,
    required this.evidenceSpans,
  });

  factory ImportProposalModel.fromJson(Map<String, dynamic> json) {
    final rawSpans = json['evidence_spans'];
    return ImportProposalModel(
      id: json['id'] as String,
      fieldName: json['field_name'] as String? ?? '',
      proposedValue: json['proposed_value'] as String? ?? '',
      provenance: json['provenance'] as String? ?? 'ai',
      applyPolicy: json['apply_policy'] as String? ?? 'suggest_only',
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
      status: json['status'] as String? ?? 'pending',
      evidenceSpans: rawSpans is List
          ? rawSpans.whereType<Map<String, dynamic>>().toList()
          : const <Map<String, dynamic>>[],
    );
  }

  final String id;
  final String fieldName;
  final String proposedValue;
  final String provenance;
  final String applyPolicy;
  final double confidence;
  final String status;
  final List<Map<String, dynamic>> evidenceSpans;

  bool get isAccepted => status == 'accepted';

  /// tags / key_points 的数组值；其余字段原样。
  List<String> get proposedValues => _parseArray(proposedValue);
}

/// 批量导入的一行，对应后端 `entity.ImportRow`。
class ImportRowModel {
  const ImportRowModel({
    required this.id,
    required this.importJobId,
    required this.userId,
    required this.rowNumber,
    required this.rawPayload,
    required this.normalizedPayload,
    required this.validationErrors,
    required this.dedupeStatus,
    required this.status,
    required this.completionProposals,
    required this.createdAt,
    required this.updatedAt,
    this.externalId,
    this.content,
    this.contentHash,
    this.captureId,
    this.importedAt,
  });

  factory ImportRowModel.fromJson(Map<String, dynamic> json) {
    final raw = json['raw_payload'];
    final normalized = json['normalized_payload'];
    final errors = json['validation_errors'];
    final proposals = json['completion_proposals'];
    return ImportRowModel(
      id: json['id'] as String,
      importJobId: json['import_job_id'] as String,
      userId: json['user_id'] as String,
      rowNumber: (json['row_number'] as num?)?.toInt() ?? 0,
      externalId: json['external_id'] as String?,
      rawPayload: raw is Map<String, dynamic> ? raw : const <String, dynamic>{},
      normalizedPayload: normalized is Map<String, dynamic>
          ? normalized
          : const <String, dynamic>{},
      content: json['content'] as String?,
      contentHash: json['content_hash'] as String?,
      validationErrors: errors is List
          ? errors
                .whereType<Map<String, dynamic>>()
                .map(ImportValidationErrorModel.fromJson)
                .toList()
          : const <ImportValidationErrorModel>[],
      dedupeStatus: json['dedupe_status'] as String? ?? 'none',
      captureId: json['capture_id'] as String?,
      status: json['status'] as String? ?? 'pending',
      completionProposals: proposals is List
          ? proposals
                .whereType<Map<String, dynamic>>()
                .map(ImportProposalModel.fromJson)
                .toList()
          : const <ImportProposalModel>[],
      importedAt: _parseDateTime(json['imported_at']),
      createdAt: _parseDateTime(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseDateTime(json['updated_at']) ?? DateTime.now(),
    );
  }

  final String id;
  final String importJobId;
  final String userId;
  final int rowNumber;
  final String? externalId;
  final Map<String, dynamic> rawPayload;
  final Map<String, dynamic> normalizedPayload;
  final String? content;
  final String? contentHash;
  final List<ImportValidationErrorModel> validationErrors;
  final String dedupeStatus;
  final String? captureId;
  final String status;
  final List<ImportProposalModel> completionProposals;
  final DateTime? importedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  bool get hasErrors => validationErrors.isNotEmpty;
  bool get isDuplicateExternal => dedupeStatus == 'duplicate_external';
  bool get isSuggested => dedupeStatus == 'suggested';
  bool get needsInput => status == 'needs_input';
  bool get isImported => status == 'imported';

  /// 行内可展示的正文摘要。
  String get contentPreview {
    final text = content ?? normalizedPayload['content'] as String? ?? '';
    final trimmed = text.trim();
    if (trimmed.isEmpty) return '(空内容)';
    if (trimmed.length <= 60) return trimmed;
    return '${trimmed.substring(0, 60)}…';
  }
}

/// 预览一页，对应后端 `entity.ImportPreviewResult`。
class ImportPreviewModel {
  const ImportPreviewModel({
    required this.job,
    required this.rows,
    this.nextCursor,
  });

  factory ImportPreviewModel.fromJson(Map<String, dynamic> json) {
    final job = json['job'];
    final rows = json['rows'];
    return ImportPreviewModel(
      job: job is Map<String, dynamic>
          ? ImportJobModel.fromJson(job)
          : ImportJobModel.fromJson(const <String, dynamic>{}),
      rows: rows is List
          ? rows
                .whereType<Map<String, dynamic>>()
                .map(ImportRowModel.fromJson)
                .toList()
          : const <ImportRowModel>[],
      nextCursor: json['next_cursor'] as String?,
    );
  }

  final ImportJobModel job;
  final List<ImportRowModel> rows;
  final String? nextCursor;
}

/// completion:preview / completion:apply 结果，对应后端 `entity.ImportCompletionResult`。
class ImportCompletionModel {
  const ImportCompletionModel({required this.rows});

  factory ImportCompletionModel.fromJson(Map<String, dynamic> json) {
    final rows = json['rows'];
    return ImportCompletionModel(
      rows: rows is List
          ? rows
                .whereType<Map<String, dynamic>>()
                .map(ImportRowModel.fromJson)
                .toList()
          : const <ImportRowModel>[],
    );
  }

  final List<ImportRowModel> rows;
}

/// commit 结果，对应后端 `entity.ImportCommitResult`。
class ImportCommitResultModel {
  const ImportCommitResultModel({
    required this.imported,
    required this.failed,
    required this.skipped,
    required this.needsInput,
    required this.total,
    required this.jobStatus,
    this.committedAt,
  });

  factory ImportCommitResultModel.fromJson(Map<String, dynamic> json) {
    return ImportCommitResultModel(
      imported: (json['imported'] as num?)?.toInt() ?? 0,
      failed: (json['failed'] as num?)?.toInt() ?? 0,
      skipped: (json['skipped'] as num?)?.toInt() ?? 0,
      needsInput: (json['needs_input'] as num?)?.toInt() ?? 0,
      total: (json['total'] as num?)?.toInt() ?? 0,
      jobStatus: json['job_status'] as String? ?? 'completed',
      committedAt: _parseDateTime(json['committed_at']),
    );
  }

  final int imported;
  final int failed;
  final int skipped;
  final int needsInput;
  final int total;
  final String jobStatus;
  final DateTime? committedAt;
}

/// 错误报告一行，对应后端 `entity.ImportErrorReportEntry`。
class ImportErrorReportEntryModel {
  const ImportErrorReportEntryModel({
    required this.rowNumber,
    required this.status,
    required this.dedupeStatus,
    required this.validationErrors,
    this.externalId,
  });

  factory ImportErrorReportEntryModel.fromJson(Map<String, dynamic> json) {
    final errors = json['validation_errors'];
    return ImportErrorReportEntryModel(
      rowNumber: (json['row_number'] as num?)?.toInt() ?? 0,
      externalId: json['external_id'] as String?,
      status: json['status'] as String? ?? 'failed',
      dedupeStatus: json['dedupe_status'] as String? ?? 'none',
      validationErrors: errors is List
          ? errors
                .whereType<Map<String, dynamic>>()
                .map(ImportValidationErrorModel.fromJson)
                .toList()
          : const <ImportValidationErrorModel>[],
    );
  }

  final int rowNumber;
  final String? externalId;
  final String status;
  final String dedupeStatus;
  final List<ImportValidationErrorModel> validationErrors;
}

/// 单条导入草稿，对应 `POST /captures` kind=import 的请求字段。
class SingleImportDraft {
  const SingleImportDraft({
    required this.text,
    this.externalId,
    this.sourceName,
    this.title,
    this.tags = const [],
    this.primaryType,
    this.capturedAt,
    this.timezone,
  });

  final String text;
  final String? externalId;
  final String? sourceName;
  final String? title;
  final List<String> tags;
  final String? primaryType;
  final String? capturedAt;
  final String? timezone;

  Map<String, dynamic> toRequest({required String captureId}) {
    return <String, dynamic>{
      'capture_id': captureId,
      'kind': 'import',
      'text': text,
      'client_version': 1,
      if (externalId != null && externalId!.isNotEmpty) 'external_id': externalId,
      if (sourceName != null && sourceName!.isNotEmpty)
        'source_name': sourceName,
      if (title != null && title!.isNotEmpty) 'title': title,
      if (tags.isNotEmpty) 'tags': tags,
      if (primaryType != null && primaryType!.isNotEmpty)
        'primary_type': primaryType,
      if (capturedAt != null && capturedAt!.isNotEmpty)
        'captured_at': capturedAt,
      if (timezone != null && timezone!.isNotEmpty) 'timezone': timezone,
    };
  }
}

/// 单条导入结果：新建 capture 的 id 与疑似重复提示。
class SingleImportResult {
  const SingleImportResult({
    required this.captureId,
    this.replayed = false,
    this.dedupeStatus,
    this.dedupeExistingCaptureId,
  });

  final String captureId;
  final bool replayed;

  /// 内容 hash 疑似重复（仍已创建，是否保留由用户决定）。
  final String? dedupeStatus;
  final String? dedupeExistingCaptureId;
}

/// 单条导入时 external_id 已被占用（409 PRECONDITION_FAILED）。
class ImportDuplicateException implements Exception {
  const ImportDuplicateException({this.existingCaptureId});

  final String? existingCaptureId;

  @override
  String toString() =>
      'ImportDuplicateException(existingCaptureId: $existingCaptureId)';
}

/// 一次行提案采纳（completion:apply 请求项）。
class ImportRowSelection {
  const ImportRowSelection({required this.rowNumber, required this.proposalIds});

  final int rowNumber;
  final List<String> proposalIds;
}

/// 导入数据访问抽象，便于测试替换。
abstract interface class ImportGateway {
  Future<ImportJobModel> createJob({
    required String format,
    required String sourceName,
    required String content,
    String? separator,
    String? timezone,
    String? originalFilename,
  });

  Future<ImportJobModel> getJob(String jobId);

  Future<ImportPreviewModel> preview(String jobId, {int limit = 10, int offset = 0});

  Future<ImportCompletionModel> completionPreview(String jobId, {List<int>? rowNumbers});

  Future<ImportCompletionModel> completionApply(
    String jobId, {
    required List<ImportRowSelection> selections,
  });

  Future<ImportCommitResultModel> commit(
    String jobId, {
    required String duplicateContentAction,
    Map<int, String>? rowActions,
  });

  Future<List<ImportErrorReportEntryModel>> errorReport(String jobId);

  Future<SingleImportResult> importSingle(SingleImportDraft draft);
}

class ImportApi implements ImportGateway {
  const ImportApi(this._client);

  final ApiClient _client;

  static const _uuid = Uuid();

  @override
  Future<ImportJobModel> createJob({
    required String format,
    required String sourceName,
    required String content,
    String? separator,
    String? timezone,
    String? originalFilename,
  }) async {
    final response = await _client.post(
      '/imports',
      body: <String, dynamic>{
        'format': format,
        'source_name': sourceName,
        'content': content,
        if (separator != null && separator.isNotEmpty) 'separator': separator,
        if (timezone != null && timezone.isNotEmpty) 'timezone': timezone,
        if (originalFilename != null && originalFilename.isNotEmpty)
          'original_filename': originalFilename,
      },
    );
    return ImportJobModel.fromJson(_envelopeJob(response.data));
  }

  @override
  Future<ImportJobModel> getJob(String jobId) async {
    final response = await _client.get('/imports/$jobId');
    return ImportJobModel.fromJson(_envelopeJob(response.data));
  }

  @override
  Future<ImportPreviewModel> preview(
    String jobId, {
    int limit = 10,
    int offset = 0,
  }) async {
    final response = await _client.get(
      '/imports/$jobId/preview',
      queryParams: <String, dynamic>{'limit': limit, 'offset': offset},
    );
    final data = _requireMap(response.data);
    final preview = data['preview'];
    if (preview is! Map<String, dynamic>) {
      throw const FormatException('无效的导入预览响应');
    }
    return ImportPreviewModel.fromJson(preview);
  }

  @override
  Future<ImportCompletionModel> completionPreview(
    String jobId, {
    List<int>? rowNumbers,
  }) async {
    final response = await _client.post(
      '/imports/$jobId/completion/preview',
      body: <String, dynamic>{
        if (rowNumbers != null && rowNumbers.isNotEmpty)
          'row_numbers': rowNumbers,
      },
    );
    final data = _requireMap(response.data);
    final completion = data['completion'];
    if (completion is! Map<String, dynamic>) {
      throw const FormatException('无效的补全预览响应');
    }
    return ImportCompletionModel.fromJson(completion);
  }

  @override
  Future<ImportCompletionModel> completionApply(
    String jobId, {
    required List<ImportRowSelection> selections,
  }) async {
    final response = await _client.post(
      '/imports/$jobId/completion/apply',
      body: <String, dynamic>{
        'row_selections': [
          for (final s in selections)
            <String, dynamic>{'row_number': s.rowNumber, 'proposal_ids': s.proposalIds},
        ],
      },
    );
    final data = _requireMap(response.data);
    final completion = data['completion'];
    if (completion is! Map<String, dynamic>) {
      throw const FormatException('无效的采用补全响应');
    }
    return ImportCompletionModel.fromJson(completion);
  }

  @override
  Future<ImportCommitResultModel> commit(
    String jobId, {
    required String duplicateContentAction,
    Map<int, String>? rowActions,
  }) async {
    final response = await _client.post(
      '/imports/$jobId/commit',
      body: <String, dynamic>{
        'duplicate_content_action': duplicateContentAction,
        if (rowActions != null && rowActions.isNotEmpty)
          'row_actions': {
            for (final entry in rowActions.entries) '${entry.key}': entry.value,
          },
      },
    );
    final data = _requireMap(response.data);
    final commit = data['commit'];
    if (commit is! Map<String, dynamic>) {
      throw const FormatException('无效的提交响应');
    }
    return ImportCommitResultModel.fromJson(commit);
  }

  @override
  Future<List<ImportErrorReportEntryModel>> errorReport(String jobId) async {
    final response = await _client.get('/imports/$jobId/error-report');
    final data = _requireMap(response.data);
    final entries = data['entries'];
    if (entries is! List) {
      return const <ImportErrorReportEntryModel>[];
    }
    return entries
        .whereType<Map<String, dynamic>>()
        .map(ImportErrorReportEntryModel.fromJson)
        .toList();
  }

  @override
  Future<SingleImportResult> importSingle(SingleImportDraft draft) async {
    final captureId = _uuid.v4();
    try {
      final response = await _client.post(
        '/captures',
        body: draft.toRequest(captureId: captureId),
        headers: <String, String>{'Idempotency-Key': captureId},
      );
      final data = _requireMap(response.data);
      final capture = data['capture'];
      final dedupe = data['dedupe'];
      if (capture is! Map<String, dynamic>) {
        throw const FormatException('无效的单条导入响应');
      }
      return SingleImportResult(
        captureId: capture['id'] as String? ?? captureId,
        replayed: data['replayed'] as bool? ?? false,
        dedupeStatus: dedupe is Map<String, dynamic>
            ? dedupe['status'] as String?
            : null,
        dedupeExistingCaptureId: dedupe is Map<String, dynamic>
            ? dedupe['existing_capture_id'] as String?
            : null,
      );
    } on ApiException catch (e) {
      if (e.statusCode == 409) {
        throw ImportDuplicateException(
          existingCaptureId: _detailString(e.data, 'existing_capture_id'),
        );
      }
      rethrow;
    }
  }

  static Map<String, dynamic> _envelopeJob(dynamic data) {
    final map = _requireMap(data);
    final job = map['job'];
    if (job is! Map<String, dynamic>) {
      throw const FormatException('无效的导入任务响应');
    }
    return job;
  }

  static Map<String, dynamic> _requireMap(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data;
    }
    throw const FormatException('无效的导入响应');
  }

  static String? _detailString(dynamic errorData, String key) {
    if (errorData is String && errorData.isNotEmpty) {
      try {
        final decoded = jsonDecode(errorData);
        if (decoded is Map<String, dynamic>) {
          final details = decoded['details'];
          if (details is Map<String, dynamic>) {
            final value = details[key];
            if (value is String) return value;
          }
        }
      } catch (_) {
        // 非 JSON 错误体：忽略。
      }
    }
    return null;
  }
}

/// v3 ApiClient（与 capture / memory 特性共用同一 host）。
final importApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final importGatewayProvider = Provider<ImportGateway>((ref) {
  return ImportApi(ref.watch(importApiClientProvider));
});

/// 平台相关的文本文件读取器（Web 原生选择；App/桌面提示粘贴）。
final textFileReaderProvider = Provider<TextFileReader>((ref) {
  return createTextFileReader();
});

// ---- JSON 辅助函数 ----

DateTime? _parseDateTime(dynamic value) {
  if (value is String) {
    return DateTime.tryParse(value);
  }
  return null;
}

List<String> _parseArray(String raw) {
  if (!raw.startsWith('[')) return const <String>[];
  try {
    final decoded = jsonDecode(raw);
    if (decoded is List) {
      return decoded.whereType<String>().toList();
    }
  } catch (_) {
    // 非 JSON 数组：视为普通字符串值。
  }
  return const <String>[];
}
