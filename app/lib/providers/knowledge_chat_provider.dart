import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/knowledge_assistant_service.dart';
import '../services/shopping_assistant_service.dart';
import 'chat_provider.dart';

const knowledgeAssistantConversationCacheKey =
    'knowledge_assistant_last_conversation_id';

final knowledgeAssistantServiceProvider = Provider<AssistantChatApi>((ref) {
  return KnowledgeAssistantService();
});

final knowledgeAssistantSuggestionsProvider = Provider<List<String>>((ref) {
  return const [
    'Remember a recommendation for me',
    'Add an entry to my diary',
    'Find my notes about {topic}',
    'Change a saved note',
    'Forget a note',
    'Organize my Inbox',
  ];
});

class KnowledgeChatNotifier extends AssistantChatNotifier {
  @override
  AssistantChatApi get assistantService =>
      ref.read(knowledgeAssistantServiceProvider);

  @override
  String get conversationCacheKey => knowledgeAssistantConversationCacheKey;

  @override
  ChatMessage welcomeMessage() => ChatMessage(
    text:
        'Hello! I can help with your notes and personal knowledge. Try asking me to:\n\n'
        '• Remember a recommendation\n'
        '• Add a diary entry\n'
        '• Find or change a saved note\n'
        '• Forget a note safely\n'
        '• Organize a knowledge topic',
    isUser: false,
    timestamp: DateTime.now(),
  );
}

final knowledgeChatProvider =
    NotifierProvider.autoDispose<KnowledgeChatNotifier, ChatState>(
      KnowledgeChatNotifier.new,
    );
