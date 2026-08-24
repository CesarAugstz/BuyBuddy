import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/theme.dart';
import '../models/knowledge.dart';
import '../providers/knowledge_provider.dart';
import '../widgets/knowledge_entry_tile.dart';
import 'knowledge_entry_editor_page.dart';
import 'knowledge_entry_page.dart';
import 'knowledge_search_page.dart';

class KnowledgeExplorerPage extends ConsumerStatefulWidget {
  const KnowledgeExplorerPage({super.key, this.initialTopicId});

  final String? initialTopicId;

  @override
  ConsumerState<KnowledgeExplorerPage> createState() =>
      _KnowledgeExplorerPageState();
}

class _KnowledgeExplorerPageState extends ConsumerState<KnowledgeExplorerPage> {
  String? _topicId;
  bool _recentTopicRestorationStarted = false;
  int _navigationRevision = 0;

  @override
  void initState() {
    super.initState();
    _topicId = widget.initialTopicId;
  }

  void _openTopic(String topicId) {
    _selectTopic(topicId);
  }

  void _selectTopic(String? topicId) {
    _navigationRevision++;
    setState(() => _topicId = topicId);
    final notifier = ref.read(knowledgeTreeProvider.notifier);
    if (topicId == null) {
      unawaited(notifier.forgetRecentTopic());
    } else {
      unawaited(notifier.markTopicOpened(topicId));
    }
  }

  Future<void> _restoreRecentTopic(int navigationRevision) async {
    final topicId =
        await ref.read(knowledgeTreeProvider.notifier).restoreRecentTopicId();
    if (!mounted || navigationRevision != _navigationRevision) return;
    if (topicId != null && topicId != _topicId) {
      setState(() => _topicId = topicId);
    }
  }

  void _goBack(List<KnowledgeTopicNode> roots) {
    if (_topicId == null) {
      Navigator.maybePop(context);
      return;
    }
    final selected = selectKnowledgeDirectory(roots, _topicId);
    _selectTopic(selected.topic?.topic.parentId);
  }

  Future<void> _refresh() async {
    await ref.read(knowledgeTreeProvider.notifier).refresh();
    final topicId = _topicId;
    if (topicId != null) {
      await ref.read(knowledgeTopicEntriesProvider(topicId).notifier).refresh();
    }
  }

  Future<void> _createTopic({String? parentId}) async {
    final name = await _textDialog(
      title: parentId == null ? 'New folder' : 'New subfolder',
      label: 'Folder name',
    );
    if (name == null) return;
    try {
      await ref
          .read(knowledgeTreeProvider.notifier)
          .createTopic(name: name, parentId: parentId);
    } catch (error) {
      _showError('Could not create folder: $error');
    }
  }

  Future<void> _renameTopic(KnowledgeTopic topic) async {
    final name = await _textDialog(
      title: 'Rename folder',
      label: 'Folder name',
      initialValue: topic.name,
    );
    if (name == null || name == topic.name) return;
    try {
      await ref.read(knowledgeTreeProvider.notifier).renameTopic(topic, name);
    } catch (error) {
      _showError('Could not rename folder: $error');
    }
  }

