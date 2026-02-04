import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

class CacheEntry {
  final dynamic data;
  final DateTime timestamp;
  final Duration ttl;

  CacheEntry({required this.data, required this.timestamp, required this.ttl});

  bool get isExpired => DateTime.now().difference(timestamp) > ttl;

  Map<String, dynamic> toJson() => {
    'data': data,
    'timestamp': timestamp.toIso8601String(),
    'ttlSeconds': ttl.inSeconds,
  };

  factory CacheEntry.fromJson(Map<String, dynamic> json) => CacheEntry(
    data: json['data'],
    timestamp: DateTime.parse(json['timestamp']),
    ttl: Duration(seconds: json['ttlSeconds']),
  );
}

class CacheService {
  static final CacheService _instance = CacheService._internal();
  factory CacheService() => _instance;
  CacheService._internal();

  final Map<String, CacheEntry> _memoryCache = {};
  static const String _cachePrefix = 'cache_';

  static const Duration defaultTTL = Duration(minutes: 5);
  static const Duration receiptsTTL = Duration(minutes: 10);
  static const Duration categoriesTTL = Duration(hours: 1);

  Future<T?> get<T>(String key, {bool checkDisk = true}) async {
    if (_memoryCache.containsKey(key)) {
      final entry = _memoryCache[key]!;
      if (!entry.isExpired) {
        return entry.data as T;
      }
      _memoryCache.remove(key);
    }

    if (checkDisk) {
      final prefs = await SharedPreferences.getInstance();
      final cached = prefs.getString('$_cachePrefix$key');
      if (cached != null) {
        try {
          final entry = CacheEntry.fromJson(jsonDecode(cached));
          if (!entry.isExpired) {
            _memoryCache[key] = entry;
            return entry.data as T;
          }
          await prefs.remove('$_cachePrefix$key');
        } catch (_) {
          await prefs.remove('$_cachePrefix$key');
        }
      }
    }

    return null;
  }

  Future<void> set(String key, dynamic data, {Duration? ttl, bool persistToDisk = false}) async {
    final entry = CacheEntry(
      data: data,
      timestamp: DateTime.now(),
      ttl: ttl ?? defaultTTL,
    );

    _memoryCache[key] = entry;

    if (persistToDisk) {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString('$_cachePrefix$key', jsonEncode(entry.toJson()));
    }
  }

  Future<void> invalidate(String key) async {
    _memoryCache.remove(key);
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('$_cachePrefix$key');
  }

  Future<void> invalidatePattern(String pattern) async {
    final keysToRemove = _memoryCache.keys.where((k) => k.contains(pattern)).toList();
    for (final key in keysToRemove) {
      _memoryCache.remove(key);
    }

    final prefs = await SharedPreferences.getInstance();
    final allKeys = prefs.getKeys();
    for (final key in allKeys) {
      if (key.startsWith(_cachePrefix) && key.contains(pattern)) {
        await prefs.remove(key);
      }
    }
  }

  void clearMemoryCache() {
    _memoryCache.clear();
  }

  Future<void> clearAllCache() async {
    _memoryCache.clear();
    final prefs = await SharedPreferences.getInstance();
    final keysToRemove = prefs.getKeys().where((k) => k.startsWith(_cachePrefix)).toList();
    for (final key in keysToRemove) {
      await prefs.remove(key);
    }
  }
}
