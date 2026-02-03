import 'package:flutter/material.dart';
import '../services/auth_service.dart';
import '../config/theme.dart';
import 'receipts_page.dart';
import 'shopping_assistant_page.dart';

class MainPage extends StatelessWidget {
  final AuthService _authService = AuthService();

  MainPage({super.key});

  @override
  Widget build(BuildContext context) {
    final user = _authService.currentUser;

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text(
          'Home',
          style: TextStyle(fontWeight: FontWeight.w600),
        ),
        actions: [
          IconButton(
            icon: Icon(Icons.logout_outlined),
            onPressed: () async {
              await _authService.signOut();
            },
          ),
        ],
      ),
      drawer: Drawer(
        child: ListView(
          padding: EdgeInsets.zero,
          children: [
            DrawerHeader(
              decoration: BoxDecoration(
                color: AppTheme.primaryBlue,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  CircleAvatar(
                    radius: 32,
                    backgroundColor: Colors.white,
                    backgroundImage: user?.photoUrl.isNotEmpty == true
                        ? NetworkImage(user!.photoUrl)
                        : null,
                    child: user?.photoUrl.isEmpty == true
                        ? Icon(Icons.person, size: 32, color: AppTheme.primaryBlue)
                        : null,
                  ),
                  SizedBox(height: 12),
                  Text(
                    user?.name ?? 'User',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  SizedBox(height: 4),
                  Text(
                    user?.email ?? '',
                    style: TextStyle(
                      color: Colors.white70,
                      fontSize: 13,
                    ),
                  ),
                ],
              ),
            ),
            ListTile(
              leading: Icon(Icons.receipt_long_outlined, color: AppTheme.darkGray),
              title: Text('My Receipts'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => const ReceiptsPage()),
                );
              },
            ),
            ListTile(
              leading: Icon(Icons.chat_bubble_outline, color: AppTheme.darkGray),
              title: Text('Shopping Assistant'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => const ShoppingAssistantPage()),
                );
              },
            ),
            Divider(height: 1),
            ListTile(
              leading: Icon(Icons.logout_outlined, color: AppTheme.darkGray),
              title: Text('Sign Out'),
              onTap: () async {
                await _authService.signOut();
                if (context.mounted) {
                  Navigator.pop(context);
                }
              },
            ),
          ],
        ),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              'Welcome, ${user?.name ?? 'User'}!',
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.w600,
                color: AppTheme.nearBlack,
              ),
            ),
            SizedBox(height: 40),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                ElevatedButton.icon(
                  onPressed: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => const ReceiptsPage(),
                      ),
                    );
                  },
                  icon: Icon(Icons.receipt_long),
                  label: Text('Receipts'),
                  style: ElevatedButton.styleFrom(
                    padding: EdgeInsets.symmetric(horizontal: 24, vertical: 16),
                  ),
                ),
                SizedBox(width: 16),
                ElevatedButton.icon(
                  onPressed: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => const ShoppingAssistantPage(),
                      ),
                    );
                  },
                  icon: Icon(Icons.chat_bubble_outline),
                  label: Text('Assistant'),
                  style: ElevatedButton.styleFrom(
                    padding: EdgeInsets.symmetric(horizontal: 24, vertical: 16),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