  Future<void> _moveTopic(
    KnowledgeTopicNode current,
    List<KnowledgeTopicNode> roots,
  ) async {
    const rootValue = '__knowledge_root__';
    final flat = flattenKnowledgeTopics(roots);
    final subtree = flattenKnowledgeTopics([current]);
    final descendantIds = subtree.map((node) => node.topic.id).toSet();
    final subtreeDepth = subtree
        .map((node) => node.topic.depth - current.topic.depth)
        .fold<int>(0, (maximum, value) => value > maximum ? value : maximum);
    final candidates =
        flat.where((candidate) {
          if (descendantIds.contains(candidate.topic.id)) return false;
          return candidate.topic.depth + 1 + subtreeDepth <= 4;
        }).toList();

    final destination = await showDialog<String>(
      context: context,
      builder:
          (context) => SimpleDialog(
            title: const Text('Move folder'),
            children: [
              SimpleDialogOption(
                onPressed: () => Navigator.pop(context, rootValue),
                child: const Row(
                  children: [
                    Icon(Icons.home_outlined),
                    SizedBox(width: 10),
                    Text('Personal Knowledge'),
                  ],
                ),
              ),
              ...candidates.map(
                (candidate) => SimpleDialogOption(
                  onPressed: () => Navigator.pop(context, candidate.topic.id),
                  child: Row(
                    children: [
                      const Icon(Icons.folder_outlined),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          selectKnowledgeDirectory(
                            roots,
                            candidate.topic.id,
                          ).breadcrumb.map((part) => part.name).join(' / '),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
    );
    if (destination == null) return;
    final parentId = destination == rootValue ? null : destination;
    if (parentId == current.topic.parentId) return;
    try {
      await ref
          .read(knowledgeTreeProvider.notifier)
          .moveTopic(current.topic, parentId);
    } catch (error) {
      _showError('Could not move folder: $error');
    }
  }

  Future<void> _deleteTopic(KnowledgeTopicNode topic) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder:
          (context) => AlertDialog(
            title: const Text('Delete empty folder?'),
            content: Text('"${topic.topic.name}" will be removed.'),
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
    final parentId = topic.topic.parentId;
    try {
      await ref.read(knowledgeTreeProvider.notifier).deleteTopic(topic.topic);
      if (mounted) _selectTopic(parentId);
    } catch (error) {
      _showError('Could not delete folder: $error');
    }
  }

  Future<void> _organizeTopic(KnowledgeTopic topic) async {
    final messenger = ScaffoldMessenger.of(context);
    messenger.hideCurrentSnackBar();
    messenger.showSnackBar(
      SnackBar(
        duration: const Duration(minutes: 2),
        content: Row(
          children: [
            const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: Colors.white,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text('Organizing ${topic.name}…')),
          ],
        ),
      ),
    );
    try {
      final response = await ref
          .read(knowledgeTreeProvider.notifier)
          .organizeTopic(topic.id);
      if (!mounted) return;
      messenger.hideCurrentSnackBar();
      final count = response.result.operationsApplied;
      messenger.showSnackBar(
        SnackBar(
          content: Text(
            count == 0
                ? '${topic.name} is already organized.'
                : 'Organized ${topic.name}: $count ${count == 1 ? 'change' : 'changes'} applied.',
          ),
        ),
      );
    } catch (error) {
      if (!mounted) return;
      messenger.hideCurrentSnackBar();
      _showError('Could not organize ${topic.name}: $error');
    }
  }

  Future<String?> _textDialog({
    required String title,
    required String label,
    String initialValue = '',
  }) async {
    final controller = TextEditingController(text: initialValue);
    final result = await showDialog<String>(
      context: context,
      builder:
          (context) => AlertDialog(
            title: Text(title),
            content: TextField(
              controller: controller,
              autofocus: true,
              maxLength: 120,
              textCapitalization: TextCapitalization.words,
              decoration: InputDecoration(labelText: label),
              onSubmitted: (value) {
                if (value.trim().isNotEmpty) {
                  Navigator.pop(context, value.trim());
                }
              },
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context),
                child: const Text('Cancel'),
              ),
              ElevatedButton(
                onPressed: () {
                  if (controller.text.trim().isNotEmpty) {
                    Navigator.pop(context, controller.text.trim());
                  }
                },
                child: const Text('Save'),
              ),
            ],
          ),
    );
    controller.dispose();
    return result;
  }

  Future<void> _addEntry(String topicId) async {
    await Navigator.push<KnowledgeEntry>(
      context,
      MaterialPageRoute(
        builder: (context) => KnowledgeEntryEditorPage(initialTopicId: topicId),
      ),
    );
  }

  Future<void> _openEntry(KnowledgeEntry entry) async {
    final deleted = await Navigator.push<KnowledgeEntryDeleteResult>(
      context,
      MaterialPageRoute(
        builder: (context) => KnowledgeEntryPage(entryId: entry.id),
      ),
    );
    if (deleted == null || !mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    messenger.showSnackBar(
      SnackBar(
        content: const Text('Entry deleted.'),
        action: SnackBarAction(
          label: 'Undo',
          onPressed: () async {
            try {
              await ref
                  .read(knowledgeTreeProvider.notifier)
                  .undoDeletedEntry(deleted.entry);
            } catch (error) {
              if (mounted) {
                messenger.showSnackBar(
                  SnackBar(content: Text('Could not undo deletion: $error')),
                );
              }
            }
          },
        ),
      ),
    );
  }

  void _showError(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: Colors.red.shade700),
    );
  }

  @override
  Widget build(BuildContext context) {
    final asyncTree = ref.watch(knowledgeTreeProvider);
    final roots = asyncTree.value?.topics ?? const <KnowledgeTopicNode>[];
    final selection = selectKnowledgeDirectory(roots, _topicId);
    final current = selection.topic;
    final isOrganizing =
        current != null &&
        (asyncTree.value?.organizingTopicIds.contains(current.topic.id) ??
            false);

    if (widget.initialTopicId == null &&
        asyncTree.hasValue &&
        !_recentTopicRestorationStarted) {
      _recentTopicRestorationStarted = true;
      unawaited(_restoreRecentTopic(_navigationRevision));
    }

    if (_topicId != null && asyncTree.hasValue && current == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted &&
            _topicId != null &&
            findKnowledgeTopic(
                  ref.read(knowledgeTreeProvider).value?.topics ?? const [],
                  _topicId!,
                ) ==
                null) {
          _selectTopic(null);
        }
      });
    }

