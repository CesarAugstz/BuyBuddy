import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../config/theme.dart';
import '../models/knowledge.dart';
import '../providers/knowledge_provider.dart';
import '../widgets/knowledge_entry_tile.dart';
import 'knowledge_entry_editor_page.dart';

class KnowledgeEntryDeleteResult {
  const KnowledgeEntryDeleteResult(this.entry);

  final KnowledgeEntry entry;

  int get undoVersion => entry.version + 1;
}

class KnowledgeEntryPage extends ConsumerWidget {
  const KnowledgeEntryPage({super.key, required this.entryId});

  final String entryId;

  Future<void> _edit(
    BuildContext context,
    WidgetRef ref,
    KnowledgeEntry entry,
  ) async {
    await Navigator.push<KnowledgeEntry>(
      context,
      MaterialPageRoute(
        builder:
            (context) => KnowledgeEntryEditorPage(
              initialTopicId: entry.topicId,
              entry: entry,
            ),
      ),
    );
  }

  Future<void> _move(
    BuildContext context,
    WidgetRef ref,
    KnowledgeEntry entry,
  ) async {
    final roots = ref.read(knowledgeTreeProvider).value?.topics ?? const [];
    final nodes = flattenKnowledgeTopics(roots);
    final target = await showDialog<String>(
      context: context,
      builder:
          (context) => SimpleDialog(
            title: const Text('Move to folder'),
            children:
                nodes
                    .where((node) => node.topic.id != entry.topicId)
                    .map(
                      (node) => SimpleDialogOption(
                        onPressed: () => Navigator.pop(context, node.topic.id),
                        child: Row(
                          children: [
                            const Icon(Icons.folder_outlined, size: 20),
                            const SizedBox(width: 10),
                            Expanded(
                              child: Text(
                                selectKnowledgeDirectory(roots, node.topic.id)
                                    .breadcrumb
                                    .map((part) => part.name)
                                    .join(' / '),
                              ),
                            ),
                          ],
                        ),
                      ),
                    )
                    .toList(),
          ),
    );
    if (target == null) return;
    try {
      await ref
          .read(knowledgeEntryProvider(entry.id).notifier)
          .moveEntry(target);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('Entry moved.')));
      }
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Could not move entry: $error'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  Future<void> _undo(
    BuildContext context,
    WidgetRef ref,
    KnowledgeEntry entry,
  ) async {
    try {
      await ref.read(knowledgeEntryProvider(entry.id).notifier).undo();
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Last entry change undone.')),
        );
      }
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Nothing could be undone: $error'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  Future<void> _delete(
    BuildContext context,
    WidgetRef ref,
    KnowledgeEntry entry,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder:
          (context) => AlertDialog(
            title: const Text('Delete entry?'),
            content: Text(
              '"${entry.title}" will be removed from Personal Knowledge.',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context, false),
                child: const Text('Cancel'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(context, true),
                style: TextButton.styleFrom(foregroundColor: Colors.red),
                child: const Text('Delete'),
              ),
            ],
          ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(knowledgeEntryProvider(entry.id).notifier).delete();
      if (context.mounted) {
        Navigator.pop(context, KnowledgeEntryDeleteResult(entry));
      }
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Could not delete entry: $error'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncEntry = ref.watch(knowledgeEntryProvider(entryId));
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: const Text('Knowledge entry'),
        actions: [
          if (asyncEntry.value case final state?)
            PopupMenuButton<String>(
              enabled: !state.isSaving,
              onSelected: (action) {
                switch (action) {
                  case 'edit':
                    _edit(context, ref, state.entry);
                  case 'move':
                    _move(context, ref, state.entry);
                  case 'undo':
                    _undo(context, ref, state.entry);
                  case 'delete':
                    _delete(context, ref, state.entry);
                }
              },
              itemBuilder:
                  (context) => [
                    const PopupMenuItem(
                      value: 'edit',
                      child: ListTile(
                        dense: true,
                        leading: Icon(Icons.edit_outlined),
                        title: Text('Edit'),
                      ),
                    ),
                    const PopupMenuItem(
                      value: 'move',
                      child: ListTile(
                        dense: true,
                        leading: Icon(Icons.drive_file_move_outline),
                        title: Text('Move'),
                      ),
                    ),
                    if (state.entry.version > 1)
                      const PopupMenuItem(
                        value: 'undo',
                        child: ListTile(
                          dense: true,
                          leading: Icon(Icons.undo),
                          title: Text('Undo last change'),
                        ),
                      ),
                    const PopupMenuDivider(),
                    const PopupMenuItem(
                      value: 'delete',
                      child: ListTile(
                        dense: true,
                        leading: Icon(Icons.delete_outline, color: Colors.red),
                        title: Text(
                          'Delete',
                          style: TextStyle(color: Colors.red),
                        ),
                      ),
                    ),
                  ],
            ),
        ],
      ),
      body: asyncEntry.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error:
            (error, _) => _EntryError(
              message: error.toString(),
              onRetry: () => ref.invalidate(knowledgeEntryProvider(entryId)),
            ),
        data: (state) => _EntryDetails(state: state),
      ),
    );
  }
}

