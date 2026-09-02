import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/platform/data/capabilities_api.dart';
import 'package:weave_flutter/features/workflows/ui/workflow_designer_screen.dart';
import 'package:weave_flutter/features/workflows/ui/workflow_list_screen.dart';

class _FakeCapabilitiesGateway implements CapabilitiesGateway {
  _FakeCapabilitiesGateway(this.result);

  final PlatformCapabilities result;

  @override
  Future<PlatformCapabilities> fetch() async => result;
}

/// 与后端 GET /api/v3/capabilities 一致：移动/Web 可用，工作流设计/执行关闭。
const _serverCaps = PlatformCapabilities(
  mobileCapture: true,
  webReview: true,
  workflowDesigner: false,
  workflowExecution: false,
  supportedCaptureSources: ['text', 'audio', 'import'],
);

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      capabilitiesGatewayProvider.overrideWithValue(
        _FakeCapabilitiesGateway(_serverCaps),
      ),
    ],
    child: MaterialApp(home: child),
  );
}

void main() {
  testWidgets('workflow list shows planned banner and empty state', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(const WorkflowListScreen()));
    await tester.pumpAndSettle();

    expect(find.text('工作流'), findsWidgets); // AppBar 标题
    expect(find.textContaining('当前版本不提供工作流设计与运行'), findsOneWidget);
    expect(find.textContaining('设计与执行能力均未开放'), findsOneWidget);
    expect(find.text('还没有工作流'), findsOneWidget);
    expect(find.text('新建工作流'), findsOneWidget);
  });

  testWidgets('tapping new workflow shows planned snackbar, never an editor', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(const WorkflowListScreen()));
    await tester.pumpAndSettle();

    await tester.tap(find.text('新建工作流'));
    await tester.pump();

    // 只弹「规划中」提示，绝不跳转假编辑器。
    expect(find.text('规划中 — 工作流设计器将在后续版本开放'), findsOneWidget);
    expect(find.text('工作流设计器'), findsNothing);
    expect(find.byType(WorkflowDesignerScreen), findsNothing);
  });

  testWidgets('designer placeholder shows planned status and workflow id', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(const WorkflowDesignerScreen(workflowId: 'daily-review')),
    );
    await tester.pumpAndSettle();

    expect(find.text('工作流设计器'), findsOneWidget); // AppBar 标题
    expect(find.text('规划中'), findsOneWidget);
    expect(find.textContaining('「daily-review」的方案设计器尚未开放'), findsOneWidget);
  });
}
