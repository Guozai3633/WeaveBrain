import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../shared/models/idea.dart';

class IdeaCard extends StatelessWidget {
  final Idea idea;
  final ValueChanged<String>? onTagTapped;

  const IdeaCard({super.key, required this.idea, this.onTagTapped});

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: InkWell(
        onTap: () => context.push('/ideas/${idea.id}', extra: idea),
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Raw input (truncated)
              Text(
                idea.rawInput,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: 8),

              // Tags
              if (idea.tags.isNotEmpty)
                Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  children: idea.tags
                      .map(
                        (tag) => ActionChip(
                          label: Text(
                            tag,
                            style: const TextStyle(fontSize: 12),
                          ),
                          padding: EdgeInsets.zero,
                          materialTapTargetSize:
                              MaterialTapTargetSize.shrinkWrap,
                          visualDensity: VisualDensity.compact,
                          onPressed: onTagTapped != null
                              ? () => onTagTapped!(tag)
                              : null,
                        ),
                      )
                      .toList(),
                ),

              const SizedBox(height: 8),

              // Timestamp + chevron
              Row(
                children: [
                  Text(
                    _formatTime(idea.createdAt),
                    style: Theme.of(
                      context,
                    ).textTheme.bodySmall?.copyWith(color: Colors.grey),
                  ),
                  const Spacer(),
                  const Icon(Icons.chevron_right, size: 20, color: Colors.grey),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _formatTime(DateTime? dt) {
    if (dt == null) return '';
    return DateFormat('yyyy-MM-dd HH:mm').format(dt);
  }
}
