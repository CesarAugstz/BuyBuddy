import 'dart:async';
import 'dart:convert';

import 'package:buybuddy/models/knowledge.dart';
import 'package:buybuddy/providers/cache_provider.dart';
import 'package:buybuddy/providers/knowledge_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/knowledge_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
  });

  test(
    'tree provider renders persistent cache before background refresh',
    () async {
      const scope = 'cache-test@example.com';
      final cached = [_topicNode('cached', 'Cached folder')];
      await CacheService().set(
        knowledgeScopedCacheKey(scope, 'topic_tree'),
        cached.map((topic) => topic.toJson()).toList(),
        ttl: const Duration(days: 7),
        persistToDisk: true,
      );
      final service = _FakeKnowledgeApi(
        tree: [_topicNode('remote', 'Remote folder')],
      )..treeCompleter = Completer<List<KnowledgeTopicNode>>();
      final container = ProviderContainer(
        overrides: [
          knowledgeCacheScopeProvider.overrideWithValue(scope),
          knowledgeServiceProvider.overrideWithValue(service),
        ],
      );
      addTearDown(container.dispose);

      final initial = await container.read(knowledgeTreeProvider.future);

      expect(initial.fromCache, isTrue);
      expect(initial.topics.single.topic.name, 'Cached folder');
      await service.waitForTreeCall();
      expect(
        container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
        'Cached folder',
      );

      service.treeCompleter!.complete(service.tree);
      await _flushEvents();

      expect(
        container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
        'Remote folder',
      );
    },
  );

  test('topic rename updates optimistically and keeps server result', () async {
    final original = _topicNode('topic-1', 'Original');
    final service = _FakeKnowledgeApi(tree: [original])
      ..updateTopicCompleter = Completer<KnowledgeTopic>();
    final container = _container(service, 'rename-success@example.com');
    addTearDown(container.dispose);
    await container.read(knowledgeTreeProvider.future);

    final operation = container
        .read(knowledgeTreeProvider.notifier)
        .renameTopic(original.topic, 'Optimistic');

    expect(
      container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
      'Optimistic',
    );
    await service.waitForTopicUpdate();
    service.updateTopicCompleter!.complete(
      original.topic.copyWith(
        name: 'Saved by server',
        updatedAt: DateTime.now(),
      ),
    );
    await operation;

    expect(
      container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
      'Saved by server',
    );
  });

  test(
    'entry metadata rolls back when optimistic synchronization fails',
    () async {
      final entry = _entry('entry-1', title: 'Original title');
      final service = _FakeKnowledgeApi(
        tree: [_topicNode('topic-1', 'Notes')],
        entry: entry,
      )..updateEntryCompleter = Completer<KnowledgeEntry>();
      final container = _container(service, 'entry-rollback@example.com');
      addTearDown(container.dispose);
      final provider = knowledgeEntryProvider(entry.id);
      final subscription = container.listen(provider, (_, __) {});
      addTearDown(subscription.close);
      await container.read(provider.future);

      final operation = container
          .read(provider.notifier)
          .saveChanges(
            topicId: entry.topicId,
            kind: entry.kind,
            title: 'Optimistic title',
            body: entry.body,
            attributes: entry.attributes,
            tags: const ['updated'],
            occurredAt: entry.occurredAt,
          );

      expect(container.read(provider).value?.entry.title, 'Optimistic title');
      expect(container.read(provider).value?.entry.tags, ['updated']);
      await service.waitForEntryUpdate();
      service.updateEntryCompleter!.completeError(
        Exception('network unavailable'),
      );
      await expectLater(operation, throwsException);

      expect(container.read(provider).value?.entry.title, 'Original title');
      expect(container.read(provider).value?.entry.tags, entry.tags);
      expect(
        container.read(provider).value?.error,
        contains('network unavailable'),
      );
    },
  );

  test('body edits wait for server confirmation', () async {
    final entry = _entry('entry-body', title: 'Body source of truth');
    final service = _FakeKnowledgeApi(
      tree: [_topicNode('topic-1', 'Notes')],
      entry: entry,
    )..updateEntryCompleter = Completer<KnowledgeEntry>();
    final container = _container(service, 'body-save@example.com');
    addTearDown(container.dispose);
    final provider = knowledgeEntryProvider(entry.id);
    final subscription = container.listen(provider, (_, __) {});
    addTearDown(subscription.close);
    await container.read(provider.future);

    final operation = container
        .read(provider.notifier)
        .saveChanges(
          topicId: entry.topicId,
          kind: entry.kind,
          title: entry.title,
          body: 'Server-confirmed body',
          attributes: entry.attributes,
          tags: entry.tags,
          occurredAt: entry.occurredAt,
        );

    expect(container.read(provider).value?.entry.body, entry.body);
    expect(container.read(provider).value?.isSaving, isTrue);
    await service.waitForEntryUpdate();
    service.updateEntryCompleter!.complete(
      entry.copyWith(body: 'Server-confirmed body', version: 2),
    );
    await operation;

    expect(container.read(provider).value?.entry.body, 'Server-confirmed body');
  });

  test(
    'manual entry saves request attribute replacement for removal and empty map',
    () async {
      final entry = _entry(
        'entry-attributes',
        title: 'Attribute source of truth',
      ).copyWith(attributes: const {'kept': true, 'removed': true});
      final service = _FakeKnowledgeApi(
        tree: [_topicNode('topic-1', 'Notes')],
        entry: entry,
      );
      final container = _container(service, 'attribute-save@example.com');
      addTearDown(container.dispose);
      final provider = knowledgeEntryProvider(entry.id);
      final subscription = container.listen(provider, (_, __) {});
      addTearDown(subscription.close);
      await container.read(provider.future);

      await container
          .read(provider.notifier)
          .saveChanges(
            topicId: entry.topicId,
            kind: entry.kind,
            title: entry.title,
            body: entry.body,
            attributes: const {'kept': true},
            tags: entry.tags,
            occurredAt: entry.occurredAt,
          );
      await container
          .read(provider.notifier)
          .saveChanges(
            topicId: entry.topicId,
            kind: entry.kind,
            title: entry.title,
            body: entry.body,
            attributes: const {},
            tags: entry.tags,
            occurredAt: entry.occurredAt,
          );

      expect(service.replaceAttributeCalls, [true, true]);
      expect(service.attributeCalls[0], const {'kept': true});
      expect(service.attributeCalls[1], isEmpty);
      expect(container.read(provider).value?.entry.attributes, isEmpty);
    },
  );

  test('topic entries append stable pages without duplicate IDs', () async {
    final firstPage = [
      for (var index = 0; index < 100; index++)
        _entry('entry-$index', title: 'Entry $index'),
    ];
    final secondPage = [
      for (var index = 99; index < 199; index++)
        _entry('entry-$index', title: 'Entry $index'),
    ];
    final finalPage = [
      for (var index = 199; index < 205; index++)
        _entry('entry-$index', title: 'Entry $index'),
    ];
    final service = _FakeKnowledgeApi(
      tree: [_topicNode('topic-1', 'Notes')],
      topicEntryPages: {0: firstPage, 100: secondPage, 200: finalPage},
    );
    final container = _container(service, 'pagination@example.com');
    addTearDown(container.dispose);
    final provider = knowledgeTopicEntriesProvider('topic-1');
    final subscription = container.listen(provider, (_, __) {});
    addTearDown(subscription.close);

    final initial = await container.read(provider.future);
    expect(initial.entries, hasLength(100));
    expect(initial.hasMore, isTrue);
    expect(initial.nextOffset, 100);

    await container.read(provider.notifier).loadMore();
    final second = container.read(provider).value!;
    expect(second.entries, hasLength(199));
    expect(second.entries.map((entry) => entry.id).toSet(), hasLength(199));
    expect(second.hasMore, isTrue);
    expect(second.nextOffset, 200);

    await container.read(provider.notifier).loadMore();
    final complete = container.read(provider).value!;
    expect(complete.entries, hasLength(205));
    expect(complete.entries.map((entry) => entry.id).toSet(), hasLength(205));
    expect(complete.hasMore, isFalse);
    expect(complete.nextOffset, 206);
    expect(service.topicEntryOffsets, [0, 100, 200]);
  });

  test(
    'tree refresh cannot overwrite a mutation during cache persistence',
    () async {
      const scope = 'tree-race@example.com';
      final original = _topicNode('topic-1', 'Original');
      final cache = _ControlledCacheService();
      final cacheKey = knowledgeScopedCacheKey(scope, 'topic_tree');
      await cache.set(
        cacheKey,
        [original.toJson()],
        ttl: const Duration(days: 7),
        persistToDisk: true,
      );
      cache.blockNextSet();
      final service =
          _FakeKnowledgeApi(tree: [original])
            ..treeCompleter = Completer<List<KnowledgeTopicNode>>()
            ..updateTopicCompleter = Completer<KnowledgeTopic>();
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          knowledgeCacheScopeProvider.overrideWithValue(scope),
          knowledgeServiceProvider.overrideWithValue(service),
        ],
      );
      addTearDown(container.dispose);

      await container.read(knowledgeTreeProvider.future);
      await service.waitForTreeCall();
      service.treeCompleter!.complete([original]);
      await cache.waitForBlockedSet();

      final rename = container
          .read(knowledgeTreeProvider.notifier)
          .renameTopic(original.topic, 'Optimistic');
      expect(
        container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
        'Optimistic',
      );

      cache.releaseBlockedSet();
      await service.waitForTopicUpdate();
      final optimisticCache = await cache.get<List<dynamic>>(cacheKey);
      expect(optimisticCache!.single['name'], 'Optimistic');
      expect(
        container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
        'Optimistic',
      );

      service.updateTopicCompleter!.complete(
        original.topic.copyWith(name: 'Saved by server'),
      );
      await rename;
      await _flushEvents();

      final persisted = await cache.get<List<dynamic>>(cacheKey);
      expect(persisted!.single['name'], 'Saved by server');
      expect(
        container.read(knowledgeTreeProvider).value?.topics.single.topic.name,
        'Saved by server',
      );
    },
  );

  test(
    'directory refresh cannot overwrite a mutation during cache persistence',
    () async {
      const scope = 'directory-race@example.com';
      final stale = _entry('entry-1', title: 'Stale snapshot');
      final updated = stale.copyWith(title: 'Mutation wins', version: 2);
      final cache = _ControlledCacheService();
      final cacheKey = knowledgeScopedCacheKey(scope, 'topic_entries_topic-1');
      await cache.set(
        cacheKey,
        {
          'entries': [stale.toJson()],
          'hasMore': false,
          'nextOffset': 1,
        },
        ttl: const Duration(days: 7),
        persistToDisk: true,
      );
      cache.blockNextSet();
      final service = _FakeKnowledgeApi(
        tree: [_topicNode('topic-1', 'Notes')],
        entry: stale,
      )..topicEntriesCompleter = Completer<List<KnowledgeEntry>>();
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          knowledgeCacheScopeProvider.overrideWithValue(scope),
          knowledgeServiceProvider.overrideWithValue(service),
        ],
      );
      addTearDown(container.dispose);
      final provider = knowledgeTopicEntriesProvider('topic-1');
      final subscription = container.listen(provider, (_, __) {});
      addTearDown(subscription.close);

      await container.read(provider.future);
      await service.waitForTopicEntriesCall();
      service.topicEntriesCompleter!.complete([stale]);
      await cache.waitForBlockedSet();

      final mutation = container
          .read(provider.notifier)
          .applyEntrySnapshot(updated, previousTopicId: 'topic-1');
      expect(
        container.read(provider).value?.entries.single.title,
        'Mutation wins',
      );

      cache.releaseBlockedSet();
      await mutation;
      await _flushEvents();

      final persisted = await cache.get<Map<String, dynamic>>(cacheKey);
      final persistedEntry = Map<String, dynamic>.from(
        (persisted!['entries'] as List<dynamic>).single as Map,
      );
      expect(persistedEntry['title'], 'Mutation wins');
      expect(
        container.read(provider).value?.entries.single.title,
        'Mutation wins',
      );
      expect(container.read(provider).value?.isRefreshing, isFalse);
    },
  );

  test(
    'expired knowledge caches survive repeated offline provider starts',
    () async {
      const scope = 'offline-cache@example.com';
      final topic = _topicNode('topic-1', 'Saved folder');
      final entry = _entry('entry-1', title: 'Saved entry');
      final expired = DateTime.now().subtract(const Duration(days: 30));
      String encoded(dynamic data) => jsonEncode(
        CacheEntry(
          data: data,
          timestamp: expired,
          ttl: const Duration(days: 14),
        ).toJson(),
      );
      final treeKey = knowledgeScopedCacheKey(scope, 'topic_tree');
      final directoryKey = knowledgeScopedCacheKey(
        scope,
        'topic_entries_topic-1',
      );
      final entryKey = knowledgeScopedCacheKey(scope, 'entry_entry-1');
      SharedPreferences.setMockInitialValues({
        'cache_$treeKey': encoded([topic.toJson()]),
        'cache_$directoryKey': encoded({
          'entries': [entry.toJson()],
          'hasMore': false,
          'nextOffset': 1,
        }),
        'cache_$entryKey': encoded(entry.toJson()),
      });

      Future<void> readColdStart() async {
        final container = _container(
          _FakeKnowledgeApi(tree: [topic], entry: entry, offline: true),
          scope,
        );
        final directoryProvider = knowledgeTopicEntriesProvider('topic-1');
        final entryProvider = knowledgeEntryProvider('entry-1');
        final directorySubscription = container.listen(
          directoryProvider,
          (_, __) {},
        );
        final entrySubscription = container.listen(entryProvider, (_, __) {});

        expect(
          (await container.read(
            knowledgeTreeProvider.future,
          )).topics.single.topic.name,
          'Saved folder',
        );
        expect(
          (await container.read(directoryProvider.future)).entries.single.title,
          'Saved entry',
        );
        expect(
          (await container.read(entryProvider.future)).entry.title,
          'Saved entry',
        );
        await _flushEvents();
        directorySubscription.close();
        entrySubscription.close();
        container.dispose();
      }

      await readColdStart();
      final preferences = await SharedPreferences.getInstance();
      expect(preferences.containsKey('cache_$treeKey'), isTrue);
      expect(preferences.containsKey('cache_$directoryKey'), isTrue);
      expect(preferences.containsKey('cache_$entryKey'), isTrue);

      await readColdStart();
      expect(preferences.containsKey('cache_$treeKey'), isTrue);
      expect(preferences.containsKey('cache_$directoryKey'), isTrue);
      expect(preferences.containsKey('cache_$entryKey'), isTrue);
    },
  );

  test('recent topic restoration validates the current tree', () async {
    const scope = 'recent-topic@example.com';
    final topic = _topicNode('topic-1', 'Recent folder');
    final service = _FakeKnowledgeApi(tree: [topic]);
    final container = _container(service, scope);
    addTearDown(container.dispose);
    await container.read(knowledgeTreeProvider.future);
    final notifier = container.read(knowledgeTreeProvider.notifier);

    await notifier.markTopicOpened(topic.topic.id);
    expect(await notifier.restoreRecentTopicId(), topic.topic.id);

    await notifier.markTopicOpened('deleted-topic');
    expect(await notifier.restoreRecentTopicId(), isNull);
    expect(
      await CacheService().get<String>(
        knowledgeScopedCacheKey(scope, 'recent_topic_id'),
      ),
      isNull,
    );
  });

  test(
    'organize topic exposes progress and refreshes tree and affected caches',
    () async {
      const scope = 'organize-success@example.com';
      final original = _topicNode('topic-1', 'Notes').copyWith(
        topic: _topicNode('topic-1', 'Notes').topic.copyWith(
          pendingWriteCount: 2,
          organizationDueAt: DateTime.utc(2026, 8, 26),
        ),
      );
      final organizedTopic = original.topic.copyWith(
        pendingWriteCount: 0,
        organizationDueAt: null,
        organizationLeaseUntil: null,
        lastOrganizedAt: DateTime.utc(2026, 8, 24, 16),
      );
      final service = _FakeKnowledgeApi(tree: [original])
        ..organizeCompleter = Completer<KnowledgeOrganizationResponse>();
      final container = _container(service, scope);
      addTearDown(container.dispose);
      await container.read(knowledgeTreeProvider.future);

      final operation = container
          .read(knowledgeTreeProvider.notifier)
          .organizeTopic('topic-1');
      await service.waitForOrganizeCall();
      expect(
        container
            .read(knowledgeTreeProvider)
            .value
            ?.organizingTopicIds
            .contains('topic-1'),
        isTrue,
      );

      service.tree = [original.copyWith(topic: organizedTopic)];
      service.organizeCompleter!.complete(
        KnowledgeOrganizationResponse(
          status: 'organized',
          topic: organizedTopic,
          result: const KnowledgeOrganizationResult(
            operationsApplied: 1,
            changedEntryIds: ['entry-1'],
            createdTopicIds: [],
            affectedTopicIds: ['topic-1'],
          ),
        ),
      );
      final response = await operation;

      expect(response.result.operationsApplied, 1);
      final state = container.read(knowledgeTreeProvider).value!;
      expect(state.organizingTopicIds, isEmpty);
      expect(state.topics.single.topic.pendingWriteCount, 0);
      expect(state.topics.single.topic.lastOrganizedAt, isNotNull);
      expect(
        await CacheService().get<dynamic>(
          knowledgeScopedCacheKey(scope, 'entry_entry-1'),
        ),
        isNull,
      );
    },
  );

  test(
    'organize topic failure clears progress and preserves cached tree',
    () async {
      final original = _topicNode('topic-1', 'Notes');
      final service = _FakeKnowledgeApi(
        tree: [original],
        organizeError: Exception('organizer unavailable'),
      );
      final container = _container(service, 'organize-failure@example.com');
      addTearDown(container.dispose);
      await container.read(knowledgeTreeProvider.future);

      await expectLater(
        container.read(knowledgeTreeProvider.notifier).organizeTopic('topic-1'),
        throwsException,
      );

      final state = container.read(knowledgeTreeProvider).value!;
      expect(state.organizingTopicIds, isEmpty);
      expect(state.topics.single.topic.name, 'Notes');
      expect(state.error, contains('organizer unavailable'));
    },
  );
}