    return PopScope(
      canPop: _topicId == null,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop && _topicId != null) _goBack(roots);
      },
      child: Scaffold(
        backgroundColor: AppTheme.lightGray,
        appBar: AppBar(
          leading:
              _topicId == null
                  ? null
                  : IconButton(
                    onPressed: () => _goBack(roots),
                    icon: const Icon(Icons.arrow_back),
                  ),
          title: Text(
            current?.topic.name ?? 'Personal Knowledge',
            style: const TextStyle(fontWeight: FontWeight.w600),
          ),
          actions: [
            IconButton(
              tooltip: 'Search knowledge',
              onPressed: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const KnowledgeSearchPage(),
                  ),
                );
              },
              icon: const Icon(Icons.search),
            ),
            IconButton(
              tooltip: 'Refresh',
              onPressed: _refresh,
              icon: const Icon(Icons.refresh),
            ),
            if (current != null)
              PopupMenuButton<String>(
                onSelected: (action) {
                  switch (action) {
                    case 'add-entry':
                      _addEntry(current.topic.id);
                    case 'subtopic':
                      _createTopic(parentId: current.topic.id);
                    case 'organize':
                      _organizeTopic(current.topic);
                    case 'rename':
                      _renameTopic(current.topic);
                    case 'move':
                      _moveTopic(current, roots);
                    case 'delete':
                      _deleteTopic(current);
                  }
                },
                itemBuilder:
                    (context) => [
                      const PopupMenuItem(
                        value: 'add-entry',
                        child: ListTile(
                          dense: true,
                          leading: Icon(Icons.note_add_outlined),
                          title: Text('Add entry'),
                        ),
                      ),
                      if (current.topic.depth < 4)
                        const PopupMenuItem(
                          value: 'subtopic',
                          child: ListTile(
                            dense: true,
                            leading: Icon(Icons.create_new_folder_outlined),
                            title: Text('New subfolder'),
                          ),
                        ),
                      PopupMenuItem(
                        value: 'organize',
                        enabled: !isOrganizing,
                        child: ListTile(
                          dense: true,
                          leading:
                              isOrganizing
                                  ? const SizedBox(
                                    width: 24,
                                    height: 24,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                  : const Icon(Icons.auto_fix_high_outlined),
                          title: Text(
                            isOrganizing
                                ? 'Organizing topic…'
                                : 'Organize this topic',
                          ),
                        ),
                      ),
                      if (!current.topic.isInbox) ...[
                        const PopupMenuItem(
                          value: 'rename',
                          child: ListTile(
                            dense: true,
                            leading: Icon(Icons.edit_outlined),
                            title: Text('Rename folder'),
                          ),
                        ),
                        const PopupMenuItem(
                          value: 'move',
                          child: ListTile(
                            dense: true,
                            leading: Icon(Icons.drive_file_move_outline),
                            title: Text('Move folder'),
                          ),
                        ),
                        if (current.entryCount == 0 &&
                            current.childCount == 0) ...[
                          const PopupMenuDivider(),
                          const PopupMenuItem(
                            value: 'delete',
                            child: ListTile(
                              dense: true,
                              leading: Icon(
                                Icons.delete_outline,
                                color: Colors.red,
                              ),
                              title: Text(
                                'Delete empty folder',
                                style: TextStyle(color: Colors.red),
                              ),
                            ),
                          ),
                        ],
                      ],
                    ],
              )
            else
              IconButton(
                tooltip: 'New folder',
                onPressed: () => _createTopic(),
                icon: const Icon(Icons.create_new_folder_outlined),
              ),
          ],
        ),
        body: asyncTree.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error:
              (error, _) =>
                  _KnowledgeError(message: error.toString(), onRetry: _refresh),
          data:
              (treeState) => _KnowledgeDirectory(
                treeState: treeState,
                selection: selection,
                topicId: _topicId,
                onRefresh: _refresh,
                onOpenTopic: _openTopic,
                onOpenEntry: _openEntry,
                onSelectBreadcrumb: _selectTopic,
                onAddEntry:
                    current == null ? null : () => _addEntry(current.topic.id),
                onCreateSubtopic:
                    current == null || current.topic.depth >= 4
                        ? null
                        : () => _createTopic(parentId: current.topic.id),
              ),
        ),
        floatingActionButton:
            current == null
                ? FloatingActionButton(
                  tooltip: 'New folder',
                  onPressed: () => _createTopic(),
                  child: const Icon(Icons.create_new_folder_outlined),
                )
                : FloatingActionButton(
                  tooltip: 'Add entry',
                  onPressed: () => _addEntry(current.topic.id),
                  child: const Icon(Icons.note_add_outlined),
                ),
      ),
    );
  }
}

