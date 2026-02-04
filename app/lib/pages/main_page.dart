import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/auth_provider.dart';
import '../config/theme.dart';
import 'receipts_page.dart';
import 'shopping_assistant_page.dart';
import 'shopping_lists_page.dart';
import 'model_settings_page.dart';

class MainPage extends ConsumerWidget {
  const MainPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(currentUserProvider);

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
              await ref.read(authServiceProvider).signOut();
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
              leading: Icon(Icons.shopping_cart_outlined, color: AppTheme.darkGray),
              title: Text('Shopping Lists'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => const ShoppingListsPage()),
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
            ListTile(
              leading: Icon(Icons.settings_outlined, color: AppTheme.darkGray),
              title: Text('AI Model Settings'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => const ModelSettingsPage()),
                );
              },
            ),
            Divider(height: 1),
            ListTile(
              leading: Icon(Icons.logout_outlined, color: AppTheme.darkGray),
              title: Text('Sign Out'),
              onTap: () async {
                await ref.read(authServiceProvider).signOut();
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
            Wrap(
              spacing: 16,
              runSpacing: 16,
              alignment: WrapAlignment.center,
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
                ElevatedButton.icon(
                  onPressed: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => const ShoppingListsPage(),
                      ),
                    );
                  },
                  icon: Icon(Icons.shopping_cart_outlined),
                  label: Text('Lists'),
                  style: ElevatedButton.styleFrom(
                    padding: EdgeInsets.symmetric(horizontal: 24, vertical: 16),
                  ),
                ),
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
