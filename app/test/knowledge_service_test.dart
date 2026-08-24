import 'dart:convert';

import 'package:buybuddy/models/knowledge.dart';
import 'package:buybuddy/services/auth_service.dart';
import 'package:buybuddy/services/knowledge_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

void main() {
  test('knowledge create serializes a local occurrence date as UTC', () async {
    late Map<String, dynamic> requestBody;
    final service = KnowledgeService(
      authService: _FakeAuthService(),
      client: MockClient((request) async {
        requestBody = Map<String, dynamic>.from(
          jsonDecode(request.body) as Map,
        );
        return http.Response(
          jsonEncode(_entryJson(occurredAt: '2026-08-24T15:30:00-04:00')),
          201,
        );
      }),
    );

    final entry = await service.createEntry(
      topicId: 'topic-1',
      kind: 'diary',
      title: 'Local date',
      body: 'A locally selected date.',
      attributes: const {},
      tags: const [],
      occurredAt: DateTime(2026, 8, 24, 15, 30),
    );

    final wireDate = requestBody['occurredAt'] as String;
    expect(wireDate, matches(RegExp(r'(Z|[+-]\d{2}:\d{2})$')));
    expect(wireDate, endsWith('Z'));
    expect(entry.occurredAt, DateTime.parse('2026-08-24T15:30:00-04:00'));
  });

  test(
    'knowledge update sends an empty attribute replacement and UTC date',
    () async {
      late Map<String, dynamic> requestBody;
      final service = KnowledgeService(
        authService: _FakeAuthService(),
        client: MockClient((request) async {
          requestBody = Map<String, dynamic>.from(
            jsonDecode(request.body) as Map,
          );
          return http.Response(
            jsonEncode(_entryJson(occurredAt: '2026-08-25T00:00:00Z')),
            200,
          );
        }),
      );

      await service.updateEntry(
        'entry-1',
        expectedVersion: 1,
        attributes: const {},
        replaceAttributes: true,
        occurredAt: DateTime(2026, 8, 25),
      );

      expect(requestBody['attributes'], isEmpty);
      expect(requestBody['replaceAttributes'], isTrue);
      final wireDate = requestBody['occurredAt'] as String;
      expect(wireDate, matches(RegExp(r'(Z|[+-]\d{2}:\d{2})$')));
      expect(wireDate, endsWith('Z'));
    },
  );

  test(
    'knowledge search sends inclusive local calendar days as RFC3339 instants',
    () async {
      late Uri requestUri;
      final service = KnowledgeService(
        authService: _FakeAuthService(),
        client: MockClient((request) async {
          requestUri = request.url;
          return http.Response('[]', 200);
        }),
      );
      final selectedDay = DateTime(2026, 3, 8, 12);

      await service.search(
        KnowledgeSearchFilter(
          occurredFrom: selectedDay,
          occurredTo: selectedDay,
        ),
      );

      final expectedStart = DateTime(2026, 3, 8);
      final expectedEnd = DateTime(
        2026,
        3,
        9,
      ).subtract(const Duration(microseconds: 1));
      final occurredFrom = requestUri.queryParameters['occurredFrom']!;
      final occurredTo = requestUri.queryParameters['occurredTo']!;

      expect(occurredFrom, expectedStart.toUtc().toIso8601String());
      expect(occurredTo, expectedEnd.toUtc().toIso8601String());
      expect(occurredFrom, endsWith('Z'));
      expect(occurredTo, endsWith('Z'));
      expect(DateTime.parse(occurredFrom).isUtc, isTrue);
      expect(DateTime.parse(occurredTo).isUtc, isTrue);
      expect(
        DateTime.parse(occurredTo).difference(DateTime.parse(occurredFrom)),
        expectedEnd.toUtc().difference(expectedStart.toUtc()),
      );

      if (DateTime.now().timeZoneOffset != Duration.zero) {
        expect(occurredFrom, isNot(DateTime.utc(2026, 3, 8).toIso8601String()));
      }
    },
  );

  test('local calendar boundary helper includes the final microsecond', () {
    final selectedDay = DateTime(2026, 8, 24, 18);

    expect(
      knowledgeLocalCalendarDayBoundary(selectedDay, endOfDay: false),
      DateTime(2026, 8, 24),
    );
    expect(
      knowledgeLocalCalendarDayBoundary(selectedDay, endOfDay: true),
      DateTime(2026, 8, 25).subtract(const Duration(microseconds: 1)),
    );
  });

  test('organize topic posts to explicit endpoint and parses result', () async {
    late http.Request captured;
    final service = KnowledgeService(
      authService: _FakeAuthService(),
      client: MockClient((request) async {
        captured = request;
        return http.Response(
          jsonEncode({
            'status': 'organized',
            'topic': {
              'id': 'topic-1',
              'name': 'Notes',
              'depth': 0,
              'pendingWriteCount': 0,
              'lastOrganizedAt': '2026-08-24T15:00:00Z',
              'createdAt': '2026-08-20T12:00:00Z',
              'updatedAt': '2026-08-24T15:00:00Z',
            },
            'result': {
              'operationsApplied': 1,
              'changedEntryIds': ['entry-1'],
              'createdTopicIds': <String>[],
              'affectedTopicIds': ['topic-1'],
            },
          }),
          200,
        );
      }),
    );

    final result = await service.organizeTopic('topic-1');

    expect(captured.method, 'POST');
    expect(captured.url.path, endsWith('/knowledge/topics/topic-1/organize'));
    expect(captured.headers['Authorization'], 'Bearer test-token');
    expect(result.status, 'organized');
    expect(result.result.operationsApplied, 1);
    expect(result.result.changedEntryIds, ['entry-1']);
  });
}

Map<String, dynamic> _entryJson({required String occurredAt}) {
  return {
    'id': 'entry-1',
    'topicId': 'topic-1',
    'kind': 'diary',
    'title': 'Local date',
    'body': 'A locally selected date.',
    'attributes': <String, dynamic>{},
    'tags': <String>[],
    'occurredAt': occurredAt,
    'source': 'manual',
    'status': 'active',
    'version': 2,
    'createdAt': '2026-08-24T12:00:00Z',
    'updatedAt': '2026-08-24T12:00:00Z',
  };
}

class _FakeAuthService implements AuthService {
  @override
  Future<String?> getApiToken() async => 'test-token';

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