class _KnowledgeDirectory extends ConsumerWidget {
  const _KnowledgeDirectory({
    required this.treeState,
    required this.selection,
    required this.topicId,
    required this.onRefresh,
    required this.onOpenTopic,
    required this.onOpenEntry,
    required this.onSelectBreadcrumb,
    this.onAddEntry,
    this.onCreateSubtopic,
  });

  final KnowledgeTreeState treeState;
  final KnowledgeDirectorySelection selection;
  final String? topicId;
  final Future<void> Function() onRefresh;
  final ValueChanged<String> onOpenTopic;
  final ValueChanged<KnowledgeEntry> onOpenEntry;
  final ValueChanged<String?> onSelectBreadcrumb;
  final VoidCallback? onAddEntry;
  final VoidCallback? onCreateSubtopic;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncEntries =
        topicId == null
            ? null
            : ref.watch(knowledgeTopicEntriesProvider(topicId!));
    final inboxes =
        topicId == null
            ? selection.children.where((node) => node.topic.isInbox).toList()
            : const <KnowledgeTopicNode>[];
    final folders =
        topicId == null
            ? selection.children.where((node) => !node.topic.isInbox).toList()
            : selection.children;

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 100),
        children: [
          if (treeState.isRefreshing)
            const LinearProgressIndicator(minHeight: 2),
          if (treeState.error != null)
            _OfflineBanner(message: treeState.error!),
          if (topicId == null)
            _SearchCard(
              onTap: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const KnowledgeSearchPage(),
                  ),
                );
              },
            )
          else
            _Breadcrumbs(
              parts: selection.breadcrumb,
              onSelect: onSelectBreadcrumb,
            ),
          if (inboxes.isNotEmpty) ...[
            const SizedBox(height: 14),
            ...inboxes.map(
              (inbox) => _TopicRow(
                key: Key('knowledge-topic-${inbox.topic.id}'),
                topic: inbox,
                isInbox: true,
                onTap: () => onOpenTopic(inbox.topic.id),
              ),
            ),
          ],
          if (folders.isNotEmpty) ...[
            const _DirectoryHeading('Folders'),
            ...folders.map(
              (folder) => _TopicRow(
                key: Key('knowledge-topic-${folder.topic.id}'),
                topic: folder,
                onTap: () => onOpenTopic(folder.topic.id),
              ),
            ),
          ],
          if (asyncEntries != null)
            asyncEntries.when(
              loading:
                  () => const Padding(
                    padding: EdgeInsets.all(32),
                    child: Center(child: CircularProgressIndicator()),
                  ),
              error:
                  (error, _) => _InlineError(
                    message: error.toString(),
                    onRetry:
                        () =>
                            ref
                                .read(
                                  knowledgeTopicEntriesProvider(
                                    topicId!,
                                  ).notifier,
                                )
                                .refresh(),
                  ),
              data: (directory) {
                if (directory.entries.isEmpty && folders.isEmpty) {
                  return _EmptyDirectory(
                    onAddEntry: onAddEntry,
                    onCreateSubtopic: onCreateSubtopic,
                  );
                }
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (directory.error != null)
                      _OfflineBanner(message: directory.error!),
                    if (directory.entries.isNotEmpty) ...[
                      const _DirectoryHeading('Entries'),
                      ...directory.entries.map(
                        (entry) => KnowledgeEntryTile(
                          key: Key('knowledge-entry-${entry.id}'),
                          entry: entry,
                          onTap: () => onOpenEntry(entry),
                        ),
                      ),
                    ],
                    if (directory.hasMore || directory.isLoadingMore)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: OutlinedButton(
                          key: const Key('knowledge-load-more'),
                          onPressed:
                              directory.isLoadingMore
                                  ? null
                                  : () =>
                                      ref
                                          .read(
                                            knowledgeTopicEntriesProvider(
                                              topicId!,
                                            ).notifier,
                                          )
                                          .loadMore(),
                          child:
                              directory.isLoadingMore
                                  ? const SizedBox(
                                    width: 20,
                                    height: 20,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                  : const Text('Load more'),
                        ),
                      ),
                  ],
                );
              },
            ),
          if (topicId == null &&
              folders.isEmpty &&
              inboxes.every((inbox) => inbox.entryCount == 0))
            const _RootEmptyState(),
        ],
      ),
    );
  }
}

