import 'dart:convert';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import '../config/api_config.dart';
import 'auth_service.dart';

class ReceiptService {
  final _authService = AuthService();

  Future<Map<String, dynamic>> processReceipt(Uint8List imageBytes) async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) {
        return {'success': false, 'error': 'Not authenticated'};
      }

      final base64Image = base64Encode(imageBytes);

      final response = await http
          .post(
            Uri.parse('${ApiConfig.baseUrl}/receipts/process'),
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer $token',
            },
            body: jsonEncode({'image': base64Image}),
          )
          .timeout(const Duration(seconds: 90));

      if (response.statusCode == 200) {
        final data = jsonDecode(response.body);
        return {'success': true, 'data': data};
      } else {
        final error = jsonDecode(response.body);
        return {
          'success': false,
          'error': error['message'] ?? 'Failed to process receipt',
        };
      }
    } catch (e) {
      return {'success': false, 'error': e.toString()};
    }
  }

  Future<bool> saveReceipt(Map<String, dynamic> receiptData) async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) throw Exception('Not authenticated');

      final response = await http.post(
        Uri.parse('${ApiConfig.baseUrl}/receipts'),
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer $token',
        },
        body: jsonEncode(receiptData),
      );

      if (response.statusCode != 200 && response.statusCode != 201) {
        throw Exception(
          'Failed to save receipt with status: ${response.statusCode}',
        );
      }
      return true;
    } catch (e) {
      print('Error saving receipt: $e');
      rethrow;
    }
  }

  Future<List<dynamic>> getReceipts() async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) throw Exception('Not authenticated');

      final response = await http.get(
        Uri.parse('${ApiConfig.baseUrl}/receipts'),
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer $token',
        },
      );

      if (response.statusCode == 200) {
        final data = jsonDecode(response.body);
        return data as List<dynamic>;
      } else {
        throw Exception('Failed to load receipts: ${response.statusCode}');
      }
    } catch (e) {
      print('Error loading receipts: $e');
      rethrow;
    }
  }

  Future<bool> deleteReceipt(String receiptId) async {
    try {
      final token = await _authService.getApiToken();
      if (token == null) throw Exception('Not authenticated');

      final response = await http.delete(
        Uri.parse('${ApiConfig.baseUrl}/receipts/$receiptId'),
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer $token',
        },
      );

      if (response.statusCode == 200 || response.statusCode == 204) {
        return true;
      } else {
        throw Exception('Failed to delete receipt: ${response.statusCode}');
      }
    } catch (e) {
      print('Error deleting receipt: $e');
      rethrow;
    }
  }
}
