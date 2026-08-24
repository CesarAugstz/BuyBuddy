import 'dart:convert';
import 'dart:async';

import 'package:buybuddy/providers/shopping_list_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/shopping_list_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('shopping list cache serialization preserves list and item state', () {
    final createdAt = DateTime.utc(2026, 8, 20, 12);
    final updatedAt = DateTime.utc(2026, 8, 24, 15, 30);
    final list = ShoppingList(
      id: 'list-1',
      title: 'Groceries',
      description: 'Weekly shopping',
      ownerId: 'user-1',
      owner: ShoppingListOwner(
        id: 'user-1',
        name: 'Test User',
        email: 'test@example.com',
        photoUrl: 'https://example.com/photo.png',
      ),
      createdAt: createdAt,
      updatedAt: updatedAt,
      items: [
        ShoppingListItem(
          id: 'item-1',
          listId: 'list-1',
          name: 'Milk',
          quantity: 2,
          unit: 'un',
          isChecked: true,
          sortOrder: 1,
          createdAt: createdAt,
          updatedAt: updatedAt,
        ),
      ],
      itemCount: 1,
      checkedCount: 1,
      isShared: true,
      isOwner: true,
      sharedWithCount: 2,
    );

    final restored = ShoppingList.fromJson(list.toJson());

    expect(restored.id, list.id);
    expect(restored.owner?.email, list.owner?.email);
    expect(restored.createdAt, createdAt);
    expect(restored.updatedAt, updatedAt);
    expect(restored.itemCount, 1);
    expect(restored.checkedCount, 1);
    expect(restored.sharedWithCount, 2);
    expect(restored.items.single.name, 'Milk');
    expect(restored.items.single.isChecked, isTrue);
    expect(restored.items.single.updatedAt, updatedAt);
  });

  test('recent shopping list selection prefers the last opened list', () {
    final older = _buildList(
      'older',
    ).copyWith(updatedAt: DateTime.utc(2026, 8, 20));
    final newer = _buildList(
      'newer',
    ).copyWith(updatedAt: DateTime.utc(2026, 8, 24));

    expect(selectRecentShoppingList([newer, older], 'older')?.id, 'older');
  });

  test('recent shopping list selection falls back to newest updated list', () {
    final older = _buildList(
      'older',
    ).copyWith(updatedAt: DateTime.utc(2026, 8, 20));
    final newer = _buildList(
      'newer',
    ).copyWith(updatedAt: DateTime.utc(2026, 8, 24));

    expect(selectRecentShoppingList([older, newer], null)?.id, 'newer');
    expect(selectRecentShoppingList([older, newer], 'deleted')?.id, 'newer');
    expect(selectRecentShoppingList([], null), isNull);
  });

  test('item toggle updates immediately and keeps server result', () async {
    SharedPreferences.setMockInitialValues({});
    final service = _FakeShoppingListService(_buildList('optimistic-success'));
    final container = ProviderContainer(
      overrides: [shoppingListServiceProvider.overrideWithValue(service)],
    );
    addTearDown(container.dispose);

    final provider = shoppingListDetailProvider('optimistic-success');
    final subscription = container.listen(provider, (_, __) {});
    addTearDown(subscription.close);
    await container.read(provider.future);

    final operation = container.read(provider.notifier).toggleItem('item-1');

    expect(container.read(provider).value!.items.single.isChecked, isTrue);

    await service.waitForUpdateCount(1);
    service.completeNextUpdate(isChecked: true);
    await operation;

    expect(container.read(provider).value!.items.single.isChecked, isTrue);
  });

  test('item toggle rolls back when synchronization fails', () async {
    SharedPreferences.setMockInitialValues({});
    final service = _FakeShoppingListService(_buildList('optimistic-failure'));
    final container = ProviderContainer(
      overrides: [shoppingListServiceProvider.overrideWithValue(service)],
    );
    addTearDown(container.dispose);

    final provider = shoppingListDetailProvider('optimistic-failure');
    final subscription = container.listen(provider, (_, __) {});
    addTearDown(subscription.close);
    await container.read(provider.future);

    final operation = container.read(provider.notifier).toggleItem('item-1');

    expect(container.read(provider).value!.items.single.isChecked, isTrue);

    final failure = expectLater(operation, throwsException);
    await service.waitForUpdateCount(1);
    service.failNextUpdate();
    await failure;

    expect(container.read(provider).value!.items.single.isChecked, isFalse);
  });

  test('toggle during sync completion starts another server update', () async {
    SharedPreferences.setMockInitialValues({});
    final service = _FakeShoppingListService(_buildList('completion-window'));
    final container = ProviderContainer(
      overrides: [shoppingListServiceProvider.overrideWithValue(service)],
    );
    addTearDown(container.dispose);

    final provider = shoppingListDetailProvider('completion-window');
    late Future<void> secondOperation;
    var triggeredSecondToggle = false;
    final subscription = container.listen(provider, (_, next) {
      final item = next.value?.items.single;
      if (!triggeredSecondToggle &&
          service.updateCallCount == 1 &&
          item?.isChecked == true) {
        triggeredSecondToggle = true;
        secondOperation = container
            .read(provider.notifier)
            .toggleItem('item-1');
      }
    });
    addTearDown(subscription.close);
    await container.read(provider.future);

    final firstOperation = container
        .read(provider.notifier)
        .toggleItem('item-1');
    await service.waitForUpdateCount(1);
    service.completeNextUpdate(isChecked: true);
    await service.waitForUpdateCount(2);
    service.completeNextUpdate(isChecked: false);

    await Future.wait([firstOperation, secondOperation]);

    expect(triggeredSecondToggle, isTrue);
    expect(service.updateCallCount, 2);
    expect(container.read(provider).value!.items.single.isChecked, isFalse);
  });

  test(
    'refresh waits for an optimistic toggle before replacing state',
    () async {
      SharedPreferences.setMockInitialValues({});
      final service = _FakeShoppingListService(
        _buildList('refresh-during-sync'),
      );
      final container = ProviderContainer(
        overrides: [shoppingListServiceProvider.overrideWithValue(service)],
      );
      addTearDown(container.dispose);

      final provider = shoppingListDetailProvider('refresh-during-sync');
      final subscription = container.listen(provider, (_, __) {});
      addTearDown(subscription.close);
      await container.read(provider.future);

      final toggle = container.read(provider.notifier).toggleItem('item-1');
      await service.waitForUpdateCount(1);
      final refresh = container.read(provider.notifier).refresh();

      expect(container.read(provider).value!.items.single.isChecked, isTrue);

      service.completeNextUpdate(isChecked: true);
      await Future.wait([toggle, refresh]);

      expect(container.read(provider).value!.items.single.isChecked, isTrue);
    },
  );

  test('expired cache is returned once and removed from disk', () async {
    final entry = CacheEntry(
      data: {'value': 42},
      timestamp: DateTime.now().subtract(const Duration(days: 2)),
      ttl: const Duration(days: 1),
    );
    SharedPreferences.setMockInitialValues({
      'cache_expired-shopping-list': jsonEncode(entry.toJson()),
    });

    final value = await CacheService().get<Map<String, dynamic>>(
      'expired-shopping-list',
      allowExpired: true,
    );
    final preferences = await SharedPreferences.getInstance();

    expect(value, {'value': 42});
    expect(preferences.containsKey('cache_expired-shopping-list'), isFalse);
  });
}

