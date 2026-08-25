import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/knowledge_chat_provider.dart';
import 'assistant_chat_page.dart';

class KnowledgeAssistantPage extends ConsumerWidget {
  const KnowledgeAssistantPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(knowledgeChatProvider);
    final suggestions = ref.watch(knowledgeAssistantSuggestionsProvider);
    return AssistantChatPage(
      key: const Key('knowledge-assistant-page'),
      title: 'Knowledge Assistant',
      state: state,
      suggestions: suggestions,
      inputHint: 'Remember, find, change, forget, or organize notes...',
      onSend: ref.read(knowledgeChatProvider.notifier).sendMessage,
      onRetry: ref.read(knowledgeChatProvider.notifier).retryMessage,
      onClear: ref.read(knowledgeChatProvider.notifier).clearChat,
      assistantIcon: Icons.psychology_alt_outlined,
    );
  }
}
