import 'dart:convert';

import 'package:http/http.dart' as http;

import '../config/api_config.dart';
import '../models/knowledge.dart';
import 'auth_service.dart';

DateTime knowledgeLocalCalendarDayBoundary(
  DateTime value, {
  required bool endOfDay,
}) {
  final local = value.toLocal();
  if (!endOfDay) {
    return DateTime(local.year, local.month, local.day);
  }
  return DateTime(
    local.year,
    local.month,
    local.day + 1,
  ).subtract(const Duration(microseconds: 1));
}

abstract interface class KnowledgeApi {
  Future<List<KnowledgeTopicNode>> getTopicTree();
  Future<KnowledgeTopicDetail> getTopic(String id);
  Future<KnowledgeTopic> createTopic({
    required String name,
    String? parentId,
    String description,
  });
  Future<KnowledgeTopic> updateTopic(
    String id, {
    String? name,
    String? description,
    String? parentId,
    bool moveToRoot,
  });
  Future<void> deleteTopic(String id);
  Future<KnowledgeOrganizationResponse> organizeTopic(String id);
  Future<List<KnowledgeEntry>> getTopicEntries(
    String topicId, {
    int limit,
    int offset,
  });
  Future<KnowledgeEntry> getEntry(String id);
  Future<KnowledgeEntry> createEntry({
    required String topicId,
    required String kind,
    required String title,
    required String body,
    required Map<String, dynamic> attributes,
    required List<String> tags,
    DateTime? occurredAt,
  });
  Future<KnowledgeEntry> updateEntry(
    String id, {
    required int expectedVersion,
    String? topicId,
    String? kind,
    String? title,
    String? body,
    Map<String, dynamic>? attributes,
    bool replaceAttributes,
    List<String>? tags,
    DateTime? occurredAt,
    bool clearOccurredAt,
  });
  Future<void> deleteEntry(String id, {required int expectedVersion});
  Future<KnowledgeEntry> undoEntry(String id, {required int expectedVersion});
  Future<List<KnowledgeSearchResult>> search(KnowledgeSearchFilter filter);
}

class KnowledgeApiException implements Exception {
  const KnowledgeApiException(this.message, {required this.statusCode});

  final String message;
  final int statusCode;

  @override
  String toString() => message;
}

class KnowledgeService implements KnowledgeApi {
  KnowledgeService({AuthService? authService, http.Client? client})
    : _authService = authService ?? AuthService(),
      _client = client ?? http.Client();

  final AuthService _authService;
  final http.Client _client;

  Future<Map<String, String>> _headers() async {
    final token = await _authService.getApiToken();
    if (token == null || token.isEmpty) {
      throw const KnowledgeApiException(
        'Please sign in again to access Personal Knowledge.',
        statusCode: 401,
      );
    }
    return {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $token',
    };
  }

  Uri _uri(String path, [Map<String, String?> query = const {}]) {
    final uri = Uri.parse('${ApiConfig.baseUrl}/knowledge$path');
    final parameters = <String, String>{};
    for (final entry in query.entries) {
      final value = entry.value;
      if (value != null && value.isNotEmpty) {
        parameters[entry.key] = value;
      }
    }
    return parameters.isEmpty ? uri : uri.replace(queryParameters: parameters);
  }

  dynamic _decode(http.Response response, Set<int> expectedStatuses) {
    if (!expectedStatuses.contains(response.statusCode)) {
      var message = 'Knowledge request failed (${response.statusCode}).';
      if (response.body.isNotEmpty) {
        try {
          final body = jsonDecode(response.body);
          if (body is Map && body['message'] != null) {
            message = body['message'].toString();
          }
        } catch (_) {}
      }
      throw KnowledgeApiException(message, statusCode: response.statusCode);
    }
    if (response.body.isEmpty) return null;
    return jsonDecode(response.body);
  }

  @override
  Future<List<KnowledgeTopicNode>> getTopicTree() async {
    final response = await _client.get(
      _uri('/topics/tree'),
      headers: await _headers(),
    );
    final data = _decode(response, {200}) as List<dynamic>;
    return data
        .map(
          (item) => KnowledgeTopicNode.fromJson(
            Map<String, dynamic>.from(item as Map),
          ),
        )
        .toList();
  }

