import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../config/theme.dart';
import '../providers/shopping_list_provider.dart';
import '../services/shopping_list_service.dart';
import 'share_list_page.dart';

class ShoppingListDetailPage extends ConsumerStatefulWidget {
  final String listId;
  final bool isNewList;

  const ShoppingListDetailPage({
    super.key,
    required this.listId,
    this.isNewList = false,
  });

  @override
  ConsumerState<ShoppingListDetailPage> createState() =>
      _ShoppingListDetailPageState();
}

class _ShoppingListDetailPageState
    extends ConsumerState<ShoppingListDetailPage> {
  final _addItemController = TextEditingController();
  final _addItemFocusNode = FocusNode();
  final _titleFocusNode = FocusNode();
  bool _isAddingItem = false;
  bool _checkedExpanded = false;
  bool _isEditingTitle = false;
  bool _isSavingTitle = false;
  final _titleController = TextEditingController();
  String _searchQuery = '';
  bool _initialized = false;

  @override
  void initState() {
    super.initState();
    _addItemController.addListener(() {
      setState(() => _searchQuery = _addItemController.text);
    });
    _titleFocusNode.addListener(_onTitleFocusChange);
    if (widget.isNewList) {
      _isEditingTitle = true;
    }
  }

  void _onTitleFocusChange() {
    if (!_titleFocusNode.hasFocus && _isEditingTitle) {
      _updateTitle(_titleController.text);
    }
  }

  @override
  void dispose() {
    _titleFocusNode.removeListener(_onTitleFocusChange);
    _addItemController.dispose();
    _addItemFocusNode.dispose();
    _titleFocusNode.dispose();
    _titleController.dispose();
    super.dispose();
  }

  void _initTitleForNewList(String currentTitle) {
    if (!_initialized && widget.isNewList) {
      _titleController.text = currentTitle;
      _titleController.selection = TextSelection(
        baseOffset: 0,
        extentOffset: currentTitle.length,
      );
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _titleFocusNode.requestFocus();
      });
      _initialized = true;
    }
  }

  Future<void> _cancelNewList() async {
    try {
      await ref.read(shoppingListsProvider.notifier).deleteList(widget.listId);
      if (mounted) Navigator.pop(context);
    } catch (e) {
      if (mounted) Navigator.pop(context);
    }
  }

  Future<void> _addItem() async {
    final name = _addItemController.text.trim();
    if (name.isEmpty) return;

    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.addItem(widget.listId, name);
      _addItemController.clear();
      ref.invalidate(shoppingListDetailProvider(widget.listId));
      ref.read(shoppingListsProvider.notifier).refresh();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to add item: $e')),
        );
      }
    }
  }

  Future<void> _toggleItem(ShoppingListItem item) async {
    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.updateItem(widget.listId, item.id,
          isChecked: !item.isChecked);
      ref.invalidate(shoppingListDetailProvider(widget.listId));
      ref.read(shoppingListsProvider.notifier).refresh();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to update item: $e')),
        );
      }
    }
  }

  Future<void> _deleteItem(ShoppingListItem item) async {
    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.deleteItem(widget.listId, item.id);
      ref.invalidate(shoppingListDetailProvider(widget.listId));
      ref.read(shoppingListsProvider.notifier).refresh();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to delete item: $e')),
        );
      }
    }
  }

  Future<void> _updateTitle(String title) async {
    if (title.trim().isEmpty || _isSavingTitle) return;
    if (!_isEditingTitle) return;

    _isSavingTitle = true;
    setState(() => _isEditingTitle = false);

    final service = ref.read(shoppingListServiceProvider);
    try {
      await service.updateList(widget.listId, title: title.trim());
      ref.invalidate(shoppingListDetailProvider(widget.listId));
      await ref.read(shoppingListsProvider.notifier).refresh();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to update title: $e')),
        );
      }
    } finally {
      _isSavingTitle = false;
    }
  }

  Future<void> _deleteList() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete List'),
        content: const Text('Are you sure you want to delete this list?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(foregroundColor: Colors.red),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    try {
      await ref.read(shoppingListsProvider.notifier).deleteList(widget.listId);
      if (mounted) Navigator.pop(context);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to delete list: $e')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final listAsync = ref.watch(shoppingListDetailProvider(widget.listId));

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        leading: widget.isNewList && _isEditingTitle
            ? IconButton(
                icon: const Icon(Icons.close),
                onPressed: _cancelNewList,
              )
            : null,
        title: listAsync.when(
          data: (list) {
            _initTitleForNewList(list.title);
            return _isEditingTitle
                ? TextField(
                    controller: _titleController,
                    focusNode: _titleFocusNode,
                    decoration: const InputDecoration(
                      border: InputBorder.none,
                      hintText: 'List title',
                    ),
                    style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w600,
                    ),
                    onSubmitted: _updateTitle,
                  )
                : GestureDetector(
                    onTap: () {
                      _titleController.text = list.title;
                      setState(() => _isEditingTitle = true);
                    },
                    child: Text(
                      list.title,
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
                  );
          },
          loading: () => const Text('Loading...'),
          error: (_, __) => const Text('Error'),
        ),
        actions: [
          if (_isEditingTitle)
            IconButton(
              icon: const Icon(Icons.check),
              onPressed: () => _updateTitle(_titleController.text),
            )
          else ...[
            listAsync.when(
              data: (list) => list.isOwner
                  ? IconButton(
                      icon: const Icon(Icons.people),
                      onPressed: () => Navigator.push(
                        context,
                        MaterialPageRoute(
                          builder: (context) => ShareListPage(
                            listId: widget.listId,
                            listTitle: list.title,
                          ),
                        ),
                      ),
                    )
                  : const SizedBox.shrink(),
              loading: () => const SizedBox.shrink(),
              error: (_, __) => const SizedBox.shrink(),
            ),
            PopupMenuButton<String>(
              onSelected: (value) {
                if (value == 'delete') _deleteList();
              },
              itemBuilder: (context) => [
                const PopupMenuItem(
                  value: 'delete',
                  child: Row(
                    children: [
                      Icon(Icons.delete, color: Colors.red, size: 20),
                      SizedBox(width: 8),
                      Text('Delete List', style: TextStyle(color: Colors.red)),
                    ],
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
      body: listAsync.when(
        data: (list) => _buildListContent(list),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.error_outline, size: 64, color: Colors.red.shade300),
              const SizedBox(height: 16),
              Text('Error: $error'),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () => ref
                    .invalidate(shoppingListDetailProvider(widget.listId)),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () {
          setState(() => _isAddingItem = true);
          _addItemFocusNode.requestFocus();
        },
        child: const Icon(Icons.add),
      ),
    );
  }

  Widget _buildListContent(ShoppingList list) {
    final uncheckedItems = list.uncheckedItems;
    final checkedItems = list.checkedItems;

    return Column(
      children: [
        if (_isAddingItem) _buildAddItemField(),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            children: [
              ...uncheckedItems.map((item) => _buildItemTile(item)),
              if (list.items.isEmpty && !_isAddingItem) _buildEmptyState(),
              if (checkedItems.isNotEmpty) _buildCheckedSection(checkedItems),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAddItemField() {
    final suggestionsAsync = ref.watch(itemSuggestionsProvider(_searchQuery));

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: Colors.grey.shade50,
            border: Border(bottom: BorderSide(color: Colors.grey.shade200)),
          ),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _addItemController,
                  focusNode: _addItemFocusNode,
                  decoration: InputDecoration(
                    hintText: 'Add item...',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                      borderSide: BorderSide(color: Colors.grey.shade300),
                    ),
                    contentPadding:
                        const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
                  ),
                  onSubmitted: (_) => _addItem(),
                ),
              ),
              const SizedBox(width: 8),
              IconButton(
                icon: const Icon(Icons.send),
                color: AppTheme.primaryBlue,
                onPressed: _addItem,
              ),
              IconButton(
                icon: const Icon(Icons.close),
                onPressed: () {
                  _addItemController.clear();
                  setState(() => _isAddingItem = false);
                },
              ),
            ],
          ),
        ),
        suggestionsAsync.when(
          data: (suggestions) {
            if (suggestions.isEmpty) return const SizedBox.shrink();
            return Container(
              constraints: const BoxConstraints(maxHeight: 200),
              decoration: BoxDecoration(
                color: Colors.white,
                border: Border(
                  bottom: BorderSide(color: Colors.grey.shade200),
                ),
              ),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: suggestions.length,
                itemBuilder: (context, index) {
                  final suggestion = suggestions[index];
                  return ListTile(
                    leading: Icon(Icons.history, color: Colors.grey.shade400, size: 20),
                    title: Text(suggestion),
                    dense: true,
                    onTap: () {
                      _addItemController.text = suggestion;
                      _addItem();
                    },
                  );
                },
              ),
            );
          },
          loading: () => const SizedBox.shrink(),
          error: (_, __) => const SizedBox.shrink(),
        ),
      ],
    );
  }

  Widget _buildItemTile(ShoppingListItem item) {
    return Dismissible(
      key: Key(item.id),
      direction: DismissDirection.endToStart,
      background: Container(
        alignment: Alignment.centerRight,
        padding: const EdgeInsets.only(right: 16),
        color: Colors.red,
        child: const Icon(Icons.delete, color: Colors.white),
      ),
      onDismissed: (_) => _deleteItem(item),
      child: ListTile(
        leading: Checkbox(
          value: item.isChecked,
          onChanged: (_) => _toggleItem(item),
          activeColor: AppTheme.primaryBlue,
        ),
        title: Text(
          item.name,
          style: TextStyle(
            decoration: item.isChecked ? TextDecoration.lineThrough : null,
            color: item.isChecked ? Colors.grey : null,
          ),
        ),
        subtitle: item.quantity != 1
            ? Text('${item.quantity} ${item.unit}')
            : null,
        contentPadding: EdgeInsets.zero,
      ),
    );
  }

  Widget _buildCheckedSection(List<ShoppingListItem> checkedItems) {
    return Column(
      children: [
        const SizedBox(height: 8),
        InkWell(
          onTap: () => setState(() => _checkedExpanded = !_checkedExpanded),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 8),
            decoration: BoxDecoration(
              color: Colors.grey.shade100,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(
                  _checkedExpanded
                      ? Icons.keyboard_arrow_up
                      : Icons.keyboard_arrow_down,
                  color: Colors.grey.shade600,
                ),
                const SizedBox(width: 8),
                Text(
                  'Checked items (${checkedItems.length})',
                  style: TextStyle(
                    color: Colors.grey.shade600,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ],
            ),
          ),
        ),
        if (_checkedExpanded)
          ...checkedItems.map((item) => _buildItemTile(item)),
      ],
    );
  }

  Widget _buildEmptyState() {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 64),
      child: Column(
        children: [
          Icon(
            Icons.shopping_basket_outlined,
            size: 64,
            color: Colors.grey.shade300,
          ),
          const SizedBox(height: 16),
          Text(
            'No items yet',
            style: TextStyle(
              fontSize: 16,
              color: Colors.grey.shade600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Tap + to add your first item',
            style: TextStyle(
              fontSize: 14,
              color: Colors.grey.shade500,
            ),
          ),
        ],
      ),
    );
  }
}
