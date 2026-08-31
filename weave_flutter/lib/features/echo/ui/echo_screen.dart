import 'package:flutter/material.dart';

/// 回响（AI 主动联想）占位页。R11 实现真实功能，R6 只保留导航槽位。
class EchoScreen extends StatelessWidget {
  const EchoScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('回响')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.auto_awesome_outlined, size: 64, color: Colors.grey),
            const SizedBox(height: 16),
            Text(
              '回响 · 规划中',
              style: Theme.of(
                context,
              ).textTheme.titleMedium?.copyWith(color: Colors.grey),
            ),
            const SizedBox(height: 8),
            Text(
              'AI 将主动联想你的记忆，提供新的连接与提醒',
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(color: Colors.grey),
            ),
          ],
        ),
      ),
    );
  }
}