  @override
  Future<KnowledgeTopicDetail> getTopic(String id) async {
    final response = await _client.get(
      _uri('/topics/$id'),
      headers: await _headers(),
    );
    return KnowledgeTopicDetail.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<KnowledgeTopic> createTopic({
    required String name,
    String? parentId,
    String description = '',
  }) async {
    final response = await _client.post(
      _uri('/topics'),
      headers: await _headers(),
      body: jsonEncode({
        if (parentId != null) 'parentId': parentId,
        'name': name,
        'description': description,
      }),
    );
    return KnowledgeTopic.fromJson(
      Map<String, dynamic>.from(_decode(response, {201}) as Map),
    );
  }

  @override
  Future<KnowledgeTopic> updateTopic(
    String id, {
    String? name,
    String? description,
    String? parentId,
    bool moveToRoot = false,
  }) async {
    final response = await _client.put(
      _uri('/topics/$id'),
      headers: await _headers(),
      body: jsonEncode({
        if (name != null) 'name': name,
        if (description != null) 'description': description,
        if (parentId != null) 'parentId': parentId,
        if (moveToRoot) 'moveToRoot': true,
      }),
    );
    return KnowledgeTopic.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<void> deleteTopic(String id) async {
    final response = await _client.delete(
      _uri('/topics/$id'),
      headers: await _headers(),
    );
    _decode(response, {204});
  }

  @override
  Future<KnowledgeOrganizationResponse> organizeTopic(String id) async {
    final response = await _client
        .post(_uri('/topics/$id/organize'), headers: await _headers())
        .timeout(const Duration(minutes: 2));
    return KnowledgeOrganizationResponse.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<List<KnowledgeEntry>> getTopicEntries(
    String topicId, {
    int limit = 100,
    int offset = 0,
  }) async {
    final response = await _client.get(
      _uri('/topics/$topicId/entries', {
        'limit': limit.toString(),
        'offset': offset.toString(),
      }),
      headers: await _headers(),
    );
    final data = _decode(response, {200}) as List<dynamic>;
    return data
        .map(
          (item) =>
              KnowledgeEntry.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList();
  }

  @override
  Future<KnowledgeEntry> getEntry(String id) async {
    final response = await _client.get(
      _uri('/entries/$id'),
      headers: await _headers(),
    );
    return KnowledgeEntry.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<KnowledgeEntry> createEntry({
    required String topicId,
    required String kind,
    required String title,
    required String body,
    required Map<String, dynamic> attributes,
    required List<String> tags,
    DateTime? occurredAt,
  }) async {
    final response = await _client.post(
      _uri('/entries'),
      headers: await _headers(),
      body: jsonEncode({
        'topicId': topicId,
        'kind': kind,
        'title': title,
        'body': body,
        'attributes': attributes,
        'tags': tags,
        if (occurredAt != null)
          'occurredAt': occurredAt.toUtc().toIso8601String(),
      }),
    );
    return KnowledgeEntry.fromJson(
      Map<String, dynamic>.from(_decode(response, {201}) as Map),
    );
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
  }) async {
    final response = await _client.put(
      _uri('/entries/$id'),
      headers: await _headers(),
      body: jsonEncode({
        'expectedVersion': expectedVersion,
        if (topicId != null) 'topicId': topicId,
        if (kind != null) 'kind': kind,
        if (title != null) 'title': title,
        if (body != null) 'body': body,
        if (attributes != null) 'attributes': attributes,
        if (replaceAttributes) 'replaceAttributes': true,
        if (tags != null) 'tags': tags,
        if (occurredAt != null)
          'occurredAt': occurredAt.toUtc().toIso8601String(),
        if (clearOccurredAt) 'clearOccurredAt': true,
      }),
    );
    return KnowledgeEntry.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<void> deleteEntry(String id, {required int expectedVersion}) async {
    final response = await _client.delete(
      _uri('/entries/$id', {'expectedVersion': expectedVersion.toString()}),
      headers: await _headers(),
    );
    _decode(response, {204});
  }

  @override
  Future<KnowledgeEntry> undoEntry(
    String id, {
    required int expectedVersion,
  }) async {
    final response = await _client.post(
      _uri('/entries/$id/undo'),
      headers: await _headers(),
      body: jsonEncode({'expectedVersion': expectedVersion}),
    );
    return KnowledgeEntry.fromJson(
      Map<String, dynamic>.from(_decode(response, {200}) as Map),
    );
  }

  @override
  Future<List<KnowledgeSearchResult>> search(
    KnowledgeSearchFilter filter,
  ) async {
    String? calendarDayBoundary(DateTime? value, {required bool endOfDay}) {
      if (value == null) return null;
      return knowledgeLocalCalendarDayBoundary(
        value,
        endOfDay: endOfDay,
      ).toUtc().toIso8601String();
    }

    final response = await _client.get(
      _uri('/search', {
        'q': filter.query.trim(),
        'topicId': filter.topicId,
        'includeChildren': filter.includeChildren ? 'true' : null,
        'kind': filter.kind.trim(),
        'tag': filter.tag.trim(),
        'occurredFrom': calendarDayBoundary(
          filter.occurredFrom,
          endOfDay: false,
        ),
        'occurredTo': calendarDayBoundary(filter.occurredTo, endOfDay: true),
        'limit': filter.limit.clamp(1, 50).toString(),
      }),
      headers: await _headers(),
    );
    final data = _decode(response, {200}) as List<dynamic>;
    return data
        .map(
          (item) => KnowledgeSearchResult.fromJson(
            Map<String, dynamic>.from(item as Map),
          ),
        )
        .toList();
  }
}
