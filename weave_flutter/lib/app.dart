import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/ui/login_screen.dart';
import '../features/auth/ui/register_screen.dart';
import '../features/capture/ui/capture_screen.dart';
import '../features/capture/ui/recording_screen.dart';
import '../features/capture/ui/transcript_edit_screen.dart';
import '../features/echo/ui/echo_screen.dart';
import '../features/ideas/ui/idea_detail_screen.dart';
import '../features/ideas/ui/ideas_screen.dart';
import '../features/imports/ui/import_screen.dart';
import '../features/memories/ui/memory_detail_screen.dart';
import '../features/memories/ui/memory_list_screen.dart';
import '../features/projects/ui/project_list_screen.dart';
import '../features/settings/ui/ai_settings_screen.dart';
import '../features/settings/ui/mcp_settings_screen.dart';
import '../features/settings/ui/settings_screen.dart';
import '../features/timeline/ui/timeline_screen.dart';
import '../features/workflows/ui/workflow_designer_screen.dart';
import '../features/workflows/ui/workflow_list_screen.dart';
import '../shared/auth/auth_state.dart';
import '../shared/models/idea.dart';
import '../shared/widgets/app_scaffold.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authNotifierProvider);

  return GoRouter(
    initialLocation: '/',
    redirect: (context, state) {
      final isLoggedIn = authState is Authenticated;
      final matched = state.matchedLocation;
      final isAuthRoute = matched == '/login' || matched == '/register';

      if (!isLoggedIn) {
        if (isAuthRoute) return null;
        // 游客默认落在全局捕捉页。
        if (matched == '/') return '/capture';
        final isGuestAccessible =
            matched == '/capture' ||
            matched == '/record' ||
            matched == '/echo' ||
            matched.startsWith('/captures/');
        if (!isGuestAccessible) return '/login';
        return null;
      }
      if (isAuthRoute) return '/';
      return null;
    },
    routes: [
      GoRoute(path: '/login', builder: (context, state) => const LoginScreen()),
      GoRoute(
        path: '/register',
        builder: (context, state) => const RegisterScreen(),
      ),
      GoRoute(
        path: '/record',
        builder: (context, state) => const RecordingScreen(),
      ),
      GoRoute(
        path: '/captures/:captureId/transcript',
        builder: (context, state) {
          final captureId = state.pathParameters['captureId'];
          if (captureId == null || captureId.isEmpty) {
            return const Scaffold(body: Center(child: Text('缺少转写参数')));
          }
          return TranscriptEditScreen(captureId: captureId);
        },
      ),
      // 隐藏路由：旧功能保留入口，不在底部导航展示。
      GoRoute(path: '/ideas', builder: (context, state) => const IdeasScreen()),
      GoRoute(
        path: '/ideas/:id',
        builder: (context, state) {
          final idea = state.extra as Idea?;
          if (idea != null) {
            return IdeaDetailScreen(idea: idea);
          }
          return const Scaffold(body: Center(child: Text('想法数据未找到')));
        },
      ),
      GoRoute(
        path: '/timeline',
        builder: (context, state) => const TimelineScreen(),
      ),
      GoRoute(
        path: '/projects',
        builder: (context, state) => const ProjectListScreen(),
      ),
      GoRoute(
        path: '/memories/:captureId',
        builder: (context, state) {
          final captureId = state.pathParameters['captureId'];
          if (captureId == null || captureId.isEmpty) {
            return const Scaffold(body: Center(child: Text('缺少记忆参数')));
          }
          return MemoryDetailScreen(captureId: captureId);
        },
      ),
      GoRoute(
        path: '/imports',
        builder: (context, state) => const ImportScreen(),
      ),
      GoRoute(
        path: '/workflows',
        builder: (context, state) => const WorkflowListScreen(),
      ),
      GoRoute(
        path: '/workflows/:workflowId/designer',
        builder: (context, state) {
          final workflowId = state.pathParameters['workflowId'];
          if (workflowId == null || workflowId.isEmpty) {
            return const Scaffold(body: Center(child: Text('缺少工作流参数')));
          }
          return WorkflowDesignerScreen(workflowId: workflowId);
        },
      ),
      StatefulShellRoute.indexedStack(
        builder: (context, state, shell) => AppScaffold(navigationShell: shell),
        branches: [
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/',
                builder: (context, state) => const MemoryListScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/capture',
                builder: (context, state) => const CaptureScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/echo',
                builder: (context, state) => const EchoScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/settings',
                builder: (context, state) => const SettingsScreen(),
                routes: [
                  GoRoute(
                    path: 'mcp',
                    builder: (context, state) => const McpSettingsScreen(),
                  ),
                  GoRoute(
                    path: 'ai',
                    builder: (context, state) => const AISettingsScreen(),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
});
