import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:buybuddy/config/api_config.dart';
import 'package:buybuddy/services/auth_service.dart';

class UserPreferences {
  final String receiptModel;
  final String assistantModel;

  UserPreferences({
    required this.receiptModel,
    required this.assistantModel,
  });

  factory UserPreferences.fromJson(Map<String, dynamic> json) {
    return UserPreferences(
      receiptModel: json['receipt_model'] ?? 'gemini-2.5-flash',
      assistantModel: json['assistant_model'] ?? 'gemini-2.5-flash-lite',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'receipt_model': receiptModel,
      'assistant_model': assistantModel,
    };
  }
}

class GeminiModel {
  final String id;
  final String name;
  final String description;

  GeminiModel({
    required this.id,
    required this.name,
    required this.description,
  });

  factory GeminiModel.fromJson(Map<String, dynamic> json) {
    return GeminiModel(
      id: json['id'],
      name: json['name'],
      description: json['description'],
    );
  }
}

class AvailableModels {
  final List<GeminiModel> receiptModels;
  final List<GeminiModel> assistantModels;

  AvailableModels({
    required this.receiptModels,
    required this.assistantModels,
  });

  factory AvailableModels.fromJson(Map<String, dynamic> json) {
    return AvailableModels(
      receiptModels: (json['receipt_models'] as List)
          .map((m) => GeminiModel.fromJson(m))
          .toList(),
      assistantModels: (json['assistant_models'] as List)
          .map((m) => GeminiModel.fromJson(m))
          .toList(),
    );
  }
}

class PreferencesService {
  final AuthService _authService = AuthService();

  Future<UserPreferences> getPreferences() async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/preferences'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode == 200) {
      return UserPreferences.fromJson(json.decode(response.body));
    }
    throw Exception('Failed to load preferences');
  }

  Future<UserPreferences> updatePreferences(UserPreferences prefs) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse('${ApiConfig.baseUrl}/preferences'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
      body: json.encode(prefs.toJson()),
    );

    if (response.statusCode == 200) {
      return UserPreferences.fromJson(json.decode(response.body));
    }
    throw Exception('Failed to update preferences');
  }

  Future<AvailableModels> getAvailableModels() async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/preferences/models'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
    );

    if (response.statusCode == 200) {
      return AvailableModels.fromJson(json.decode(response.body));
    }
    throw Exception('Failed to load available models');
  }
}
