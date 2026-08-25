import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/shopping_assistant_service.dart';
import 'cache_provider.dart';

const receiptAssistantConversationCacheKey =
    'receipt_assistant_last_conversation_id';
const _legacyAssistantConversationCacheKey = 'last_conversation_id';
const assistantConversationCacheTTL = Duration(days: 90);

final shoppingAssistantServiceProvider = Provider<ReceiptAssistantApi>((ref) {
  return ShoppingAssistantService();
});

final assistantSuggestionsProvider = FutureProvider.autoDispose<List<String>>((
  ref,
) {
  return ref.read(shoppingAssistantServiceProvider).getSuggestions();
});

class ChatMessage {
  const ChatMessage({
    required this.text,
    required this.isUser,
    required this.timestamp,
    this.isError = false,
    this.isRetryable = false,
    this.turnMayHaveBeenPersisted = false,
    this.originalQuestion,
  });

  final String text;
  final bool isUser;
  final DateTime timestamp;
  final bool isError;
  final bool isRetryable;
  final bool turnMayHaveBeenPersisted;
  final String? originalQuestion;

  Map<String, dynamic> toJson() => {
    'text': text,
    'isUser': isUser,
    'timestamp': timestamp.toIso8601String(),
    'isError': isError,
    'isRetryable': isRetryable,
    'turnMayHaveBeenPersisted': turnMayHaveBeenPersisted,
    'originalQuestion': originalQuestion,
  };

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
    text: json['text']?.toString() ?? '',
    isUser: json['isUser'] == true,
    timestamp:
        DateTime.tryParse(json['timestamp']?.toString() ?? '') ??
        DateTime.now(),
    isError: json['isError'] == true,
    isRetryable: json['isRetryable'] == true,
    turnMayHaveBeenPersisted: json['turnMayHaveBeenPersisted'] == true,
    originalQuestion: json['originalQuestion']?.toString(),
  );
}

class ChatState {
  const ChatState({
    this.messages = const [],
    this.conversationId,
    this.isLoading = false,
  });

  final List<ChatMessage> messages;
  final String? conversationId;
  final bool isLoading;

