import 'dart:convert';
import 'package:http/http.dart' as http;
import '../config/api_config.dart';
import 'auth_service.dart';

class ShoppingAssistantService {
  final _authService = AuthService();

  Future<Map<String, String>> askQuestion(String question, {String? conversationId}) async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) {
        return {
          'answer': 'Please log in to use the shopping assistant.',
          'conversationId': '',
        };
      }

      final requestBody = {
        'question': question,
        if (conversationId != null && conversationId.isNotEmpty)
          'conversationId': conversationId,
      };

      final response = await http.post(
        Uri.parse('${ApiConfig.baseUrl}/assistant/ask'),
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer $token',
        },
        body: jsonEncode(requestBody),
      ).timeout(const Duration(seconds: 30));

      if (response.statusCode == 200) {
        final data = jsonDecode(response.body);
        return {
          'answer': data['answer'] ?? 'I could not find an answer to your question.',
          'conversationId': data['conversationId'] ?? '',
        };
      } else {
        final error = jsonDecode(response.body);
        return {
          'answer': error['message'] ?? 'Failed to get answer from assistant.',
          'conversationId': conversationId ?? '',
        };
      }
    } catch (e) {
      return {
        'answer': 'Error: ${e.toString()}',
        'conversationId': conversationId ?? '',
      };
    }
  }

  Future<List<Map<String, dynamic>>> getConversationHistory(String conversationId) async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) {
        return [];
      }

      final response = await http.get(
        Uri.parse('${ApiConfig.baseUrl}/assistant/conversation/$conversationId'),
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer $token',
        },
      );

      if (response.statusCode == 200) {
        final List<dynamic> data = jsonDecode(response.body);
        return data.map((item) => item as Map<String, dynamic>).toList();
      }
      return [];
    } catch (e) {
      print('Error loading conversation history: $e');
      return [];
    }
  }
}
