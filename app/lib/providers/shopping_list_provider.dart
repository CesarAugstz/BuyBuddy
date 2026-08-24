import 'dart:async';
import 'dart:developer' as developer;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/shopping_list_service.dart';
import 'cache_provider.dart';

final shoppingListServiceProvider = Provider<ShoppingListService>((ref) {
  return ShoppingListService();
});

class ShoppingListsNotifier extends AsyncNotifier<List<ShoppingList>> {
  static const _cacheKey = 'shopping_lists';
  static const _recentListCacheKey = 'recent_shopping_list_id';
  static const _cacheTtl = Duration(days: 7);

  @override
  Future<List<ShoppingList>> build() async {
    final cached = await _readCache();
    if (cached != null) {
      unawaited(Future.microtask(_refreshInBackground));
      return cached;
    }
    return _fetchRemoteLists();
  }

  Future<List<ShoppingList>?> _readCache() async {
    final cache = ref.read(cacheServiceProvider);
    final cached = await cache.get<List<dynamic>>(
      _cacheKey,
      allowExpired: true,
    );
    if (cached == null) return null;
    return cached
        .map(
          (item) =>
              ShoppingList.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList();
  }

  Future<List<ShoppingList>> _fetchRemoteLists() async {
    final lists = await ref.read(shoppingListServiceProvider).getLists();
    await _cacheLists(lists);
    return lists;
  }

  Future<void> _refreshInBackground() async {
    try {
      final lists = await _fetchRemoteLists();
      if (ref.mounted) {
        state = AsyncValue.data(lists);
      }
    } catch (error, stackTrace) {
      developer.log(
        'Failed to refresh cached shopping lists',
        name: 'ShoppingListsNotifier',
        error: error,
        stackTrace: stackTrace,
      );
    }
  }

  Future<void> _cacheLists(List<ShoppingList> lists) async {
    final cache = ref.read(cacheServiceProvider);
    await cache.set(
      _cacheKey,
      lists.map((e) => e.toJson()).toList(),
      ttl: _cacheTtl,
      persistToDisk: true,
    );
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetchRemoteLists);
  }

  Future<void> invalidateCache() async {
    final cache = ref.read(cacheServiceProvider);
    await cache.invalidate(_cacheKey);
  }

  Future<void> markListRecentlyUsed(String listId) async {
    try {
      await ref
          .read(cacheServiceProvider)
          .set(
            _recentListCacheKey,
            listId,
            ttl: const Duration(days: 365),
            persistToDisk: true,
          );
      ref.invalidate(recentShoppingListProvider);
    } catch (error, stackTrace) {
      developer.log(
        'Failed to remember recently used shopping list',
        name: 'ShoppingListsNotifier',
        error: error,
        stackTrace: stackTrace,
      );
    }
  }

  Future<void> applyDetailSnapshot(ShoppingList detail) async {
    final lists = state.value;
    if (lists == null) return;

    final index = lists.indexWhere((list) => list.id == detail.id);
    if (index < 0) return;

    final updatedLists = [...lists];
    updatedLists[index] = lists[index].copyWith(
      title: detail.title,
      description: detail.description,
      updatedAt: detail.updatedAt,
      itemCount: detail.items.length,
      checkedCount: detail.items.where((item) => item.isChecked).length,
    );
    state = AsyncValue.data(updatedLists);
    await _cacheLists(updatedLists);
  }

  Future<ShoppingList> createList(String title, {String? description}) async {
    final service = ref.read(shoppingListServiceProvider);
    final newList = await service.createList(title, description: description);
    await invalidateCache();
    await refresh();
    return newList;
  }

  Future<void> deleteList(String id) async {
    final service = ref.read(shoppingListServiceProvider);
    await service.deleteList(id);
    final cache = ref.read(cacheServiceProvider);
    await cache.invalidate('shopping_list_$id');
    final recentListId = await cache.get<String>(
      _recentListCacheKey,
      allowExpired: true,
    );
    if (recentListId == id) {
      await cache.invalidate(_recentListCacheKey);
    }
    await invalidateCache();
    await refresh();
    ref.invalidate(recentShoppingListProvider);
  }
}