ProviderContainer _container(_FakeKnowledgeApi service, String scope) {
  return ProviderContainer(
    overrides: [
      knowledgeCacheScopeProvider.overrideWithValue(scope),
      knowledgeServiceProvider.overrideWithValue(service),
    ],
  );
}

Future<void> _flushEvents() async {
  for (var index = 0; index < 8; index++) {
    await Future<void>.delayed(Duration.zero);
  }
}

KnowledgeTopicNode _topicNode(String id, String name) {
  final now = DateTime.utc(2026, 8, 24);
  return KnowledgeTopicNode(
    topic: KnowledgeTopic(
      id: id,
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

KnowledgeEntry _entry(String id, {required String title}) {
  final now = DateTime.utc(2026, 8, 24);
  return KnowledgeEntry(
    id: id,
    topicId: 'topic-1',
    kind: 'note',
    title: title,
    body: 'Full entry body',
    attributes: const {'priority': 'high'},
    tags: const ['original'],
    source: 'manual',
    status: 'active',
    version: 1,
    createdAt: now,
    updatedAt: now,
  );
}

class _FakeKnowledgeApi implements KnowledgeApi {
  _FakeKnowledgeApi({
    required this.tree,
    KnowledgeEntry? entry,
    this.topicEntryPages,
    this.offline = false,
    this.organizeError,
  }) : entry = entry ?? _entry('default', title: 'Default');

  List<KnowledgeTopicNode> tree;
  KnowledgeEntry entry;
  final Map<int, List<KnowledgeEntry>>? topicEntryPages;
  final bool offline;
  final Object? organizeError;
  final List<int> topicEntryOffsets = [];
  final List<Map<String, dynamic>?> attributeCalls = [];
  final List<bool> replaceAttributeCalls = [];
  Completer<List<KnowledgeTopicNode>>? treeCompleter;
  Completer<KnowledgeTopic>? updateTopicCompleter;
  Completer<KnowledgeEntry>? updateEntryCompleter;
  Completer<List<KnowledgeEntry>>? topicEntriesCompleter;
  Completer<KnowledgeOrganizationResponse>? organizeCompleter;
  final Completer<void> _treeCalled = Completer<void>();
  final Completer<void> _topicUpdateCalled = Completer<void>();
  final Completer<void> _entryUpdateCalled = Completer<void>();
  final Completer<void> _topicEntriesCalled = Completer<void>();
  final Completer<void> _organizeCalled = Completer<void>();

  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() {
    if (!_treeCalled.isCompleted) _treeCalled.complete();
    if (offline) return Future.error(Exception('offline'));
    return treeCompleter?.future ?? Future.value(tree);
  }

  Future<void> waitForTreeCall() => _treeCalled.future;
  Future<void> waitForTopicUpdate() => _topicUpdateCalled.future;
  Future<void> waitForEntryUpdate() => _entryUpdateCalled.future;
  Future<void> waitForTopicEntriesCall() => _topicEntriesCalled.future;
  Future<void> waitForOrganizeCall() => _organizeCalled.future;

  @override
  Future<KnowledgeOrganizationResponse> organizeTopic(String id) {
    if (!_organizeCalled.isCompleted) _organizeCalled.complete();
    if (organizeError != null) return Future.error(organizeError!);
    final pending = organizeCompleter;
    if (pending != null) return pending.future;
    return Future.value(
      KnowledgeOrganizationResponse(
        status: 'organized',
        topic: tree.first.topic.copyWith(
          pendingWriteCount: 0,
          organizationDueAt: null,
          organizationLeaseUntil: null,
          lastOrganizedAt: DateTime.now().toUtc(),
        ),
        result: KnowledgeOrganizationResult(
          operationsApplied: 0,
          changedEntryIds: const [],
          createdTopicIds: const [],
          affectedTopicIds: [id],
        ),
      ),
    );
  }

  @override
  Future<KnowledgeTopic> updateTopic(
    String id, {
    String? name,
    String? description,
    String? parentId,
    bool moveToRoot = false,
  }) {
    if (!_topicUpdateCalled.isCompleted) _topicUpdateCalled.complete();
    return updateTopicCompleter?.future ??
        Future.value(
          tree.first.topic.copyWith(
            name: name,
            description: description,
            parentId: moveToRoot ? null : parentId,
          ),
        );
  }

  @override
  Future<KnowledgeEntry> getEntry(String id) async {
    if (offline) throw Exception('offline');
    return entry;
  }

  @override
  Future<KnowledgeEntry> updateEntry(
    String id, {
    required int expectedVersion,
    String? topicId,
    String? kind,
    String? title,
    String? body,
    Map<String, dynamic>? attributes,
    bool replaceAttributes = false,
    List<String>? tags,
    DateTime? occurredAt,
    bool clearOccurredAt = false,
  }) {
    if (!_entryUpdateCalled.isCompleted) _entryUpdateCalled.complete();
    attributeCalls.add(attributes);
    replaceAttributeCalls.add(replaceAttributes);
    final pending = updateEntryCompleter;
    if (pending != null) return pending.future;
    entry = entry.copyWith(
      topicId: topicId,
      kind: kind,
      title: title,
      body: body,
      attributes: attributes,
      tags: tags,
      occurredAt: clearOccurredAt ? null : (occurredAt ?? entry.occurredAt),
      version: expectedVersion + 1,
    );
    return Future.value(entry);
  }

  @override
  Future<List<KnowledgeEntry>> getTopicEntries(
    String topicId, {
    int limit = 100,
    int offset = 0,
  }) async {
    topicEntryOffsets.add(offset);
    if (!_topicEntriesCalled.isCompleted) _topicEntriesCalled.complete();
    if (offline) throw Exception('offline');
    final pending = topicEntriesCompleter;
    if (pending != null) return pending.future;
    final pages = topicEntryPages;
    if (pages != null) return pages[offset] ?? const [];
    return entry.topicId == topicId ? [entry] : [];
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _ControlledCacheService extends CacheService {
  _ControlledCacheService() : super.testing();

  int _setCallCount = 0;
  int? _blockedSetCall;
  Completer<void>? _blockedSetStarted;
  Completer<void>? _blockedSetRelease;

  void blockNextSet() {
    _blockedSetCall = _setCallCount + 1;
    _blockedSetStarted = Completer<void>();
    _blockedSetRelease = Completer<void>();
  }

  Future<void> waitForBlockedSet() => _blockedSetStarted!.future;

  void releaseBlockedSet() => _blockedSetRelease!.complete();

  @override
  Future<void> set(
    String key,
    dynamic data, {
    Duration? ttl,
    bool persistToDisk = false,
  }) async {
    _setCallCount++;
    if (_setCallCount == _blockedSetCall) {
      _blockedSetStarted!.complete();
      await _blockedSetRelease!.future;
    }
    await super.set(key, data, ttl: ttl, persistToDisk: persistToDisk);
  }
}
