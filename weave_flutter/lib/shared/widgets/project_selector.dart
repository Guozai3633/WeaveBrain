import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/voice/data/project_api.dart';
import '../auth/auth_state.dart';
import '../models/project.dart';

final projectsProvider = FutureProvider<List<Project>>((ref) async {
  final apiClient = ref.watch(apiClientProvider);
  final api = ProjectApi(apiClient);
  return api.listProjects();
});

class ProjectSelector extends ConsumerWidget {
  final int? selectedProjectId;
  final ValueChanged<int?> onChanged;

  const ProjectSelector({
    super.key,
    this.selectedProjectId,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final projectsAsync = ref.watch(projectsProvider);

    return projectsAsync.when(
      loading: () => Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
              const SizedBox(width: 12),
              Text('加载项目...', style: Theme.of(context).textTheme.bodyLarge),
            ],
          ),
        ),
      ),
      error: (e, _) => Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              const Icon(Icons.error_outline, color: Colors.red),
              const SizedBox(width: 12),
              Expanded(
                child: Text('加载失败', style: Theme.of(context).textTheme.bodyLarge),
              ),
              IconButton(
                icon: const Icon(Icons.refresh),
                onPressed: () => ref.invalidate(projectsProvider),
              ),
            ],
          ),
        ),
      ),
      data: (projects) {
        if (projects.isEmpty) {
          return Card(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              child: Row(
                children: [
                  const Icon(Icons.folder_outlined),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text('暂无项目', style: Theme.of(context).textTheme.bodyLarge),
                  ),
                ],
              ),
            ),
          );
        }

        // Auto-select default project if nothing selected
        final effectiveId = selectedProjectId ??
            projects.firstWhere(
              (p) => p.defaultProject,
              orElse: () => projects.first,
            ).id;

        return Card(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: DropdownButtonHideUnderline(
              child: DropdownButton<int>(
                value: effectiveId,
                isExpanded: true,
                icon: const Icon(Icons.arrow_drop_down),
                items: projects.map((p) {
                  return DropdownMenuItem<int>(
                    value: p.id,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 8),
                      child: Row(
                        children: [
                          const Icon(Icons.folder_outlined, size: 20),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              p.name,
                              style: Theme.of(context).textTheme.bodyLarge,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          if (p.defaultProject)
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 6,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: Theme.of(context)
                                    .colorScheme
                                    .primaryContainer,
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: Text(
                                '默认',
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            ),
                        ],
                      ),
                    ),
                  );
                }).toList(),
                onChanged: onChanged,
              ),
            ),
          ),
        );
      },
    );
  }
}