final shoppingListsProvider =
    AsyncNotifierProvider<ShoppingListsNotifier, List<ShoppingList>>(() {
      return ShoppingListsNotifier();
    });

ShoppingList? selectRecentShoppingList(
  List<ShoppingList> lists,
  String? recentListId,
) {
  if (lists.isEmpty) return null;

  if (recentListId != null) {
    for (final list in lists) {
      if (list.id == recentListId) return list;
    }
  }

  return lists.reduce(
    (current, candidate) =>
        candidate.updatedAt.isAfter(current.updatedAt) ? candidate : current,
  );
}

final recentShoppingListProvider = FutureProvider<ShoppingList?>((ref) async {
  final lists = await ref.watch(shoppingListsProvider.future);
  final cache = ref.read(cacheServiceProvider);
  final recentListId = await cache.get<String>(
    ShoppingListsNotifier._recentListCacheKey,
    allowExpired: true,
  );
  final recentList = selectRecentShoppingList(lists, recentListId);

  if (recentList != null && recentList.id != recentListId) {
    await cache.set(
      ShoppingListsNotifier._recentListCacheKey,
      recentList.id,
      ttl: const Duration(days: 365),
      persistToDisk: true,
    );
  }

  return recentList;
});

class ShoppingListDetailNotifier extends AsyncNotifier<ShoppingList> {
  ShoppingListDetailNotifier(this.listId);

  final String listId;
  final Map<String, bool> _desiredChecked = {};
  final Map<String, bool> _confirmedChecked = {};
  final Map<String, Future<void>> _syncOperations = {};
  int _mutationRevision = 0;

  String get _cacheKey => 'shopping_list_$listId';

  @override
  Future<ShoppingList> build() async {
    final cached = await _readCache();
    if (cached != null) {
      unawaited(Future.microtask(_refreshInBackground));
      return cached;
    }
    return _fetchRemoteList();
  }

  Future<ShoppingList?> _readCache() async {
    final cached = await ref
        .read(cacheServiceProvider)
        .get<Map<String, dynamic>>(_cacheKey, allowExpired: true);
    return cached == null ? null : ShoppingList.fromJson(cached);
  }

  Future<ShoppingList> _fetchRemoteList() async {
    final list = await ref.read(shoppingListServiceProvider).getList(listId);
    await _cacheList(list);
    return list;
  }

  Future<void> _refreshInBackground() async {
    try {
      final list = await _fetchRemoteListWhenIdle();
      if (!ref.mounted) return;
      state = AsyncValue.data(list);
      await _cacheList(list);
      await ref.read(shoppingListsProvider.notifier).applyDetailSnapshot(list);
    } catch (error, stackTrace) {
      developer.log(
        'Failed to refresh cached shopping list $listId',
        name: 'ShoppingListDetailNotifier',
        error: error,
        stackTrace: stackTrace,
      );
    }
  }

  Future<void> refresh() async {
    final previous = state.value;
    try {
      final list = await _fetchRemoteListWhenIdle();
      state = AsyncValue.data(list);
      await _cacheList(list);
      await ref.read(shoppingListsProvider.notifier).applyDetailSnapshot(list);
    } catch (error, stackTrace) {
      if (previous == null) {
        state = AsyncValue.error(error, stackTrace);
      }
      rethrow;
    }
  }

  Future<ShoppingList> _fetchRemoteListWhenIdle() async {
    while (true) {
      final pendingOperations = _syncOperations.values.toList();
      for (final operation in pendingOperations) {
        try {
          await operation;
        } catch (error, stackTrace) {
          developer.log(
            'Pending item synchronization failed before list refresh',
            name: 'ShoppingListDetailNotifier',
            error: error,
            stackTrace: stackTrace,
          );
        }
      }

      final revision = _mutationRevision;
      final list = await ref.read(shoppingListServiceProvider).getList(listId);
      if (_syncOperations.isEmpty && revision == _mutationRevision) {
        return list;
      }
    }
  }

