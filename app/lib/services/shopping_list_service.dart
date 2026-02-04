import 'dart:convert';
import 'package:http/http.dart' as http;
import '../config/api_config.dart';
import 'auth_service.dart';

class ShoppingListItem {
  final String id;
  final String listId;
  final String name;
  final double quantity;
  final String unit;
  final bool isChecked;
  final int sortOrder;
  final DateTime createdAt;
  final DateTime updatedAt;

  ShoppingListItem({
    required this.id,
    required this.listId,
    required this.name,
    required this.quantity,
    required this.unit,
    required this.isChecked,
    required this.sortOrder,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ShoppingListItem.fromJson(Map<String, dynamic> json) {
    return ShoppingListItem(
      id: json['id'] ?? '',
      listId: json['listId'] ?? '',
      name: json['name'] ?? '',
      quantity: (json['quantity'] ?? 1).toDouble(),
      unit: json['unit'] ?? 'un',
      isChecked: json['isChecked'] ?? false,
      sortOrder: json['sortOrder'] ?? 0,
      createdAt: DateTime.tryParse(json['createdAt'] ?? '') ?? DateTime.now(),
      updatedAt: DateTime.tryParse(json['updatedAt'] ?? '') ?? DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'listId': listId,
        'name': name,
        'quantity': quantity,
        'unit': unit,
        'isChecked': isChecked,
        'sortOrder': sortOrder,
      };

  ShoppingListItem copyWith({
    String? id,
    String? listId,
    String? name,
    double? quantity,
    String? unit,
    bool? isChecked,
    int? sortOrder,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return ShoppingListItem(
      id: id ?? this.id,
      listId: listId ?? this.listId,
      name: name ?? this.name,
      quantity: quantity ?? this.quantity,
      unit: unit ?? this.unit,
      isChecked: isChecked ?? this.isChecked,
      sortOrder: sortOrder ?? this.sortOrder,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

class ShoppingListOwner {
  final String id;
  final String name;
  final String email;
  final String? photoUrl;

  ShoppingListOwner({
    required this.id,
    required this.name,
    required this.email,
    this.photoUrl,
  });

  factory ShoppingListOwner.fromJson(Map<String, dynamic> json) {
    return ShoppingListOwner(
      id: json['id'] ?? '',
      name: json['name'] ?? '',
      email: json['email'] ?? '',
      photoUrl: json['photoUrl'],
    );
  }
}

class ShoppingList {
  final String id;
  final String title;
  final String? description;
  final String ownerId;
  final ShoppingListOwner? owner;
  final DateTime createdAt;
  final DateTime updatedAt;
  final List<ShoppingListItem> items;
  final int itemCount;
  final int checkedCount;
  final bool isShared;
  final bool isOwner;
  final int sharedWithCount;

  ShoppingList({
    required this.id,
    required this.title,
    this.description,
    required this.ownerId,
    this.owner,
    required this.createdAt,
    required this.updatedAt,
    this.items = const [],
    this.itemCount = 0,
    this.checkedCount = 0,
    this.isShared = false,
    this.isOwner = true,
    this.sharedWithCount = 0,
  });

  factory ShoppingList.fromJson(Map<String, dynamic> json) {
    return ShoppingList(
      id: json['id'] ?? '',
      title: json['title'] ?? '',
      description: json['description'],
      ownerId: json['ownerId'] ?? '',
      owner: json['owner'] != null
          ? ShoppingListOwner.fromJson(json['owner'])
          : null,
      createdAt: DateTime.tryParse(json['createdAt'] ?? '') ?? DateTime.now(),
      updatedAt: DateTime.tryParse(json['updatedAt'] ?? '') ?? DateTime.now(),
      items: (json['items'] as List<dynamic>?)
              ?.map((e) => ShoppingListItem.fromJson(e))
              .toList() ??
          [],
      itemCount: json['itemCount'] ?? 0,
      checkedCount: json['checkedCount'] ?? 0,
      isShared: json['isShared'] ?? false,
      isOwner: json['isOwner'] ?? true,
      sharedWithCount: json['sharedWithCount'] ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'description': description,
        'ownerId': ownerId,
      };

  int get uncheckedCount => itemCount - checkedCount;

  List<ShoppingListItem> get uncheckedItems =>
      items.where((item) => !item.isChecked).toList();

  List<ShoppingListItem> get checkedItems =>
      items.where((item) => item.isChecked).toList();

  ShoppingList copyWith({
    String? id,
    String? title,
    String? description,
    String? ownerId,
    ShoppingListOwner? owner,
    DateTime? createdAt,
    DateTime? updatedAt,
    List<ShoppingListItem>? items,
    int? itemCount,
    int? checkedCount,
    bool? isShared,
    bool? isOwner,
    int? sharedWithCount,
  }) {
    return ShoppingList(
      id: id ?? this.id,
      title: title ?? this.title,
      description: description ?? this.description,
      ownerId: ownerId ?? this.ownerId,
      owner: owner ?? this.owner,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      items: items ?? this.items,
      itemCount: itemCount ?? this.itemCount,
      checkedCount: checkedCount ?? this.checkedCount,
      isShared: isShared ?? this.isShared,
      isOwner: isOwner ?? this.isOwner,
      sharedWithCount: sharedWithCount ?? this.sharedWithCount,
    );
  }
}

class ShoppingListService {
  final _authService = AuthService();

  Future<List<ShoppingList>> getLists() async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body) as List<dynamic>;
      return data.map((e) => ShoppingList.fromJson(e)).toList();
    } else {
      throw Exception('Failed to load shopping lists: ${response.statusCode}');
    }
  }

  Future<ShoppingList> getList(String id) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$id'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      return ShoppingList.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to load shopping list: ${response.statusCode}');
    }
  }

  Future<ShoppingList> createList(String title, {String? description}) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.post(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({
        'title': title,
        if (description != null) 'description': description,
      }),
    );

    if (response.statusCode == 201) {
      return ShoppingList.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to create shopping list: ${response.statusCode}');
    }
  }

  Future<ShoppingList> updateList(String id,
      {String? title, String? description}) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$id'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({
        if (title != null) 'title': title,
        if (description != null) 'description': description,
      }),
    );

    if (response.statusCode == 200) {
      return ShoppingList.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to update shopping list: ${response.statusCode}');
    }
  }

  Future<void> deleteList(String id) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.delete(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$id'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode != 204) {
      throw Exception('Failed to delete shopping list: ${response.statusCode}');
    }
  }

  Future<ShoppingListItem> addItem(String listId, String name,
      {double quantity = 1, String unit = 'un'}) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.post(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/items'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({
        'name': name,
        'quantity': quantity,
        'unit': unit,
      }),
    );

    if (response.statusCode == 201) {
      return ShoppingListItem.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to add item: ${response.statusCode}');
    }
  }

  Future<ShoppingListItem> updateItem(
    String listId,
    String itemId, {
    String? name,
    double? quantity,
    String? unit,
    bool? isChecked,
    int? sortOrder,
  }) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/items/$itemId'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({
        if (name != null) 'name': name,
        if (quantity != null) 'quantity': quantity,
        if (unit != null) 'unit': unit,
        if (isChecked != null) 'isChecked': isChecked,
        if (sortOrder != null) 'sortOrder': sortOrder,
      }),
    );

    if (response.statusCode == 200) {
      return ShoppingListItem.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to update item: ${response.statusCode}');
    }
  }

  Future<void> deleteItem(String listId, String itemId) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.delete(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/items/$itemId'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode != 204) {
      throw Exception('Failed to delete item: ${response.statusCode}');
    }
  }

  Future<void> reorderItems(
      String listId, List<Map<String, dynamic>> items) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/items/reorder'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({'items': items}),
    );

    if (response.statusCode != 204) {
      throw Exception('Failed to reorder items: ${response.statusCode}');
    }
  }

  Future<List<String>> getSuggestions(String query) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse(
          '${ApiConfig.baseUrl}/shopping-lists/suggestions?q=${Uri.encodeComponent(query)}'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body) as List<dynamic>;
      return data.map((e) => e.toString()).toList();
    } else {
      throw Exception('Failed to get suggestions: ${response.statusCode}');
    }
  }

  Future<List<SearchUser>> searchUsers(String email) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse(
          '${ApiConfig.baseUrl}/users/search?email=${Uri.encodeComponent(email)}'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body) as List<dynamic>;
      return data.map((e) => SearchUser.fromJson(e)).toList();
    } else {
      throw Exception('Failed to search users: ${response.statusCode}');
    }
  }

  Future<ShoppingListShare> shareList(String listId, String email) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.post(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/share'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode({'email': email}),
    );

    if (response.statusCode == 201) {
      return ShoppingListShare.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 404) {
      throw Exception('User not found');
    } else if (response.statusCode == 409) {
      throw Exception('User already has access');
    } else {
      throw Exception('Failed to share list: ${response.statusCode}');
    }
  }

  Future<List<ShoppingListShare>> getListShares(String listId) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/shares'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body) as List<dynamic>;
      return data.map((e) => ShoppingListShare.fromJson(e)).toList();
    } else {
      throw Exception('Failed to get shares: ${response.statusCode}');
    }
  }

  Future<void> removeShare(String listId, String userId) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.delete(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/$listId/share/$userId'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode != 204) {
      throw Exception('Failed to remove share: ${response.statusCode}');
    }
  }

  Future<List<ShoppingListShare>> getPendingInvites() async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.get(
      Uri.parse('${ApiConfig.baseUrl}/shopping-lists/invites'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body) as List<dynamic>;
      return data.map((e) => ShoppingListShare.fromJson(e)).toList();
    } else {
      throw Exception('Failed to get invites: ${response.statusCode}');
    }
  }

  Future<ShoppingListShare> acceptInvite(String inviteId) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse(
          '${ApiConfig.baseUrl}/shopping-lists/invites/$inviteId/accept'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      return ShoppingListShare.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to accept invite: ${response.statusCode}');
    }
  }

  Future<ShoppingListShare> rejectInvite(String inviteId) async {
    final token = await _authService.getApiToken();
    if (token == null) throw Exception('Not authenticated');

    final response = await http.put(
      Uri.parse(
          '${ApiConfig.baseUrl}/shopping-lists/invites/$inviteId/reject'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      return ShoppingListShare.fromJson(jsonDecode(response.body));
    } else {
      throw Exception('Failed to reject invite: ${response.statusCode}');
    }
  }
}

