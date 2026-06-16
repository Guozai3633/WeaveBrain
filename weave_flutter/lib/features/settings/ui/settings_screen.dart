import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/user.dart';
import '../data/user_api.dart';
import 'profile_edit_dialog.dart';

final userApiProvider = Provider<UserApi>((ref) {
  final client = ref.watch(apiClientProvider);
  return UserApi(client);
});

final sttConfigProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  final client = ref.watch(apiClientProvider);
  final resp = await client.get('/config/stt');
  return resp.data as Map<String, dynamic>;
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
        ],

        // --- STT Provider ---
        ListTile(
          leading: const Icon(Icons.mic_outlined),
          title: const Text('语音识别引擎'),
          subtitle: sttAsync.when(
            data: (data) => Text(data['provider'] as String? ?? 'unknown'),
            loading: () => const Text('加载中...'),
            error: (_, __) => const Text('获取失败'),
          ),
        ),
        const Divider(),

        // --- API Config (placeholder) ---
        ListTile(
          leading: const Icon(Icons.key_outlined),
          title: const Text('API 配置'),
          subtitle: const Text('管理第三方 API Key'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () {
            showDialog(
              context: context,
              builder: (ctx) => AlertDialog(
                title: const Text('API 配置'),
                content: const Text('此功能即将支持，敬请期待！'),
                actions: [
                  TextButton(
                    onPressed: () => Navigator.pop(ctx),
                    child: const Text('好的'),
                  ),
                ],
              ),
            );
          },
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
              children: const [
                Text('以语音为入口、Agent为引擎的个人思维外脑'),
              ],
            );
          },
        ),
        const Divider(),

        // --- Logout ---
        ListTile(
          leading: Icon(Icons.logout, color: Theme.of(context).colorScheme.error),
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
      final updated = await userApi.updateProfile(user.id, displayName: newName);
      await ref.read(authNotifierProvider.notifier).updateUser(updated);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('保存失败: $e')),
        );
      }
    }
  }
}
