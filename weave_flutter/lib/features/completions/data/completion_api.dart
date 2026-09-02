import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/api/api_client.dart';
import '../../../shared/api/api_host.dart';
import '../../../shared/auth/auth_state.dart';
import '../../memories/data/memory_api.dart';

/// 证据片段：原文中的逐字引文（start/end 为字节偏移），支持事实型建议核对。
class EvidenceSpanModel {
  const EvidenceSpanModel({
    required this.start,
    required this.end,
    required this.quote,
  });

  factory EvidenceSpanModel.fromJson(Map<String, dynamic> json) {
    return EvidenceSpanModel(
      start: (json['start'] as num?)?.toInt() ?? 0,
      end: (json['end'] as num?)?.toInt() ?? 0,
      quote: json['quote'] as String? ?? '',
    );
  }

  final int start;
  final int end;
  final String quote;
}

/// 一条字段提案，对应后端 `entity.CompletionProposal`。
///
/// tags / key_points 的 `proposed_value` 是 JSON 数组字符串；
/// `proposedValues` 负责把它解析成可渲染的列表。
class CompletionProposalModel {
  const CompletionProposalModel({
    required this.id,
    required this.captureId,
    required this.previewId,
    required this.sourceRevision,
    required this.fieldName,
    required this.originalValue,
    required this.proposedValue,
    required this.provenance,
    required this.applyPolicy,
    required this.status,
    required this.evidenceSpans,
    this.confidence,
    this.riskLevel,
    this.provider,
    this.model,
    this.configVersion,
  });

  factory CompletionProposalModel.fromJson(Map<String, dynamic> json) {
    final rawSpans = json['evidence_spans'];
    return CompletionProposalModel(
      id: json['id'] as String,
      captureId: json['capture_id'] as String? ?? '',
      previewId: json['preview_id'] as String? ?? '',
      sourceRevision: (json['source_revision'] as num?)?.toInt() ?? 0,
      fieldName: json['field_name'] as String? ?? '',
      originalValue: json['original_value'] as String? ?? '',
      proposedValue: json['proposed_value'] as String? ?? '',
      provenance: json['provenance'] as String? ?? 'ai',
      applyPolicy: json['apply_policy'] as String? ?? 'suggest_only',
      confidence: (json['confidence'] as num?)?.toDouble(),
      riskLevel: json['risk_level'] as String?,
      evidenceSpans: rawSpans is List
          ? rawSpans
                .whereType<Map<String, dynamic>>()
                .map(EvidenceSpanModel.fromJson)
                .toList()
          : const <EvidenceSpanModel>[],
      status: json['status'] as String? ?? 'pending',
      provider: json['provider'] as String?,
      model: json['model'] as String?,
      configVersion: json['config_version'] as String?,
    );
  }

  final String id;
  final String captureId;
  final String previewId;
  final int sourceRevision;
  final String fieldName;
  final String originalValue;
  final String proposedValue;
  final String provenance;
  final String applyPolicy;
  final double? confidence;
  final String? riskLevel;
  final List<EvidenceSpanModel> evidenceSpans;
  final String status;
  final String? provider;
  final String? model;
  final String? configVersion;

  /// tags / key_points 的建议值列表；非数组字段为空。
  List<String> get proposedValues => _parseArray(proposedValue);

  /// 是否可安全批量采用（apply_policy == safe_auto 且仍处于 pending）。
  bool get canAutoApply =>
      applyPolicy == 'safe_auto' && status == 'pending';

  bool get isPending => status == 'pending';

  /// 是否有可展示的证据引文。
  bool get hasEvidence => evidenceSpans.isNotEmpty;
}

/// completion:preview 结果（不修改业务对象）。
class CompletionPreviewResultModel {
  const CompletionPreviewResultModel({
    required this.captureId,
    required this.sourceRevision,
    required this.missingFields,
    required this.proposals,
  });

