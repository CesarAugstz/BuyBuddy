import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/shopping_assistant_service.dart';
import 'cache_provider.dart';

final shoppingAssistantServiceProvider = Provider<ShoppingAssistantService>((ref) {
  return ShoppingAssistantService();
});

class ChatMessage {
  final String text;
  final bool isUser;
  final DateTime timestamp;
  final bool isError;
  final String? originalQuestion;

  ChatMessage({
    required this.text,
    required this.isUser,
    required this.timestamp,
    this.isError = false,
    this.originalQuestion,
  });

  Map<String, dynamic> toJson() => {
    'text': text,
    'isUser': isUser,
    'timestamp': timestamp.toIso8601String(),
    'isError': isError,
    'originalQuestion': originalQuestion,
  };

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
    text: json['text'],
    isUser: json['isUser'],
    timestamp: DateTime.parse(json['timestamp']),
    isError: json['isError'] ?? false,
    originalQuestion: json['originalQuestion'],
  );
}

class ChatState {
  final List<ChatMessage> messages;
  final String? conversationId;
  final bool isLoading;

  ChatState({
    this.messages = const [],
    this.conversationId,
    this.isLoading = false,
  });

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

class ChatNotifier extends Notifier<ChatState> {
  @override
  ChatState build() {
    _loadLastConversation();
    return ChatState(messages: [_welcomeMessage()]);
  }

  ChatMessage _welcomeMessage() => ChatMessage(
    text: 'Hello! I can help you find information about your purchases. Try asking me:\n\n• "How much did I pay for milk last time?"\n• "Where did I buy bread?"\n• "Show me all my purchases from Walmart"\n• "What was the price of eggs?"',
    isUser: false,
    timestamp: DateTime.now(),
  );

  Future<void> _loadLastConversation() async {
    final cache = ref.read(cacheServiceProvider);
    final lastConversationId = await cache.get<String>('last_conversation_id');

    if (lastConversationId != null && lastConversationId.isNotEmpty) {
      state = state.copyWith(isLoading: true);

      try {
        final service = ref.read(shoppingAssistantServiceProvider);
        final history = await service.getConversationHistory(lastConversationId);

        if (history.isNotEmpty) {
          final messages = history.map((m) => ChatMessage(
            text: m['content'] ?? '',
            isUser: m['role'] == 'user',
            timestamp: DateTime.parse(m['createdAt'] ?? DateTime.now().toIso8601String()),
          )).toList();

          state = ChatState(
            messages: messages,
            conversationId: lastConversationId,
            isLoading: false,
          );
          return;
        }
      } catch (e) {
        debugPrint('Error loading conversation: $e');
      }
    }

    state = ChatState(messages: [_welcomeMessage()], isLoading: false);
  }

  Future<void> sendMessage(String text) async {
    if (text.trim().isEmpty) return;

    final userMessage = ChatMessage(
      text: text,
      isUser: true,
      timestamp: DateTime.now(),
    );

    state = state.copyWith(
      messages: [...state.messages, userMessage],
      isLoading: true,
    );

    try {
      final service = ref.read(shoppingAssistantServiceProvider);
      final response = await service.askQuestion(text, conversationId: state.conversationId);

      final newConversationId = response['conversationId'] ?? state.conversationId;
      
      if (newConversationId != null && newConversationId.isNotEmpty) {
        final cache = ref.read(cacheServiceProvider);
        await cache.set('last_conversation_id', newConversationId, persistToDisk: true);
      }

      final answer = response['answer'] ?? 'No response';
      final isError = answer.toLowerCase().contains('error') ||
          answer.toLowerCase().contains('failed') ||
          answer.toLowerCase().contains('please log in');

      final assistantMessage = ChatMessage(
        text: answer,
        isUser: false,
        timestamp: DateTime.now(),
        isError: isError,
        originalQuestion: isError ? text : null,
      );

      state = state.copyWith(
        messages: [...state.messages, assistantMessage],
        conversationId: newConversationId,
        isLoading: false,
      );
    } catch (e) {
      final errorMessage = ChatMessage(
        text: 'Sorry, I encountered an error: ${e.toString()}',
        isUser: false,
        timestamp: DateTime.now(),
        isError: true,
        originalQuestion: text,
      );

      state = state.copyWith(
        messages: [...state.messages, errorMessage],
        isLoading: false,
      );
    }
  }

  Future<void> retryMessage(ChatMessage errorMessage) async {
    if (errorMessage.originalQuestion == null) return;

    final messages = state.messages.where((m) => m != errorMessage).toList();
    state = state.copyWith(messages: messages);
    
    await sendMessage(errorMessage.originalQuestion!);
  }

  void clearChat() {
    state = ChatState(
      messages: [_welcomeMessage()],
      conversationId: null,
      isLoading: false,
    );
  }
}

final chatProvider = NotifierProvider<ChatNotifier, ChatState>(() {
  return ChatNotifier();
});
