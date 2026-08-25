import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/chat_provider.dart';
import 'assistant_chat_page.dart';

class ShoppingAssistantPage extends ConsumerWidget {
  const ShoppingAssistantPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(chatProvider);
    final suggestions =
        ref.watch(assistantSuggestionsProvider).value ?? const <String>[];
    return AssistantChatPage(
      key: const Key('shopping-assistant-page'),
      title: 'Shopping Assistant',
      state: state,
      suggestions: suggestions,
      inputHint: 'Ask about receipts, purchases, or prices...',
      onSend: ref.read(chatProvider.notifier).sendMessage,
      onRetry: ref.read(chatProvider.notifier).retryMessage,
      onClear: ref.read(chatProvider.notifier).clearChat,
      assistantIcon: Icons.receipt_long_outlined,
    );
  }
}
