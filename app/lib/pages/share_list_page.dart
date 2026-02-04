import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../config/theme.dart';
import '../providers/shopping_list_provider.dart';
import '../services/shopping_list_service.dart';

class ShareListPage extends ConsumerStatefulWidget {
  final String listId;
  final String listTitle;

  const ShareListPage({
    super.key,
    required this.listId,
    required this.listTitle,
  });

  @override
  ConsumerState<ShareListPage> createState() => _ShareListPageState();
}

class _ShareListPageState extends ConsumerState<ShareListPage> {
  final _searchController = TextEditingController();
  Timer? _debounce;
  String _searchQuery = '';

  @override
  void dispose() {
    _searchController.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () {
      setState(() => _searchQuery = value);
    });
  }

  Future<void> _inviteUser(SearchUser user) async {
    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.shareList(widget.listId, user.email);
      ref.invalidate(listSharesProvider(widget.listId));
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Invite sent to ${user.name}')),
        );
        _searchController.clear();
        setState(() => _searchQuery = '');
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to invite: $e')),
        );
      }
    }
  }

  Future<void> _removeShare(ShoppingListShare share) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Remove Access'),
        content:
            Text('Remove ${share.user?.name ?? 'this user'} from the list?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(foregroundColor: Colors.red),
            child: const Text('Remove'),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.removeShare(widget.listId, share.userId);
      ref.invalidate(listSharesProvider(widget.listId));
      ref.invalidate(shoppingListsProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to remove: $e')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final sharesAsync = ref.watch(listSharesProvider(widget.listId));
    final searchAsync = ref.watch(userSearchProvider(_searchQuery));

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text('Share "${widget.listTitle}"'),
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Search by email...',
                prefixIcon: const Icon(Icons.search),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              onChanged: _onSearchChanged,
            ),
          ),
          if (_searchQuery.length >= 3)
            searchAsync.when(
              data: (users) => _buildSearchResults(users),
              loading: () => const Padding(
                padding: EdgeInsets.all(16),
                child: Center(child: CircularProgressIndicator()),
              ),
              error: (e, _) => Padding(
                padding: const EdgeInsets.all(16),
                child: Text('Error: $e'),
              ),
            ),
          const Divider(height: 1),
          Padding(
            padding: const EdgeInsets.all(16),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text(
                'Shared with',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: AppTheme.nearBlack,
                ),
              ),
            ),
          ),
          Expanded(
            child: sharesAsync.when(
              data: (shares) => _buildSharesList(shares),
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => Center(child: Text('Error: $e')),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchResults(List<SearchUser> users) {
    if (users.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Text(
          'No users found',
          style: TextStyle(color: Colors.grey.shade600),
        ),
      );
    }

    return Container(
      constraints: const BoxConstraints(maxHeight: 200),
      child: ListView.builder(
        shrinkWrap: true,
        itemCount: users.length,
        itemBuilder: (context, index) {
          final user = users[index];
          return ListTile(
            leading: CircleAvatar(
              backgroundImage:
                  user.photoUrl != null ? NetworkImage(user.photoUrl!) : null,
              child: user.photoUrl == null ? Text(user.name[0]) : null,
            ),
            title: Text(user.name),
            subtitle: Text(user.email),
            trailing: IconButton(
              icon: const Icon(Icons.person_add),
              color: AppTheme.primaryBlue,
              onPressed: () => _inviteUser(user),
            ),
          );
        },
      ),
    );
  }

  Widget _buildSharesList(List<ShoppingListShare> shares) {
    if (shares.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.people_outline,
              size: 64,
              color: Colors.grey.shade300,
            ),
            const SizedBox(height: 16),
            Text(
              'Not shared with anyone',
              style: TextStyle(
                fontSize: 16,
                color: Colors.grey.shade600,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Search by email to invite someone',
              style: TextStyle(
                fontSize: 14,
                color: Colors.grey.shade500,
              ),
            ),
          ],
        ),
      );
    }

    return ListView.builder(
      itemCount: shares.length,
      itemBuilder: (context, index) {
        final share = shares[index];
        final user = share.user;
        return ListTile(
          leading: CircleAvatar(
            backgroundImage:
                user?.photoUrl != null ? NetworkImage(user!.photoUrl!) : null,
            child: user?.photoUrl == null ? Text(user?.name[0] ?? '?') : null,
          ),
          title: Text(user?.name ?? 'Unknown'),
          subtitle: Text(user?.email ?? ''),
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: share.isPending
                      ? Colors.orange.shade100
                      : Colors.green.shade100,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  share.isPending ? 'Pending' : 'Accepted',
                  style: TextStyle(
                    fontSize: 12,
                    color: share.isPending
                        ? Colors.orange.shade800
                        : Colors.green.shade800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              IconButton(
                icon: const Icon(Icons.close, size: 20),
                color: Colors.red,
                onPressed: () => _removeShare(share),
              ),
            ],
          ),
        );
      },
    );
  }
}