  ChatState copyWith({
    List<ChatMessage>? messages,
    String? conversationId,
    bool? isLoading,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      conversationId: conversationId ?? this.conversationId,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

abstract class AssistantChatNotifier extends Notifier<ChatState> {
  AssistantChatApi get assistantService;
  String get conversationCacheKey;
  String? get legacyConversationCacheKey => null;
  ChatMessage welcomeMessage();

  @override
  ChatState build() {
    Future<void>.microtask(_loadLastConversation);
    return ChatState(messages: [welcomeMessage()]);
  }

  Future<void> _loadLastConversation() async {
    if (!ref.mounted) return;
    final cache = ref.read(cacheServiceProvider);
    var lastConversationId = await cache.get<String>(
      conversationCacheKey,
      allowExpired: true,
      retainExpiredOnDisk: true,
    );
    if (!ref.mounted) return;
    final legacyKey = legacyConversationCacheKey;
    if ((lastConversationId == null || lastConversationId.isEmpty) &&
        legacyKey != null) {
      lastConversationId = await cache.get<String>(
        legacyKey,
        allowExpired: true,
        retainExpiredOnDisk: true,
      );
      if (!ref.mounted) return;
      if (lastConversationId != null && lastConversationId.isNotEmpty) {
        await cache.set(
          conversationCacheKey,
          lastConversationId,
          ttl: assistantConversationCacheTTL,
          persistToDisk: true,
        );
        if (!ref.mounted) return;
        await cache.invalidate(legacyKey);
        if (!ref.mounted) return;
      }
    }

    if (lastConversationId == null || lastConversationId.isEmpty) return;
    await cache.set(
      conversationCacheKey,
      lastConversationId,
      ttl: assistantConversationCacheTTL,
      persistToDisk: true,
    );
    if (!ref.mounted) return;
    state = state.copyWith(isLoading: true);
    try {
      final history = await assistantService.getConversationHistory(
        lastConversationId,
      );
      if (!ref.mounted) return;
      if (history.isNotEmpty) {
        final messages =
            history.map((message) {
              return ChatMessage(
                text: message['content']?.toString() ?? '',
                isUser: message['role'] == 'user',
                timestamp:
                    DateTime.tryParse(message['createdAt']?.toString() ?? '') ??
                    DateTime.now(),
              );
            }).toList();
        state = ChatState(
          messages: messages,
          conversationId: lastConversationId,
        );
        return;
      }
    } catch (error) {
      if (!ref.mounted) return;
      debugPrint('Error loading assistant conversation: $error');
    }
    state = ChatState(messages: [welcomeMessage()]);
  }

  Future<void> sendMessage(String text) async {
    await _sendMessage(text, appendUserMessage: true);
  }

  Future<void> _sendMessage(
    String text, {
    required bool appendUserMessage,
  }) async {
    final question = text.trim();
    if (question.isEmpty || !ref.mounted || state.isLoading) return;

    final keepAliveLink = ref.keepAlive();
    state = state.copyWith(
      messages:
          appendUserMessage
              ? [
                ...state.messages,
                ChatMessage(
                  text: question,
                  isUser: true,
                  timestamp: DateTime.now(),
                ),
              ]
              : state.messages,
      isLoading: true,
    );
    try {
      final response = await assistantService.askQuestion(
        question,
        conversationId: state.conversationId,
      );
      if (!ref.mounted) return;
      final newConversationId =
          response.conversationId.isNotEmpty
              ? response.conversationId
              : state.conversationId;
      if (newConversationId != null && newConversationId.isNotEmpty) {
        await ref
            .read(cacheServiceProvider)
            .set(
              conversationCacheKey,
              newConversationId,
              ttl: assistantConversationCacheTTL,
              persistToDisk: true,
            );
        if (!ref.mounted) return;
      }

      state = state.copyWith(
        messages: [
          ...state.messages,
          ChatMessage(
            text: response.answer,
            isUser: false,
            timestamp: DateTime.now(),
            isError: response.isError,
            isRetryable: response.retryable,
            turnMayHaveBeenPersisted: response.turnMayHaveBeenPersisted,
            originalQuestion:
                response.isError && response.retryable ? question : null,
          ),
        ],
        conversationId: newConversationId,
        isLoading: false,
      );
    } catch (error) {
      if (!ref.mounted) return;
      state = state.copyWith(
        messages: [
          ...state.messages,
          ChatMessage(
            text: 'Sorry, I encountered an error: $error',
            isUser: false,
            timestamp: DateTime.now(),
            isError: true,
            isRetryable: true,
            turnMayHaveBeenPersisted: true,
            originalQuestion: question,
          ),
        ],
        isLoading: false,
      );
    } finally {
      keepAliveLink.close();
    }
  }

  Future<void> retryMessage(ChatMessage errorMessage) async {
    final question = errorMessage.originalQuestion;
    if (question == null || !ref.mounted || state.isLoading) return;
    final messages = [...state.messages];
    final errorIndex = messages.indexOf(errorMessage);
    if (errorIndex >= 0) {
      messages.removeAt(errorIndex);
    }
    state = state.copyWith(messages: messages);
    await _sendMessage(
      question,
      appendUserMessage: errorMessage.turnMayHaveBeenPersisted,
    );
  }

  Future<void> clearChat() async {
    if (!ref.mounted) return;
    final cache = ref.read(cacheServiceProvider);
    await cache.invalidate(conversationCacheKey);
    final legacyKey = legacyConversationCacheKey;
    if (legacyKey != null) {
      await cache.invalidate(legacyKey);
    }
    if (!ref.mounted) return;
    state = ChatState(messages: [welcomeMessage()]);
  }
}

class ChatNotifier extends AssistantChatNotifier {
  @override
  AssistantChatApi get assistantService =>
      ref.read(shoppingAssistantServiceProvider);

  @override
  String get conversationCacheKey => receiptAssistantConversationCacheKey;

  @override
  String? get legacyConversationCacheKey =>
      _legacyAssistantConversationCacheKey;

  @override
  ChatMessage welcomeMessage() => ChatMessage(
    text:
        'Hello! I can help only with your receipts and purchase history. Try asking:\n\n'
        '• "How much did I pay for milk last time?"\n'
        '• "Where did I buy bread?"\n'
        '• "Show my purchases from Walmart"\n'
        '• "What was the lowest price of eggs?"',
    isUser: false,
    timestamp: DateTime.now(),
  );
}

final chatProvider = NotifierProvider.autoDispose<ChatNotifier, ChatState>(
  ChatNotifier.new,
);
