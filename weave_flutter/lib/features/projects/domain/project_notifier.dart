import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/auth/auth_state.dart';
import '../../../shared/models/project.dart';
import '../../voice/data/project_api.dart';

sealed class ProjectState {
  const ProjectState();
}

class ProjectInitial extends ProjectState {
  const ProjectInitial();
}

class ProjectLoading extends ProjectState {
  const ProjectLoading();
}

class ProjectLoaded extends ProjectState {
  final List<Project> projects;
  const ProjectLoaded({required this.projects});
}

class ProjectError extends ProjectState {
  final String message;
  const ProjectError({required this.message});
}

class ProjectNotifier extends Notifier<ProjectState> {
  late ProjectApi _api;

  @override
  ProjectState build() {
    final apiClient = ref.watch(apiClientProvider);
    _api = ProjectApi(apiClient);
    return const ProjectInitial();
  }

  Future<void> loadProjects() async {
    state = const ProjectLoading();
    try {
      final projects = await _api.listProjects();
      state = ProjectLoaded(projects: projects);
    } catch (e) {
      state = ProjectError(message: '加载项目失败: $e');
    }
  }

  Future<void> createProject(String name, {bool defaultProject = false}) async {
    try {
      await _api.createProject(name: name, defaultProject: defaultProject);
      await loadProjects();
    } catch (e) {
      state = ProjectError(message: '创建项目失败: $e');
    }
  }

  Future<void> updateProject(int id, String name, {bool? defaultProject}) async {
    try {
      await _api.updateProject(id, name: name, defaultProject: defaultProject);
      await loadProjects();
    } catch (e) {
      state = ProjectError(message: '更新项目失败: $e');
    }
  }

  Future<void> deleteProject(int id) async {
    try {
      await _api.deleteProject(id);
      await loadProjects();
    } catch (e) {
      state = ProjectError(message: '删除项目失败: $e');
    }
  }
}

final projectNotifierProvider =
    NotifierProvider<ProjectNotifier, ProjectState>(() => ProjectNotifier());
