import 'dart:convert';

import 'package:buybuddy/services/cache_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
  });

  test('retainExpiredOnDisk returns stale data without evicting it', () async {
    final entry = CacheEntry(
      data: {'value': 42},
      timestamp: DateTime.now().subtract(const Duration(days: 2)),
      ttl: const Duration(days: 1),
    );
    SharedPreferences.setMockInitialValues({
      'cache_retained-expired': jsonEncode(entry.toJson()),
    });

    final value = await CacheService().get<Map<String, dynamic>>(
      'retained-expired',
      allowExpired: true,
      retainExpiredOnDisk: true,
    );
    final preferences = await SharedPreferences.getInstance();

    expect(value, {'value': 42});
    expect(preferences.containsKey('cache_retained-expired'), isTrue);
  });

  test('long-lived entries remain available beyond the default TTL', () async {
    final entry = CacheEntry(
      data: 'conversation-123',
      timestamp: DateTime.now().subtract(const Duration(days: 30)),
      ttl: const Duration(days: 90),
    );
    SharedPreferences.setMockInitialValues({
      'cache_long-lived-conversation': jsonEncode(entry.toJson()),
    });

    final value = await CacheService().get<String>('long-lived-conversation');

    expect(value, 'conversation-123');
  });
}
