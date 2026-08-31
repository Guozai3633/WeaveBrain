import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/settings/data/ai_settings_api.dart';
import 'package:weave_flutter/features/settings/domain/ai_settings_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class _FakeGateway implements AISettingsGateway {
  _FakeGateway({AISettingsResult? initial})
    : result =
          initial ??
          AISettingsResult(
            settings: const AISettings(
              userId: 'u1',
              aiMemoryEnabled: false,
              aiCompletionEnabled: false,
              speechToTextEnabled: false,
              cloudTextAllowed: false,
              cloudAudioAllowed: false,
              revision: 0,
            ),
            pendingReorganize: 0,
          );

  AISettingsResult result;
  bool getCalled = false;
  bool reorganizeCalled = false;
  ApiException? getError;
  ApiException? updateError;
  ApiException? reorganizeError;
  int? lastExpectedRevision;
  bool? lastAiMemory;
  bool? lastAiCompletion;
  bool? lastSpeech;
  bool? lastCloudText;
  bool? lastCloudAudio;

  @override
  Future<AISettingsResult> get() async {
    getCalled = true;
    final error = getError;
    if (error != null) throw error;
    return result;
  }

  @override
  Future<AISettings> update({
    required int expectedRevision,
    bool? aiMemoryEnabled,
    bool? aiCompletionEnabled,
    bool? speechToTextEnabled,
    bool? cloudTextAllowed,
    bool? cloudAudioAllowed,
  }) async {
    final error = updateError;
    if (error != null) throw error;
    lastExpectedRevision = expectedRevision;
    lastAiMemory = aiMemoryEnabled;
    lastAiCompletion = aiCompletionEnabled;
    lastSpeech = speechToTextEnabled;
    lastCloudText = cloudTextAllowed;
    lastCloudAudio = cloudAudioAllowed;
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
    reorganizeCalled = true;
    final error = reorganizeError;
    if (error != null) throw error;
    // 模拟补整理完成后 pending 计数归零。
    result = result.copyWith(pendingReorganize: 0);
    return 1;
  }
}

void main() {
  final initialSettings = const AISettings(
    userId: 'u1',
    aiMemoryEnabled: false,
    aiCompletionEnabled: true,
    speechToTextEnabled: false,
    cloudTextAllowed: true,
    cloudAudioAllowed: false,
    revision: 3,
  );

  ProviderContainer container({AISettingsGateway? gateway}) {
    final c = ProviderContainer(
      overrides: [
        aiSettingsGatewayProvider.overrideWithValue(
          gateway ??
              _FakeGateway(
                initial: AISettingsResult(
                  settings: initialSettings,
                  pendingReorganize: 4,
                ),
              ),
        ),
      ],
    );
    addTearDown(c.dispose);
    return c;
  }

  test('load publishes loaded state with settings and pending count', () async {
    final c = container();
    final notifier = c.read(aiSettingsNotifierProvider.notifier);

    await notifier.load();

    final state = c.read(aiSettingsNotifierProvider);
    expect(state, isA<AISettingsLoaded>());
    final loaded = state as AISettingsLoaded;
    expect(loaded.result.settings.revision, 3);
    expect(loaded.result.pendingReorganize, 4);
  });

  test('load failure publishes error state', () async {
    final gateway = _FakeGateway()..getError = ApiException(message: 'boom');
    final c = container(gateway: gateway);
    final notifier = c.read(aiSettingsNotifierProvider.notifier);

    await notifier.load();

    expect(c.read(aiSettingsNotifierProvider), isA<AISettingsError>());
  });

  test('setSwitch sends expected_revision and only the target field', () async {
    final c = container();
    final notifier = c.read(aiSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.setSwitch(AISettingsField.aiMemory, true);

    final gateway = c.read(aiSettingsGatewayProvider) as _FakeGateway;
    expect(gateway.lastExpectedRevision, 3);
    expect(gateway.lastAiMemory, isTrue);
    expect(gateway.lastAiCompletion, isNull);
    expect(gateway.lastSpeech, isNull);
    expect(gateway.lastCloudText, isNull);
    expect(gateway.lastCloudAudio, isNull);

    final loaded = c.read(aiSettingsNotifierProvider) as AISettingsLoaded;
    expect(loaded.result.settings.aiMemoryEnabled, isTrue);
    expect(loaded.result.settings.revision, 4);
    expect(loaded.message, '已保存');
  });

  test('setSwitch surfaces version-conflict message on 409', () async {
    final c = container();
    final gateway = c.read(aiSettingsGatewayProvider) as _FakeGateway;
    gateway.updateError = ApiException(statusCode: 409, message: 'HTTP 409');
    final notifier = c.read(aiSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.setSwitch(AISettingsField.cloudText, true);

    final loaded = c.read(aiSettingsNotifierProvider) as AISettingsLoaded;
    expect(loaded.message, '设置已在其他设备修改，请刷新');
    expect(loaded.saving, isFalse);
  });

  test('reorganize re-enqueues and refreshes the pending count', () async {
    final c = container();
    final notifier = c.read(aiSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.reorganize();

    final gateway = c.read(aiSettingsGatewayProvider) as _FakeGateway;
    expect(gateway.reorganizeCalled, isTrue);
    final loaded = c.read(aiSettingsNotifierProvider) as AISettingsLoaded;
    expect(loaded.result.pendingReorganize, 0);
    expect(loaded.message, '已重新加入整理队列');
  });

  test('reorganize surfaces feature-not-enabled message on 409', () async {
    final c = container();
    final gateway = c.read(aiSettingsGatewayProvider) as _FakeGateway;
    gateway.reorganizeError = ApiException(statusCode: 409, message: 'HTTP 409');
    final notifier = c.read(aiSettingsNotifierProvider.notifier);
    await notifier.load();

    await notifier.reorganize();

    final loaded = c.read(aiSettingsNotifierProvider) as AISettingsLoaded;
    expect(loaded.message, '请先开启 AI 记忆整理');
  });

  test('clearMessage clears the last operation message', () async {
    final c = container();
    final notifier = c.read(aiSettingsNotifierProvider.notifier);
    await notifier.load();
    await notifier.setSwitch(AISettingsField.aiMemory, true);
    expect(
      (c.read(aiSettingsNotifierProvider) as AISettingsLoaded).message,
      '已保存',
    );

    notifier.clearMessage();

    expect(
      (c.read(aiSettingsNotifierProvider) as AISettingsLoaded).message,
      isNull,
    );
  });
}
