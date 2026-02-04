import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/receipt_service.dart';
import '../services/cache_service.dart';
import 'cache_provider.dart';

final receiptServiceProvider = Provider<ReceiptService>((ref) {
  return ReceiptService();
});

class ReceiptsNotifier extends AsyncNotifier<List<dynamic>> {
  static const _cacheKey = 'receipts_list';

  @override
  Future<List<dynamic>> build() async {
    return _fetchReceipts();
  }

  Future<List<dynamic>> _fetchReceipts({bool forceRefresh = false}) async {
    final cache = ref.read(cacheServiceProvider);
    
    if (!forceRefresh) {
      final cached = await cache.get<List<dynamic>>(_cacheKey);
      if (cached != null) {
        return cached;
      }
    }

    final receiptService = ref.read(receiptServiceProvider);
    final receipts = await receiptService.getReceipts();
    
    await cache.set(
      _cacheKey,
      receipts,
      ttl: CacheService.receiptsTTL,
      persistToDisk: true,
    );
    
    return receipts;
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => _fetchReceipts(forceRefresh: true));
  }

  Future<void> invalidateCache() async {
    final cache = ref.read(cacheServiceProvider);
    await cache.invalidate(_cacheKey);
  }

  Future<bool> deleteReceipt(String receiptId) async {
    final receiptService = ref.read(receiptServiceProvider);
    final success = await receiptService.deleteReceipt(receiptId);
    if (success) {
      await invalidateCache();
      await refresh();
    }
    return success;
  }

  Future<bool> saveReceipt(Map<String, dynamic> receiptData) async {
    final receiptService = ref.read(receiptServiceProvider);
    final success = await receiptService.saveReceipt(receiptData);
    if (success) {
      await invalidateCache();
      await refresh();
    }
    return success;
  }
}

final receiptsProvider = AsyncNotifierProvider<ReceiptsNotifier, List<dynamic>>(() {
  return ReceiptsNotifier();
});
