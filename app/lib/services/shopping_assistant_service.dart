import 'dart:convert';

import 'package:http/http.dart' as http;

import '../config/api_config.dart';
import 'auth_service.dart';

class AssistantChatResponse {
  const AssistantChatResponse({
    required this.answer,
    required this.conversationId,
    required this.isError,
    required this.retryable,
    required this.turnMayHaveBeenPersisted,
  });

  final String answer;
  final String conversationId;
  final bool isError;
  final bool retryable;
  final bool turnMayHaveBeenPersisted;
}

abstract interface class AssistantChatApi {
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  });

  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  );
}

abstract interface class ReceiptAssistantApi implements AssistantChatApi {
  Future<List<String>> getSuggestions();
}

class ShoppingAssistantService implements ReceiptAssistantApi {
  ShoppingAssistantService({
    AuthService? authService,
    http.Client? client,
    String assistantPath = '/assistant',
  }) : _authService = authService ?? AuthService(),
       _client = client ?? http.Client(),
       _assistantPath = assistantPath;

  final AuthService _authService;
  final http.Client _client;
  final String _assistantPath;

  Future<Map<String, String>> _headers() async {
    final token = await _authService.getApiToken();
    if (token == null || token.isEmpty) {
      throw StateError('Please log in to use the assistant.');
    }
    return {
      'Content-Type': 'application/json',
      'Authorization': ['Bearer', token].join(' '),
    };
  }

  @override
  Future<AssistantChatResponse> askQuestion(
    String question, {
    String? conversationId,
  }) async {
    try {
      final requestBody = {
        'question': question,
        if (conversationId != null && conversationId.isNotEmpty)
          'conversationId': conversationId,
      };
      final response = await _client
          .post(
            Uri.parse('${ApiConfig.baseUrl}$_assistantPath/ask'),
            headers: await _headers(),
            body: jsonEncode(requestBody),
          )
          .timeout(const Duration(seconds: 30));

      if (response.statusCode == 200) {
        try {
          final data = jsonDecode(response.body) as Map<String, dynamic>;
          return AssistantChatResponse(
            answer:
                data['answer']?.toString() ??
                'I could not find an answer to your question.',
            conversationId: data['conversationId']?.toString() ?? '',
            isError: false,
            retryable: false,
            turnMayHaveBeenPersisted: true,
          );
        } catch (_) {
          return AssistantChatResponse(
            answer: 'The assistant returned an invalid response.',
            conversationId: conversationId ?? '',
            isError: true,
            retryable: true,
            turnMayHaveBeenPersisted: true,
          );
        }
      }
      final error = _decodeError(response.body);
      return AssistantChatResponse(
        answer:
            error ??
            'The assistant request was not completed (${response.statusCode}).',
        conversationId: conversationId ?? '',
        isError: true,
        retryable: _isRetryableStatus(response.statusCode),
        turnMayHaveBeenPersisted: false,
      );
    } catch (error) {
      final authenticationError = error is StateError;
      return AssistantChatResponse(
        answer:
            authenticationError
                ? error.message
                : 'The assistant request could not be completed. Please try again.',
        conversationId: conversationId ?? '',
        isError: true,
        retryable: !authenticationError,
        turnMayHaveBeenPersisted: !authenticationError,
      );
    }
  }

  @override
  Future<List<Map<String, dynamic>>> getConversationHistory(
    String conversationId,
  ) async {
    try {
      final response = await _client
          .get(
            Uri.parse(
              '${ApiConfig.baseUrl}$_assistantPath/conversation/$conversationId',
            ),
            headers: await _headers(),
          )
          .timeout(const Duration(seconds: 30));
      if (response.statusCode != 200) return [];

      final data = jsonDecode(response.body) as List<dynamic>;
      return data
          .map((item) => Map<String, dynamic>.from(item as Map))
          .toList();
    } catch (_) {
      return [];
    }
  }

  @override
  Future<List<String>> getSuggestions() async {
    final response = await _client
        .get(
          Uri.parse('${ApiConfig.baseUrl}$_assistantPath/suggestions'),
          headers: await _headers(),
        )
        .timeout(const Duration(seconds: 30));
    if (response.statusCode != 200) {
      throw Exception(
        'Failed to load assistant suggestions: ${response.statusCode}',
      );
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final suggestions = data['suggestions'] as List<dynamic>? ?? const [];
    return suggestions
        .whereType<String>()
        .map((suggestion) => suggestion.trim())
        .where((suggestion) => suggestion.isNotEmpty)
        .toList();
  }

  String? _decodeError(String body) {
    if (body.isEmpty) return null;
    try {
      final data = jsonDecode(body);
      if (data is Map) {
        final message = data['message'];
        if (message is String && message.trim().isNotEmpty) {
          return message;
        }
        if (message is Map) {
          final nestedMessage = message['message'];
          if (nestedMessage is String && nestedMessage.trim().isNotEmpty) {
            return nestedMessage;
          }
        }
      }
    } catch (_) {}
    return null;
  }

  bool _isRetryableStatus(int statusCode) {
    return statusCode == 408 ||
        statusCode == 429 ||
        statusCode >= 500 && statusCode <= 599;
  }
}
