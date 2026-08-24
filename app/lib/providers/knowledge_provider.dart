import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/knowledge.dart';
import '../services/knowledge_service.dart';
import 'auth_provider.dart';
import 'cache_provider.dart';

final knowledgeServiceProvider = Provider<KnowledgeApi>((ref) {
  return KnowledgeService();
});

final knowledgeCacheScopeProvider = Provider<String>((ref) {
  final email = ref.watch(currentUserProvider)?.email.trim().toLowerCase();
  return email == null || email.isEmpty ? 'signed-out' : email;
});

String knowledgeScopedCacheKey(String scope, String value) {
  return 'knowledge_${Uri.encodeComponent(scope)}_$value';
}

class KnowledgeTreeState {
  const KnowledgeTreeState({
    required this.topics,
    this.isRefreshing = false,
    this.fromCache = false,
    this.organizingTopicIds = const <String>{},
    this.error,
  });

  final List<KnowledgeTopicNode> topics;
  final bool isRefreshing;
  final bool fromCache;
  final Set<String> organizingTopicIds;
  final String? error;

  KnowledgeTreeState copyWith({
    List<KnowledgeTopicNode>? topics,
    bool? isRefreshing,
    bool? fromCache,
    Set<String>? organizingTopicIds,
    Object? error = _providerUnset,
  }) {
    return KnowledgeTreeState(
      topics: topics ?? this.topics,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      fromCache: fromCache ?? this.fromCache,
      organizingTopicIds: organizingTopicIds ?? this.organizingTopicIds,
      error: identical(error, _providerUnset) ? this.error : error as String?,
    );
  }
}

const Object _providerUnset = Object();

class KnowledgeTreeNotifier extends AsyncNotifier<KnowledgeTreeState> {
  static const _treeCacheSuffix = 'topic_tree';
  static const _recentTopicCacheSuffix = 'recent_topic_id';
  static const _cacheTtl = Duration(days: 14);

  int _mutationRevision = 0;
  int _activeMutations = 0;
  int _refreshGeneration = 0;
  Future<void> _cacheWriteTail = Future<void>.value();

  String get _scope => ref.read(knowledgeCacheScopeProvider);
  String get _treeCacheKey => knowledgeScopedCacheKey(_scope, _treeCacheSuffix);

  @override
  Future<KnowledgeTreeState> build() async {
    ref.watch(knowledgeCacheScopeProvider);
    final cached = await _readCache();
    if (cached != null) {
      unawaited(Future.microtask(_refreshInBackground));
      return KnowledgeTreeState(topics: cached, fromCache: true);
    }
    final topics = await ref.read(knowledgeServiceProvider).getTopicTree();
    await _cacheTopics(topics);
    return KnowledgeTreeState(topics: topics);
  }

  Future<List<KnowledgeTopicNode>?> _readCache() async {
    final cached = await ref
        .read(cacheServiceProvider)
        .get<List<dynamic>>(
          _treeCacheKey,
          allowExpired: true,
          retainExpiredOnDisk: true,
        );
    if (cached == null) return null;
    try {
      return cached
          .map(
            (item) => KnowledgeTopicNode.fromJson(
              Map<String, dynamic>.from(item as Map),
            ),
          )
          .toList();
    } catch (error, stackTrace) {
      developer.log(
        'Discarding invalid knowledge tree cache',
        name: 'KnowledgeTreeNotifier',
        error: error,
        stackTrace: stackTrace,
      );
      await ref.read(cacheServiceProvider).invalidate(_treeCacheKey);
      return null;
    }
  }

  Future<void> _writeTopics(List<KnowledgeTopicNode> topics) {
    return ref
        .read(cacheServiceProvider)
        .set(
          _treeCacheKey,
          topics.map((topic) => topic.toJson()).toList(),
          ttl: _cacheTtl,
          persistToDisk: true,
        );
  }

  Future<T> _withCacheWriteLock<T>(Future<T> Function() operation) async {
    final previous = _cacheWriteTail;
    final completed = Completer<void>();
    _cacheWriteTail = completed.future;
    await previous;
    try {
      return await operation();
    } finally {
      completed.complete();
    }
  }

  Future<void> _cacheTopics(List<KnowledgeTopicNode> topics) {
    return _withCacheWriteLock(() => _writeTopics(topics));
  }

