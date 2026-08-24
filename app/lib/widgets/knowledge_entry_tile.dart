import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../config/theme.dart';
import '../models/knowledge.dart';

IconData knowledgeKindIcon(String kind) {
  switch (kind.toLowerCase()) {
    case 'diary':
    case 'journal':
      return Icons.menu_book_outlined;
    case 'recommendation':
      return Icons.thumb_up_alt_outlined;
    case 'preference':
      return Icons.favorite_outline;
    case 'rating':
      return Icons.star_outline;
    case 'decision':
      return Icons.gavel_outlined;
    case 'reminder':
      return Icons.notifications_none;
    case 'travel':
      return Icons.flight_outlined;
    case 'project':
      return Icons.work_outline;
    default:
      return Icons.description_outlined;
  }
}

class KnowledgeEntryTile extends StatelessWidget {
  const KnowledgeEntryTile({
    super.key,
    required this.entry,
    required this.onTap,
    this.breadcrumb = const [],
  });

  final KnowledgeEntry entry;
  final VoidCallback onTap;
  final List<KnowledgeTopic> breadcrumb;

  @override
  Widget build(BuildContext context) {
    final excerpt = entry.body.replaceAll(RegExp(r'\s+'), ' ').trim();
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      elevation: 0,
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppTheme.mediumGray),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(
                  color: AppTheme.primaryBlue.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Icon(
                  knowledgeKindIcon(entry.kind),
                  color: AppTheme.primaryBlue,
                  size: 22,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (breadcrumb.isNotEmpty) ...[
                      Text(
                        breadcrumb.map((part) => part.name).join(' / '),
                        style: TextStyle(
                          color: Colors.grey.shade600,
                          fontSize: 11,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 3),
                    ],
                    Text(
                      entry.title,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: AppTheme.nearBlack,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (excerpt.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(
                        excerpt,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: AppTheme.darkGray,
                          height: 1.3,
                        ),
                      ),
                    ],
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        ...entry.tags
                            .take(3)
                            .map(
                              (tag) => Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 7,
                                  vertical: 3,
                                ),
                                decoration: BoxDecoration(
                                  color: AppTheme.lightGray,
                                  borderRadius: BorderRadius.circular(10),
                                ),
                                child: Text(
                                  tag,
                                  style: const TextStyle(
                                    fontSize: 11,
                                    color: AppTheme.darkGray,
                                  ),
                                ),
                              ),
                            ),
                        Text(
                          DateFormat.yMMMd().format(
                            entry.displayDate.toLocal(),
                          ),
                          style: TextStyle(
                            fontSize: 11,
                            color: Colors.grey.shade600,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Padding(
                padding: EdgeInsets.only(top: 12),
                child: Icon(
                  Icons.chevron_right,
                  size: 20,
                  color: AppTheme.darkGray,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