ShoppingList _buildList(String id) {
  final now = DateTime.utc(2026, 8, 24);
  return ShoppingList(
    id: id,
    title: 'Groceries',
    ownerId: 'user-1',
    createdAt: now,
    updatedAt: now,
    items: [
      ShoppingListItem(
        id: 'item-1',
        listId: id,
        name: 'Milk',
        quantity: 1,
        unit: 'un',
        isChecked: false,
        sortOrder: 0,
        createdAt: now,
        updatedAt: now,
      ),
    ],
    itemCount: 1,
    checkedCount: 0,
  );
}

class _FakeShoppingListService extends ShoppingListService {
  _FakeShoppingListService(this.list);

  ShoppingList list;
  final List<Completer<ShoppingListItem>> _pendingUpdates = [];
  final Map<int, Completer<void>> _updateCountWaiters = {};
  int updateCallCount = 0;

  @override
  Future<List<ShoppingList>> getLists() async => [list];

  @override
  Future<ShoppingList> getList(String id) async => list;

  @override
  Future<ShoppingListItem> updateItem(
    String listId,
    String itemId, {
    String? name,
    double? quantity,
    String? unit,
    bool? isChecked,
    int? sortOrder,
  }) {
    updateCallCount++;
    final waiter = _updateCountWaiters.remove(updateCallCount);
    if (waiter != null && !waiter.isCompleted) {
      waiter.complete();
    }
    final update = Completer<ShoppingListItem>();
    _pendingUpdates.add(update);
    return update.future;
  }

  Future<void> waitForUpdateCount(int count) {
    if (updateCallCount >= count) return Future.value();
    return (_updateCountWaiters[count] ??= Completer<void>()).future;
  }

  void completeNextUpdate({required bool isChecked}) {
    final serverItem = list.items.single.copyWith(
      isChecked: isChecked,
      updatedAt: DateTime.utc(2026, 8, 24, updateCallCount),
    );
    list = list.copyWith(
      items: [serverItem],
      checkedCount: isChecked ? 1 : 0,
      updatedAt: serverItem.updatedAt,
    );
    _pendingUpdates.removeAt(0).complete(serverItem);
  }

  void failNextUpdate() {
    _pendingUpdates.removeAt(0).completeError(Exception('network unavailable'));
  }
}