  Future<({List<KnowledgeTopicNode> topics, int revision})>
  _fetchWhenIdle() async {
    while (true) {
      while (_activeMutations > 0) {
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
      final revision = _mutationRevision;
      final topics = await ref.read(knowledgeServiceProvider).getTopicTree();
      if (_activeMutations == 0 && revision == _mutationRevision) {
        return (topics: topics, revision: revision);
      }
    }
  }

  Future<void> _refreshInBackground() async {
    final current = state.value;
    if (current == null) return;
    final generation = ++_refreshGeneration;
    state = AsyncValue.data(current.copyWith(isRefreshing: true, error: null));
    try {
      final snapshot = await _fetchWhenIdle();
      await _withCacheWriteLock(() async {
        bool canApply() =>
            ref.mounted &&
            generation == _refreshGeneration &&
            _activeMutations == 0 &&
            snapshot.revision == _mutationRevision;

        if (!canApply()) {
          _finishRefreshIfCurrent(generation);
          return;
        }
        await _writeTopics(snapshot.topics);
        if (!canApply()) {
          final displayed = state.value;
          if (displayed != null) {
            await _writeTopics(displayed.topics);
          }
          _finishRefreshIfCurrent(generation);
          return;
        }
        state = AsyncValue.data(KnowledgeTreeState(topics: snapshot.topics));
      });
    } catch (error, stackTrace) {
      developer.log(
        'Failed to refresh the knowledge tree',
        name: 'KnowledgeTreeNotifier',
        error: error,
        stackTrace: stackTrace,
      );
      if (ref.mounted && generation == _refreshGeneration) {
        final displayed = state.value ?? current;
        state = AsyncValue.data(
          displayed.copyWith(
            isRefreshing: false,
            error: 'Offline — showing saved knowledge folders.',
          ),
        );
      }
    }
  }

  void _finishRefreshIfCurrent(int generation) {
    if (!ref.mounted || generation != _refreshGeneration) return;
    final displayed = state.value;
    if (displayed != null && displayed.isRefreshing) {
      state = AsyncValue.data(displayed.copyWith(isRefreshing: false));
    }
  }

  Future<void> refresh() async {
    final current = state.value;
    if (current == null) {
      state = const AsyncValue.loading();
      state = await AsyncValue.guard(() async {
        final snapshot = await _fetchWhenIdle();
        await _cacheTopics(snapshot.topics);
        return KnowledgeTreeState(topics: snapshot.topics);
      });
      return;
    }
    await _refreshInBackground();
  }

  Future<void> markTopicOpened(String topicId) {
    return ref
        .read(cacheServiceProvider)
        .set(
          knowledgeScopedCacheKey(_scope, _recentTopicCacheSuffix),
          topicId,
          ttl: const Duration(days: 365),
          persistToDisk: true,
        );
  }

  Future<String?> restoreRecentTopicId() async {
    final cache = ref.read(cacheServiceProvider);
    final key = knowledgeScopedCacheKey(_scope, _recentTopicCacheSuffix);
    final topicId = await cache.get<String>(key, allowExpired: true);
    if (topicId == null || !ref.mounted) return null;
    final topics = state.value?.topics;
    if (topics != null && findKnowledgeTopic(topics, topicId) != null) {
      return topicId;
    }
    await cache.invalidate(key);
    return null;
  }

  Future<void> forgetRecentTopic() {
    return ref
        .read(cacheServiceProvider)
        .invalidate(knowledgeScopedCacheKey(_scope, _recentTopicCacheSuffix));
  }

  Future<KnowledgeTopic> createTopic({
    required String name,
    String? parentId,
    String description = '',
  }) async {
    final topic = await ref
        .read(knowledgeServiceProvider)
        .createTopic(name: name, parentId: parentId, description: description);
    final current = state.value;
    if (current != null) {
      _mutationRevision++;
      final nodes = [
        ...flattenKnowledgeTopics(current.topics),
        KnowledgeTopicNode(topic: topic, entryCount: 0, childCount: 0),
      ];
      final topics = rebuildKnowledgeTopicTree(nodes);
      state = AsyncValue.data(current.copyWith(topics: topics, error: null));
      await _cacheTopics(topics);
    } else {
      ref.invalidateSelf();
    }
    return topic;
  }

  Future<KnowledgeEntry> createEntry({
    required String topicId,
    required String kind,
    required String title,
    required String body,
    required Map<String, dynamic> attributes,
    required List<String> tags,
    DateTime? occurredAt,
  }) async {
    final entry = await ref
        .read(knowledgeServiceProvider)
        .createEntry(
          topicId: topicId,
          kind: kind,
          title: title,
          body: body,
          attributes: attributes,
          tags: tags,
          occurredAt: occurredAt,
        );
    await adjustEntryCounts(topicId: topicId, delta: 1);
    await markTopicWrites([topicId]);
    await ref
        .read(cacheServiceProvider)
        .invalidate(knowledgeScopedCacheKey(_scope, 'topic_entries_$topicId'));
    await cacheKnowledgeEntry(ref, entry);
    ref.invalidate(knowledgeTopicEntriesProvider(topicId));
    return entry;
  }

  Future<KnowledgeTopic> renameTopic(KnowledgeTopic topic, String name) async {
    return _optimisticTopicUpdate(
      topic,
      topic.copyWith(name: name, updatedAt: DateTime.now()),
      () =>
          ref.read(knowledgeServiceProvider).updateTopic(topic.id, name: name),
    );
  }

  Future<KnowledgeTopic> moveTopic(
    KnowledgeTopic topic,
    String? parentId,
  ) async {
    return _optimisticTopicUpdate(
      topic,
      topic.copyWith(parentId: parentId, updatedAt: DateTime.now()),
      () => ref
          .read(knowledgeServiceProvider)
          .updateTopic(
            topic.id,
            parentId: parentId,
            moveToRoot: parentId == null,
          ),
    );
  }

  Future<KnowledgeTopic> _optimisticTopicUpdate(
    KnowledgeTopic previousTopic,
    KnowledgeTopic optimisticTopic,
    Future<KnowledgeTopic> Function() synchronize,
  ) async {
    final previous = state.value;
    if (previous == null) {
      throw StateError('Knowledge folders are not loaded.');
    }
    if (_activeMutations > 0) {
      throw StateError('Another folder change is still being saved.');
    }

    _activeMutations++;
    _mutationRevision++;
    final optimistic = replaceKnowledgeTopic(previous.topics, optimisticTopic);
    state = AsyncValue.data(previous.copyWith(topics: optimistic, error: null));
    await _cacheTopics(optimistic);

    try {
      final updated = await synchronize();
      final displayed = state.value ?? previous;
      final topics = replaceKnowledgeTopic(displayed.topics, updated);
      state = AsyncValue.data(displayed.copyWith(topics: topics, error: null));
      await _cacheTopics(topics);
      return updated;
    } catch (error) {
      state = AsyncValue.data(
        previous.copyWith(error: 'Could not save the folder change: $error'),
      );
      await _cacheTopics(previous.topics);
      rethrow;
    } finally {
      _activeMutations--;
    }
  }

  Future<void> deleteTopic(KnowledgeTopic topic) async {
    final previous = state.value;
    if (previous == null) {
      throw StateError('Knowledge folders are not loaded.');
    }

    if (_activeMutations > 0) {
      throw StateError('Another folder change is still being saved.');
    }
    _activeMutations++;
    _mutationRevision++;
    final optimistic = removeKnowledgeTopic(previous.topics, topic.id);
    state = AsyncValue.data(previous.copyWith(topics: optimistic, error: null));
    await _cacheTopics(optimistic);
    try {
      await ref.read(knowledgeServiceProvider).deleteTopic(topic.id);
      await ref
          .read(cacheServiceProvider)
          .invalidate(
            knowledgeScopedCacheKey(_scope, 'topic_entries_${topic.id}'),
          );
    } catch (error) {
      state = AsyncValue.data(
        previous.copyWith(error: 'Could not delete the folder: $error'),
      );
      await _cacheTopics(previous.topics);
      rethrow;
    } finally {
      _activeMutations--;
    }
  }

  Future<KnowledgeOrganizationResponse> organizeTopic(String topicId) async {
    final current = state.value;
    if (current == null) {
      throw StateError('Knowledge folders are not loaded.');
    }
    if (_activeMutations > 0 || current.organizingTopicIds.contains(topicId)) {
      throw StateError('Another knowledge change is still being saved.');
    }

    _activeMutations++;
    _mutationRevision++;
    state = AsyncValue.data(
      current.copyWith(
        organizingTopicIds: {...current.organizingTopicIds, topicId},
        error: null,
      ),
    );

    KnowledgeOrganizationResponse response;
    try {
      response = await ref
          .read(knowledgeServiceProvider)
          .organizeTopic(topicId);
    } catch (error) {
      final displayed = state.value ?? current;
      state = AsyncValue.data(
        displayed.copyWith(
          organizingTopicIds: {
            ...displayed.organizingTopicIds.where((id) => id != topicId),
          },
          error: 'Could not organize this folder: $error',
        ),
      );
      rethrow;
    } finally {
      _activeMutations--;
    }

    _mutationRevision++;
    final displayed = state.value ?? current;
    final topics = replaceKnowledgeTopic(displayed.topics, response.topic);
    state = AsyncValue.data(
      displayed.copyWith(
        topics: topics,
        organizingTopicIds: {
          ...displayed.organizingTopicIds.where((id) => id != topicId),
        },
        error: null,
      ),
    );

    final cache = ref.read(cacheServiceProvider);
    for (final affectedTopicId in response.result.affectedTopicIds) {
      await cache.invalidate(
        knowledgeScopedCacheKey(_scope, 'topic_entries_$affectedTopicId'),
      );
      await ref
          .read(knowledgeTopicEntriesProvider(affectedTopicId).notifier)
          .refresh();
    }
    for (final entryId in response.result.changedEntryIds) {
      await cache.invalidate(knowledgeScopedCacheKey(_scope, 'entry_$entryId'));
      await ref.read(knowledgeEntryProvider(entryId).notifier).refresh();
    }
    await _refreshInBackground();
    return response;
  }

  Future<void> adjustEntryCounts({
    required String topicId,
    required int delta,
  }) async {
    final current = state.value;
    if (current == null || delta == 0) return;
    _mutationRevision++;
    final topics = adjustKnowledgeEntryCount(current.topics, topicId, delta);
    state = AsyncValue.data(current.copyWith(topics: topics));
    await _cacheTopics(topics);
  }

  Future<void> markTopicWrites(Iterable<String> topicIds) async {
    final current = state.value;
    if (current == null) return;
    final uniqueIds = topicIds.where((id) => id.isNotEmpty).toSet();
    if (uniqueIds.isEmpty) return;
    final now = DateTime.now();
    var topics = current.topics;
    for (final topicId in uniqueIds) {
      final node = findKnowledgeTopic(topics, topicId);
      if (node == null) continue;
      final pendingWriteCount = node.topic.pendingWriteCount + 1;
      topics = replaceKnowledgeTopic(
        topics,
        node.topic.copyWith(
          pendingWriteCount: pendingWriteCount,
          organizationDueAt:
              pendingWriteCount >= 5 ? now : now.add(const Duration(days: 2)),
          updatedAt: now,
        ),
      );
    }
    _mutationRevision++;
    state = AsyncValue.data(current.copyWith(topics: topics));
    await _cacheTopics(topics);
  }

  Future<KnowledgeEntry> undoDeletedEntry(KnowledgeEntry deletedEntry) async {
    final restored = await ref
        .read(knowledgeServiceProvider)
        .undoEntry(deletedEntry.id, expectedVersion: deletedEntry.version + 1);
    await adjustEntryCounts(topicId: restored.topicId, delta: 1);
    await markTopicWrites([restored.topicId]);
    await ref
        .read(cacheServiceProvider)
        .invalidate(
          knowledgeScopedCacheKey(_scope, 'topic_entries_${restored.topicId}'),
        );
    await ref
        .read(knowledgeTopicEntriesProvider(restored.topicId).notifier)
        .applyEntrySnapshot(restored, previousTopicId: restored.topicId);
    await cacheKnowledgeEntry(ref, restored);
    return restored;
  }
}

final knowledgeTreeProvider =
    AsyncNotifierProvider<KnowledgeTreeNotifier, KnowledgeTreeState>(
      KnowledgeTreeNotifier.new,
    );

class KnowledgeDirectoryState {
  const KnowledgeDirectoryState({
    required this.entries,
    required this.hasMore,
    required this.nextOffset,
    this.isRefreshing = false,
    this.isLoadingMore = false,
    this.fromCache = false,
    this.error,
  });

