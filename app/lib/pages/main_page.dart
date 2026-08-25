import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/auth_provider.dart';
import '../providers/shopping_list_provider.dart';
import '../config/theme.dart';
import 'receipt_scanner_page.dart';
import 'receipts_page.dart';
import 'shopping_assistant_page.dart';
import 'shopping_list_detail_page.dart';
import 'shopping_lists_page.dart';
import 'model_settings_page.dart';
import 'knowledge_explorer_page.dart';
import 'knowledge_assistant_page.dart';

class MainPage extends ConsumerWidget {
  const MainPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(currentUserProvider);
    final recentListAsync = ref.watch(recentShoppingListProvider);

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text('Home', style: TextStyle(fontWeight: FontWeight.w600)),
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
              decoration: BoxDecoration(color: AppTheme.primaryBlue),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  CircleAvatar(
                    radius: 32,
                    backgroundColor: Colors.white,
                    backgroundImage:
                        user?.photoUrl.isNotEmpty == true
                            ? NetworkImage(user!.photoUrl)
                            : null,
                    child:
                        user?.photoUrl.isEmpty == true
                            ? Icon(
                              Icons.person,
                              size: 32,
                              color: AppTheme.primaryBlue,
                            )
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
                    style: TextStyle(color: Colors.white70, fontSize: 13),
                  ),
                ],
              ),
            ),
            ListTile(
              leading: Icon(
                Icons.receipt_long_outlined,
                color: AppTheme.darkGray,
              ),
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
              leading: Icon(
                Icons.shopping_cart_outlined,
                color: AppTheme.darkGray,
              ),
              title: Text('Shopping Lists'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const ShoppingListsPage(),
                  ),
                );
              },
            ),
            ListTile(
              leading: Icon(
                Icons.chat_bubble_outline,
                color: AppTheme.darkGray,
              ),
              title: Text('Shopping Assistant'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const ShoppingAssistantPage(),
                  ),
                );
              },
            ),
            ListTile(
              leading: Icon(
                Icons.auto_stories_outlined,
                color: AppTheme.darkGray,
              ),
              title: Text('Knowledge Assistant'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const KnowledgeAssistantPage(),
                  ),
                );
              },
            ),
            ListTile(
              leading: Icon(Icons.folder_outlined, color: AppTheme.darkGray),
              title: Text('Personal Knowledge'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const KnowledgeExplorerPage(),
                  ),
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
                  MaterialPageRoute(
                    builder: (context) => const ModelSettingsPage(),
                  ),
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
      body: SingleChildScrollView(
        padding: EdgeInsets.all(24),
        child: Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: 600),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  'Welcome, ${user?.name ?? 'User'}!',
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w600,
                    color: AppTheme.nearBlack,
                  ),
                ),
                SizedBox(height: 32),
                Text(
                  'Quick Actions',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: AppTheme.nearBlack,
                  ),
                ),
                SizedBox(height: 12),
                recentListAsync.when(
                  data:
                      (list) =>
                          list == null
                              ? const SizedBox.shrink()
                              : Padding(
                                padding: const EdgeInsets.only(bottom: 12),
                                child: _QuickActionCard(
                                  icon: Icons.shopping_cart_checkout,
                                  title: 'Continue Shopping List',
                                  description:
                                      '${list.title} · ${list.uncheckedCount} remaining',
                                  onTap: () {
                                    Navigator.push(
                                      context,
                                      MaterialPageRoute(
                                        builder:
                                            (context) => ShoppingListDetailPage(
                                              listId: list.id,
                                            ),
                                      ),
                                    );
                                  },
                                ),
                              ),
                  loading: () => const SizedBox.shrink(),
                  error: (_, __) => const SizedBox.shrink(),
                ),
                _QuickActionCard(
                  icon: Icons.photo_library_outlined,
                  title: 'Add Receipt from Gallery',
                  description: 'Choose an image and scan it instantly',
                  onTap: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder:
                            (context) => const ReceiptScannerPage(
                              openGalleryOnStart: true,
                            ),
                      ),
                    );
                  },
                ),
                SizedBox(height: 32),
                Text(
                  'Explore',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: AppTheme.nearBlack,
                  ),
                ),
                SizedBox(height: 16),
                Wrap(
                  spacing: 16,
                  runSpacing: 16,
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
                        padding: EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 16,
                        ),
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
                        padding: EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 16,
                        ),
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
                      label: Text('Shopping Assistant'),
                      style: ElevatedButton.styleFrom(
                        padding: EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 16,
                        ),
                      ),
                    ),
                    ElevatedButton.icon(
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder:
                                (context) => const KnowledgeAssistantPage(),
                          ),
                        );
                      },
                      icon: Icon(Icons.psychology_alt_outlined),
                      label: Text('Knowledge Assistant'),
                      style: ElevatedButton.styleFrom(
                        padding: EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 16,
                        ),
                      ),
                    ),
                    ElevatedButton.icon(
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => const KnowledgeExplorerPage(),
                          ),
                        );
                      },
                      icon: Icon(Icons.auto_stories_outlined),
                      label: Text('Personal Knowledge'),
                      style: ElevatedButton.styleFrom(
                        padding: EdgeInsets.symmetric(
                          horizontal: 24,
                          vertical: 16,
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _QuickActionCard extends StatelessWidget {
  const _QuickActionCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.onTap,
  });

  final IconData icon;
  final String title;
  final String description;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      color: Colors.white,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: AppTheme.mediumGray),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Row(
            children: [
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: AppTheme.primaryBlue.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(icon, color: AppTheme.primaryBlue, size: 28),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppTheme.nearBlack,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      description,
                      style: TextStyle(fontSize: 14, color: AppTheme.darkGray),
                    ),
                  ],
                ),
              ),
              Icon(Icons.arrow_forward_ios, size: 18, color: AppTheme.darkGray),
            ],
          ),
        ),
      ),
    );
  }
}
