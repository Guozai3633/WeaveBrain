import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/auth/auth_state.dart';
import '../data/mcp_api.dart';

final mcpApiProvider = Provider<McpApi>((ref) {
  final client = ref.watch(apiClientProvider);
  return McpApi(client);
});

final mcpConfigsProvider = FutureProvider.autoDispose<List<McpConfig>>((
  ref,
) async {
  final api = ref.watch(mcpApiProvider);
  return api.getConfigs();
});

class McpSettingsScreen extends ConsumerWidget {
  const McpSettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncConfigs = ref.watch(mcpConfigsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('MCP 工具集成')),
      body: asyncConfigs.when(
        data: (configs) {
          final namespaces = ['notion', 'email'];
          return ListView.builder(
            itemCount: namespaces.length,
            itemBuilder: (context, index) {
              final ns = namespaces[index];
              // Using firstWhere with orElse to support older dart versions safely
              final configList = configs
                  .where((c) => c.namespace == ns)
                  .toList();
              final config = configList.isNotEmpty ? configList.first : null;

              final isConfigured = config?.isConfigured ?? false;
              final isEnabled = config?.enabled ?? false;

              return ListTile(
                leading: Icon(ns == 'notion' ? Icons.description : Icons.email),
                title: Text(ns.toUpperCase()),
                subtitle: Text(
                  isConfigured
                      ? (isEnabled ? '已配置 (已启用)' : '已配置 (已禁用)')
                      : '未配置',
                ),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  _showConfigDialog(context, ref, ns, config);
                },
              );
            },
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, st) => Center(child: Text('加载失败: $e')),
      ),
    );
  }

  void _showConfigDialog(
    BuildContext context,
    WidgetRef ref,
    String namespace,
    McpConfig? config,
  ) {
    final credCtrl = TextEditingController();
    bool enabled = config?.enabled ?? true;

    String hint = namespace == 'notion'
        ? '输入 Notion API Token (secret_...)'
        : '{"host":"smtp.qiye.aliyun.com","port":"465","username":"...","password":"..."}';

    showDialog(
      context: context,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (context, setState) {
            return AlertDialog(
              title: Text('配置 ${namespace.toUpperCase()}'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (config?.isConfigured ?? false)
                      const Padding(
                        padding: EdgeInsets.only(bottom: 8.0),
                        child: Text(
                          '提示：如需更新状态或凭证，请重新输入完整的凭证。留空并保存将删除该配置。',
                          style: TextStyle(color: Colors.grey, fontSize: 12),
                        ),
                      ),
                    TextField(
                      controller: credCtrl,
                      decoration: InputDecoration(
                        labelText: '凭证 (留空以删除)',
                        hintText: hint,
                      ),
                      obscureText: namespace == 'notion',
                      maxLines: namespace == 'email' ? 5 : 1,
                    ),
                    const SizedBox(height: 16),
                    SwitchListTile(
                      title: const Text('启用该工具'),
                      value: enabled,
                      onChanged: (val) {
                        setState(() => enabled = val);
                      },
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx),
                  child: const Text('取消'),
                ),
                TextButton(
                  onPressed: () async {
                    final cred = credCtrl.text.trim();
                    final api = ref.read(mcpApiProvider);
                    try {
                      if (cred.isEmpty) {
                        if (config != null) {
                          await api.deleteConfig(namespace);
                        }
                      } else {
                        await api.upsertConfig(namespace, cred, enabled);
                      }
                      ref.invalidate(mcpConfigsProvider);
                      if (context.mounted) Navigator.pop(ctx);
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(
                          context,
                        ).showSnackBar(SnackBar(content: Text('操作失败: $e')));
                      }
                    }
                  },
                  child: const Text('保存'),
                ),
              ],
            );
          },
        );
      },
    );
  }
}
