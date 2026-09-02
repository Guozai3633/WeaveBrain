import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:weave_flutter/features/platform/data/capabilities_api.dart';
import 'package:weave_flutter/features/platform/domain/capabilities_notifier.dart';
import 'package:weave_flutter/shared/api/api_exception.dart';

class _FakeCapabilitiesGateway implements CapabilitiesGateway {
  PlatformCapabilities result = const PlatformCapabilities.disabled();
  ApiException? error;
  int calls = 0;

  @override
  Future<PlatformCapabilities> fetch() async {
    calls++;
    final err = error;
    if (err != null) throw err;
    return result;
  }
}

ProviderContainer _container(_FakeCapabilitiesGateway gateway) {
  final c = ProviderContainer(
    overrides: [capabilitiesGatewayProvider.overrideWithValue(gateway)],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  group('capabilitiesProvider', () {
    test('exposes server capabilities as data', () async {
      final gateway = _FakeCapabilitiesGateway()
        ..result = const PlatformCapabilities(
          mobileCapture: true,
          webReview: true,
          workflowDesigner: false,
          workflowExecution: false,
          supportedCaptureSources: ['text', 'audio'],
        );
      final container = _container(gateway);

      final capabilities = await container.read(capabilitiesProvider.future);

      expect(gateway.calls, 1);
      expect(capabilities.mobileCapture, isTrue);
      expect(capabilities.workflowAvailable, isFalse);
      expect(capabilities.supportedCaptureSources, ['text', 'audio']);
    });

    test('falls back to disabled defaults on server error', () async {
      final gateway = _FakeCapabilitiesGateway()
        ..error = ApiException(statusCode: 503, message: 'unreachable');
      final container = _container(gateway);

      final capabilities = await container.read(capabilitiesProvider.future);

      // Failure must never imply workflow is available.
      expect(capabilities.mobileCapture, isFalse);
      expect(capabilities.webReview, isFalse);
      expect(capabilities.workflowDesigner, isFalse);
      expect(capabilities.workflowExecution, isFalse);
      expect(capabilities.workflowAvailable, isFalse);
      expect(capabilities.supportedCaptureSources, isEmpty);
    });
  });

  test('workflowPlannedOnly is true while both workflow flags are false', () {
    const caps = PlatformCapabilities(
      mobileCapture: true,
      webReview: true,
      workflowDesigner: false,
      workflowExecution: false,
      supportedCaptureSources: ['text'],
    );
    expect(workflowPlannedOnly(caps), isTrue);
  });
}