  factory CompletionPreviewResultModel.fromJson(Map<String, dynamic> json) {
    final rawMissing = json['missing_fields'];
    final rawProposals = json['proposals'];
    return CompletionPreviewResultModel(
      captureId: json['capture_id'] as String? ?? '',
      sourceRevision: (json['source_revision'] as num?)?.toInt() ?? 0,
      missingFields: rawMissing is List
          ? rawMissing.whereType<String>().toList()
          : const <String>[],
      proposals: rawProposals is List
          ? rawProposals
                .whereType<Map<String, dynamic>>()
                .map(CompletionProposalModel.fromJson)
                .toList()
          : const <CompletionProposalModel>[],
    );
  }

  final String captureId;
  final int sourceRevision;
  final List<String> missingFields;
  final List<CompletionProposalModel> proposals;
}

/// completion:apply / completion:undo 结果。
class CompletionApplyResultModel {
  const CompletionApplyResultModel({
    required this.memoryCard,
    required this.appliedProposalIds,
  });

  factory CompletionApplyResultModel.fromJson(Map<String, dynamic> json) {
    final rawCard = json['memory_card'];
    final rawIds = json['applied_proposal_ids'];
    return CompletionApplyResultModel(
      memoryCard: rawCard is Map<String, dynamic>
          ? MemoryCardModel.fromJson(rawCard)
          : MemoryCardModel.fromJson(const <String, dynamic>{}),
      appliedProposalIds: rawIds is List
          ? rawIds.whereType<String>().toList()
          : const <String>[],
    );
  }

  final MemoryCardModel memoryCard;
  final List<String> appliedProposalIds;
}

/// 补全数据访问抽象，便于测试替换。
abstract interface class CompletionGateway {
  Future<CompletionPreviewResultModel> preview(String captureId);

  Future<CompletionApplyResultModel> apply(
    String captureId, {
    required List<String> proposalIds,
    required int sourceRevision,
  });

  Future<CompletionApplyResultModel> undo(String captureId);
}

class CompletionApi implements CompletionGateway {
  const CompletionApi(this._client);

  final ApiClient _client;

  @override
  Future<CompletionPreviewResultModel> preview(String captureId) async {
    final response = await _client.post('/captures/$captureId/completion/preview');
    final data = _requireMap(response.data);
    final preview = data['preview'];
    if (preview is! Map<String, dynamic>) {
      throw const FormatException('无效的补全预览响应');
    }
    return CompletionPreviewResultModel.fromJson(preview);
  }

  @override
  Future<CompletionApplyResultModel> apply(
    String captureId, {
    required List<String> proposalIds,
    required int sourceRevision,
  }) async {
    final response = await _client.post(
      '/captures/$captureId/completion/apply',
      body: <String, dynamic>{
        'proposal_ids': proposalIds,
        'source_revision': sourceRevision,
      },
    );
    return _parseApplyEnvelope(response.data);
  }

  @override
  Future<CompletionApplyResultModel> undo(String captureId) async {
    final response = await _client.post('/captures/$captureId/completion/undo');
    final data = _requireMap(response.data);
    final undo = data['undo'];
    if (undo is! Map<String, dynamic>) {
      throw const FormatException('无效的撤销补全响应');
    }
    return CompletionApplyResultModel.fromJson(undo);
  }

  static CompletionApplyResultModel _parseApplyEnvelope(dynamic data) {
    final map = _requireMap(data);
    final apply = map['apply'];
    if (apply is! Map<String, dynamic>) {
      throw const FormatException('无效的采用补全响应');
    }
    return CompletionApplyResultModel.fromJson(apply);
  }

  static Map<String, dynamic> _requireMap(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data;
    }
    throw const FormatException('无效的补全响应');
  }
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

/// v3 ApiClient（与 ai-settings / memory 特性共用同一 host）。
final completionApiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: 'http://$apiHost/api/v3',
    authRepository: ref.watch(authRepositoryProvider),
  );
});

final completionGatewayProvider = Provider<CompletionGateway>((ref) {
  return CompletionApi(ref.watch(completionApiClientProvider));
});
