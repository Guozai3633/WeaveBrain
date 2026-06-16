import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/models/project.dart';
import '../domain/project_notifier.dart';
import 'project_form_dialog.dart';

class ProjectListScreen extends ConsumerStatefulWidget {
  const ProjectListScreen({super.key});

  @override
  ConsumerState<ProjectListScreen> createState() => _ProjectListScreenState();
}

class _ProjectListScreenState extends ConsumerState<ProjectListScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      ref.read(projectNotifierProvider.notifier).loadProjects();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(projectNotifierProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('项目管理')),
      body: switch (state) {
        ProjectInitial() || ProjectLoading() =>
          const Center(child: CircularProgressIndicator()),
        ProjectError(:final message) => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(message),
                const SizedBox(height: 16),
                OutlinedButton(
                  onPressed: () => ref
                      .read(projectNotifierProvider.notifier)
                      .loadProjects(),
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
        ProjectLoaded(:final projects) => projects.isEmpty
            ? _buildEmptyState(context)
            : ListView.builder(
                itemCount: projects.length,
                itemBuilder: (context, index) =>
                    _buildProjectTile(context, projects[index]),
              ),
      },
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showCreateDialog(context),
        child: const Icon(Icons.add),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.folder_open, size: 64, color: Colors.grey),
          const SizedBox(height: 16),
          Text(
            '还没有项目',
            style: Theme.of(context)
                .textTheme
                .titleMedium
                ?.copyWith(color: Colors.grey),
          ),
          const SizedBox(height: 8),
          Text(
            '点击右下角按钮创建第一个项目',
            style: Theme.of(context)
                .textTheme
                .bodyMedium
                ?.copyWith(color: Colors.grey),
          ),
        ],
      ),
    );
  }

  Widget _buildProjectTile(BuildContext context, Project project) {
    return ListTile(
      leading: Icon(
        project.defaultProject ? Icons.star : Icons.folder_outlined,
        color: project.defaultProject ? Colors.amber : null,
      ),
      title: Text(project.name),
      subtitle: project.defaultProject ? const Text('默认项目') : null,
      trailing: PopupMenuButton<String>(
        onSelected: (action) => _handleAction(action, project),
        itemBuilder: (context) => [
          const PopupMenuItem(value: 'edit', child: Text('编辑')),
          if (!project.defaultProject)
            const PopupMenuItem(value: 'default', child: Text('设为默认')),
          const PopupMenuItem(
            value: 'delete',
            child: Text('删除', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }

  Future<void> _showCreateDialog(BuildContext context) async {
    final result = await showDialog<({String name, bool defaultProject})>(
      context: context,
      builder: (context) => const ProjectFormDialog(),
    );
    if (result != null && mounted) {
      await ref
          .read(projectNotifierProvider.notifier)
          .createProject(result.name, defaultProject: result.defaultProject);
    }
  }

  Future<void> _showEditDialog(BuildContext context, Project project) async {
    final result = await showDialog<({String name, bool defaultProject})>(
      context: context,
      builder: (context) => ProjectFormDialog(project: project),
    );
    if (result != null && mounted) {
      await ref
          .read(projectNotifierProvider.notifier)
          .updateProject(project.id, result.name,
              defaultProject: result.defaultProject);
    }
  }

  Future<void> _handleAction(String action, Project project) async {
    final notifier = ref.read(projectNotifierProvider.notifier);
    switch (action) {
      case 'edit':
        await _showEditDialog(context, project);
      case 'default':
        await notifier.updateProject(project.id, project.name,
            defaultProject: true);
      case 'delete':
        final confirmed = await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            title: const Text('确认删除'),
            content: Text('确定要删除项目 "${project.name}" 吗？'),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('取消'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('删除',
                    style: TextStyle(color: Colors.red)),
              ),
            ],
          ),
        );
        if (confirmed == true && mounted) {
          await notifier.deleteProject(project.id);
        }
    }
  }
}
