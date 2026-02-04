import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'dart:async';
import 'dart:convert';
import '../config/api_config.dart';

class UserData {
  final String email;
  final String name;
  final String photoUrl;
  final String? apiToken;
  final String? clientId;

  UserData({
    required this.email,
    required this.name,
    required this.photoUrl,
    this.apiToken,
    this.clientId,
  });

  factory UserData.fromGoogleAccount(
    GoogleSignInAccount account, {
    String? apiToken,
  }) {
    return UserData(
      email: account.email,
      name: account.displayName ?? '',
      photoUrl: account.photoUrl ?? '',
      apiToken: apiToken,
      clientId: account.id,
    );
  }

  Map<String, dynamic> toJson() => {
    'email': email,
    'name': name,
    'photoUrl': photoUrl,
    'apiToken': apiToken,
    'clientId': clientId,
  };

  factory UserData.fromJson(Map<String, dynamic> json) {
    return UserData(
      email: json['email'],
      name: json['name'],
      photoUrl: json['photoUrl'],
      apiToken: json['apiToken'],
      clientId: json['clientId'],
    );
  }
}

class AuthService {
  static final AuthService _instance = AuthService._internal();
  factory AuthService() => _instance;

  AuthService._internal() {
    try {
      _googleSignIn = GoogleSignIn(
        scopes: ['email', 'profile'],
      );
    } catch (e) {
      debugPrint('Error initializing GoogleSignIn: $e');
    }
    _initAuth();
  }

  late GoogleSignIn _googleSignIn;
  final _authStateController = StreamController<UserData?>.broadcast();

  UserData? _currentUser;

  Future<void> _initAuth() async {
    final savedUser = await _getSavedUser();
    if (savedUser != null) {
      _currentUser = savedUser;
      _authStateController.add(savedUser);
    } else {
      _authStateController.add(null);
    }

    _googleSignIn.onCurrentUserChanged.listen((account) async {
      if (account != null) {
        await _handleGoogleSignIn(account);
      }
    });

    _googleSignIn.signInSilently();
  }

  UserData? get currentUser => _currentUser;
  Stream<UserData?> get authStateChanges => _authStateController.stream;

  Future<UserData?> signInWithGoogle() async {
    try {
      final account = await _googleSignIn.signIn();
      if (account == null) return null;

      return await _handleGoogleSignIn(account);
    } on PlatformException catch (e) {
      debugPrint('Error signing in with Google: ${e.code} - ${e.message}');
      return null;
    } catch (e) {
      debugPrint('Unexpected error signing in with Google: $e');
      return null;
    }
  }

  Future<UserData?> _handleGoogleSignIn(GoogleSignInAccount account) async {
    try {
      final googleAuth = await account.authentication;
      final idToken = googleAuth.idToken;
      final accessToken = googleAuth.accessToken;

      final apiToken = await _sendTokenToBackend(idToken, accessToken, account);

      if (apiToken == null) {
        await _googleSignIn.signOut();
        return null;
      }

      final userData = UserData.fromGoogleAccount(account, apiToken: apiToken);
      _currentUser = userData;
      _authStateController.add(userData);
      await _saveUserSession(userData);

      return userData;
    } catch (e) {
      debugPrint('Error handling Google sign in: $e');
      await _googleSignIn.signOut();
      return null;
    }
  }

  Future<String?> _sendTokenToBackend(
    String? idToken,
    String? accessToken,
    GoogleSignInAccount account,
  ) async {
    try {
      final response = await http
          .post(
            Uri.parse(ApiConfig.loginEndpoint),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'idToken': idToken ?? '',
              'accessToken': accessToken ?? '',
              'email': account.email,
              'name': account.displayName,
              'photoUrl': account.photoUrl,
              'clientId': account.id,
            }),
          )
          .timeout(
            Duration(seconds: 10),
            onTimeout: () {
              throw Exception('Backend login timeout');
            },
          );

      if (response.statusCode == 200) {
        final data = jsonDecode(response.body);
        if (data['token'] == null || data['token'].toString().isEmpty) {
          throw Exception('Invalid token received from backend');
        }
        return data['token'];
      }

      debugPrint('Backend login failed: ${response.statusCode}');
      throw Exception(
        'Backend authentication failed with status: ${response.statusCode}',
      );
    } catch (e) {
      debugPrint('Error sending token to backend: $e');
      rethrow;
    }
  }

  Future<void> signOut() async {
    if (_currentUser?.apiToken != null) {
      try {
        await http.post(
          Uri.parse(ApiConfig.logoutEndpoint),
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ${_currentUser!.apiToken}',
          },
        );
      } catch (e) {
        debugPrint('Error notifying backend of logout: $e');
      }
    }

    await _googleSignIn.signOut();
    await _clearUserSession();
    _currentUser = null;
    _authStateController.add(null);
  }

  Future<void> _saveUserSession(UserData user) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('user_data', jsonEncode(user.toJson()));
  }

  Future<UserData?> _getSavedUser() async {
    final prefs = await SharedPreferences.getInstance();
    final userJson = prefs.getString('user_data');
    if (userJson != null) {
      return UserData.fromJson(jsonDecode(userJson));
    }
    return null;
  }

  Future<void> _clearUserSession() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
  }

  Future<String?> getApiToken() async {
    return _currentUser?.apiToken;
  }

  void dispose() {
    _authStateController.close();
  }
}
