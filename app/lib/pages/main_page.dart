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
                  'Your tools',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: AppTheme.nearBlack,
                  ),
                ),
                SizedBox(height: 16),
                _ToolSection(
                  title: 'Shopping',
                  children: [
                    _ToolCard(
                      icon: Icons.receipt_long_outlined,
                      title: 'Receipts',
                      description: 'Review your purchase history',
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => const ReceiptsPage(),
                          ),
                        );
                      },
                    ),
                    _ToolCard(
                      icon: Icons.shopping_cart_outlined,
                      title: 'Shopping Lists',
                      description: 'Plan and track what to buy',
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => const ShoppingListsPage(),
                          ),
                        );
                      },
                    ),
                    _ToolCard(
                      icon: Icons.chat_bubble_outline,
                      title: 'Shopping Assistant',
                      description: 'Ask about receipts and prices',
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => const ShoppingAssistantPage(),
                          ),
                        );
                      },
                    ),
                  ],
                ),
                const SizedBox(height: 24),
                _ToolSection(
                  title: 'Knowledge',
                  children: [
                    _ToolCard(
                      icon: Icons.psychology_alt_outlined,
                      title: 'Knowledge Assistant',
                      description: 'Save and manage personal notes',
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder:
                                (context) => const KnowledgeAssistantPage(),
                          ),
                        );
                      },
                    ),
                    _ToolCard(
                      icon: Icons.folder_outlined,
                      title: 'Personal Knowledge',
                      description: 'Browse your topics and entries',
                      onPressed: () {
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => const KnowledgeExplorerPage(),
                          ),
                        );
                      },
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

class _ToolSection extends StatelessWidget {
  const _ToolSection({required this.title, required this.children});

  final String title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: TextStyle(
            color: AppTheme.darkGray,
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 10),
        LayoutBuilder(
          builder: (context, constraints) {
            final columns = constraints.maxWidth >= 360 ? 2 : 1;
            const spacing = 12.0;
            final cardWidth =
                (constraints.maxWidth - spacing * (columns - 1)) / columns;
            return Wrap(
              spacing: spacing,
              runSpacing: spacing,
              children: [
                for (final child in children)
                  SizedBox(width: cardWidth, child: child),
              ],
            );
          },
        ),
      ],
    );
  }
}

class _ToolCard extends StatelessWidget {
  const _ToolCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.onPressed,
  });

  final IconData icon;
  final String title;
  final String description;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Card(
      color: Colors.white,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: AppTheme.mediumGray),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onPressed,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 156),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Container(
                      padding: const EdgeInsets.all(11),
                      decoration: BoxDecoration(
                        color: AppTheme.primaryBlue.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(11),
                      ),
                      child: Icon(icon, color: AppTheme.primaryBlue, size: 24),
                    ),
                    Icon(
                      Icons.arrow_forward_ios,
                      size: 15,
                      color: AppTheme.darkGray,
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Text(
                  title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: AppTheme.nearBlack,
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 5),
                Text(
                  description,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: AppTheme.darkGray,
                    fontSize: 12.5,
                    height: 1.25,
                  ),
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