  final List<KnowledgeEntry> entries;
  final bool hasMore;
  final int nextOffset;
  final bool isRefreshing;
  final bool isLoadingMore;
  final bool fromCache;
  final String? error;

  KnowledgeDirectoryState copyWith({
    List<KnowledgeEntry>? entries,
    bool? hasMore,
    int? nextOffset,
    bool? isRefreshing,
    bool? isLoadingMore,
    bool? fromCache,
    Object? error = _providerUnset,
  }) {
    return KnowledgeDirectoryState(
      entries: entries ?? this.entries,
      hasMore: hasMore ?? this.hasMore,
      nextOffset: nextOffset ?? this.nextOffset,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      fromCache: fromCache ?? this.fromCache,
      error: identical(error, _providerUnset) ? this.error : error as String?,
    );
  }
}

class KnowledgeTopicEntriesNotifier
    extends AsyncNotifier<KnowledgeDirectoryState> {
  KnowledgeTopicEntriesNotifier(this.topicId);

  static const pageSize = 100;

  final String topicId;
  int _mutationRevision = 0;
  int _refreshGeneration = 0;
  bool _loadingMore = false;
  Future<void> _cacheWriteTail = Future<void>.value();

  String get _scope => ref.read(knowledgeCacheScopeProvider);
  String get _cacheKey =>
      knowledgeScopedCacheKey(_scope, 'topic_entries_$topicId');

  @override
  Future<KnowledgeDirectoryState> build() async {
    ref.watch(knowledgeCacheScopeProvider);
    final cached = await _readCache();
    if (cached != null) {
      unawaited(Future.microtask(_refreshInBackground));
      return cached.copyWith(fromCache: true);
    }
    final snapshot = await _fetchPageWhenStable(offset: 0);
    final directory = KnowledgeDirectoryState(
      entries: snapshot.entries,
      hasMore: snapshot.entries.length == pageSize,
      nextOffset: snapshot.entries.length,
    );
    await _cacheDirectory(directory);
    return directory;
  }

  Future<KnowledgeDirectoryState?> _readCache() async {
    final cached = await ref
        .read(cacheServiceProvider)
        .get<dynamic>(_cacheKey, allowExpired: true, retainExpiredOnDisk: true);
    if (cached == null) return null;
    try {
      final rawEntries =
          cached is Map
              ? (cached['entries'] as List<dynamic>? ?? const [])
              : cached as List<dynamic>;
      final entries =
          rawEntries
              .map(
                (item) => KnowledgeEntry.fromJson(
                  Map<String, dynamic>.from(item as Map),
                ),
              )
              .toList();
      if (cached is Map) {
        return KnowledgeDirectoryState(
          entries: entries,
          hasMore: cached['hasMore'] == true,
          nextOffset: (cached['nextOffset'] as num?)?.toInt() ?? entries.length,
        );
      }
      return KnowledgeDirectoryState(
        entries: entries,
        hasMore: entries.length >= pageSize,
        nextOffset: entries.length,
      );
    } catch (_) {
      await ref.read(cacheServiceProvider).invalidate(_cacheKey);
      return null;
    }
  }

  Future<void> _writeDirectory(KnowledgeDirectoryState directory) {
    return ref
        .read(cacheServiceProvider)
        .set(
          _cacheKey,
          {
            'entries':
                directory.entries.map((entry) => entry.toJson()).toList(),
            'hasMore': directory.hasMore,
            'nextOffset': directory.nextOffset,
          },
          ttl: const Duration(days: 14),
          persistToDisk: true,
        );
  }

  Future<T> _withCacheWriteLock<T>(Future<T> Function() operation) async {
    final previous = _cacheWriteTail;
    final completed = Completer<void>();
    _cacheWriteTail = completed.future;
    await previous;
    try {
      return await operation();
    } finally {
      completed.complete();
    }
  }

  Future<void> _cacheDirectory(KnowledgeDirectoryState directory) {
    return _withCacheWriteLock(() => _writeDirectory(directory));
  }

  Future<({List<KnowledgeEntry> entries, int revision})> _fetchPageWhenStable({
    required int offset,
  }) async {
    while (true) {
      final revision = _mutationRevision;
      final entries = await ref
          .read(knowledgeServiceProvider)
          .getTopicEntries(topicId, limit: pageSize, offset: offset);
      if (revision == _mutationRevision) {
        return (entries: entries, revision: revision);
      }
    }
  }

  Future<void> _refreshInBackground() async {
    final current = state.value;
    if (current == null) return;
    final generation = ++_refreshGeneration;
    state = AsyncValue.data(
      current.copyWith(isRefreshing: true, isLoadingMore: false, error: null),
    );
    try {
      final snapshot = await _fetchPageWhenStable(offset: 0);
      final directory = KnowledgeDirectoryState(
        entries: snapshot.entries,
        hasMore: snapshot.entries.length == pageSize,
        nextOffset: snapshot.entries.length,
      );
      await _withCacheWriteLock(() async {
        bool canApply() =>
            ref.mounted &&
            generation == _refreshGeneration &&
            snapshot.revision == _mutationRevision;

        if (!canApply()) {
          _finishRefreshIfCurrent(generation);
          return;
        }
        await _writeDirectory(directory);
        if (!canApply()) {
          final displayed = state.value;
          if (displayed != null) {
            await _writeDirectory(displayed);
          }
          _finishRefreshIfCurrent(generation);
          return;
        }
        state = AsyncValue.data(directory);
      });
    } catch (error, stackTrace) {
      developer.log(
        'Failed to refresh knowledge entries for $topicId',
        name: 'KnowledgeTopicEntriesNotifier',
        error: error,
        stackTrace: stackTrace,
      );
      if (ref.mounted && generation == _refreshGeneration) {
        state = AsyncValue.data(
          (state.value ?? current).copyWith(
            isRefreshing: false,
            isLoadingMore: false,
            error: 'Offline — showing saved entries.',
          ),
        );
      }
    }
  }

  void _finishRefreshIfCurrent(int generation) {
    if (!ref.mounted || generation != _refreshGeneration) return;
    final displayed = state.value;
    if (displayed != null && displayed.isRefreshing) {
      state = AsyncValue.data(
        displayed.copyWith(isRefreshing: false, isLoadingMore: false),
      );
    }
  }

  Future<void> refresh() async => _refreshInBackground();

  Future<void> loadMore() async {
    final initial = state.value;
    if (_loadingMore ||
        initial == null ||
        initial.isRefreshing ||
        !initial.hasMore) {
      return;
    }

    _loadingMore = true;
    final generation = _refreshGeneration;
    state = AsyncValue.data(initial.copyWith(isLoadingMore: true, error: null));
    try {
      while (ref.mounted && generation == _refreshGeneration) {
        final current = state.value;
        if (current == null || !current.hasMore) return;
        final revision = _mutationRevision;
        final offset = current.nextOffset;
        final page = await ref
            .read(knowledgeServiceProvider)
            .getTopicEntries(topicId, limit: pageSize, offset: offset);
        if (!ref.mounted || generation != _refreshGeneration) return;
        if (revision != _mutationRevision) continue;

        final latest = state.value;
        if (latest == null || latest.nextOffset != offset) continue;
        final entries = _appendUniqueEntries(latest.entries, page);
        final directory = latest.copyWith(
          entries: entries,
          hasMore: page.length == pageSize,
          nextOffset: offset + page.length,
          isLoadingMore: false,
          fromCache: false,
          error: null,
        );
        state = AsyncValue.data(directory);
        await _cacheDirectory(directory);
        return;
      }
    } catch (error, stackTrace) {
      developer.log(
        'Failed to load more knowledge entries for $topicId',
        name: 'KnowledgeTopicEntriesNotifier',
        error: error,
        stackTrace: stackTrace,
      );
      if (ref.mounted && generation == _refreshGeneration) {
        final current = state.value ?? initial;
        state = AsyncValue.data(
          current.copyWith(
            isLoadingMore: false,
            error: 'Could not load more entries. Try again.',
          ),
        );
      }
    } finally {
      _loadingMore = false;
      if (ref.mounted && generation == _refreshGeneration) {
        final current = state.value;
        if (current != null && current.isLoadingMore) {
          state = AsyncValue.data(current.copyWith(isLoadingMore: false));
        }
      }
    }
  }

  Future<KnowledgeEntry> createEntry({
    required String kind,
    required String title,
    required String body,
    required Map<String, dynamic> attributes,
    required List<String> tags,
    DateTime? occurredAt,
  }) async {
    final entry = await ref
        .read(knowledgeServiceProvider)
        .createEntry(
          topicId: topicId,
          kind: kind,
          title: title,
          body: body,
          attributes: attributes,
          tags: tags,
          occurredAt: occurredAt,
        );
    final current = state.value;
    if (current != null) {
      _mutationRevision++;
      final alreadyLoaded = current.entries.any(
        (candidate) => candidate.id == entry.id,
      );
      final entries = [
        entry,
        ...current.entries.where((candidate) => candidate.id != entry.id),
      ];
      final directory = current.copyWith(
        entries: entries,
        nextOffset: current.nextOffset + (alreadyLoaded ? 0 : 1),
        error: null,
      );
      state = AsyncValue.data(directory);
      await _cacheDirectory(directory);
    } else {
      ref.invalidateSelf();
    }
    await ref
        .read(knowledgeTreeProvider.notifier)
        .adjustEntryCounts(topicId: topicId, delta: 1);
    await ref.read(knowledgeTreeProvider.notifier).markTopicWrites([topicId]);
    await cacheKnowledgeEntry(ref, entry);
    return entry;
  }

  Future<void> applyEntrySnapshot(
    KnowledgeEntry entry, {
    required String previousTopicId,
  }) async {
    final current = state.value;
    if (current == null) return;
    _mutationRevision++;
    var entries = [...current.entries];
    final index = entries.indexWhere((candidate) => candidate.id == entry.id);
    var nextOffset = current.nextOffset;

    if (entry.topicId != topicId) {
      if (index >= 0) {
        entries.removeAt(index);
        nextOffset = nextOffset > 0 ? nextOffset - 1 : 0;
      }
    } else if (index >= 0) {
      entries[index] = entry;
    } else {
      entries.insert(0, entry);
      nextOffset++;
    }
    final directory = current.copyWith(
      entries: entries,
      nextOffset: nextOffset,
      error: null,
    );
    state = AsyncValue.data(directory);
    await _cacheDirectory(directory);
  }

  Future<void> removeEntry(String entryId) async {
    final current = state.value;
    if (current == null) return;
    final entries =
        current.entries.where((entry) => entry.id != entryId).toList();
    if (entries.length == current.entries.length) return;
    _mutationRevision++;
    final directory = current.copyWith(
      entries: entries,
      nextOffset: current.nextOffset > 0 ? current.nextOffset - 1 : 0,
    );
    state = AsyncValue.data(directory);
    await _cacheDirectory(directory);
  }
}

List<KnowledgeEntry> _appendUniqueEntries(
  List<KnowledgeEntry> existing,
  List<KnowledgeEntry> page,
) {
  final seen = existing.map((entry) => entry.id).toSet();
  return [
    ...existing,
    for (final entry in page)
      if (seen.add(entry.id)) entry,
  ];
}

final knowledgeTopicEntriesProvider = AsyncNotifierProvider.autoDispose
    .family<KnowledgeTopicEntriesNotifier, KnowledgeDirectoryState, String>(
      KnowledgeTopicEntriesNotifier.new,
    );

class KnowledgeEntryState {
  const KnowledgeEntryState({
    required this.entry,
    this.isRefreshing = false,
    this.isSaving = false,
    this.fromCache = false,
    this.error,
  });

