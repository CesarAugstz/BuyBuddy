import 'package:buybuddy/models/knowledge.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('knowledge topic tree JSON round trip preserves every cached field', () {
    final created = DateTime.utc(2026, 8, 20, 12);
    final updated = DateTime.utc(2026, 8, 24, 15);
    final due = DateTime.utc(2026, 8, 26);
    final lease = DateTime.utc(2026, 8, 24, 15, 10);
    final tree = KnowledgeTopicNode(
      topic: KnowledgeTopic(
        id: 'root',
        name: 'Projects',
        description: 'Active work',
        depth: 0,
        pendingWriteCount: 2,
        organizationDueAt: due,
        organizationLeaseUntil: lease,
        lastOrganizedAt: updated,
        createdAt: created,
        updatedAt: updated,
      ),
      entryCount: 3,
      childCount: 1,
      children: [
        KnowledgeTopicNode(
          topic: KnowledgeTopic(
            id: 'child',
            parentId: 'root',
            name: 'BuyBuddy',
            depth: 1,
            pendingWriteCount: 0,
            createdAt: created,
            updatedAt: updated,
          ),
          entryCount: 4,
          childCount: 0,
        ),
      ],
    );

    final restored = KnowledgeTopicNode.fromJson(tree.toJson());

    expect(restored.topic.description, 'Active work');
    expect(restored.topic.pendingWriteCount, 2);
    expect(restored.topic.organizationDueAt, due);
    expect(restored.topic.organizationLeaseUntil, lease);
    expect(restored.topic.lastOrganizedAt, updated);
    expect(restored.entryCount, 3);
    expect(restored.children.single.topic.parentId, 'root');
    expect(restored.children.single.entryCount, 4);
  });

  test('organization response JSON preserves refresh identifiers', () {
    final now = DateTime.utc(2026, 8, 24);
    final response = KnowledgeOrganizationResponse(
      status: 'organized',
      topic: KnowledgeTopic(
        id: 'topic-1',
        name: 'Notes',
        depth: 0,
        pendingWriteCount: 0,
        lastOrganizedAt: now,
        createdAt: now,
        updatedAt: now,
      ),
      result: const KnowledgeOrganizationResult(
        operationsApplied: 2,
        changedEntryIds: ['entry-1'],
        createdTopicIds: ['topic-2'],
        affectedTopicIds: ['topic-1', 'topic-2'],
      ),
    );

    final restored = KnowledgeOrganizationResponse.fromJson(response.toJson());

    expect(restored.status, 'organized');
    expect(restored.topic.pendingWriteCount, 0);
    expect(restored.result.operationsApplied, 2);
    expect(restored.result.changedEntryIds, ['entry-1']);
    expect(restored.result.affectedTopicIds, ['topic-1', 'topic-2']);
  });

  test(
    'knowledge entry JSON round trip preserves arbitrary data and dates',
    () {
      final occurred = DateTime.utc(2026, 8, 24, 9, 30);
      final entry = KnowledgeEntry(
        id: 'entry-1',
        topicId: 'topic-1',
        kind: 'recommendation',
        title: 'Preferred milk',
        body: 'Brand X tastes best.',
        attributes: {
          'rating': 5,
          'product': 'Milk',
          'nested': {
            'available': true,
            'stores': ['A', 'B'],
          },
        },
        tags: const ['milk', 'favorite'],
        occurredAt: occurred,
        source: 'assistant',
        status: 'active',
        version: 3,
        createdAt: DateTime.utc(2026, 8, 20),
        updatedAt: DateTime.utc(2026, 8, 24),
        topic: KnowledgeTopic(
          id: 'topic-1',
          name: 'Recommendations',
          depth: 1,
          pendingWriteCount: 0,
          createdAt: DateTime.utc(2026, 8, 1),
          updatedAt: DateTime.utc(2026, 8, 2),
        ),
      );

      final restored = KnowledgeEntry.fromJson(entry.toJson());

      expect(restored.toJson(), entry.toJson());
      expect(restored.occurredAt, occurred);
      expect(restored.attributes['nested'], {
        'available': true,
        'stores': ['A', 'B'],
      });
      expect(restored.topic?.name, 'Recommendations');
    },
  );

  test('directory selection returns children and ordered breadcrumbs', () {
    final roots = rebuildKnowledgeTopicTree([
      _node('daily', 'Daily', parentId: 'journal'),
      _node('inbox', 'Inbox'),
      _node('journal', 'Journal'),
      _node('travel', 'Travel', parentId: 'journal'),
    ]);

    final selection = selectKnowledgeDirectory(roots, 'daily');

    expect(roots.first.topic.id, 'inbox');
    expect(selection.breadcrumb.map((topic) => topic.name), [
      'Journal',
      'Daily',
    ]);
    expect(selection.topic?.topic.id, 'daily');
    expect(selection.children, isEmpty);
    expect(
      selectKnowledgeDirectory(
        roots,
        'journal',
      ).children.map((child) => child.topic.name),
      ['Daily', 'Travel'],
    );
  });
}

KnowledgeTopicNode _node(String id, String name, {String? parentId}) {
  final now = DateTime.utc(2026, 8, 24);
  return KnowledgeTopicNode(
    topic: KnowledgeTopic(
      id: id,
      parentId: parentId,
      name: name,
      depth: 0,
      pendingWriteCount: 0,
      createdAt: now,
      updatedAt: now,
    ),
    entryCount: 0,
    childCount: 0,
  );
}
