import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/auth/auth_state.dart';
import '../domain/capture_providers.dart';
import '../domain/local_capture_store.dart';

/// Shows the latest transcript of an audio capture and lets the user correct
/// it. Corrections are pushed to the server (PATCH transcript) when signed in
/// and mirrored into the local store so they survive restarts.
class TranscriptEditScreen extends ConsumerStatefulWidget {
  const TranscriptEditScreen({super.key, required this.captureId});

  final String captureId;

  @override
  ConsumerState<TranscriptEditScreen> createState() =>
      _TranscriptEditScreenState();
}

class _TranscriptEditScreenState extends ConsumerState<TranscriptEditScreen> {
  final _controller = TextEditingController();
  bool _loading = true;
  bool _saving = false;
  String? _source;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final LocalCaptureStore store;
    try {
      store = await ref.read(localCaptureStoreProvider.future);
    } catch (_) {
      if (mounted) setState(() => _loading = false);
      return;
    }

    final capture = await store.getById(widget.captureId);
    final localTranscript = capture?.transcript;
    if (localTranscript != null && localTranscript.isNotEmpty) {
      _controller.text = localTranscript;
      _source = capture?.transcriptSource;
    }

    final authState = ref.read(authNotifierProvider);
    if (authState is Authenticated) {
      try {
        final gateway = ref.read(audioRemoteGatewayProvider);
        final latest = await gateway.getLatestTranscript(widget.captureId);
        if (latest.text.isNotEmpty) {
          _controller.text = latest.text;
          _source = latest.source;
        }
      } catch (_) {
        // Server transcript unavailable (e.g. not synced yet); keep local.
      }
    }

    if (mounted) setState(() => _loading = false);
  }

  Future<void> _save() async {
    final text = _controller.text.trim();
    if (text.isEmpty) return;
    setState(() => _saving = true);

    final LocalCaptureStore store;
    try {
      store = await ref.read(localCaptureStoreProvider.future);
    } catch (e) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存失败：$e')));
      }
      return;
    }

    try {
      final authState = ref.read(authNotifierProvider);
      if (authState is Authenticated) {
        final gateway = ref.read(audioRemoteGatewayProvider);
        final updated = await gateway.correctTranscript(
          widget.captureId,
          text,
        );
        await store.saveTranscript(
          widget.captureId,
          text: updated.text,
          source: updated.source,
          version: updated.version,
        );
        _source = updated.source;
      } else {
        // Guest: keep the corrected text locally for later sync.
        await store.saveTranscript(
          widget.captureId,
          text: text,
          source: 'user',
          version: 1,
        );
        _source = 'user';
      }
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('转写已保存')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('转写修正')),
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      children: [
                        const Icon(Icons.mic, size: 18),
                        const SizedBox(width: 6),
                        Text(
                          _source == 'user' ? '人工修正' : '自动转写',
                          style: Theme.of(context).textTheme.labelLarge,
                        ),
                        const Spacer(),
                        if (_controller.text.isEmpty)
                          const Text('转写生成中，请稍后回来查看…'),
                      ],
                    ),
                    const SizedBox(height: 12),
                    Expanded(
                      child: TextField(
                        key: const Key('transcript_edit_field'),
                        controller: _controller,
                        minLines: 8,
                        maxLines: 20,
                        textInputAction: TextInputAction.newline,
                        decoration: const InputDecoration(
                          border: OutlineInputBorder(),
                          hintText: '这里显示语音转写结果，可直接修改。',
                        ),
                      ),
                    ),
                    const SizedBox(height: 12),
                    FilledButton.icon(
                      key: const Key('transcript_save_button'),
                      onPressed: _saving ? null : _save,
                      icon: _saving
                          ? const SizedBox.square(
                              dimension: 18,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.save_outlined),
                      label: const Text('保存修正'),
                    ),
                  ],
                ),
              ),
      ),
    );
  }
}
