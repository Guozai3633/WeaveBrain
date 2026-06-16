import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../../../shared/models/idea.dart';

class IdeaDetailScreen extends StatelessWidget {
  final Idea idea;

  const IdeaDetailScreen({super.key, required this.idea});

  @override
  Widget build(BuildContext context) {
    final sd = idea.structuredData;
    final feasibility = sd?['feasibility'] as String?;
    final suggestions = (sd?['suggestions'] as List<dynamic>?)
            ?.map((e) => e.toString())
            .toList() ??
        [];
    final aiMeanEnv = sd?['ai_mean_env'] as String?;

    return Scaffold(
      appBar: AppBar(
        title: const Text('想法详情'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Original input
            Text('原始输入', style: Theme.of(context).textTheme.labelLarge),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Text(
                  idea.rawInput,
                  style: Theme.of(context).textTheme.bodyLarge,
                ),
              ),
            ),
            const SizedBox(height: 16),

            // Tags
            if (idea.tags.isNotEmpty) ...[
              Text('标签', style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 4,
                children: idea.tags.map((tag) {
                  return Chip(
                    label: Text(tag),
                    backgroundColor:
                        _tagColor(context, feasibility).withValues(alpha: 0.15),
                  );
                }).toList(),
              ),
              const SizedBox(height: 16),
            ],

            // Feasibility
            if (feasibility != null) ...[
              Text('可行性评估', style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Row(
                    children: [
                      _feasibilityIndicator(context, feasibility),
                      const SizedBox(width: 12),
                      Text(
                        _feasibilityLabel(feasibility),
                        style:
                            Theme.of(context).textTheme.bodyLarge?.copyWith(
                                  color: _feasibilityColor(context, feasibility),
                                  fontWeight: FontWeight.bold,
                                ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 16),
            ],

            // AI semantic analysis
            if (aiMeanEnv != null && aiMeanEnv.isNotEmpty) ...[
              Text('AI 语义分析', style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(aiMeanEnv,
                      style: Theme.of(context).textTheme.bodyMedium),
                ),
              ),
              const SizedBox(height: 16),
            ],

            // Suggestions
            if (suggestions.isNotEmpty) ...[
              Text('建议行动', style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  child: Column(
                    children: suggestions.asMap().entries.map((entry) {
                      return ListTile(
                        leading: CircleAvatar(
                          radius: 12,
                          child: Text(
                            '${entry.key + 1}',
                            style: const TextStyle(fontSize: 12),
                          ),
                        ),
                        title: Text(entry.value),
                        dense: true,
                      );
                    }).toList(),
                  ),
                ),
              ),
              const SizedBox(height: 16),
            ],

            // Structured data (raw, for debugging)
            if (sd != null && sd.isNotEmpty) ...[
              ExpansionTile(
                title: Text('原始结构化数据',
                    style: Theme.of(context).textTheme.labelLarge),
                children: sd.entries.map((entry) {
                  return Padding(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SizedBox(
                          width: 120,
                          child: Text(entry.key,
                              style: Theme.of(context)
                                  .textTheme
                                  .bodySmall
                                  ?.copyWith(fontWeight: FontWeight.bold)),
                        ),
                        Expanded(
                          child: Text(entry.value.toString(),
                              style: Theme.of(context).textTheme.bodySmall),
                        ),
                      ],
                    ),
                  );
                }).toList(),
              ),
            ],

            // Timestamp
            if (idea.createdAt != null) ...[
              const SizedBox(height: 16),
              Text(
                '创建于 ${DateFormat('yyyy-MM-dd HH:mm').format(idea.createdAt!)}',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Colors.grey,
                    ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Color _tagColor(BuildContext context, String? feasibility) {
    return _feasibilityColor(context, feasibility);
  }

  Widget _feasibilityIndicator(BuildContext context, String feasibility) {
    final color = _feasibilityColor(context, feasibility);
    final filled = switch (feasibility) {
      'high' => 3,
      'medium' => 2,
      'low' => 1,
      _ => 0,
    };
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(3, (i) {
        return Icon(
          i < filled ? Icons.circle : Icons.circle_outlined,
          size: 16,
          color: color,
        );
      }),
    );
  }

  Color _feasibilityColor(BuildContext context, String? feasibility) {
    return switch (feasibility) {
      'high' => Colors.green,
      'medium' => Colors.orange,
      'low' => Colors.red,
      _ => Colors.grey,
    };
  }

  String _feasibilityLabel(String feasibility) {
    return switch (feasibility) {
      'high' => '高可行性',
      'medium' => '中等可行性',
      'low' => '低可行性',
      _ => feasibility,
    };
  }
}
