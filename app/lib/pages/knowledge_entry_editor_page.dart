import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../config/theme.dart';
import '../models/knowledge.dart';
import '../providers/knowledge_provider.dart';

class KnowledgeEntryEditorPage extends ConsumerStatefulWidget {
  const KnowledgeEntryEditorPage({
    super.key,
    required this.initialTopicId,
    this.entry,
  });

  final String initialTopicId;
  final KnowledgeEntry? entry;

  @override
  ConsumerState<KnowledgeEntryEditorPage> createState() =>
      _KnowledgeEntryEditorPageState();
}

class _KnowledgeEntryEditorPageState
    extends ConsumerState<KnowledgeEntryEditorPage> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _titleController;
  late final TextEditingController _bodyController;
  late final TextEditingController _kindController;
  late final TextEditingController _tagsController;
  late final TextEditingController _attributesController;
  late String _topicId;
  DateTime? _occurredAt;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    final entry = widget.entry;
    _topicId = entry?.topicId ?? widget.initialTopicId;
    _titleController = TextEditingController(text: entry?.title ?? '');
    _bodyController = TextEditingController(text: entry?.body ?? '');
    _kindController = TextEditingController(text: entry?.kind ?? 'note');
    _tagsController = TextEditingController(text: entry?.tags.join(', ') ?? '');
    _attributesController = TextEditingController(
      text: const JsonEncoder.withIndent(
        '  ',
      ).convert(entry?.attributes ?? <String, dynamic>{}),
    );
    _occurredAt = entry?.occurredAt;
  }

  @override
  void dispose() {
    _titleController.dispose();
    _bodyController.dispose();
    _kindController.dispose();
    _tagsController.dispose();
    _attributesController.dispose();
    super.dispose();
  }

  List<String> _parseTags() {
    final result = <String>[];
    final seen = <String>{};
    for (final value in _tagsController.text.split(',')) {
      final tag = value.trim();
      if (tag.isEmpty || !seen.add(tag.toLowerCase())) continue;
      result.add(tag);
    }
    return result;
  }

  Map<String, dynamic>? _parseAttributes() {
    try {
      final decoded = jsonDecode(_attributesController.text.trim());
      if (decoded is! Map) return null;
      return Map<String, dynamic>.from(decoded);
    } catch (_) {
      return null;
    }
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    final attributes = _parseAttributes();
    if (attributes == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Attributes must be a valid JSON object.'),
        ),
      );
      return;
    }
    final tags = _parseTags();
    if (tags.length > 20) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Use no more than 20 tags.')),
      );
      return;
    }

    setState(() => _saving = true);
    try {
      final KnowledgeEntry saved;
      if (widget.entry == null) {
        saved = await ref
            .read(knowledgeTreeProvider.notifier)
            .createEntry(
              topicId: _topicId,
              kind: _kindController.text.trim(),
              title: _titleController.text.trim(),
              body: _bodyController.text,
              attributes: attributes,
              tags: tags,
              occurredAt: _occurredAt,
            );
      } else {
        saved = await ref
            .read(knowledgeEntryProvider(widget.entry!.id).notifier)
            .saveChanges(
              topicId: _topicId,
              kind: _kindController.text.trim(),
              title: _titleController.text.trim(),
              body: _bodyController.text,
              attributes: attributes,
              tags: tags,
              occurredAt: _occurredAt,
            );
      }
      if (mounted) Navigator.pop(context, saved);
    } catch (error) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Could not save entry: $error'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  Future<void> _pickDate() async {
    final selected = await showDatePicker(
      context: context,
      firstDate: DateTime(1900),
      lastDate: DateTime(2200),
      initialDate: _occurredAt?.toLocal() ?? DateTime.now(),
    );
    if (selected != null) setState(() => _occurredAt = selected);
  }

  @override
  Widget build(BuildContext context) {
    final tree = ref.watch(knowledgeTreeProvider).value?.topics ?? const [];
    final topics = flattenKnowledgeTopics(tree);
    final validTopic =
        topics.any((node) => node.topic.id == _topicId)
            ? _topicId
            : (topics.isEmpty ? null : topics.first.topic.id);
    if (validTopic != null && validTopic != _topicId) {
      _topicId = validTopic;
    }

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text(widget.entry == null ? 'New entry' : 'Edit entry'),
        actions: [
          TextButton(
            onPressed: _saving || validTopic == null ? null : _save,
            child:
                _saving
                    ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                    : const Text('Save'),
          ),
        ],
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            if (topics.isEmpty)
              const Padding(
                padding: EdgeInsets.only(bottom: 16),
                child: Text('Knowledge folders are still loading.'),
              )
            else
              DropdownButtonFormField<String>(
                initialValue: validTopic,
                decoration: const InputDecoration(
                  labelText: 'Folder',
                  border: OutlineInputBorder(),
                  prefixIcon: Icon(Icons.folder_outlined),
                ),
                items:
                    topics
                        .map(
                          (node) => DropdownMenuItem(
                            value: node.topic.id,
                            child: Text(
                              '${'  ' * node.topic.depth}${node.topic.name}',
                            ),
                          ),
                        )
                        .toList(),
                onChanged:
                    _saving
                        ? null
                        : (value) {
                          if (value != null) {
                            setState(() => _topicId = value);
                          }
                        },
              ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _titleController,
              maxLength: 200,
              textCapitalization: TextCapitalization.sentences,
              decoration: const InputDecoration(
                labelText: 'Title',
                border: OutlineInputBorder(),
              ),
              validator:
                  (value) =>
                      value == null || value.trim().isEmpty
                          ? 'Title is required.'
                          : null,
            ),
            const SizedBox(height: 8),
            TextFormField(
              controller: _kindController,
              maxLength: 64,
              decoration: const InputDecoration(
                labelText: 'Kind',
                hintText: 'note, diary, recommendation…',
                border: OutlineInputBorder(),
              ),
              validator:
                  (value) =>
                      value == null || value.trim().isEmpty
                          ? 'Kind is required.'
                          : null,
            ),
            const SizedBox(height: 8),
            TextFormField(
              controller: _bodyController,
              minLines: 7,
              maxLines: null,
              maxLength: 50000,
              textCapitalization: TextCapitalization.sentences,
              decoration: const InputDecoration(
                labelText: 'Body',
                alignLabelWithHint: true,
                border: OutlineInputBorder(),
              ),
              validator:
                  (value) =>
                      value == null || value.trim().isEmpty
                          ? 'Body is required.'
                          : null,
            ),
            const SizedBox(height: 8),
            TextFormField(
              controller: _tagsController,
              decoration: const InputDecoration(
                labelText: 'Tags',
                hintText: 'favorite, milk, shopping',
                helperText: 'Separate tags with commas.',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            Card(
              elevation: 0,
              color: AppTheme.lightGray,
              child: ListTile(
                leading: const Icon(Icons.event_outlined),
                title: Text(
                  _occurredAt == null
                      ? 'No occurrence date'
                      : DateFormat.yMMMd().format(_occurredAt!.toLocal()),
                ),
                subtitle: const Text(
                  'Optional for diary or time-based entries',
                ),
                onTap: _saving ? null : _pickDate,
                trailing:
                    _occurredAt == null
                        ? const Icon(Icons.chevron_right)
                        : IconButton(
                          tooltip: 'Clear date',
                          onPressed:
                              _saving
                                  ? null
                                  : () => setState(() => _occurredAt = null),
                          icon: const Icon(Icons.close),
                        ),
              ),
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _attributesController,
              minLines: 4,
              maxLines: null,
              decoration: const InputDecoration(
                labelText: 'Attributes (JSON)',
                alignLabelWithHint: true,
                helperText: 'The saved object will exactly match this JSON.',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: _saving || validTopic == null ? null : _save,
              icon: const Icon(Icons.save_outlined),
              label: Text(_saving ? 'Saving…' : 'Save entry'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 16),
              ),
            ),
            if (widget.entry != null)
              const Padding(
                padding: EdgeInsets.only(top: 12),
                child: Text(
                  'Body changes are only applied after the server confirms the save.',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 12, color: AppTheme.darkGray),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