class _SearchCard extends StatelessWidget {
  const _SearchCard({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      elevation: 0,
      color: Colors.white,
      child: ListTile(
        onTap: onTap,
        leading: const Icon(Icons.search),
        title: const Text('Search knowledge…'),
        trailing: const Icon(Icons.tune, size: 20),
      ),
    );
  }
}

class _Breadcrumbs extends StatelessWidget {
  const _Breadcrumbs({required this.parts, required this.onSelect});
  final List<KnowledgeTopic> parts;
  final ValueChanged<String?> onSelect;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          TextButton(
            onPressed: () => onSelect(null),
            child: const Text('Knowledge'),
          ),
          for (final part in parts) ...[
            const Icon(Icons.chevron_right, size: 17),
            TextButton(
              key: Key('knowledge-breadcrumb-${part.id}'),
              onPressed: () => onSelect(part.id),
              child: Text(part.name),
            ),
          ],
        ],
      ),
    );
  }
}

class _TopicRow extends StatelessWidget {
  const _TopicRow({
    super.key,
    required this.topic,
    required this.onTap,
    this.isInbox = false,
  });

  final KnowledgeTopicNode topic;
  final VoidCallback onTap;
  final bool isInbox;

  @override
  Widget build(BuildContext context) {
    final counts = <String>[
      '${topic.entryCount} ${topic.entryCount == 1 ? 'entry' : 'entries'}',
      if (topic.childCount > 0)
        '${topic.childCount} ${topic.childCount == 1 ? 'folder' : 'folders'}',
    ].join(' · ');
    return Card(
      elevation: 0,
      color: Colors.white,
      margin: const EdgeInsets.only(bottom: 10),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppTheme.mediumGray),
      ),
      child: ListTile(
        onTap: onTap,
        contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 5),
        leading: Badge(
          isLabelVisible: isInbox && topic.entryCount > 0,
          label: Text('${topic.entryCount}'),
          backgroundColor: AppTheme.primaryBlue,
          child: Icon(
            isInbox ? Icons.inbox_outlined : Icons.folder_outlined,
            color: AppTheme.primaryBlue,
            size: 30,
          ),
        ),
        title: Text(
          topic.topic.name,
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
        subtitle: Text(counts),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (topic.topic.pendingWriteCount > 0 ||
                topic.topic.organizationDueAt != null)
              Tooltip(
                message:
                    topic.topic.organizationLeaseUntil != null &&
                            topic.topic.organizationLeaseUntil!.isAfter(
                              DateTime.now(),
                            )
                        ? 'Organization in progress'
                        : 'Organization pending',
                child: Icon(
                  topic.topic.organizationLeaseUntil != null &&
                          topic.topic.organizationLeaseUntil!.isAfter(
                            DateTime.now(),
                          )
                      ? Icons.sync
                      : Icons.auto_fix_high,
                  key: Key(
                    'knowledge-organization-indicator-${topic.topic.id}',
                  ),
                  size: 17,
                  color: Colors.amber.shade800,
                ),
              ),
            const SizedBox(width: 4),
            const Icon(Icons.chevron_right),
          ],
        ),
      ),
    );
  }
}

