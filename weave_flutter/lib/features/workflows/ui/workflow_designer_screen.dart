import 'package:flutter/material.dart';

/// 工作流设计器占位页（预留路由 `/workflows/:workflowId/designer`）。
///
/// MVP 明确「规划中」：不渲染画布/节点编辑器，避免被误认为可用
/// （PRODUCT_BLUEPRINT_V3 §14.6 / MVP_IMPLEMENTATION_BACKLOG WORKFLOW-001）。
class WorkflowDesignerScreen extends StatelessWidget {
  const WorkflowDesignerScreen({super.key, required this.workflowId});

  final String workflowId;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(title: const Text('工作流设计器')),
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(Icons.construction, size: 56, color: scheme.primary),
                const SizedBox(height: 16),
                Text(
                  '规划中',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: scheme.primary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  '「$workflowId」的方案设计器尚未开放。\n触发编排、条件配置、校验与发布在当前版本均不可执行。',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
