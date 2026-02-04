import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/shopping_list_service.dart';
import 'cache_provider.dart';

final shoppingListServiceProvider = Provider<ShoppingListService>((ref) {
  return ShoppingListService();
});

class ShoppingListsNotifier extends AsyncNotifier<List<ShoppingList>> {
  static const _cacheKey = 'shopping_lists';

  @override
  Future<List<ShoppingList>> build() async {
    return _fetchLists();
  }

  Future<List<ShoppingList>> _fetchLists({bool forceRefresh = false}) async {
    final cache = ref.read(cacheServiceProvider);

    if (!forceRefresh) {
      final cached = await cache.get<List<dynamic>>(_cacheKey);
      if (cached != null) {
        return cached.map((e) => ShoppingList.fromJson(e)).toList();
      }
    }

    final service = ref.read(shoppingListServiceProvider);
    final lists = await service.getLists();

    await cache.set(
      _cacheKey,
      lists.map((e) => e.toJson()).toList(),
      ttl: const Duration(minutes: 5),
      persistToDisk: true,
    );

    return lists;
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => _fetchLists(forceRefresh: true));
  }

  Future<void> invalidateCache() async {
    final cache = ref.read(cacheServiceProvider);
    await cache.invalidate(_cacheKey);
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
    await invalidateCache();
    await refresh();
  }
}

final shoppingListsProvider =
    AsyncNotifierProvider<ShoppingListsNotifier, List<ShoppingList>>(() {
  return ShoppingListsNotifier();
});

final shoppingListDetailProvider =
    FutureProvider.autoDispose.family<ShoppingList, String>((ref, listId) async {
  final service = ref.read(shoppingListServiceProvider);
  return service.getList(listId);
});

final itemSuggestionsProvider =
    FutureProvider.autoDispose.family<List<String>, String>((ref, query) async {
  if (query.isEmpty) return [];
  final service = ref.read(shoppingListServiceProvider);
  return service.getSuggestions(query);
});

final userSearchProvider =
    FutureProvider.autoDispose.family<List<SearchUser>, String>((ref, email) async {
  if (email.length < 3) return [];
  final service = ref.read(shoppingListServiceProvider);
  return service.searchUsers(email);
});

final listSharesProvider =
    FutureProvider.autoDispose.family<List<ShoppingListShare>, String>((ref, listId) async {
  final service = ref.read(shoppingListServiceProvider);
  return service.getListShares(listId);
});

final pendingInvitesProvider =
    FutureProvider.autoDispose<List<ShoppingListShare>>((ref) async {
  final service = ref.read(shoppingListServiceProvider);
  return service.getPendingInvites();
});
