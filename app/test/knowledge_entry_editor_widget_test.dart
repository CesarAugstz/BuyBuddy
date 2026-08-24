import 'package:buybuddy/models/knowledge.dart';
import 'package:buybuddy/pages/knowledge_entry_editor_page.dart';
import 'package:buybuddy/providers/knowledge_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/knowledge_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/intl.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('entry editor renders occurredAt in the local timezone', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
    final occurredAt = DateTime.utc(2026, 8, 25, 1);
    final api = _EditorKnowledgeApi();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue(
            'editor-widget@example.com',
          ),
          knowledgeServiceProvider.overrideWithValue(api),
        ],
        child: MaterialApp(
          home: KnowledgeEntryEditorPage(
            initialTopicId: api.topic.topic.id,
            entry: KnowledgeEntry(
              id: 'entry-1',
              topicId: api.topic.topic.id,
              kind: 'diary',
              title: 'Timezone test',
              body: 'Occurred near a UTC date boundary.',
              attributes: const {},
              tags: const [],
              occurredAt: occurredAt,
              source: 'manual',
              status: 'active',
              version: 1,
              createdAt: occurredAt,
              updatedAt: occurredAt,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final localDate = find.text(
      DateFormat.yMMMd().format(occurredAt.toLocal()),
    );
    await tester.scrollUntilVisible(
      localDate,
      300,
      scrollable: find.byType(Scrollable).first,
    );
    expect(localDate, findsOneWidget);
  });
}

class _EditorKnowledgeApi implements KnowledgeApi {
  _EditorKnowledgeApi()
    : topic = KnowledgeTopicNode(
        topic: KnowledgeTopic(
          id: 'topic-1',
          name: 'Diary',
          depth: 0,
          pendingWriteCount: 0,
          createdAt: DateTime.utc(2026, 8, 24),
          updatedAt: DateTime.utc(2026, 8, 24),
        ),
        entryCount: 1,
        childCount: 0,
      );

  final KnowledgeTopicNode topic;

  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() async => [topic];

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