  final KnowledgeEntry entry;
  final bool isRefreshing;
  final bool isSaving;
  final bool fromCache;
  final String? error;

  KnowledgeEntryState copyWith({
    KnowledgeEntry? entry,
    bool? isRefreshing,
    bool? isSaving,
    bool? fromCache,
    Object? error = _providerUnset,
  }) {
    return KnowledgeEntryState(
      entry: entry ?? this.entry,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      isSaving: isSaving ?? this.isSaving,
      fromCache: fromCache ?? this.fromCache,
      error: identical(error, _providerUnset) ? this.error : error as String?,
    );
  }
}

Future<void> cacheKnowledgeEntry(Ref ref, KnowledgeEntry entry) {
  final scope = ref.read(knowledgeCacheScopeProvider);
  return ref
      .read(cacheServiceProvider)
      .set(
        knowledgeScopedCacheKey(scope, 'entry_${entry.id}'),
        entry.toJson(),
        ttl: const Duration(days: 14),
        persistToDisk: true,
      );
}

class KnowledgeEntryNotifier extends AsyncNotifier<KnowledgeEntryState> {
  KnowledgeEntryNotifier(this.entryId);

  final String entryId;
  int _mutationRevision = 0;
  bool _mutationActive = false;

  String get _scope => ref.read(knowledgeCacheScopeProvider);
  String get _cacheKey => knowledgeScopedCacheKey(_scope, 'entry_$entryId');

