import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../config/theme.dart';
import '../models/knowledge.dart';
import '../providers/knowledge_provider.dart';
import '../widgets/knowledge_entry_tile.dart';
import 'knowledge_entry_page.dart';

class KnowledgeSearchPage extends ConsumerStatefulWidget {
  const KnowledgeSearchPage({super.key});

  @override
  ConsumerState<KnowledgeSearchPage> createState() =>
      _KnowledgeSearchPageState();
}

class _KnowledgeSearchPageState extends ConsumerState<KnowledgeSearchPage> {
  static const _allTopics = '__all_topics__';

  final _queryController = TextEditingController();
  final _kindController = TextEditingController();
  final _tagController = TextEditingController();
  String? _topicId;
  DateTime? _from;
  DateTime? _to;
  bool _showFilters = false;
  bool _hasSearched = false;

  @override
  void dispose() {
    _queryController.dispose();
    _kindController.dispose();
    _tagController.dispose();
    super.dispose();
  }

  Future<void> _search() async {
    FocusScope.of(context).unfocus();
    setState(() => _hasSearched = true);
    await ref
        .read(knowledgeSearchProvider.notifier)
        .run(
          KnowledgeSearchFilter(
            query: _queryController.text,
            topicId: _topicId,
            includeChildren: _topicId != null,
            kind: _kindController.text,
            tag: _tagController.text,
            occurredFrom: _from,
            occurredTo: _to,
            limit: 50,
          ),
        );
  }

  Future<void> _pickDate({required bool from}) async {
    final selected = await showDatePicker(
      context: context,
      initialDate: (from ? _from : _to) ?? DateTime.now(),
      firstDate: DateTime(1900),
      lastDate: DateTime(2200),
    );
    if (selected != null) {
      setState(() {
        if (from) {
          _from = selected;
        } else {
          _to = selected;
        }
      });
    }
  }

