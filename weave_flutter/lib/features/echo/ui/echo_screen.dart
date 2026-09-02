import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../shared/auth/auth_state.dart';
import '../../memories/ui/memory_list_screen.dart' show kPrimaryTypeLabels;
import '../data/echo_api.dart';
import '../domain/echo_notifier.dart';

/// 回响：服务端选定的单张记忆卡片 + 出现原因。本页按状态展示：
/// 游客登录引导 / 已关闭 / 空态（下次到点或暂无候选）/ 待回应的卡片。
class EchoScreen extends ConsumerStatefulWidget {
  const EchoScreen({super.key});

  @override
  ConsumerState<EchoScreen> createState() => _EchoScreenState();
}

class _EchoScreenState extends ConsumerState<EchoScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(echoNotifierProvider.notifier).load();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(echoNotifierProvider);
    final auth = ref.watch(authNotifierProvider);

    ref.listen<EchoState>(echoNotifierProvider, (previous, next) {
      if (next is EchoLoaded && next.message != null) {
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(SnackBar(content: Text(next.message!)));
        ref.read(echoNotifierProvider.notifier).clearMessage();
      }
    });

    // 登录态变化时按需刷新；游客不发请求。
    ref.listen<AuthState>(authNotifierProvider, (previous, next) {
      if (next is Authenticated) {
        ref.read(echoNotifierProvider.notifier).load();
      } else if (next is! Authenticated && state is! EchoLoading) {
        ref.read(echoNotifierProvider.notifier).load();
      }
    });

    return Scaffold(
      appBar: AppBar(
        title: const Text('回响'),
        actions: [
          if (auth is Authenticated)
            IconButton(
              tooltip: '回响设置',
              onPressed: () => context.push('/settings/echo'),
              icon: const Icon(Icons.tune),
            ),
        ],
      ),
      body: SafeArea(child: _bodyFor(state)),
    );
  }

  Widget _bodyFor(EchoState state) {
    return switch (state) {
      EchoLoading() => const Center(child: CircularProgressIndicator()),
      EchoGuest() => const _GuestView(),
      EchoDisabled() => _DisabledView(cadence: state.cadence),
      EchoEmpty() => _EmptyView(state: state),
      EchoLoaded() => _EchoCardView(state: state),
      EchoError() => _ErrorView(
        message: state.message,
        onRetry: () => ref.read(echoNotifierProvider.notifier).load(),
      ),
    };
  }
}

class _GuestView extends StatelessWidget {
  const _GuestView();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.auto_awesome_outlined, size: 64),
            const SizedBox(height: 16),
            Text(
              '登录后开启回响',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              '回响会把较早的记忆带回眼前。登录后即可在设置中开启。',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: 20),
            FilledButton(
              key: const Key('echo_login_button'),
              onPressed: () => context.push('/login'),
              child: const Text('登录 / 注册'),
            ),
          ],
        ),
      ),
    );
  }
}

class _DisabledView extends StatelessWidget {
  const _DisabledView({required this.cadence});

  final String cadence;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.notifications_off_outlined, size: 64),
            const SizedBox(height: 16),
            Text('回响已关闭', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              '开启后，织脑会隔一段时间挑一条旧记忆回来，提醒你重新看看。',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: 20),
            FilledButton.tonal(
              onPressed: () => context.push('/settings/echo'),
              child: const Text('去开启回响'),
            ),
          ],
        ),
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({required this.state});

  final EchoEmpty state;

  @override
  Widget build(BuildContext context) {
    final icon = state.noCandidates
        ? Icons.inbox_outlined
        : Icons.schedule_outlined;
    final title = state.noCandidates ? '还没有可回看的记忆' : '下次回响还没到';
    final next = state.nextDueAt;
    final detail = state.noCandidates
        ? '先记下一些想法，等它们沉淀后就会在这里出现。'
        : next == null
        ? '稍后再来看看。'
        : '预计 ${DateFormat('M月d日 HH:mm').format(next.toLocal())} 之后再来。';

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 64),
            const SizedBox(height: 16),
            Text(title, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              detail,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: 20),
            OutlinedButton.icon(
              onPressed: () => context.push('/settings/echo'),
              icon: const Icon(Icons.tune),
              label: const Text('回响设置'),
            ),
          ],
        ),
      ),
    );
  }
}

class _EchoCardView extends ConsumerWidget {
  const _EchoCardView({required this.state});

  final EchoLoaded state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final card = state.card;
    final memory = card.memory;
    final notifier = ref.read(echoNotifierProvider.notifier);

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // 原因说明：每次回响都要解释「为什么是这条」。
        Card(
          color: Theme.of(
            context,
          ).colorScheme.secondaryContainer.withValues(alpha: 0.4),
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                const Icon(Icons.auto_awesome, size: 18),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    '这次回响：${card.reason.text}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        // 记忆卡片（可点进详情）。
        Card(
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            key: const Key('echo_memory_card'),
            onTap: memory.captureId.isEmpty
                ? null
                : () => context.push('/memories/${memory.captureId}'),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      _typeChip(context, memory.primaryType),
                      if (memory.isPinned) ...[
                        const SizedBox(width: 8),
                        const Icon(Icons.push_pin, size: 14),
                      ],
                      const Spacer(),
                      if (memory.capturedAt != null)
                        Text(
                          DateFormat(
                            'M月d日',
                          ).format(memory.capturedAt!.toLocal()),
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Text(
                    memory.title.isEmpty ? '（无标题）' : memory.title,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  if (memory.summary != null && memory.summary!.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      memory.summary!,
                      maxLines: 6,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  ],
                  const SizedBox(height: 8),
                  Text(
                    '点按查看完整记忆',
                    style: Theme.of(
                      context,
                    ).textTheme.bodySmall?.copyWith(color: Colors.blueGrey),
                  ),
                ],
              ),
            ),
          ),
        ),
        const SizedBox(height: 24),
        Text(
          '这条回响对你有用吗？',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodySmall,
        ),
        const SizedBox(height: 8),
        if (state.feedbackSaving)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(8),
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          )
        else
          Wrap(
            alignment: WrapAlignment.center,
            spacing: 8,
            children: [
              _FeedbackButton(
                key: const Key('echo_feedback_done'),
                label: '有用，回顾了',
                icon: Icons.check,
                onPressed: () => notifier.feedback(echoVerdictDone),
              ),
              _FeedbackButton(
                key: const Key('echo_feedback_later'),
                label: '稍后再说',
                icon: Icons.schedule,
                onPressed: () => notifier.feedback(echoVerdictLater),
              ),
              _FeedbackButton(
                key: const Key('echo_feedback_not_relevant'),
                label: '不太相关',
                icon: Icons.close,
                onPressed: () => notifier.feedback(echoVerdictNotRelevant),
              ),
            ],
          ),
      ],
    );
  }

  Widget _typeChip(BuildContext context, String primaryType) {
    final label = kPrimaryTypeLabels[primaryType] ?? '未分类';
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(label, style: Theme.of(context).textTheme.labelSmall),
    );
  }
}

class _FeedbackButton extends StatelessWidget {
  const _FeedbackButton({
    super.key,
    required this.label,
    required this.icon,
    required this.onPressed,
  });

  final String label;
  final IconData icon;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton.icon(
      onPressed: onPressed,
      icon: Icon(icon, size: 18),
      label: Text(label),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48),
            const SizedBox(height: 16),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
