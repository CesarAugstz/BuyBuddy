import 'package:buybuddy/models/knowledge.dart';
import 'package:buybuddy/pages/knowledge_search_page.dart';
import 'package:buybuddy/providers/knowledge_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/knowledge_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('search explains the 50-result display limit', (tester) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue(
            'search-widget@example.com',
          ),
          knowledgeServiceProvider.overrideWithValue(_SearchKnowledgeApi()),
        ],
        child: const MaterialApp(home: KnowledgeSearchPage()),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('knowledge-search-field')),
      'entry',
    );
    await tester.tap(find.byTooltip('Search'));
    await tester.pumpAndSettle();

    final results = find.byKey(const Key('knowledge-search-results'));
    expect(results, findsOneWidget);
    for (
      var attempt = 0;
      attempt < 12 &&
          find
              .byKey(const Key('knowledge-search-limit-message'))
              .evaluate()
              .isEmpty;
      attempt++
    ) {
      await tester.drag(results, const Offset(0, -500));
      await tester.pumpAndSettle();
    }

    expect(
      find.byKey(const Key('knowledge-search-limit-message')),
      findsOneWidget,
    );
    expect(
      find.text(
        'Showing the first 50 results. Refine your search or filters to find more.',
      ),
      findsOneWidget,
    );
  });
}

class _SearchKnowledgeApi implements KnowledgeApi {
  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() async => const [];

  @override
  Future<List<KnowledgeSearchResult>> search(
    KnowledgeSearchFilter filter,
  ) async {
    final now = DateTime.utc(2026, 8, 24);
    return [
      for (var index = 0; index < 50; index++)
        KnowledgeSearchResult(
          entry: KnowledgeEntry(
            id: 'entry-$index',
            topicId: 'topic-1',
            kind: 'note',
            title: 'Entry $index',
            body: 'Body $index',
            attributes: const {},
            tags: const [],
            source: 'manual',
            status: 'active',
            version: 1,
            createdAt: now,
            updatedAt: now,
          ),
          breadcrumb: const [],
        ),
    ];
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