  Future<void> _openResult(KnowledgeSearchResult result) async {
    final deleted = await Navigator.push<KnowledgeEntryDeleteResult>(
      context,
      MaterialPageRoute(
        builder: (context) => KnowledgeEntryPage(entryId: result.entry.id),
      ),
    );
    if (!mounted) return;
    if (deleted == null) {
      await _search();
      return;
    }
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
              await _search();
            } catch (error) {
              if (mounted) {
                messenger.showSnackBar(
                  SnackBar(content: Text('Could not undo: $error')),
                );
              }
            }
          },
        ),
      ),
    );
    await _search();
  }

  void _clearFilters() {
    setState(() {
      _topicId = null;
      _kindController.clear();
      _tagController.clear();
      _from = null;
      _to = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final results = ref.watch(knowledgeSearchProvider);
    final roots = ref.watch(knowledgeTreeProvider).value?.topics ?? const [];
    final topics = flattenKnowledgeTopics(roots);
    final activeFilterCount =
        [
          _topicId,
          _kindController.text.trim().isEmpty ? null : _kindController.text,
          _tagController.text.trim().isEmpty ? null : _tagController.text,
          _from,
          _to,
        ].where((value) => value != null).length;

    return Scaffold(
      backgroundColor: AppTheme.lightGray,
      appBar: AppBar(title: const Text('Search knowledge')),
      body: Column(
        children: [
          Container(
            color: Colors.white,
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 14),
            child: Column(
              children: [
                TextField(
                  key: const Key('knowledge-search-field'),
                  controller: _queryController,
                  textInputAction: TextInputAction.search,
                  onSubmitted: (_) => _search(),
                  decoration: InputDecoration(
                    hintText: 'Search titles and entry bodies…',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: IconButton(
                      tooltip: 'Search',
                      onPressed: _search,
                      icon: const Icon(Icons.arrow_forward),
                    ),
                    filled: true,
                    fillColor: AppTheme.lightGray,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    OutlinedButton.icon(
                      onPressed:
                          () => setState(() => _showFilters = !_showFilters),
                      icon: Badge(
                        isLabelVisible: activeFilterCount > 0,
                        label: Text('$activeFilterCount'),
                        child: const Icon(Icons.tune, size: 20),
                      ),
                      label: const Text('Filters'),
                    ),
                    if (activeFilterCount > 0) ...[
                      const SizedBox(width: 8),
                      TextButton(
                        onPressed: _clearFilters,
                        child: const Text('Clear'),
                      ),
                    ],
                  ],
                ),
                if (_showFilters) ...[
                  const SizedBox(height: 8),
                  DropdownButtonFormField<String>(
                    initialValue: _topicId ?? _allTopics,
                    decoration: const InputDecoration(
                      labelText: 'Folder',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                    items: [
                      const DropdownMenuItem<String>(
                        value: _allTopics,
                        child: Text('All folders'),
                      ),
                      ...topics.map(
                        (node) => DropdownMenuItem<String>(
                          value: node.topic.id,
                          child: Text(
                            '${'  ' * node.topic.depth}${node.topic.name}',
                          ),
                        ),
                      ),
                    ],
                    onChanged:
                        (value) => setState(
                          () => _topicId = value == _allTopics ? null : value,
                        ),
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(
                        child: TextField(
                          controller: _kindController,
                          decoration: const InputDecoration(
                            labelText: 'Kind',
                            hintText: 'diary',
                            border: OutlineInputBorder(),
                            isDense: true,
                          ),
                          onChanged: (_) => setState(() {}),
                        ),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: TextField(
                          controller: _tagController,
                          decoration: const InputDecoration(
                            labelText: 'Tag',
                            hintText: 'favorite',
                            border: OutlineInputBorder(),
                            isDense: true,
                          ),
                          onChanged: (_) => setState(() {}),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(
                        child: _DateFilterButton(
                          label:
                              _from == null
                                  ? 'From date'
                                  : DateFormat.yMd().format(_from!),
                          onPressed: () => _pickDate(from: true),
                          onClear:
                              _from == null
                                  ? null
                                  : () => setState(() => _from = null),
                        ),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: _DateFilterButton(
                          label:
                              _to == null
                                  ? 'To date'
                                  : DateFormat.yMd().format(_to!),
                          onPressed: () => _pickDate(from: false),
                          onClear:
                              _to == null
                                  ? null
                                  : () => setState(() => _to = null),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton.icon(
                      onPressed: _search,
                      icon: const Icon(Icons.search),
                      label: const Text('Apply and search'),
                    ),
                  ),
                ],
              ],
            ),
          ),
          Expanded(
            child: results.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error:
                  (error, _) => _SearchMessage(
                    icon: Icons.cloud_off_outlined,
                    title: 'Search unavailable',
                    message: error.toString(),
                    action: TextButton(
                      onPressed: _search,
                      child: const Text('Retry'),
                    ),
                  ),
              data: (items) {
                if (!_hasSearched) {
                  return const _SearchMessage(
                    icon: Icons.manage_search,
                    title: 'Search all Personal Knowledge',
                    message:
                        'Use text or filters to find entries in every folder.',
                  );
                }
                if (items.isEmpty) {
                  return const _SearchMessage(
                    icon: Icons.search_off,
                    title: 'No entries found',
                    message: 'Try fewer filters or a different search phrase.',
                  );
                }
                final isCapped = items.length >= 50;
                return ListView.builder(
                  key: const Key('knowledge-search-results'),
                  padding: const EdgeInsets.all(16),
                  itemCount: items.length + (isCapped ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == items.length) {
                      return const Card(
                        key: Key('knowledge-search-limit-message'),
                        elevation: 0,
                        child: Padding(
                          padding: EdgeInsets.all(16),
                          child: Text(
                            'Showing the first 50 results. Refine your search or filters to find more.',
                            textAlign: TextAlign.center,
                            style: TextStyle(color: AppTheme.darkGray),
                          ),
                        ),
                      );
                    }
                    final item = items[index];
                    return KnowledgeEntryTile(
                      entry: item.entry,
                      breadcrumb: item.breadcrumb,
                      onTap: () => _openResult(item),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _DateFilterButton extends StatelessWidget {
  const _DateFilterButton({
    required this.label,
    required this.onPressed,
    this.onClear,
  });

  final String label;
  final VoidCallback onPressed;
  final VoidCallback? onClear;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton.icon(
      onPressed: onPressed,
      icon:
          onClear == null
              ? const Icon(Icons.event_outlined, size: 18)
              : GestureDetector(
                onTap: onClear,
                child: const Icon(Icons.close, size: 18),
              ),
      label: Text(label, overflow: TextOverflow.ellipsis),
    );
  }
}

class _SearchMessage extends StatelessWidget {
  const _SearchMessage({
    required this.icon,
    required this.title,
    required this.message,
    this.action,
  });

  final IconData icon;
  final String title;
  final String message;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 64, color: Colors.grey.shade400),
            const SizedBox(height: 12),
            Text(
              title,
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: AppTheme.darkGray),
            ),
            if (action != null) ...[const SizedBox(height: 8), action!],
          ],
        ),
      ),
    );
  }
}
