import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/models/agent_response.dart';
import '../../../shared/widgets/project_selector.dart';
import '../domain/voice_notifier.dart';
import 'voice_button.dart';

class VoiceScreen extends ConsumerStatefulWidget {
  const VoiceScreen({super.key});

  @override
  ConsumerState<VoiceScreen> createState() => _VoiceScreenState();
}

class _VoiceScreenState extends ConsumerState<VoiceScreen> {
  int? _selectedProjectId;

  @override
  Widget build(BuildContext context) {
    final voiceState = ref.watch(voiceNotifierProvider);

    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 24, 24, 16),
      child: Column(
        children: [
          // Project selector
          ProjectSelector(
            selectedProjectId: _selectedProjectId,
            onChanged: (id) => setState(() => _selectedProjectId = id),
          ),
          const Spacer(flex: 2),

          // Voice button
          VoiceButton(
            state: _buttonState(voiceState),
            onPressed: () => _toggleRecording(ref, voiceState),
          ),
          const SizedBox(height: 32),

          // Status / transcription text
          Expanded(flex: 3, child: _buildResultArea(context, ref, voiceState)),
        ],
      ),
    );
  }

  VoiceButtonState _buttonState(VoiceState state) {
    return switch (state) {
      VoiceConnecting() => VoiceButtonState.recording,
      VoiceRecording() => VoiceButtonState.recording,
      VoiceProcessing() => VoiceButtonState.processing,
      _ => VoiceButtonState.idle,
    };
  }

  void _toggleRecording(WidgetRef ref, VoiceState state) {
    final notifier = ref.read(voiceNotifierProvider.notifier);
    if (state is VoiceIdle ||
        state is VoiceResult ||
        state is VoiceError ||
        state is VoiceSaved) {
      notifier.startRecording();
    } else if (state is VoiceRecording || state is VoiceConnecting) {
      notifier.stopRecording();
    }
  }

  Widget _buildResultArea(
    BuildContext context,
    WidgetRef ref,
    VoiceState state,
  ) {
    return switch (state) {
      VoiceIdle() => Text(
        '点击麦克风开始录音',
        style: Theme.of(
          context,
        ).textTheme.bodyLarge?.copyWith(color: Colors.grey),
        textAlign: TextAlign.center,
      ),
      VoiceConnecting() => const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('正在连接...'),
        ],
      ),
      VoiceRecording(:final partialText) => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Text('录音中...', style: TextStyle(color: Colors.red)),
          const SizedBox(height: 16),
          if (partialText.isNotEmpty)
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Text(
                  partialText,
                  style: Theme.of(context).textTheme.bodyLarge,
                ),
              ),
            ),
        ],
      ),
      VoiceProcessing(:final finalText) => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text('正在处理: $finalText'),
        ],
      ),
      VoiceResult(:final text, :final agentResponse) => _buildStructuredResult(
        context,
        ref,
        text,
        agentResponse,
      ),
      VoiceError(:final message) => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.error_outline,
            size: 48,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            message,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
          const SizedBox(height: 16),
          OutlinedButton(
            onPressed: () => ref.read(voiceNotifierProvider.notifier).reset(),
            child: const Text('重试'),
          ),
        ],
      ),
      VoiceSaving() => const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('正在保存...'),
        ],
      ),
      VoiceSaved() => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.check_circle, size: 48, color: Colors.green.shade600),
          const SizedBox(height: 16),
          const Text('想法已保存', style: TextStyle(fontWeight: FontWeight.bold)),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: () => ref.read(voiceNotifierProvider.notifier).reset(),
            icon: const Icon(Icons.mic),
            label: const Text('继续录音'),
          ),
        ],
      ),
    };
  }

  Widget _buildStructuredResult(
    BuildContext context,
    WidgetRef ref,
    String text,
    AgentResponse? agentResponse,
  ) {
    final feasibility = agentResponse?.feasibility;

    return SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Transcription card
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('转写结果', style: Theme.of(context).textTheme.labelLarge),
                  const SizedBox(height: 8),
                  Text(text, style: Theme.of(context).textTheme.bodyLarge),
                ],
              ),
            ),
          ),

          if (agentResponse != null) ...[
            const SizedBox(height: 12),

            // Tags
            if (agentResponse.tags.isNotEmpty)
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('标签', style: Theme.of(context).textTheme.labelLarge),
                      const SizedBox(height: 8),
                      Wrap(
                        spacing: 8,
                        runSpacing: 4,
                        children: agentResponse.tags.map((tag) {
                          return Chip(
                            label: Text(tag),
                            backgroundColor: _feasibilityColor(
                              context,
                              feasibility,
                            ).withValues(alpha: 0.15),
                          );
                        }).toList(),
                      ),
                    ],
                  ),
                ),
              ),

            // Feasibility
            if (feasibility != null) ...[
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Row(
                    children: [
                      _feasibilityDots(context, feasibility),
                      const SizedBox(width: 12),
                      Text(
                        _feasibilityLabel(feasibility),
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: _feasibilityColor(context, feasibility),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],

            // Suggestions
            if (agentResponse.suggestions.isNotEmpty) ...[
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '建议行动',
                        style: Theme.of(context).textTheme.labelLarge,
                      ),
                      const SizedBox(height: 8),
                      ...agentResponse.suggestions.asMap().entries.map((e) {
                        return Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '${e.key + 1}. ',
                                style: Theme.of(context).textTheme.bodyMedium
                                    ?.copyWith(fontWeight: FontWeight.bold),
                              ),
                              Expanded(
                                child: Text(
                                  e.value,
                                  style: Theme.of(context).textTheme.bodyMedium,
                                ),
                              ),
                            ],
                          ),
                        );
                      }),
                    ],
                  ),
                ),
              ),
            ],

            // Agent reply
            if (agentResponse.response.isNotEmpty) ...[
              const SizedBox(height: 8),
              Card(
                color: Theme.of(context).colorScheme.primaryContainer,
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    agentResponse.response,
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                ),
              ),
            ],
          ],

          const SizedBox(height: 16),

          // Action buttons
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              OutlinedButton.icon(
                onPressed: () =>
                    ref.read(voiceNotifierProvider.notifier).reset(),
                icon: const Icon(Icons.refresh),
                label: const Text('重新录制'),
              ),
              const SizedBox(width: 16),
              FilledButton.icon(
                onPressed: () => ref
                    .read(voiceNotifierProvider.notifier)
                    .saveIdea(_selectedProjectId ?? 1),
                icon: const Icon(Icons.check),
                label: const Text('确认保存'),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _feasibilityDots(BuildContext context, String feasibility) {
    final filled = switch (feasibility) {
      'high' => 3,
      'medium' => 2,
      'low' => 1,
      _ => 0,
    };
    final color = _feasibilityColor(context, feasibility);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(3, (i) {
        return Icon(
          i < filled ? Icons.circle : Icons.circle_outlined,
          size: 16,
          color: color,
        );
      }),
    );
  }

  Color _feasibilityColor(BuildContext context, String? feasibility) {
    return switch (feasibility) {
      'high' => Colors.green,
      'medium' => Colors.orange,
      'low' => Colors.red,
      _ => Colors.grey,
    };
  }

  String _feasibilityLabel(String feasibility) {
    return switch (feasibility) {
      'high' => '高可行性',
      'medium' => '中等可行性',
      'low' => '低可行性',
      _ => feasibility,
    };
  }
}
