import 'package:buybuddy/pages/knowledge_assistant_page.dart';
import 'package:buybuddy/pages/main_page.dart';
import 'package:buybuddy/pages/shopping_assistant_page.dart';
import 'package:buybuddy/pages/assistant_chat_page.dart';
import 'package:buybuddy/providers/auth_provider.dart';
import 'package:buybuddy/providers/chat_provider.dart';
import 'package:buybuddy/providers/knowledge_chat_provider.dart';
import 'package:buybuddy/providers/shopping_list_provider.dart';
import 'package:buybuddy/services/auth_service.dart';
import 'package:buybuddy/services/shopping_assistant_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('home and drawer expose a distinct Knowledge Assistant', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          currentUserProvider.overrideWithValue(
            UserData(
              email: 'chat@example.test',
              name: 'Chat User',
              photoUrl: '',
            ),
          ),
          recentShoppingListProvider.overrideWith((ref) async => null),
          knowledgeAssistantServiceProvider.overrideWithValue(
            _NavigationAssistantApi(),
          ),
        ],
        child: const MaterialApp(home: MainPage()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Shopping Assistant'), findsOneWidget);
    expect(find.text('Knowledge Assistant'), findsOneWidget);
    expect(find.text('Personal Knowledge'), findsOneWidget);

    await tester.tap(find.byTooltip('Open navigation menu'));
    await tester.pumpAndSettle();

    expect(
      find.widgetWithText(ListTile, 'Knowledge Assistant'),
      findsOneWidget,
    );
    expect(find.widgetWithText(ListTile, 'Personal Knowledge'), findsOneWidget);
    await tester.tap(find.widgetWithText(ListTile, 'Knowledge Assistant'));
    await tester.pumpAndSettle();

    expect(find.byType(KnowledgeAssistantPage), findsOneWidget);
    expect(find.text('Knowledge Assistant'), findsOneWidget);
    expect(find.textContaining('notes and personal knowledge'), findsOneWidget);
    expect(find.text('Shopping Assistant'), findsNothing);
  });

  testWidgets('shopping assistant renders receipt-only guidance', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          shoppingAssistantServiceProvider.overrideWithValue(
            _NavigationReceiptAssistantApi(),
          ),
        ],
        child: const MaterialApp(home: ShoppingAssistantPage()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Shopping Assistant'), findsOneWidget);
    expect(
      find.textContaining('receipts and purchase history'),
      findsOneWidget,
    );
    expect(find.textContaining('notes and personal knowledge'), findsNothing);
    expect(
      find.text('Ask about receipts, purchases, or prices...'),
      findsOneWidget,
    );
  });

  testWidgets('retry is disabled while an assistant request is loading', (
    tester,
  ) async {
    var retryCalls = 0;
    final errorMessage = ChatMessage(
      text: 'Request failed',
      isUser: false,
      timestamp: DateTime(2026),
      isError: true,
      isRetryable: true,
      originalQuestion: 'Try this again',
    );

    await tester.pumpWidget(
      MaterialApp(
        home: AssistantChatPage(
          title: 'Test Assistant',
          state: ChatState(messages: [errorMessage], isLoading: true),
          suggestions: const [],
          inputHint: 'Ask...',
          onSend: (_) async {},
          onRetry: (_) async {
            retryCalls++;
          },
          onClear: () async {},
        ),
      ),
    );

    final retryButton = tester.widget<TextButton>(find.bySubtype<TextButton>());
    expect(retryButton.onPressed, isNull);
    await tester.tap(find.text('Retry'));
    expect(retryCalls, 0);
  });
}

class _NavigationAssistantApi implements AssistantChatApi {
  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) async {
    return const AssistantChatResponse(
      answer: 'Knowledge answer',
      conversationId: 'knowledge-conversation',
      isError: false,
      retryable: false,
      turnMayHaveBeenPersisted: true,
    );
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async {
    return [];
  }
}

class _NavigationReceiptAssistantApi extends _NavigationAssistantApi
    implements ReceiptAssistantApi {
  @override
  Future<List<String>> getSuggestions() async => const [
    'Where did I buy {item}?',
  ];
}
