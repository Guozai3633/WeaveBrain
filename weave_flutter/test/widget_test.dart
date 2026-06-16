import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:weave_flutter/main.dart';

void main() {
  testWidgets('App renders without crashing', (WidgetTester tester) async {
    await tester.pumpWidget(const ProviderScope(child: WeaveBrainApp()));
    await tester.pumpAndSettle();

    // Should show login screen when not authenticated
    expect(find.text('织脑 - 登录'), findsOneWidget);
  });
}