  Future<void> toggleItem(String itemId) async {
    final list = state.value;
    if (list == null) {
      throw StateError('Shopping list is not loaded');
    }

    final item = list.items.firstWhere((candidate) => candidate.id == itemId);
    _mutationRevision++;
    _confirmedChecked.putIfAbsent(itemId, () => item.isChecked);
    _desiredChecked[itemId] = !item.isChecked;

    await _applyCheckedState(itemId, !item.isChecked);

    final existingOperation = _syncOperations[itemId];
    if (existingOperation != null) return existingOperation;

    final keepAlive = ref.keepAlive();
    late final Future<void> operation;
    operation = _syncItem(itemId).whenComplete(() {
      if (identical(_syncOperations[itemId], operation)) {
        _syncOperations.remove(itemId);
      }
      keepAlive.close();
    });
    _syncOperations[itemId] = operation;
    return operation;
  }

  Future<void> _syncItem(String itemId) async {
    while (_desiredChecked.containsKey(itemId)) {
      final target = _desiredChecked[itemId]!;
      ShoppingListItem serverItem;
      try {
        serverItem = await ref
            .read(shoppingListServiceProvider)
            .updateItem(listId, itemId, isChecked: target);
      } catch (_) {
        final confirmed = _confirmedChecked.remove(itemId);
        _desiredChecked.remove(itemId);
        if (confirmed != null) {
          await _applyCheckedState(itemId, confirmed);
        }
        rethrow;
      }

      _confirmedChecked[itemId] = serverItem.isChecked;
      if (_desiredChecked[itemId] == target) {
        _desiredChecked.remove(itemId);
        _confirmedChecked.remove(itemId);
        await _applyServerItem(serverItem);
        if (!_desiredChecked.containsKey(itemId)) {
          return;
        }
      }
    }
  }

  Future<void> _applyCheckedState(String itemId, bool isChecked) async {
    final list = state.value;
    if (list == null) return;

    final items =
        list.items
            .map(
              (item) =>
                  item.id == itemId
                      ? item.copyWith(
                        isChecked: isChecked,
                        updatedAt: DateTime.now(),
                      )
                      : item,
            )
            .toList();
    await _applyList(
      list.copyWith(
        items: items,
        itemCount: items.length,
        checkedCount: items.where((item) => item.isChecked).length,
        updatedAt: DateTime.now(),
      ),
    );
  }

  Future<void> _applyServerItem(ShoppingListItem serverItem) async {
    final list = state.value;
    if (list == null) return;

    final items =
        list.items
            .map((item) => item.id == serverItem.id ? serverItem : item)
            .toList();
    await _applyList(
      list.copyWith(
        items: items,
        itemCount: items.length,
        checkedCount: items.where((item) => item.isChecked).length,
        updatedAt: serverItem.updatedAt,
      ),
    );
  }

  Future<void> _applyList(ShoppingList list) async {
    state = AsyncValue.data(list);
    await Future.wait([
      _cacheList(list),
      ref.read(shoppingListsProvider.notifier).applyDetailSnapshot(list),
    ]);
  }

  Future<void> _cacheList(ShoppingList list) {
    return ref
        .read(cacheServiceProvider)
        .set(
          _cacheKey,
          list.toJson(),
          ttl: const Duration(days: 7),
          persistToDisk: true,
        );
  }
}

final shoppingListDetailProvider = AsyncNotifierProvider.autoDispose
    .family<ShoppingListDetailNotifier, ShoppingList, String>(
      ShoppingListDetailNotifier.new,
    );

final itemSuggestionsProvider = FutureProvider.autoDispose
    .family<List<String>, String>((ref, query) async {
      if (query.isEmpty) return [];
      final service = ref.read(shoppingListServiceProvider);
      return service.getSuggestions(query);
    });

final userSearchProvider = FutureProvider.autoDispose
    .family<List<SearchUser>, String>((ref, email) async {
      if (email.length < 3) return [];
      final service = ref.read(shoppingListServiceProvider);
      return service.searchUsers(email);
    });

final listSharesProvider = FutureProvider.autoDispose
    .family<List<ShoppingListShare>, String>((ref, listId) async {
      final service = ref.read(shoppingListServiceProvider);
      return service.getListShares(listId);
    });

final pendingInvitesProvider =
    FutureProvider.autoDispose<List<ShoppingListShare>>((ref) async {
      final service = ref.read(shoppingListServiceProvider);
      return service.getPendingInvites();
    });