  @override
  Future<KnowledgeEntryState> build() async {
    ref.watch(knowledgeCacheScopeProvider);
    final cached = await ref
        .read(cacheServiceProvider)
        .get<Map<String, dynamic>>(
          _cacheKey,
          allowExpired: true,
          retainExpiredOnDisk: true,
        );
    if (cached != null) {
      final entry = KnowledgeEntry.fromJson(cached);
      unawaited(Future.microtask(_refreshInBackground));
      return KnowledgeEntryState(entry: entry, fromCache: true);
    }
    final entry = await ref.read(knowledgeServiceProvider).getEntry(entryId);
    await cacheKnowledgeEntry(ref, entry);
    return KnowledgeEntryState(entry: entry);
  }

  Future<KnowledgeEntry> _fetchWhenIdle() async {
    while (true) {
      while (_mutationActive) {
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
      final revision = _mutationRevision;
      final entry = await ref.read(knowledgeServiceProvider).getEntry(entryId);
      if (!_mutationActive && revision == _mutationRevision) return entry;
    }
  }

  Future<void> _refreshInBackground() async {
    final current = state.value;
    if (current == null) return;
    state = AsyncValue.data(current.copyWith(isRefreshing: true, error: null));
    try {
      final entry = await _fetchWhenIdle();
      if (!ref.mounted) return;
      await cacheKnowledgeEntry(ref, entry);
      state = AsyncValue.data(KnowledgeEntryState(entry: entry));
    } catch (error) {
      if (ref.mounted) {
        state = AsyncValue.data(
          (state.value ?? current).copyWith(
            isRefreshing: false,
            error: 'Offline — showing the saved entry.',
          ),
        );
      }
    }
  }

  Future<void> refresh() => _refreshInBackground();

  Future<KnowledgeEntry> saveChanges({
    required String topicId,
    required String kind,
    required String title,
    required String body,
    required Map<String, dynamic> attributes,
    required List<String> tags,
    DateTime? occurredAt,
  }) async {
    final current = state.value;
    if (current == null) throw StateError('Knowledge entry is not loaded.');
    if (_mutationActive) {
      throw StateError('This entry is already being saved.');
    }

    final previous = current.entry;
    final bodyChanged = body != previous.body;
    final metadataChanged =
        topicId != previous.topicId ||
        kind != previous.kind ||
        title != previous.title ||
        !_sameTags(tags, previous.tags) ||
        !_sameAttributes(attributes, previous.attributes) ||
        occurredAt != previous.occurredAt;
    if (!bodyChanged && !metadataChanged) return previous;

    _mutationActive = true;
    _mutationRevision++;
    final optimistic = previous.copyWith(
      topicId: topicId,
      kind: kind,
      title: title,
      attributes: attributes,
      tags: tags,
      occurredAt: occurredAt,
      updatedAt: DateTime.now(),
    );
    state = AsyncValue.data(
      current.copyWith(
        entry: bodyChanged ? previous : optimistic,
        isSaving: true,
        error: null,
      ),
    );
    if (!bodyChanged) {
      await cacheKnowledgeEntry(ref, optimistic);
    }

    try {
      final updated = await ref
          .read(knowledgeServiceProvider)
          .updateEntry(
            entryId,
            expectedVersion: previous.version,
            topicId: topicId != previous.topicId ? topicId : null,
            kind: kind != previous.kind ? kind : null,
            title: title != previous.title ? title : null,
            body: bodyChanged ? body : null,
            attributes:
                !_sameAttributes(attributes, previous.attributes)
                    ? attributes
                    : null,
            replaceAttributes:
                !_sameAttributes(attributes, previous.attributes),
            tags: !_sameTags(tags, previous.tags) ? tags : null,
            occurredAt:
                occurredAt != null && occurredAt != previous.occurredAt
                    ? occurredAt
                    : null,
            clearOccurredAt: occurredAt == null && previous.occurredAt != null,
          );
      state = AsyncValue.data(KnowledgeEntryState(entry: updated));
      await cacheKnowledgeEntry(ref, updated);
      await _publishEntryChange(updated, previous.topicId);
      return updated;
    } catch (error) {
      state = AsyncValue.data(
        current.copyWith(
          entry: previous,
          isSaving: false,
          error: 'Could not save this entry: $error',
        ),
      );
      await cacheKnowledgeEntry(ref, previous);
      rethrow;
    } finally {
      _mutationActive = false;
    }
  }

  Future<KnowledgeEntry> moveEntry(String topicId) async {
    final entry = state.value?.entry;
    if (entry == null) throw StateError('Knowledge entry is not loaded.');
    return saveChanges(
      topicId: topicId,
      kind: entry.kind,
      title: entry.title,
      body: entry.body,
      attributes: entry.attributes,
      tags: entry.tags,
      occurredAt: entry.occurredAt,
    );
  }

  Future<KnowledgeEntry> undo() async {
    final current = state.value;
    if (current == null) throw StateError('Knowledge entry is not loaded.');
    if (_mutationActive) {
      throw StateError('This entry is already being saved.');
    }
    _mutationActive = true;
    _mutationRevision++;
    state = AsyncValue.data(current.copyWith(isSaving: true, error: null));
    try {
      final restored = await ref
          .read(knowledgeServiceProvider)
          .undoEntry(entryId, expectedVersion: current.entry.version);
      state = AsyncValue.data(KnowledgeEntryState(entry: restored));
      await cacheKnowledgeEntry(ref, restored);
      await _publishEntryChange(restored, current.entry.topicId);
      return restored;
    } catch (error) {
      state = AsyncValue.data(
        current.copyWith(
          isSaving: false,
          error: 'Could not undo the last change: $error',
        ),
      );
      rethrow;
    } finally {
      _mutationActive = false;
    }
  }

  Future<void> delete() async {
    final current = state.value;
    if (current == null) throw StateError('Knowledge entry is not loaded.');
    if (_mutationActive) {
      throw StateError('This entry is already being saved.');
    }
    _mutationActive = true;
    _mutationRevision++;
    state = AsyncValue.data(current.copyWith(isSaving: true, error: null));
    try {
      await ref
          .read(knowledgeServiceProvider)
          .deleteEntry(entryId, expectedVersion: current.entry.version);
      await ref
          .read(cacheServiceProvider)
          .invalidate(
            knowledgeScopedCacheKey(
              _scope,
              'topic_entries_${current.entry.topicId}',
            ),
          );
      await ref
          .read(knowledgeTopicEntriesProvider(current.entry.topicId).notifier)
          .removeEntry(entryId);
      await ref
          .read(knowledgeTreeProvider.notifier)
          .adjustEntryCounts(topicId: current.entry.topicId, delta: -1);
      await ref.read(cacheServiceProvider).invalidate(_cacheKey);
    } catch (error) {
      state = AsyncValue.data(
        current.copyWith(
          isSaving: false,
          error: 'Could not delete this entry: $error',
        ),
      );
      rethrow;
    } finally {
      _mutationActive = false;
    }
  }

  Future<void> _publishEntryChange(
    KnowledgeEntry updated,
    String previousTopicId,
  ) async {
    await ref
        .read(cacheServiceProvider)
        .invalidate(
          knowledgeScopedCacheKey(_scope, 'topic_entries_$previousTopicId'),
        );
    await ref
        .read(knowledgeTopicEntriesProvider(previousTopicId).notifier)
        .applyEntrySnapshot(updated, previousTopicId: previousTopicId);
    if (previousTopicId != updated.topicId) {
      await ref
          .read(knowledgeTreeProvider.notifier)
          .adjustEntryCounts(topicId: previousTopicId, delta: -1);
      await ref
          .read(knowledgeTreeProvider.notifier)
          .adjustEntryCounts(topicId: updated.topicId, delta: 1);
      await ref
          .read(cacheServiceProvider)
          .invalidate(
            knowledgeScopedCacheKey(_scope, 'topic_entries_${updated.topicId}'),
          );
    }
    await ref.read(knowledgeTreeProvider.notifier).markTopicWrites({
      previousTopicId,
      updated.topicId,
    });
  }
}

bool _sameTags(List<String> left, List<String> right) {
  if (left.length != right.length) return false;
  for (var index = 0; index < left.length; index++) {
    if (left[index] != right[index]) return false;
  }
  return true;
}

bool _sameAttributes(Map<String, dynamic> left, Map<String, dynamic> right) {
  return jsonEncode(left) == jsonEncode(right);
}

final knowledgeEntryProvider = AsyncNotifierProvider.autoDispose
    .family<KnowledgeEntryNotifier, KnowledgeEntryState, String>(
      KnowledgeEntryNotifier.new,
    );

class KnowledgeSearchNotifier
    extends AsyncNotifier<List<KnowledgeSearchResult>> {
  @override
  Future<List<KnowledgeSearchResult>> build() async => const [];

  Future<void> run(KnowledgeSearchFilter filter) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(
      () => ref.read(knowledgeServiceProvider).search(filter),
    );
  }

  void clear() => state = const AsyncValue.data([]);
}

final knowledgeSearchProvider = AsyncNotifierProvider.autoDispose<
  KnowledgeSearchNotifier,
  List<KnowledgeSearchResult>
>(KnowledgeSearchNotifier.new);
