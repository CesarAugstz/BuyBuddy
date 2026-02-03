import 'package:flutter/material.dart';
import '../services/auth_service.dart';
import '../config/theme.dart';

class LoginPage extends StatelessWidget {
  final AuthService _authService = AuthService();

  LoginPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              padding: EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: AppTheme.lightGray,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.lock_outline,
                size: 64,
                color: AppTheme.darkGray,
              ),
            ),
            SizedBox(height: 40),
            Text(
              'Welcome',
              style: TextStyle(
                fontSize: 32,
                fontWeight: FontWeight.w600,
                color: AppTheme.nearBlack,
                letterSpacing: -0.5,
              ),
            ),
            SizedBox(height: 8),
            Text(
              'Sign in to continue',
              style: TextStyle(
                fontSize: 16,
                color: AppTheme.darkGray,
                fontWeight: FontWeight.w400,
              ),
            ),
            SizedBox(height: 60),
            ElevatedButton.icon(
              onPressed: () async {
                final result = await _authService.signInWithGoogle();
                if (result == null) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('Failed to sign in'),
                        backgroundColor: AppTheme.nearBlack,
                      ),
                    );
                  }
                }
              },
              icon: Icon(Icons.login, size: 20),
              label: Text(
                'Sign in with Google',
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w500,
                ),
              ),
              style: ElevatedButton.styleFrom(
                padding: EdgeInsets.symmetric(horizontal: 32, vertical: 16),
                backgroundColor: AppTheme.primaryBlue,
                foregroundColor: Colors.white,
                elevation: 0,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
