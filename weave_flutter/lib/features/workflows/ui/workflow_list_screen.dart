import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../platform/data/capabilities_api.dart';
import '../../platform/domain/capabilities_notifier.dart';

/// 工作流「我的方案」占位页。
///
/// MVP 只做可见入口与「规划中」标记（PRODUCT_BLUEPRINT_V3 §14.6 / WORKFLOW-001），
/// 不制作可被误认为可用的假编辑器。设计/执行能力以服务端
/// GET /api/v3/capabilities 为准（workflow_designer / workflow_execution 恒 false）。
class WorkflowListScreen extends ConsumerWidget {
  const WorkflowListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final capabilities = ref.watch(capabilitiesProvider).valueOrNull;
    final planned = capabilities?.workflowAvailable != true;

    return Scaffold(
      appBar: AppBar(title: const Text('工作流')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
          children: [
            if (planned) ...[
              _PlannedBanner(capabilities: capabilities),
              const SizedBox(height: 12),
            ],
            Text('我的方案', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Column(
                  children: [
                    Icon(
                      Icons.account_tree_outlined,
                      size: 40,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(height: 12),
                    const Text('还没有工作流'),
                    const SizedBox(height: 4),
                    Text(
                      '工作流设计将在后续版本开放，敬请期待。',
                      style: Theme.of(context).textTheme.bodySmall,
                      textAlign: TextAlign.center,
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            ListTile(
              leading: const Icon(Icons.add_circle_outline),
              title: const Text('新建工作流'),
              subtitle: const Text('创建记忆处理方案'),
              trailing: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const _PlannedChip(),
                  const Icon(Icons.chevron_right),
                ],
              ),
              onTap: planned ? () => _showPlanned(context) : null,
            ),
            ListTile(
              leading: const Icon(Icons.auto_awesome_outlined),
              title: const Text('示例方案：每日回顾'),
              subtitle: const Text('官方示例，仅用于预览占位'),
              trailing: const Row(
                mainAxisSize: MainAxisSize.min,
                children: [_PlannedChip(), Icon(Icons.chevron_right)],
              ),
              onTap: planned
                  ? () =>
                        context.push('/workflows/example-daily-review/designer')
                  : null,
            ),
          ],
        ),
      ),
    );
  }

  void _showPlanned(BuildContext context) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('规划中 — 工作流设计器将在后续版本开放')));
  }
}

class _PlannedBanner extends StatelessWidget {
  const _PlannedBanner({this.capabilities});

  final PlatformCapabilities? capabilities;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Card(
      color: scheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.construction, color: scheme.primary),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    '规划中',
                    style: TextStyle(fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '当前版本不提供工作流设计与运行。'
                    '${capabilities == null ? '能力状态读取中…' : '设计与执行能力均未开放。'}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _PlannedChip extends StatelessWidget {
  const _PlannedChip();

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        '规划中',
        style: Theme.of(
          context,
        ).textTheme.labelSmall?.copyWith(color: scheme.onSurfaceVariant),
      ),
    );
  }
}
