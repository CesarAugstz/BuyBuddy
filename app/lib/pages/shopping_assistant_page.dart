import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../config/theme.dart';
import '../services/shopping_assistant_service.dart';

class ShoppingAssistantPage extends StatefulWidget {
  const ShoppingAssistantPage({super.key});

  @override
  State<ShoppingAssistantPage> createState() => _ShoppingAssistantPageState();
}

class _ShoppingAssistantPageState extends State<ShoppingAssistantPage> {
  final _assistantService = ShoppingAssistantService();
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  final List<ChatMessage> _messages = [];
  bool _isLoading = false;
  String? _conversationId;

  @override
  void initState() {
    super.initState();
    _loadLastConversation();
  }

  Future<void> _loadLastConversation() async {
    final prefs = await SharedPreferences.getInstance();
    final lastConversationId = prefs.getString('last_conversation_id');

    if (lastConversationId != null && lastConversationId.isNotEmpty) {
      setState(() {
        _conversationId = lastConversationId;
        _isLoading = true;
      });

      try {
        final history = await _assistantService.getConversationHistory(lastConversationId);
        
        if (history.isNotEmpty) {
          setState(() {
            _messages.clear();
            for (var message in history) {
              _messages.add(ChatMessage(
                text: message['content'] ?? '',
                isUser: message['role'] == 'user',
                timestamp: DateTime.parse(message['createdAt'] ?? DateTime.now().toIso8601String()),
              ));
            }
            _isLoading = false;
          });
          _scrollToBottom();
          return;
        }
      } catch (e) {
        print('Error loading conversation: $e');
      }
    }

    _addWelcomeMessage();
    setState(() => _isLoading = false);
  }

  void _addWelcomeMessage() {
    _addMessage(
      'Hello! I can help you find information about your purchases. Try asking me:\n\n• "How much did I pay for milk last time?"\n• "Where did I buy bread?"\n• "Show me all my purchases from Walmart"\n• "What was the price of eggs?"',
      isUser: false,
    );
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _addMessage(String text, {required bool isUser}) {
    setState(() {
      _messages.add(ChatMessage(
        text: text,
        isUser: isUser,
        timestamp: DateTime.now(),
      ));
    });
    _scrollToBottom();
  }

  void _scrollToBottom() {
    Future.delayed(const Duration(milliseconds: 100), () {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  Future<void> _sendMessage() async {
    final text = _messageController.text.trim();
    if (text.isEmpty) return;

    _addMessage(text, isUser: true);
    _messageController.clear();

    setState(() => _isLoading = true);

    try {
      final response = await _assistantService.askQuestion(text, conversationId: _conversationId);
      setState(() {
        _conversationId = response['conversationId'];
      });
      
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString('last_conversation_id', _conversationId ?? '');
      
      _addMessage(response['answer'] ?? 'No response', isUser: false);
    } catch (e) {
      _addMessage(
        'Sorry, I encountered an error: ${e.toString()}',
        isUser: false,
      );
    } finally {
      setState(() => _isLoading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: const Text(
          'Shopping Assistant',
          style: TextStyle(fontWeight: FontWeight.w600),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.delete_outline),
            onPressed: () {
              setState(() {
                _messages.clear();
                _conversationId = null;
                _addMessage(
                  'Hello! I can help you find information about your purchases. Try asking me:\n\n• "How much did I pay for milk last time?"\n• "Where did I buy bread?"\n• "Show me all my purchases from Walmart"\n• "What was the price of eggs?"',
                  isUser: false,
                );
              });
            },
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
              itemCount: _messages.length,
              itemBuilder: (context, index) {
                final message = _messages[index];
                return _buildMessageBubble(message);
              },
            ),
          ),
          if (_isLoading)
            Padding(
              padding: const EdgeInsets.all(8.0),
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
                        Text(
                          'Thinking...',
                          style: TextStyle(color: AppTheme.darkGray),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          _buildInputArea(),
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
                color: AppTheme.primaryBlue.withOpacity(0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.smart_toy,
                size: 20,
                color: AppTheme.primaryBlue,
              ),
            ),
          Flexible(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                color: message.isUser
                    ? AppTheme.primaryBlue
                    : AppTheme.lightGray,
                borderRadius: BorderRadius.circular(20),
              ),
              child: message.isUser
                  ? Text(
                      message.text,
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 15,
                      ),
                    )
                  : MarkdownBody(
                      data: message.text,
                      styleSheet: MarkdownStyleSheet(
                        p: TextStyle(
                          color: AppTheme.nearBlack,
                          fontSize: 15,
                        ),
                        strong: TextStyle(
                          color: AppTheme.nearBlack,
                          fontWeight: FontWeight.bold,
                        ),
                        em: TextStyle(
                          color: AppTheme.nearBlack,
                          fontStyle: FontStyle.italic,
                        ),
                        code: TextStyle(
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
          ),
          if (message.isUser)
            Container(
              margin: const EdgeInsets.only(left: 8),
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppTheme.primaryBlue.withOpacity(0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.person,
                size: 20,
                color: AppTheme.primaryBlue,
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildInputArea() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.05),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Row(
        children: [
          Expanded(
            child: KeyboardListener(
              focusNode: FocusNode(),
              onKeyEvent: (event) {
                if (event is KeyDownEvent &&
                    event.logicalKey == LogicalKeyboardKey.enter &&
                    HardwareKeyboard.instance.isControlPressed) {
                  _sendMessage();
                }
              },
              child: TextField(
                controller: _messageController,
                decoration: InputDecoration(
                  hintText: 'Ask about your purchases...',
                  hintStyle: TextStyle(color: AppTheme.darkGray),
                  filled: true,
                  fillColor: AppTheme.lightGray,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(25),
                    borderSide: BorderSide.none,
                  ),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(25),
                    borderSide: BorderSide.none,
                  ),
                  focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(25),
                    borderSide: BorderSide.none,
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 20,
                    vertical: 12,
                  ),
                ),
                maxLines: null,
                keyboardType: TextInputType.multiline,
                textCapitalization: TextCapitalization.sentences,
              ),
            ),
          ),
          const SizedBox(width: 8),
          Container(
            decoration: BoxDecoration(
              color: AppTheme.primaryBlue,
              shape: BoxShape.circle,
            ),
            child: IconButton(
              icon: const Icon(Icons.send),
              color: Colors.white,
              onPressed: _isLoading ? null : _sendMessage,
            ),
          ),
        ],
      ),
    );
  }
}

class ChatMessage {
  final String text;
  final bool isUser;
  final DateTime timestamp;

  ChatMessage({
    required this.text,
    required this.isUser,
    required this.timestamp,
  });
}
