import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/user.dart';
import '../../capture/domain/capture_providers.dart';
import '../data/user_api.dart';
import 'profile_edit_dialog.dart';

final userApiProvider = Provider<UserApi>((ref) {
  final client = ref.watch(apiClientProvider);
  return UserApi(client);
});

final sttConfigProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  final client = ref.watch(apiClientProvider);
  try {
    final resp = await client.get('/config/stt');
    final data = resp.data;
    if (data is Map<String, dynamic>) return data;
    return {'provider': 'unknown'};
  } catch (_) {
    return {'provider': 'unavailable'};
  }
});

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authNotifierProvider);
    final sttAsync = ref.watch(sttConfigProvider);

    return ListView(
      children: [
        // --- Personal Info ---
        if (authState is Authenticated) ...[
          const SizedBox(height: 16),
          _buildProfileSection(context, ref, authState),
          const Divider(),
          // 游客记录合并状态（登录后自动认领，无重复）。
          const _MergeStatusCard(),
          const Divider(),
        ],

        // --- STT Provider ---
        ListTile(
          leading: const Icon(Icons.mic_outlined),
          title: const Text('语音识别引擎'),
          subtitle: sttAsync.when(
            data: (data) => Text(data['provider'] as String? ?? 'unknown'),
            loading: () => const Text('加载中...'),
            error: (_, _) => const Text('获取失败'),
          ),
        ),
        const Divider(),

        // --- API Config (placeholder) ---
        ListTile(
          leading: const Icon(Icons.integration_instructions_outlined),
          title: const Text('MCP 集成'),
          subtitle: const Text('管理 Notion、Email 等工具集成'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () {
            context.push('/settings/mcp');
          },
        ),
        const Divider(),

        // --- AI Settings ---
        ListTile(
          leading: const Icon(Icons.auto_awesome_outlined),
          title: const Text('AI 与自动化'),
          subtitle: const Text('AI 记忆整理、语音转写与云端处理开关'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () {
            context.push('/settings/ai');
          },
        ),
        const Divider(),

        // --- Echo & Reminders ---
        ListTile(
          leading: const Icon(Icons.notifications_active_outlined),
          title: const Text('回响与提醒'),
          subtitle: const Text('让旧记忆定期回来，声音/触觉速记入口'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => context.push('/settings/echo'),
        ),
        const Divider(),

        // --- Import Old Memories ---
        ListTile(
          leading: const Icon(Icons.file_download_outlined),
          title: const Text('导入旧记忆'),
          subtitle: const Text('从文本/CSV/JSONL 批量导入'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => context.push('/imports'),
        ),
        const Divider(),

        // --- Workflows (planned, reserved) ---
        ListTile(
          leading: const Icon(Icons.account_tree_outlined),
          title: const Text('工作流'),
          subtitle: const Text('设计记忆处理方案'),
          trailing: const Row(
            mainAxisSize: MainAxisSize.min,
            children: [_PlannedChip(), Icon(Icons.chevron_right)],
          ),
          onTap: () => context.push('/workflows'),
        ),
        const Divider(),

        // --- Project Management ---
        ListTile(
          leading: const Icon(Icons.folder_outlined),
          title: const Text('项目管理'),
          subtitle: const Text('创建和管理你的项目'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => context.push('/projects'),
        ),
        const Divider(),

        // --- About ---
        ListTile(
          leading: const Icon(Icons.info_outline),
          title: const Text('关于织脑'),
          subtitle: const Text('v1.0.0'),
          onTap: () {
            showAboutDialog(
              context: context,
              applicationName: '织脑 WeaveBrain',
              applicationVersion: '1.0.0',
              children: const [Text('以语音为入口、Agent为引擎的个人思维外脑')],
            );
          },
        ),
        const Divider(),

        // --- Logout ---
        ListTile(
          leading: Icon(
            Icons.logout,
            color: Theme.of(context).colorScheme.error,
          ),
          title: Text(
            '退出登录',
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
          onTap: () async {
            final confirmed = await showDialog<bool>(
              context: context,
              builder: (ctx) => AlertDialog(
                title: const Text('确认退出'),
                content: const Text('确定要退出登录吗？'),
                actions: [
                  TextButton(
                    onPressed: () => Navigator.pop(ctx, false),
                    child: const Text('取消'),
                  ),
                  TextButton(
                    onPressed: () => Navigator.pop(ctx, true),
                    child: const Text('确定'),
                  ),
                ],
              ),
            );
            if (confirmed == true) {
              await ref.read(authNotifierProvider.notifier).logout();
            }
          },
        ),
      ],
    );
  }

  Widget _buildProfileSection(
    BuildContext context,
    WidgetRef ref,
    Authenticated authState,
  ) {
    final user = authState.user;
    final initial = (user.displayName?.isNotEmpty == true)
        ? user.displayName!.substring(0, 1).toUpperCase()
        : '?';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          CircleAvatar(
            radius: 28,
            child: Text(initial, style: const TextStyle(fontSize: 24)),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  user.displayName ?? '未设置昵称',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                Text(
                  user.id,
                  style: Theme.of(context).textTheme.bodySmall,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.edit_outlined),
            onPressed: () => _editProfile(context, ref, user),
          ),
        ],
      ),
    );
  }

  Future<void> _editProfile(
    BuildContext context,
    WidgetRef ref,
    User user,
  ) async {
    final newName = await showDialog<String>(
      context: context,
      builder: (_) => ProfileEditDialog(initialDisplayName: user.displayName),
    );
    if (newName == null) return;

    try {
      final userApi = ref.read(userApiProvider);
      final updated = await userApi.updateProfile(
        user.id,
        displayName: newName,
      );
      await ref.read(authNotifierProvider.notifier).updateUser(updated);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存失败: $e')));
      }
    }
  }
}

/// 游客记录合并状态（只读）。
///
/// 游客记录在本地保存时 `ownerUserId == null`；登录后的首次自动同步经
/// `claimOwner` 认领为当前账号。此处仅展示待合并数量，不触发任何同步——
/// 合并流程仍由 capture 特性自动完成，且按 capture_id 幂等、不产生重复。
///
/// 注意：这里必须读取设备级、未按当前登录会话过滤的
/// [guestLocalCapturesProvider]。若复用按会话过滤的 `localCapturesProvider`，
/// 登录状态下游客记录（owner 为空）会被过滤掉，条数永远为 0。
class _MergeStatusCard extends ConsumerWidget {
  const _MergeStatusCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final captures = ref.watch(guestLocalCapturesProvider);
    final guestCount = captures.valueOrNull?.length ?? 0;
    final loaded = captures.valueOrNull != null;
    final theme = Theme.of(context);

    final statusText = !loaded
        ? '正在读取本地记录…'
        : guestCount == 0
        ? '暂无待合并的游客记录。'
        : '游客记录 $guestCount 条将在登录后自动合并到账号，不产生重复。';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          Icon(Icons.merge_outlined, color: theme.colorScheme.primary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('游客记录与同步', style: theme.textTheme.titleSmall),
                const SizedBox(height: 4),
                Text(statusText, style: theme.textTheme.bodySmall),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 设置页内部使用的「规划中」小标签（避免跨 feature 依赖 workflow 页面）。
class _PlannedChip extends StatelessWidget {
  const _PlannedChip();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        '规划中',
        style: theme.textTheme.labelSmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}