class _EntryDetails extends ConsumerWidget {
  const _EntryDetails({required this.state});

  final KnowledgeEntryState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final entry = state.entry;
    final roots = ref.watch(knowledgeTreeProvider).value?.topics ?? const [];
    final breadcrumb =
        selectKnowledgeDirectory(roots, entry.topicId).breadcrumb;
    final dateFormat = DateFormat.yMMMd().add_jm();

    return Stack(
      children: [
        ListView(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
          children: [
            if (state.error != null)
              Container(
                margin: const EdgeInsets.only(bottom: 12),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Colors.amber.shade50,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Row(
                  children: [
                    const Icon(Icons.cloud_off_outlined, size: 20),
                    const SizedBox(width: 8),
                    Expanded(child: Text(state.error!)),
                  ],
                ),
              ),
            Text(
              breadcrumb.isEmpty
                  ? entry.topic?.name ?? 'Personal Knowledge'
                  : breadcrumb.map((part) => part.name).join(' / '),
              style: const TextStyle(color: AppTheme.darkGray, fontSize: 13),
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    color: AppTheme.primaryBlue.withValues(alpha: 0.08),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Icon(
                    knowledgeKindIcon(entry.kind),
                    color: AppTheme.primaryBlue,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        entry.title,
                        style: const TextStyle(
                          fontSize: 24,
                          fontWeight: FontWeight.w700,
                          color: AppTheme.nearBlack,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        entry.kind,
                        style: const TextStyle(
                          color: AppTheme.darkGray,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            if (entry.tags.isNotEmpty) ...[
              const SizedBox(height: 18),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children:
                    entry.tags
                        .map(
                          (tag) => Chip(
                            visualDensity: VisualDensity.compact,
                            label: Text(tag),
                          ),
                        )
                        .toList(),
              ),
            ],
            const SizedBox(height: 24),
            const _SectionTitle('Body'),
            const SizedBox(height: 8),
            SelectableText(
              entry.body,
              style: const TextStyle(
                fontSize: 16,
                height: 1.55,
                color: AppTheme.nearBlack,
              ),
            ),
            if (entry.attributes.isNotEmpty) ...[
              const SizedBox(height: 28),
              const _SectionTitle('Attributes'),
              const SizedBox(height: 8),
              Card(
                elevation: 0,
                color: AppTheme.lightGray,
                child: Column(
                  children:
                      entry.attributes.entries
                          .map(
                            (attribute) => ListTile(
                              dense: true,
                              title: Text(
                                attribute.key,
                                style: const TextStyle(
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                              subtitle: Text(_attributeText(attribute.value)),
                            ),
                          )
                          .toList(),
                ),
              ),
            ],
            const SizedBox(height: 28),
            const _SectionTitle('Dates'),
            const SizedBox(height: 8),
            if (entry.occurredAt != null)
              _DateRow(
                label: 'Occurred',
                value: dateFormat.format(entry.occurredAt!.toLocal()),
              ),
            _DateRow(
              label: 'Created',
              value: dateFormat.format(entry.createdAt.toLocal()),
            ),
            _DateRow(
              label: 'Updated',
              value: dateFormat.format(entry.updatedAt.toLocal()),
            ),
          ],
        ),
        if (state.isRefreshing || state.isSaving)
          const LinearProgressIndicator(minHeight: 2),
      ],
    );
  }

  String _attributeText(dynamic value) {
    if (value is String || value is num || value is bool || value == null) {
      return value?.toString() ?? 'null';
    }
    return const JsonEncoder.withIndent('  ').convert(value);
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);
  final String text;

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: const TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w700,
        color: AppTheme.nearBlack,
      ),
    );
  }
}

class _DateRow extends StatelessWidget {
  const _DateRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        children: [
          SizedBox(
            width: 90,
            child: Text(
              label,
              style: const TextStyle(color: AppTheme.darkGray),
            ),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}

class _EntryError extends StatelessWidget {
  const _EntryError({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.cloud_off_outlined,
              size: 56,
              color: Colors.grey.shade400,
            ),
            const SizedBox(height: 12),
            const Text(
              'Entry unavailable',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: 8),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            ElevatedButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ),
      ),
    );
  }
}
