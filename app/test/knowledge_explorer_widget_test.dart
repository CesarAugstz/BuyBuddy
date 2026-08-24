import 'dart:async';

import 'package:buybuddy/models/knowledge.dart';
import 'package:buybuddy/pages/knowledge_explorer_page.dart';
import 'package:buybuddy/providers/cache_provider.dart';
import 'package:buybuddy/providers/knowledge_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/knowledge_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('explorer drills into folders and opens entry details', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
    final api = _ExplorerKnowledgeApi();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue('widget@example.com'),
          knowledgeServiceProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: KnowledgeExplorerPage()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Personal Knowledge'), findsOneWidget);
    expect(find.text('Inbox'), findsOneWidget);
    expect(find.text('Projects'), findsOneWidget);
    expect(find.text('2 entries'), findsOneWidget);

    await tester.tap(find.byKey(const Key('knowledge-topic-projects')));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('knowledge-breadcrumb-projects')),
      findsOneWidget,
    );
    expect(find.text('Entries'), findsOneWidget);
    expect(find.text('BuyBuddy architecture decision'), findsOneWidget);
    expect(find.text('Keep PostgreSQL as the only database.'), findsOneWidget);

    await tester.tap(find.byKey(const Key('knowledge-entry-entry-1')));
    await tester.pumpAndSettle();

    expect(find.text('Knowledge entry'), findsOneWidget);
    expect(find.text('Body'), findsOneWidget);
    expect(find.text('Keep PostgreSQL as the only database.'), findsOneWidget);
    expect(find.text('Attributes'), findsOneWidget);
  });

  testWidgets('explorer loads a second topic page without using entryCount', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
    final api = _PaginationKnowledgeApi();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue(
            'pagination-widget@example.com',
          ),
          knowledgeServiceProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: KnowledgeExplorerPage()),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('knowledge-topic-projects')));
    await tester.pumpAndSettle();

    final loadMore = find.byKey(const Key('knowledge-load-more'));
    expect(loadMore, findsOneWidget);
    await tester.ensureVisible(loadMore);
    await tester.pumpAndSettle();
    await tester.tap(loadMore);
    await tester.pumpAndSettle();

    expect(find.text('Entry 100'), findsOneWidget);
    expect(find.byKey(const Key('knowledge-load-more')), findsNothing);
    expect(api.offsets, [0, 100]);
  });

  testWidgets(
    'explorer renders cached root before restoring the recent directory',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      await CacheService().clearAllCache();
      const scope = 'recent-widget@example.com';
      final api = _ExplorerKnowledgeApi();
      final cache = _ControlledRecentCacheService();
      await cache.set(
        knowledgeScopedCacheKey(scope, 'topic_tree'),
        [api.inbox.toJson(), api.projects.toJson()],
        ttl: const Duration(days: 7),
        persistToDisk: true,
      );
      await cache.set(
        knowledgeScopedCacheKey(scope, 'recent_topic_id'),
        api.projects.topic.id,
        ttl: const Duration(days: 365),
        persistToDisk: true,
      );
      cache.blockRecentRead();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            cacheServiceProvider.overrideWithValue(cache),
            knowledgeCacheScopeProvider.overrideWithValue(scope),
            knowledgeServiceProvider.overrideWithValue(api),
          ],
          child: const MaterialApp(home: KnowledgeExplorerPage()),
        ),
      );
      for (var index = 0; index < 10; index++) {
        await tester.pump();
      }
      await cache.waitForRecentRead();

      expect(find.text('Personal Knowledge'), findsOneWidget);
      expect(find.text('Projects'), findsOneWidget);
      expect(
        find.byKey(const Key('knowledge-breadcrumb-projects')),
        findsNothing,
      );

      cache.releaseRecentRead();
      await tester.pumpAndSettle();

      expect(find.text('Personal Knowledge'), findsNothing);
      expect(
        find.byKey(const Key('knowledge-breadcrumb-projects')),
        findsOneWidget,
      );
      expect(find.text('BuyBuddy architecture decision'), findsOneWidget);
    },
  );

  testWidgets('recent restoration does not update a disposed explorer', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
    const scope = 'disposed-recent-widget@example.com';
    final api = _ExplorerKnowledgeApi();
    final cache = _ControlledRecentCacheService();
    await cache.set(
      knowledgeScopedCacheKey(scope, 'topic_tree'),
      [api.projects.toJson()],
      ttl: const Duration(days: 7),
      persistToDisk: true,
    );
    await cache.set(
      knowledgeScopedCacheKey(scope, 'recent_topic_id'),
      api.projects.topic.id,
      ttl: const Duration(days: 365),
      persistToDisk: true,
    );
    cache.blockRecentRead();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          knowledgeCacheScopeProvider.overrideWithValue(scope),
          knowledgeServiceProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: KnowledgeExplorerPage()),
      ),
    );
    for (var index = 0; index < 10; index++) {
      await tester.pump();
    }
    await cache.waitForRecentRead();

    await tester.pumpWidget(const MaterialApp(home: SizedBox()));
    cache.releaseRecentRead();
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'topic menu organizes Inbox-compatible topics and reports success',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      await CacheService().clearAllCache();
      final api = _ExplorerKnowledgeApi();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            knowledgeCacheScopeProvider.overrideWithValue(
              'organize-widget@example.com',
            ),
            knowledgeServiceProvider.overrideWithValue(api),
          ],
          child: const MaterialApp(home: KnowledgeExplorerPage()),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('knowledge-organization-indicator-projects')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const Key('knowledge-topic-projects')));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();

      expect(find.text('Organize this topic'), findsOneWidget);
      await tester.tap(find.text('Organize this topic'));
      await tester.pumpAndSettle();

      expect(api.organizeCalls, ['projects']);
      expect(
        find.textContaining('Organized Projects: 1 change'),
        findsOneWidget,
      );
    },
  );

  testWidgets('topic organization failure reports useful error UX', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
    final api = _ExplorerKnowledgeApi(
      organizeError: Exception('service unavailable'),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue(
            'organize-failure-widget@example.com',
          ),
          knowledgeServiceProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: KnowledgeExplorerPage()),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('knowledge-topic-projects')));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(PopupMenuButton<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Organize this topic'));
    await tester.pumpAndSettle();

    expect(find.textContaining('service unavailable'), findsWidgets);
    expect(tester.takeException(), isNull);
  });
}

