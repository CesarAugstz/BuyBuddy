import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../config/theme.dart';
import '../providers/chat_provider.dart';

class AssistantChatPage extends StatefulWidget {
  const AssistantChatPage({
    super.key,
    required this.title,
    required this.state,
    required this.suggestions,
    required this.inputHint,
    required this.onSend,
    required this.onRetry,
    required this.onClear,
    this.assistantIcon = Icons.smart_toy_outlined,
  });

  final String title;
  final ChatState state;
  final List<String> suggestions;
  final String inputHint;
  final Future<void> Function(String message) onSend;
  final Future<void> Function(ChatMessage message) onRetry;
  final Future<void> Function() onClear;
  final IconData assistantIcon;

  @override
  State<AssistantChatPage> createState() => _AssistantChatPageState();
}

class _AssistantChatPageState extends State<AssistantChatPage> {
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  final _messageFocusNode = FocusNode();
  final _keyboardFocusNode = FocusNode();

  @override
  void didUpdateWidget(covariant AssistantChatPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.state.messages.length != widget.state.messages.length) {
      _scrollToBottom();
    }
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    _messageFocusNode.dispose();
    _keyboardFocusNode.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _sendMessage() async {
    final text = _messageController.text.trim();
    if (text.isEmpty || widget.state.isLoading) return;
    _messageController.clear();
    await widget.onSend(text);
  }

  void _useSuggestion(String suggestion) {
    _messageController.text = suggestion;
    final placeholder = RegExp(r'\{[^{}]+\}').firstMatch(suggestion);
    _messageController.selection =
        placeholder == null
            ? TextSelection.collapsed(offset: suggestion.length)
            : TextSelection(
              baseOffset: placeholder.start,
              extentOffset: placeholder.end,
            );
    _messageFocusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      key: Key('assistant-chat-${widget.title}'),
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text(
          widget.title,
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.delete_outline),
            onPressed: widget.state.isLoading ? null : widget.onClear,
            tooltip: 'Clear chat',
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.all(16),
              itemCount: widget.state.messages.length,
              itemBuilder:
                  (context, index) =>
                      _buildMessageBubble(widget.state.messages[index]),
            ),
          ),
          if (widget.state.isLoading) _buildLoadingIndicator(),
          _buildInputArea(),
        ],
      ),
    );
  }

  Widget _buildLoadingIndicator() {
    return Padding(
      padding: const EdgeInsets.all(8),
      child: Row(
        children: [
          const SizedBox(width: 16),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppTheme.lightGray,
              borderRadius: BorderRadius.circular(20),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    valueColor: AlwaysStoppedAnimation(AppTheme.darkGray),
                  ),
                ),
                const SizedBox(width: 8),
                Text('Thinking...', style: TextStyle(color: AppTheme.darkGray)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessageBubble(ChatMessage message) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Row(
        mainAxisAlignment:
            message.isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!message.isUser)
            Container(
              margin: const EdgeInsets.only(right: 8),
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color:
                    message.isError
                        ? Colors.red.withValues(alpha: 0.1)
                        : AppTheme.primaryBlue.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                message.isError ? Icons.error_outline : widget.assistantIcon,
                size: 20,
                color: message.isError ? Colors.red : AppTheme.primaryBlue,
              ),
            ),
          Flexible(
            child: Column(
              crossAxisAlignment:
                  message.isUser
                      ? CrossAxisAlignment.end
                      : CrossAxisAlignment.start,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 12,
                  ),
                  decoration: BoxDecoration(
                    color:
                        message.isUser
                            ? AppTheme.primaryBlue
                            : message.isError
                            ? Colors.red.shade50
                            : AppTheme.lightGray,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child:
                      message.isUser
                          ? SelectableText(
                            message.text,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 15,
                            ),
                          )
                          : MarkdownBody(
                            data: message.text,
                            selectable: true,
                            styleSheet: MarkdownStyleSheet(
                              p: TextStyle(
                                color:
                                    message.isError
                                        ? Colors.red.shade900
                                        : AppTheme.nearBlack,
                                fontSize: 15,
                              ),
                              strong: TextStyle(
                                color: AppTheme.nearBlack,
                                fontWeight: FontWeight.bold,
                              ),
                              code: const TextStyle(
                                backgroundColor: Colors.black12,
                                fontFamily: 'monospace',
                                fontSize: 14,
                              ),
                              listBullet: TextStyle(
                                color: AppTheme.nearBlack,
                                fontSize: 15,
                              ),
                            ),
                          ),
                ),
                if (message.isError &&
                    message.isRetryable &&
                    message.originalQuestion != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: TextButton.icon(
                      onPressed:
                          widget.state.isLoading
                              ? null
                              : () => widget.onRetry(message),
                      icon: const Icon(Icons.refresh, size: 18),
                      label: const Text('Retry'),
                    ),
                  ),
              ],
            ),
          ),
          if (message.isUser)
            Container(
              margin: const EdgeInsets.only(left: 8),
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppTheme.primaryBlue.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(Icons.person, size: 20, color: AppTheme.primaryBlue),
            ),
        ],
      ),
    );
  }

  Widget _buildInputArea() {
    return Container(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).padding.bottom + 16,
      ),
      decoration: BoxDecoration(
        color: Colors.white,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (widget.suggestions.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: SizedBox(
                height: 40,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: widget.suggestions.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (context, index) {
                    final suggestion = widget.suggestions[index];
                    return ActionChip(
                      avatar: const Icon(Icons.auto_awesome, size: 16),
                      label: Text(suggestion),
                      onPressed: () => _useSuggestion(suggestion),
                    );
                  },
                ),
              ),
            ),
          Row(
            children: [
              Expanded(
                child: KeyboardListener(
                  focusNode: _keyboardFocusNode,
                  onKeyEvent: (event) {
                    if (event is KeyDownEvent &&
                        event.logicalKey == LogicalKeyboardKey.enter &&
                        HardwareKeyboard.instance.isControlPressed) {
                      _sendMessage();
                    }
                  },
                  child: TextField(
                    key: const Key('assistant-message-field'),
                    controller: _messageController,
                    focusNode: _messageFocusNode,
                    decoration: InputDecoration(
                      hintText: widget.inputHint,
                      filled: true,
                      fillColor: AppTheme.lightGray,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(25),
                        borderSide: BorderSide.none,
                      ),
                    ),
                    maxLines: null,
                    keyboardType: TextInputType.multiline,
                    textCapitalization: TextCapitalization.sentences,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              DecoratedBox(
                decoration: BoxDecoration(
                  color: AppTheme.primaryBlue,
                  shape: BoxShape.circle,
                ),
                child: IconButton(
                  key: const Key('assistant-send-button'),
                  icon: const Icon(Icons.send),
                  color: Colors.white,
                  onPressed: widget.state.isLoading ? null : _sendMessage,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
