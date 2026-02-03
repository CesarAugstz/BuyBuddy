import 'package:flutter/material.dart';
import 'services/auth_service.dart';
import 'pages/login_page.dart';
import 'pages/main_page.dart';
import 'config/theme.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    final authService = AuthService();

    return MaterialApp(
      title: 'Flutter App',
      theme: AppTheme.theme,
      home: StreamBuilder<UserData?>(
        stream: authService.authStateChanges,
        builder: (context, snapshot) {
          print(snapshot.connectionState);
          print(snapshot.hasData);
          if (snapshot.connectionState == ConnectionState.waiting) {
            return Scaffold(
              body: Center(child: CircularProgressIndicator()),
            );
          }
          
          if (snapshot.hasData && snapshot.data != null) {
            return MainPage();
          }
          
          return LoginPage();
        },
      ),
    );
  }
}
