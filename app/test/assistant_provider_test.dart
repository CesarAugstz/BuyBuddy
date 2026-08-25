import 'dart:async';
import 'dart:convert';

import 'package:buybuddy/providers/cache_provider.dart';
import 'package:buybuddy/providers/chat_provider.dart';
import 'package:buybuddy/providers/knowledge_chat_provider.dart';
import 'package:buybuddy/services/cache_service.dart';
import 'package:buybuddy/services/shopping_assistant_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    await CacheService().clearAllCache();
  });

  test(
    'receipt and knowledge chat providers keep state and cache independent',
    () async {
      final cache = CacheService.testing();
      final receiptApi = _FakeReceiptAssistantApi(
        answer: 'Receipt-only answer',
        conversationId: 'receipt-conversation',
      );
      final knowledgeApi = _FakeAssistantApi(
        answer: 'Knowledge-only answer',
        conversationId: 'knowledge-conversation',
      );
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          shoppingAssistantServiceProvider.overrideWithValue(receiptApi),
          knowledgeAssistantServiceProvider.overrideWithValue(knowledgeApi),
        ],
      );
      addTearDown(container.dispose);

      final receiptSubscription = container.listen(chatProvider, (_, __) {});
      final knowledgeSubscription = container.listen(
        knowledgeChatProvider,
        (_, __) {},
      );
      addTearDown(receiptSubscription.close);
      addTearDown(knowledgeSubscription.close);
      await _flushEvents();
      await container
          .read(chatProvider.notifier)
          .sendMessage('Where was milk?');
      await container
          .read(knowledgeChatProvider.notifier)
          .sendMessage('Remember Cafe A');

      final receiptState = container.read(chatProvider);
      final knowledgeState = container.read(knowledgeChatProvider);
      expect(receiptState.conversationId, 'receipt-conversation');
      expect(knowledgeState.conversationId, 'knowledge-conversation');
      expect(
        receiptState.messages.map((message) => message.text),
        contains('Receipt-only answer'),
      );
      expect(
        receiptState.messages.map((message) => message.text),
        isNot(contains('Knowledge-only answer')),
      );
      expect(
        knowledgeState.messages.map((message) => message.text),
        contains('Knowledge-only answer'),
      );
      expect(
        knowledgeState.messages.map((message) => message.text),
        isNot(contains('Receipt-only answer')),
      );
      expect(
        await cache.get<String>(receiptAssistantConversationCacheKey),
        'receipt-conversation',
      );
      expect(
        await cache.get<String>(knowledgeAssistantConversationCacheKey),
        'knowledge-conversation',
      );

      final preferences = await SharedPreferences.getInstance();
      final receiptEntry = CacheEntry.fromJson(
        jsonDecode(
          preferences.getString('cache_$receiptAssistantConversationCacheKey')!,
        ),
      );
      final knowledgeEntry = CacheEntry.fromJson(
        jsonDecode(
          preferences.getString(
            'cache_$knowledgeAssistantConversationCacheKey',
          )!,
        ),
      );
      expect(receiptEntry.ttl, assistantConversationCacheTTL);
      expect(knowledgeEntry.ttl, assistantConversationCacheTTL);

      await container.read(chatProvider.notifier).clearChat();

      expect(container.read(chatProvider).conversationId, isNull);
      expect(
        container.read(knowledgeChatProvider).conversationId,
        'knowledge-conversation',
      );
      expect(
        await cache.get<String>(receiptAssistantConversationCacheKey),
        isNull,
      );
      expect(
        await cache.get<String>(knowledgeAssistantConversationCacheKey),
        'knowledge-conversation',
      );
    },
  );

  test(
    'retry during in-flight work preserves messages and makes no new call',
    () async {
      final api = _RetryAssistantApi();
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(CacheService.testing()),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(chatProvider, (_, __) {});
      addTearDown(subscription.close);
      await _flushEvents();

      final notifier = container.read(chatProvider.notifier);
      await notifier.sendMessage('First question');
      final errorMessage = container
          .read(chatProvider)
          .messages
          .singleWhere((message) => message.isError);

      final inFlight = notifier.sendMessage('Question in flight');
      await api.secondRequestStarted.future;
      final stateBeforeRetry = container.read(chatProvider);

      await notifier.retryMessage(errorMessage);

      expect(api.callCount, 2);
      expect(identical(container.read(chatProvider), stateBeforeRetry), isTrue);
      expect(container.read(chatProvider).messages, stateBeforeRetry.messages);

      api.secondResponse.complete(
        _successResponse('Completed answer', 'receipt-conversation'),
      );
      await inFlight;
    },
  );

  test(
    'disposing during a delayed request does not write disposed state',
    () async {
      final api = _DelayedAssistantApi();
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(CacheService.testing()),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      final subscription = container.listen(chatProvider, (_, __) {});
      await _flushEvents();

      final send = container
          .read(chatProvider.notifier)
          .sendMessage('Delayed question');
      await api.requestStarted.future;
      subscription.close();
      container.dispose();

      api.response.complete(
        _successResponse('Late answer', 'late-conversation'),
      );

      await expectLater(send, completes);
    },
  );

  test(
    'disposing during a delayed cache write does not write disposed state',
    () async {
      final cache = _DelayedSetCache();
      final api = _FakeReceiptAssistantApi(
        answer: 'Answer before cache',
        conversationId: 'delayed-cache-conversation',
      );
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      final subscription = container.listen(chatProvider, (_, __) {});
      await _flushEvents();

      final send = container
          .read(chatProvider.notifier)
          .sendMessage('Cache this answer');
      await cache.setStarted.future;
      subscription.close();
      container.dispose();

      cache.releaseSet.complete();

      await expectLater(send, completes);
    },
  );

  test('reopening during a request receives its completed turn', () async {
    final api = _DelayedAssistantApi();
    final container = ProviderContainer(
      overrides: [
        cacheServiceProvider.overrideWithValue(CacheService.testing()),
        shoppingAssistantServiceProvider.overrideWithValue(api),
      ],
    );
    addTearDown(container.dispose);
    var subscription = container.listen(chatProvider, (_, __) {});
    await _flushEvents();

    final send = container
        .read(chatProvider.notifier)
        .sendMessage('Question while navigating');
    await api.requestStarted.future;
    subscription.close();
    await _flushEvents();
    subscription = container.listen(chatProvider, (_, __) {});
    addTearDown(subscription.close);

    api.response.complete(
      _successResponse('Answer after reopening', 'navigation-conversation'),
    );
    await send;

    expect(api.callCount, 1);
    expect(
      container.read(chatProvider).messages.map((message) => message.text),
      contains('Answer after reopening'),
    );
  });

  test(
    'one reopen after a completed background request restores its turn',
    () async {
      final api = _HistoryTrackingAssistantApi();
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(CacheService.testing()),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      addTearDown(container.dispose);
      var subscription = container.listen(chatProvider, (_, __) {});
      await _flushEvents();

      final send = container
          .read(chatProvider.notifier)
          .sendMessage('Question before leaving');
      await api.requestStarted.future;
      subscription.close();
      api.completeRequest();
      await send;
      await _flushEvents();

      subscription = container.listen(chatProvider, (_, __) {});
      addTearDown(subscription.close);
      await _waitUntil(
        () =>
            api.historyCallCount > 0 &&
            container
                .read(chatProvider)
                .messages
                .any((message) => message.text == 'Background answer'),
      );

      expect(api.callCount, 1);
      expect(api.historyCallCount, 1);
    },
  );

  test(
    'both providers restore expired conversation IDs and renew their TTL',
    () async {
      final staleReceiptEntry = CacheEntry(
        data: 'stale-receipt-conversation',
        timestamp: DateTime.now().subtract(const Duration(days: 1)),
        ttl: const Duration(minutes: 5),
      );
      final staleKnowledgeEntry = CacheEntry(
        data: 'stale-knowledge-conversation',
        timestamp: DateTime.now().subtract(const Duration(days: 1)),
        ttl: const Duration(minutes: 5),
      );
      SharedPreferences.setMockInitialValues({
        'cache_$receiptAssistantConversationCacheKey': jsonEncode(
          staleReceiptEntry.toJson(),
        ),
        'cache_$knowledgeAssistantConversationCacheKey': jsonEncode(
          staleKnowledgeEntry.toJson(),
        ),
      });
      final cache = CacheService.testing();
      final receiptApi = _HistoryAssistantApi(
        expectedConversationId: 'stale-receipt-conversation',
        answer: 'Restored receipt turn',
      );
      final knowledgeApi = _HistoryAssistantApi(
        expectedConversationId: 'stale-knowledge-conversation',
        answer: 'Restored knowledge turn',
      );
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(cache),
          shoppingAssistantServiceProvider.overrideWithValue(receiptApi),
          knowledgeAssistantServiceProvider.overrideWithValue(knowledgeApi),
        ],
      );
      addTearDown(container.dispose);
      final receiptSubscription = container.listen(chatProvider, (_, __) {});
      final knowledgeSubscription = container.listen(
        knowledgeChatProvider,
        (_, __) {},
      );
      addTearDown(receiptSubscription.close);
      addTearDown(knowledgeSubscription.close);

      await _waitUntil(
        () =>
            container
                .read(chatProvider)
                .messages
                .any((message) => message.text == 'Restored receipt turn') &&
            container
                .read(knowledgeChatProvider)
                .messages
                .any((message) => message.text == 'Restored knowledge turn'),
      );

      expect(receiptApi.requestedConversationId, 'stale-receipt-conversation');
      expect(
        knowledgeApi.requestedConversationId,
        'stale-knowledge-conversation',
      );
      final preferences = await SharedPreferences.getInstance();
      for (final key in [
        receiptAssistantConversationCacheKey,
        knowledgeAssistantConversationCacheKey,
      ]) {
        final renewed = CacheEntry.fromJson(
          jsonDecode(preferences.getString('cache_$key')!),
        );
        expect(renewed.ttl, assistantConversationCacheTTL);
        expect(renewed.isExpired, isFalse);
      }
    },
  );

  test(
    'successful answer containing failed is not rendered as an error',
    () async {
      final api = _FakeReceiptAssistantApi(
        answer: 'The failed deployment was recorded in your receipt note.',
        conversationId: 'receipt-conversation',
      );
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(CacheService.testing()),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(chatProvider, (_, __) {});
      addTearDown(subscription.close);
      await _flushEvents();

      await container
          .read(chatProvider.notifier)
          .sendMessage('Tell me about the failed deployment');

      final answer = container.read(chatProvider).messages.last;
      expect(answer.text, contains('failed deployment'));
      expect(answer.isError, isFalse);
      expect(answer.originalQuestion, isNull);
    },
  );

  test(
    'retryable 503 response exposes retry without deleting user turn',
    () async {
      final api = _SequenceReceiptAssistantApi([
        const AssistantChatResponse(
          answer: 'Assistant classification is temporarily unavailable.',
          conversationId: '',
          isError: true,
          retryable: true,
          turnMayHaveBeenPersisted: false,
        ),
        _successResponse('Retry succeeded', 'receipt-conversation'),
      ]);
      final container = ProviderContainer(
        overrides: [
          cacheServiceProvider.overrideWithValue(CacheService.testing()),
          shoppingAssistantServiceProvider.overrideWithValue(api),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(chatProvider, (_, __) {});
      addTearDown(subscription.close);
      await _flushEvents();

      final notifier = container.read(chatProvider.notifier);
      await notifier.sendMessage('Where did I buy milk?');
      final failedTurn = container.read(chatProvider).messages.last;
      expect(failedTurn.isError, isTrue);
      expect(failedTurn.isRetryable, isTrue);

      await notifier.retryMessage(failedTurn);

      final messages = container.read(chatProvider).messages;
      expect(
        messages.where(
          (message) =>
              message.isUser && message.text == 'Where did I buy milk?',
        ),
        hasLength(1),
      );
      expect(
        messages.map((message) => message.text),
        contains('Retry succeeded'),
      );
      expect(messages.any((message) => message.isError), isFalse);
      expect(api.callCount, 2);
    },
  );
}

Future<void> _flushEvents() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

Future<void> _waitUntil(bool Function() condition) async {
  for (var attempt = 0; attempt < 100; attempt++) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
  fail('Condition was not met before timeout');
}

class _FakeAssistantApi implements AssistantChatApi {
  _FakeAssistantApi({required this.answer, required this.conversationId});

  final String answer;
  final String conversationId;

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) async {
    return _successResponse(answer, this.conversationId);
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async {
    return [];
  }
}

class _FakeReceiptAssistantApi extends _FakeAssistantApi
    implements ReceiptAssistantApi {
  _FakeReceiptAssistantApi({
    required super.answer,
    required super.conversationId,
  });

  @override
  Future<List<String>> getSuggestions() async => const [
    'Where did I buy {item}?',
  ];
}

class _RetryAssistantApi implements ReceiptAssistantApi {
  final secondRequestStarted = Completer<void>();
  final secondResponse = Completer<AssistantChatResponse>();
  var callCount = 0;

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) {
    callCount++;
    if (callCount == 1) {
      return Future.value(
        const AssistantChatResponse(
          answer: 'First request could not be completed.',
          conversationId: 'receipt-conversation',
          isError: true,
          retryable: true,
          turnMayHaveBeenPersisted: false,
        ),
      );
    }
    secondRequestStarted.complete();
    return secondResponse.future;
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async => [];

  @override
  Future<List<String>> getSuggestions() async => [];
}

class _DelayedAssistantApi implements ReceiptAssistantApi {
  final requestStarted = Completer<void>();
  final response = Completer<AssistantChatResponse>();
  var callCount = 0;

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) {
    callCount++;
    requestStarted.complete();
    return response.future;
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async => [];

  @override
  Future<List<String>> getSuggestions() async => [];
}

class _DelayedSetCache extends CacheService {
  _DelayedSetCache() : super.testing();

  final setStarted = Completer<void>();
  final releaseSet = Completer<void>();

  @override
  Future<void> set(
    String key,
    dynamic data, {
    Duration? ttl,
    bool persistToDisk = false,
  }) async {
    setStarted.complete();
    await releaseSet.future;
    await super.set(key, data, ttl: ttl, persistToDisk: persistToDisk);
  }
}

class _HistoryTrackingAssistantApi implements ReceiptAssistantApi {
  final requestStarted = Completer<void>();
  final _response = Completer<AssistantChatResponse>();
  var callCount = 0;
  var historyCallCount = 0;
  var _completed = false;

  void completeRequest() {
    _completed = true;
    _response.complete(
      _successResponse('Background answer', 'background-conversation'),
    );
  }

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) {
    callCount++;
    requestStarted.complete();
    return _response.future;
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async {
    historyCallCount++;
    if (!_completed || conversationId != 'background-conversation') return [];
    return [
      {
        'role': 'user',
        'content': 'Question before leaving',
        'createdAt': '2026-08-25T12:00:00Z',
      },
      {
        'role': 'assistant',
        'content': 'Background answer',
        'createdAt': '2026-08-25T12:00:01Z',
      },
    ];
  }

  @override
  Future<List<String>> getSuggestions() async => [];
}

class _HistoryAssistantApi implements ReceiptAssistantApi {
  _HistoryAssistantApi({
    required this.expectedConversationId,
    required this.answer,
  });

  final String expectedConversationId;
  final String answer;
  String? requestedConversationId;

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) async => throw UnimplementedError();

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async {
    requestedConversationId = conversationId;
    if (conversationId != expectedConversationId) return [];
    return [
      {
        'role': 'assistant',
        'content': answer,
        'createdAt': '2026-08-25T12:00:00Z',
      },
    ];
  }

  @override
  Future<List<String>> getSuggestions() async => [];
}

class _SequenceReceiptAssistantApi implements ReceiptAssistantApi {
  _SequenceReceiptAssistantApi(this.responses);

  final List<AssistantChatResponse> responses;
  var callCount = 0;

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) async {
    return responses[callCount++];
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async => [];

  @override
  Future<List<String>> getSuggestions() async => [];
}

AssistantChatResponse _successResponse(String answer, String conversationId) {
  return AssistantChatResponse(
    answer: answer,
    conversationId: conversationId,
    isError: false,
    retryable: false,
    turnMayHaveBeenPersisted: true,
  );
}