class _ExplorerKnowledgeApi implements KnowledgeApi {
  _ExplorerKnowledgeApi({this.organizeError}) {
    final now = DateTime.utc(2026, 8, 24);
    inbox = KnowledgeTopicNode(
      topic: KnowledgeTopic(
        id: 'inbox',
        name: 'Inbox',
        depth: 0,
        pendingWriteCount: 0,
        createdAt: now,
        updatedAt: now,
      ),
      entryCount: 2,
      childCount: 0,
    );
    projects = KnowledgeTopicNode(
      topic: KnowledgeTopic(
        id: 'projects',
        name: 'Projects',
        depth: 0,
        pendingWriteCount: 2,
        organizationDueAt: now.add(const Duration(days: 2)),
        createdAt: now,
        updatedAt: now,
      ),
      entryCount: 1,
      childCount: 0,
    );
    entry = KnowledgeEntry(
      id: 'entry-1',
      topicId: 'projects',
      kind: 'decision',
      title: 'BuyBuddy architecture decision',
      body: 'Keep PostgreSQL as the only database.',
      attributes: const {'database': 'PostgreSQL'},
      tags: const ['BuyBuddy', 'architecture'],
      source: 'assistant',
      status: 'active',
      version: 1,
      createdAt: now,
      updatedAt: now,
      topic: projects.topic,
    );
  }

  final Object? organizeError;
  final List<String> organizeCalls = [];
  late final KnowledgeTopicNode inbox;
  late KnowledgeTopicNode projects;
  late final KnowledgeEntry entry;

  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() async => [inbox, projects];

  @override
  Future<List<KnowledgeEntry>> getTopicEntries(
    String topicId, {
    int limit = 100,
    int offset = 0,
  }) async => topicId == 'projects' ? [entry] : [];

  @override
  Future<KnowledgeEntry> getEntry(String id) async => entry;

  @override
  Future<KnowledgeOrganizationResponse> organizeTopic(String id) async {
    organizeCalls.add(id);
    if (organizeError != null) throw organizeError!;
    final organizedTopic = projects.topic.copyWith(
      pendingWriteCount: 0,
      organizationDueAt: null,
      organizationLeaseUntil: null,
      lastOrganizedAt: DateTime.utc(2026, 8, 24, 16),
    );
    projects = projects.copyWith(topic: organizedTopic);
    return KnowledgeOrganizationResponse(
      status: 'organized',
      topic: organizedTopic,
      result: const KnowledgeOrganizationResult(
        operationsApplied: 1,
        changedEntryIds: ['entry-1'],
        createdTopicIds: [],
        affectedTopicIds: ['projects'],
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _PaginationKnowledgeApi implements KnowledgeApi {
  _PaginationKnowledgeApi() {
    final now = DateTime.utc(2026, 8, 24);
    projects = KnowledgeTopicNode(
      topic: KnowledgeTopic(
        id: 'projects',
        name: 'Projects',
        depth: 0,
        pendingWriteCount: 0,
        createdAt: now,
        updatedAt: now,
      ),
      entryCount: 1,
      childCount: 0,
    );
  }

  late final KnowledgeTopicNode projects;
  final List<int> offsets = [];

  KnowledgeEntry _entry(int index) {
    final now = DateTime.utc(2026, 8, 24);
    return KnowledgeEntry(
      id: 'entry-$index',
      topicId: 'projects',
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
    );
  }

  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() async => [projects];

  @override
  Future<List<KnowledgeEntry>> getTopicEntries(
    String topicId, {
    int limit = 100,
    int offset = 0,
  }) async {
    offsets.add(offset);
    if (offset == 0) {
      return [for (var index = 0; index < 100; index++) _entry(index)];
    }
    if (offset == 100) return [_entry(100)];
    return [];
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _ControlledRecentCacheService extends CacheService {
  _ControlledRecentCacheService() : super.testing();

  Completer<void>? _recentReadStarted;
  Completer<void>? _recentReadRelease;

  void blockRecentRead() {
    _recentReadStarted = Completer<void>();
    _recentReadRelease = Completer<void>();
  }

  Future<void> waitForRecentRead() => _recentReadStarted!.future;

  void releaseRecentRead() => _recentReadRelease!.complete();

  @override
  Future<T?> get<T>(
    String key, {
    bool checkDisk = true,
    bool allowExpired = false,
    bool retainExpiredOnDisk = false,
  }) async {
    if (key.endsWith('recent_topic_id') &&
        _recentReadStarted != null &&
        !_recentReadStarted!.isCompleted) {
      _recentReadStarted!.complete();
      await _recentReadRelease!.future;
    }
    return super.get<T>(
      key,
      checkDisk: checkDisk,
      allowExpired: allowExpired,
      retainExpiredOnDisk: retainExpiredOnDisk,
    );
  }
}