class _DirectoryHeading extends StatelessWidget {
  const _DirectoryHeading(this.text);
  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 18, 4, 9),
      child: Text(
        text,
        style: const TextStyle(
          fontSize: 15,
          fontWeight: FontWeight.w700,
          color: AppTheme.nearBlack,
        ),
      ),
    );
  }
}

class _OfflineBanner extends StatelessWidget {
  const _OfflineBanner({required this.message});
  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: Colors.amber.shade50,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        children: [
          const Icon(Icons.cloud_off_outlined, size: 20),
          const SizedBox(width: 8),
          Expanded(child: Text(message)),
        ],
      ),
    );
  }
}

class _EmptyDirectory extends StatelessWidget {
  const _EmptyDirectory({this.onAddEntry, this.onCreateSubtopic});
  final VoidCallback? onAddEntry;
  final VoidCallback? onCreateSubtopic;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 16),
      child: Column(
        children: [
          Icon(
            Icons.folder_open_outlined,
            size: 64,
            color: Colors.grey.shade400,
          ),
          const SizedBox(height: 12),
          const Text(
            'This folder is empty.',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 6),
          const Text(
            'Add an entry or create a subfolder.',
            style: TextStyle(color: AppTheme.darkGray),
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 8,
            children: [
              if (onAddEntry != null)
                ElevatedButton.icon(
                  onPressed: onAddEntry,
                  icon: const Icon(Icons.note_add_outlined),
                  label: const Text('Add entry'),
                ),
              if (onCreateSubtopic != null)
                OutlinedButton.icon(
                  onPressed: onCreateSubtopic,
                  icon: const Icon(Icons.create_new_folder_outlined),
                  label: const Text('New subfolder'),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _RootEmptyState extends StatelessWidget {
  const _RootEmptyState();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 36, horizontal: 16),
      child: Column(
        children: [
          Icon(Icons.auto_stories_outlined, size: 56, color: AppTheme.darkGray),
          SizedBox(height: 12),
          Text(
            'Your knowledge base is empty.',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
          ),
          SizedBox(height: 6),
          Text(
            'Ask the assistant to remember something or add a note.',
            textAlign: TextAlign.center,
            style: TextStyle(color: AppTheme.darkGray),
          ),
        ],
      ),
    );
  }
}

class _InlineError extends StatelessWidget {
  const _InlineError({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        children: [
          const Icon(Icons.cloud_off_outlined, size: 40),
          const SizedBox(height: 8),
          Text(message, textAlign: TextAlign.center),
          TextButton(onPressed: onRetry, child: const Text('Retry')),
        ],
      ),
    );
  }
}

class _KnowledgeError extends StatelessWidget {
  const _KnowledgeError({required this.message, required this.onRetry});
  final String message;
  final Future<void> Function() onRetry;

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
              size: 64,
              color: Colors.grey.shade400,
            ),
            const SizedBox(height: 12),
            const Text(
              'Personal Knowledge is unavailable',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
              textAlign: TextAlign.center,
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
