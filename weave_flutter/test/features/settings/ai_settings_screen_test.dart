import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/settings/data/ai_settings_api.dart';
import 'package:weave_flutter/features/settings/ui/ai_settings_screen.dart';

class _FakeGateway implements AISettingsGateway {
  _FakeGateway({required AISettingsResult initial}) : result = initial;

  AISettingsResult result;
  bool? lastAiMemory;
  bool? lastSpeech;
  int reorganizeCalls = 0;

  @override
  Future<AISettingsResult> get() async => result;

  @override
  Future<AISettings> update({
    required int expectedRevision,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
  }) async {
    lastAiMemory = aiMemoryEnabled;
    lastSpeech = speechToTextEnabled;
    final updated = result.settings.copyWith(
      aiMemoryEnabled: aiMemoryEnabled,
      aiCompletionEnabled: aiCompletionEnabled,
      speechToTextEnabled: speechToTextEnabled,
      cloudTextAllowed: cloudTextAllowed,
      cloudAudioAllowed: cloudAudioAllowed,
      revision: expectedRevision + 1,
    );
    result = result.copyWith(settings: updated);
    return updated;
  }

  @override
  Future<int> reorganize() async {
    reorganizeCalls += 1;
    result = result.copyWith(pendingReorganize: 0);
    return 1;
  }
}

const _defaultSettings = AISettings(
  userId: 'u1',
  aiMemoryEnabled: false,
  aiCompletionEnabled: true,
  speechToTextEnabled: false,
  cloudTextAllowed: true,
  cloudAudioAllowed: false,
  revision: 3,
);

Widget _buildScreen(_FakeGateway gateway) {
  return ProviderScope(
    overrides: [aiSettingsGatewayProvider.overrideWithValue(gateway)],
    child: const MaterialApp(home: AISettingsScreen()),
  );
}

SwitchListTile _switchTile(WidgetTester tester, String label) {
  return tester.widget<SwitchListTile>(
    find.widgetWithText(SwitchListTile, label),
  );
}

void main() {
  testWidgets('renders switches, gating values and revision', (tester) async {
    final gateway = _FakeGateway(
      initial: const AISettingsResult(
        settings: _defaultSettings,
        pendingReorganize: 4,
      ),
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pump();
    await tester.pump();

    expect(_switchTile(tester, 'AI 记忆整理').value, isFalse);
    expect(_switchTile(tester, '语音转写').value, isFalse);
    expect(_switchTile(tester, 'AI 补全').value, isTrue);
    expect(_switchTile(tester, '云端文本处理').value, isTrue);
    expect(_switchTile(tester, '云端音频处理').value, isFalse);
    expect(find.text('revision 3'), findsOneWidget);

    // master off => no reorganize button despite pending count.
    expect(find.text('补整理未处理记忆 (4 条)'), findsNothing);
  });

  testWidgets('gated sub-switches are disabled while master is off', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      initial: const AISettingsResult(
        settings: _defaultSettings,
        pendingReorganize: 0,
      ),
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pump();
    await tester.pump();

    expect(_switchTile(tester, 'AI 补全').onChanged, isNull);
    expect(_switchTile(tester, '云端文本处理').onChanged, isNull);
    expect(_switchTile(tester, '云端音频处理').onChanged, isNull);
    // 语音转写独立于总开关，仍然可切换。
    expect(_switchTile(tester, '语音转写').onChanged, isNotNull);

    // 点击被禁用的子开关不会改变其值。
    await tester.tap(find.text('云端音频处理'));
    await tester.pump();
    expect(_switchTile(tester, '云端音频处理').value, isFalse);
  });

  testWidgets('independent speech switch saves and shows a snackbar', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      initial: const AISettingsResult(
        settings: _defaultSettings,
        pendingReorganize: 0,
      ),
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.text('语音转写'));
    await tester.pumpAndSettle();

    expect(gateway.lastSpeech, isTrue);
    expect(_switchTile(tester, '语音转写').value, isTrue);
    expect(find.text('已保存'), findsOneWidget);
  });

  testWidgets('master switch toggles and unlocks gated switches', (tester) async {
    final gateway = _FakeGateway(
      initial: const AISettingsResult(
        settings: _defaultSettings,
        pendingReorganize: 0,
      ),
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.text('AI 记忆整理'));
    await tester.pumpAndSettle();

    expect(gateway.lastAiMemory, isTrue);
    expect(_switchTile(tester, 'AI 记忆整理').value, isTrue);
    expect(_switchTile(tester, 'AI 补全').onChanged, isNotNull);
    expect(_switchTile(tester, '云端音频处理').onChanged, isNotNull);
  });

  testWidgets('reorganize button appears when pending>0 and tap clears count', (
    tester,
  ) async {
    final gateway = _FakeGateway(
      initial: const AISettingsResult(
        settings: AISettings(
          userId: 'u1',
          aiMemoryEnabled: true,
          aiCompletionEnabled: true,
          speechToTextEnabled: false,
          cloudTextAllowed: true,
          cloudAudioAllowed: false,
          revision: 3,
        ),
        pendingReorganize: 3,
      ),
    );
    await tester.pumpWidget(_buildScreen(gateway));
    await tester.pump();
    await tester.pump();

    expect(find.text('补整理未处理记忆 (3 条)'), findsOneWidget);

    await tester.tap(find.text('补整理未处理记忆 (3 条)'));
    await tester.pumpAndSettle();

    expect(gateway.reorganizeCalls, 1);
    expect(find.text('补整理未处理记忆 (3 条)'), findsNothing);
    expect(find.text('已重新加入整理队列'), findsOneWidget);
  });
}