class SearchUser {
  final String id;
  final String name;
  final String email;
  final String? photoUrl;

  SearchUser({
    required this.id,
    required this.name,
    required this.email,
    this.photoUrl,
  });

  factory SearchUser.fromJson(Map<String, dynamic> json) {
    return SearchUser(
      id: json['id'] ?? '',
      name: json['name'] ?? '',
      email: json['email'] ?? '',
      photoUrl: json['photoUrl'],
    );
  }
}

class ShoppingListShare {
  final String id;
  final String listId;
  final String userId;
  final String invitedBy;
  final String status;
  final DateTime createdAt;
  final SearchUser? user;
  final SearchUser? inviter;
  final ShoppingList? list;

  ShoppingListShare({
    required this.id,
    required this.listId,
    required this.userId,
    required this.invitedBy,
    required this.status,
    required this.createdAt,
    this.user,
    this.inviter,
    this.list,
  });

  factory ShoppingListShare.fromJson(Map<String, dynamic> json) {
    return ShoppingListShare(
      id: json['id'] ?? '',
      listId: json['listId'] ?? '',
      userId: json['userId'] ?? '',
      invitedBy: json['invitedBy'] ?? '',
      status: json['status'] ?? 'pending',
      createdAt: DateTime.tryParse(json['createdAt'] ?? '') ?? DateTime.now(),
      user: json['user'] != null ? SearchUser.fromJson(json['user']) : null,
      inviter:
          json['inviter'] != null ? SearchUser.fromJson(json['inviter']) : null,
      list: json['list'] != null ? ShoppingList.fromJson(json['list']) : null,
    );
  }

  bool get isPending => status == 'pending';
  bool get isAccepted => status == 'accepted';
}
