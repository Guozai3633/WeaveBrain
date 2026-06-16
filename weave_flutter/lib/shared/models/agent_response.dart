class AgentResponse {
  final String response;
  final String? ideaId;
  final String? workflowId;
  final List<String> tags;
  final String? feasibility;
  final List<String> suggestions;
  final String? baseInput;

  const AgentResponse({
    required this.response,
    this.ideaId,
    this.workflowId,
    this.tags = const [],
    this.feasibility,
    this.suggestions = const [],
    this.baseInput,
  });

  factory AgentResponse.fromJson(Map<String, dynamic> json) {
    return AgentResponse(
      response: json['response'] as String? ?? '',
      ideaId: json['idea_id'] as String?,
      workflowId: json['workflow_id'] as String?,
      tags: (json['tags'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      feasibility: json['feasibility'] as String?,
      suggestions: (json['suggestions'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      baseInput: json['base_input'] as String?,
    );
  }
}
