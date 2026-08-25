import 'dart:convert';

import 'package:buybuddy/services/auth_service.dart';
import 'package:buybuddy/services/knowledge_assistant_service.dart';
import 'package:buybuddy/services/shopping_assistant_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

void main() {
  test('shopping assistant uses receipt-only endpoint contracts', () async {
    final requests = <http.Request>[];
    final service = ShoppingAssistantService(
      authService: _AssistantFakeAuthService(),
      client: MockClient((request) async {
        requests.add(request);
        if (request.url.path.endsWith('/suggestions')) {
          return http.Response(
            jsonEncode({
              'suggestions': ['Where did I buy {item}?'],
            }),
            200,
          );
        }
        if (request.method == 'GET') {
          return http.Response('[]', 200);
        }
        return http.Response(
          jsonEncode({
            'answer': 'Receipt answer',
            'conversationId': 'receipt-conversation',
          }),
          200,
        );
      }),
    );

    final answer = await service.askQuestion(
      'Latest milk?',
      conversationId: 'receipt-old',
    );
    await service.getConversationHistory('receipt-conversation');
    await service.getSuggestions();

    expect(requests[0].url.path, endsWith('/api/assistant/ask'));
    expect(
      jsonDecode(requests[0].body),
      containsPair('conversationId', 'receipt-old'),
    );
    expect(
      requests[1].url.path,
      endsWith('/api/assistant/conversation/receipt-conversation'),
    );
    expect(requests[2].url.path, endsWith('/api/assistant/suggestions'));
    expect(answer.answer, 'Receipt answer');
    expect(answer.isError, isFalse);
    for (final request in requests) {
      expect(request.headers['Authorization'], 'Bearer test-token');
      expect(request.url.path, isNot(contains('/knowledge/assistant')));
    }
  });

  test(
    'knowledge assistant uses dedicated knowledge endpoint contracts',
    () async {
      final requests = <http.Request>[];
      final service = KnowledgeAssistantService(
        authService: _AssistantFakeAuthService(),
        client: MockClient((request) async {
          requests.add(request);
          if (request.method == 'GET') {
            return http.Response('[]', 200);
          }
          return http.Response(
            jsonEncode({
              'answer': 'Knowledge answer',
              'conversationId': 'knowledge-conversation',
            }),
            200,
          );
        }),
      );

      await service.askQuestion('Remember this');
      await service.getConversationHistory('knowledge-conversation');

      expect(requests[0].url.path, endsWith('/api/knowledge/assistant/ask'));
      expect(
        requests[1].url.path,
        endsWith(
          '/api/knowledge/assistant/conversation/knowledge-conversation',
        ),
      );
      for (final request in requests) {
        expect(request.headers['Authorization'], 'Bearer test-token');
        expect(request.url.path, isNot(equals('/api/assistant/ask')));
      }
    },
  );

  test('successful answer prose is never interpreted as an error', () async {
    final service = ShoppingAssistantService(
      authService: _AssistantFakeAuthService(),
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({
            'answer': 'The failed deployment happened yesterday.',
            'conversationId': 'receipt-conversation',
          }),
          200,
        ),
      ),
    );

    final response = await service.askQuestion('What failed?');

    expect(response.answer, contains('failed deployment'));
    expect(response.isError, isFalse);
    expect(response.retryable, isFalse);
    expect(response.turnMayHaveBeenPersisted, isTrue);
  });

  test('classification 503 is an explicit retryable failure', () async {
    final service = ShoppingAssistantService(
      authService: _AssistantFakeAuthService(),
      client: MockClient(
        (_) async => http.Response(
          jsonEncode({
            'message':
                'assistant request could not be classified; please retry',
          }),
          503,
        ),
      ),
    );

    final response = await service.askQuestion('Latest milk?');

    expect(response.isError, isTrue);
    expect(response.retryable, isTrue);
    expect(response.turnMayHaveBeenPersisted, isFalse);
    expect(response.answer, contains('could not be classified'));
  });

  test('400 and 401 failures are not retryable', () async {
    for (final status in [400, 401]) {
      final service = ShoppingAssistantService(
        authService: _AssistantFakeAuthService(),
        client: MockClient(
          (_) async => http.Response(
            jsonEncode({'message': 'Request rejected'}),
            status,
          ),
        ),
      );

      final response = await service.askQuestion('Question');

      expect(response.isError, isTrue, reason: 'status $status');
      expect(response.retryable, isFalse, reason: 'status $status');
      expect(
        response.turnMayHaveBeenPersisted,
        isFalse,
        reason: 'status $status',
      );
    }
  });

  test('service exceptions become explicit retryable failures', () async {
    final service = ShoppingAssistantService(
      authService: _AssistantFakeAuthService(),
      client: MockClient((_) async => throw Exception('network unavailable')),
    );

    final response = await service.askQuestion('Question');

    expect(response.isError, isTrue);
    expect(response.retryable, isTrue);
    expect(response.turnMayHaveBeenPersisted, isTrue);
    expect(response.answer, contains('Please try again'));
  });
}

class _AssistantFakeAuthService implements AuthService {
  @override
  Future<String?> getApiToken() async => 'test-token';

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
