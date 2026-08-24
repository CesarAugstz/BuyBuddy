const Object _knowledgeUnset = Object();

DateTime _knowledgeDate(dynamic value) {
  return DateTime.tryParse(value?.toString() ?? '') ??
      DateTime.fromMillisecondsSinceEpoch(0, isUtc: true);
}

class KnowledgeTopic {
  const KnowledgeTopic({
    required this.id,
    required this.name,
    required this.depth,
    required this.pendingWriteCount,
    required this.createdAt,
    required this.updatedAt,
    this.parentId,
    this.description = '',
    this.organizationDueAt,
    this.organizationLeaseUntil,
    this.lastOrganizedAt,
  });

  final String id;
  final String? parentId;
  final String name;
  final String description;
  final int depth;
  final int pendingWriteCount;
  final DateTime? organizationDueAt;
  final DateTime? organizationLeaseUntil;
  final DateTime? lastOrganizedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  bool get isInbox => parentId == null && name.toLowerCase() == 'inbox';

  factory KnowledgeTopic.fromJson(Map<String, dynamic> json) {
    return KnowledgeTopic(
      id: json['id']?.toString() ?? '',
      parentId: json['parentId']?.toString(),
      name: json['name']?.toString() ?? '',
      description: json['description']?.toString() ?? '',
      depth: (json['depth'] as num?)?.toInt() ?? 0,
      pendingWriteCount: (json['pendingWriteCount'] as num?)?.toInt() ?? 0,
      organizationDueAt:
          json['organizationDueAt'] == null
              ? null
              : _knowledgeDate(json['organizationDueAt']),
      organizationLeaseUntil:
          json['organizationLeaseUntil'] == null
              ? null
              : _knowledgeDate(json['organizationLeaseUntil']),
      lastOrganizedAt:
          json['lastOrganizedAt'] == null
              ? null
              : _knowledgeDate(json['lastOrganizedAt']),
      createdAt: _knowledgeDate(json['createdAt']),
      updatedAt: _knowledgeDate(json['updatedAt']),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    if (parentId != null) 'parentId': parentId,
    'name': name,
    if (description.isNotEmpty) 'description': description,
    'depth': depth,
    'pendingWriteCount': pendingWriteCount,
    if (organizationDueAt != null)
      'organizationDueAt': organizationDueAt!.toIso8601String(),
    if (organizationLeaseUntil != null)
      'organizationLeaseUntil': organizationLeaseUntil!.toIso8601String(),
    if (lastOrganizedAt != null)
      'lastOrganizedAt': lastOrganizedAt!.toIso8601String(),
    'createdAt': createdAt.toIso8601String(),
    'updatedAt': updatedAt.toIso8601String(),
  };

  KnowledgeTopic copyWith({
    String? id,
    Object? parentId = _knowledgeUnset,
    String? name,
    String? description,
    int? depth,
    int? pendingWriteCount,
    Object? organizationDueAt = _knowledgeUnset,
    Object? organizationLeaseUntil = _knowledgeUnset,
    Object? lastOrganizedAt = _knowledgeUnset,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return KnowledgeTopic(
      id: id ?? this.id,
      parentId:
          identical(parentId, _knowledgeUnset)
              ? this.parentId
              : parentId as String?,
      name: name ?? this.name,
      description: description ?? this.description,
      depth: depth ?? this.depth,
      pendingWriteCount: pendingWriteCount ?? this.pendingWriteCount,
      organizationDueAt:
          identical(organizationDueAt, _knowledgeUnset)
              ? this.organizationDueAt
              : organizationDueAt as DateTime?,
      organizationLeaseUntil:
          identical(organizationLeaseUntil, _knowledgeUnset)
              ? this.organizationLeaseUntil
              : organizationLeaseUntil as DateTime?,
      lastOrganizedAt:
          identical(lastOrganizedAt, _knowledgeUnset)
              ? this.lastOrganizedAt
              : lastOrganizedAt as DateTime?,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

class KnowledgeOrganizationResult {
  const KnowledgeOrganizationResult({
    required this.operationsApplied,
    required this.changedEntryIds,
    required this.createdTopicIds,
    required this.affectedTopicIds,
  });

  final int operationsApplied;
  final List<String> changedEntryIds;
  final List<String> createdTopicIds;
  final List<String> affectedTopicIds;

  factory KnowledgeOrganizationResult.fromJson(Map<String, dynamic> json) {
    List<String> strings(String key) =>
        (json[key] as List<dynamic>? ?? const [])
            .map((value) => value.toString())
            .toList();
    return KnowledgeOrganizationResult(
      operationsApplied: (json['operationsApplied'] as num?)?.toInt() ?? 0,
      changedEntryIds: strings('changedEntryIds'),
      createdTopicIds: strings('createdTopicIds'),
      affectedTopicIds: strings('affectedTopicIds'),
    );
  }

  Map<String, dynamic> toJson() => {
    'operationsApplied': operationsApplied,
    'changedEntryIds': changedEntryIds,
    'createdTopicIds': createdTopicIds,
    'affectedTopicIds': affectedTopicIds,
  };
}

class KnowledgeOrganizationResponse {
  const KnowledgeOrganizationResponse({
    required this.status,
    required this.topic,
    required this.result,
  });

  final String status;
  final KnowledgeTopic topic;
  final KnowledgeOrganizationResult result;

  factory KnowledgeOrganizationResponse.fromJson(Map<String, dynamic> json) {
    return KnowledgeOrganizationResponse(
      status: json['status']?.toString() ?? '',
      topic: KnowledgeTopic.fromJson(
        Map<String, dynamic>.from(json['topic'] as Map),
      ),
      result: KnowledgeOrganizationResult.fromJson(
        Map<String, dynamic>.from(json['result'] as Map),
      ),
    );
  }

  Map<String, dynamic> toJson() => {
    'status': status,
    'topic': topic.toJson(),
    'result': result.toJson(),
  };
}

class KnowledgeTopicNode {
  const KnowledgeTopicNode({
    required this.topic,
    required this.entryCount,
    required this.childCount,
    this.children = const [],
  });

  final KnowledgeTopic topic;
  final int entryCount;
  final int childCount;
  final List<KnowledgeTopicNode> children;

  factory KnowledgeTopicNode.fromJson(Map<String, dynamic> json) {
    final children =
        (json['children'] as List<dynamic>? ?? const [])
            .map(
              (item) => KnowledgeTopicNode.fromJson(
                Map<String, dynamic>.from(item as Map),
              ),
            )
            .toList();
    return KnowledgeTopicNode(
      topic: KnowledgeTopic.fromJson(json),
      entryCount: (json['entryCount'] as num?)?.toInt() ?? 0,
      childCount: (json['childCount'] as num?)?.toInt() ?? children.length,
      children: children,
    );
  }

  Map<String, dynamic> toJson() => {
    ...topic.toJson(),
    'entryCount': entryCount,
    'childCount': childCount,
    'children': children.map((child) => child.toJson()).toList(),
  };

  KnowledgeTopicNode copyWith({
    KnowledgeTopic? topic,
    int? entryCount,
    int? childCount,
    List<KnowledgeTopicNode>? children,
  }) {
    return KnowledgeTopicNode(
      topic: topic ?? this.topic,
      entryCount: entryCount ?? this.entryCount,
      childCount: childCount ?? this.childCount,
      children: children ?? this.children,
    );
  }
}

class KnowledgeTopicDetail {
  const KnowledgeTopicDetail({
    required this.topic,
    required this.entryCount,
    required this.childCount,
    required this.breadcrumb,
  });

  final KnowledgeTopic topic;
  final int entryCount;
  final int childCount;
  final List<KnowledgeTopic> breadcrumb;

  factory KnowledgeTopicDetail.fromJson(Map<String, dynamic> json) {
    return KnowledgeTopicDetail(
      topic: KnowledgeTopic.fromJson(json),
      entryCount: (json['entryCount'] as num?)?.toInt() ?? 0,
      childCount: (json['childCount'] as num?)?.toInt() ?? 0,
      breadcrumb:
          (json['breadcrumb'] as List<dynamic>? ?? const [])
              .map(
                (item) => KnowledgeTopic.fromJson(
                  Map<String, dynamic>.from(item as Map),
                ),
              )
              .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    ...topic.toJson(),
    'entryCount': entryCount,
    'childCount': childCount,
    'breadcrumb': breadcrumb.map((part) => part.toJson()).toList(),
  };
}

class KnowledgeEntry {
  const KnowledgeEntry({
    required this.id,
    required this.topicId,
    required this.kind,
    required this.title,
    required this.body,
    required this.attributes,
    required this.tags,
    required this.source,
    required this.status,
    required this.version,
    required this.createdAt,
    required this.updatedAt,
    this.occurredAt,
    this.topic,
  });

  final String id;
  final String topicId;
  final String kind;
  final String title;
  final String body;
  final Map<String, dynamic> attributes;
  final List<String> tags;
  final DateTime? occurredAt;
  final String source;
  final String status;
  final int version;
  final DateTime createdAt;
  final DateTime updatedAt;
  final KnowledgeTopic? topic;

  DateTime get displayDate => occurredAt ?? updatedAt;

  factory KnowledgeEntry.fromJson(Map<String, dynamic> json) {
    return KnowledgeEntry(
      id: json['id']?.toString() ?? '',
      topicId: json['topicId']?.toString() ?? '',
      kind: json['kind']?.toString() ?? 'note',
      title: json['title']?.toString() ?? '',
      body: json['body']?.toString() ?? '',
      attributes:
          json['attributes'] is Map
              ? Map<String, dynamic>.from(json['attributes'] as Map)
              : <String, dynamic>{},
      tags:
          (json['tags'] as List<dynamic>? ?? const [])
              .map((tag) => tag.toString())
              .toList(),
      occurredAt:
          json['occurredAt'] == null
              ? null
              : _knowledgeDate(json['occurredAt']),
      source: json['source']?.toString() ?? '',
      status: json['status']?.toString() ?? 'active',
      version: (json['version'] as num?)?.toInt() ?? 1,
      createdAt: _knowledgeDate(json['createdAt']),
      updatedAt: _knowledgeDate(json['updatedAt']),
      topic:
          json['topic'] is Map
              ? KnowledgeTopic.fromJson(
                Map<String, dynamic>.from(json['topic'] as Map),
              )
              : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'topicId': topicId,
    'kind': kind,
    'title': title,
    'body': body,
    'attributes': attributes,
    'tags': tags,
    if (occurredAt != null) 'occurredAt': occurredAt!.toIso8601String(),
    'source': source,
    'status': status,
    'version': version,
    'createdAt': createdAt.toIso8601String(),
    'updatedAt': updatedAt.toIso8601String(),
    if (topic != null) 'topic': topic!.toJson(),
  };

  KnowledgeEntry copyWith({
    String? id,
    String? topicId,
    String? kind,
    String? title,
    String? body,
    Map<String, dynamic>? attributes,
    List<String>? tags,
    Object? occurredAt = _knowledgeUnset,
    String? source,
    String? status,
    int? version,
    DateTime? createdAt,
    DateTime? updatedAt,
    Object? topic = _knowledgeUnset,
  }) {
    return KnowledgeEntry(
      id: id ?? this.id,
      topicId: topicId ?? this.topicId,
      kind: kind ?? this.kind,
      title: title ?? this.title,
      body: body ?? this.body,
      attributes: attributes ?? this.attributes,
      tags: tags ?? this.tags,
      occurredAt:
          identical(occurredAt, _knowledgeUnset)
              ? this.occurredAt
              : occurredAt as DateTime?,
      source: source ?? this.source,
      status: status ?? this.status,
      version: version ?? this.version,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      topic:
          identical(topic, _knowledgeUnset)
              ? this.topic
              : topic as KnowledgeTopic?,
    );
  }
}

class KnowledgeSearchResult {
  const KnowledgeSearchResult({required this.entry, required this.breadcrumb});

  final KnowledgeEntry entry;
  final List<KnowledgeTopic> breadcrumb;

  factory KnowledgeSearchResult.fromJson(Map<String, dynamic> json) {
    return KnowledgeSearchResult(
      entry: KnowledgeEntry.fromJson(
        Map<String, dynamic>.from(json['entry'] as Map),
      ),
      breadcrumb:
          (json['breadcrumb'] as List<dynamic>? ?? const [])
              .map(
                (item) => KnowledgeTopic.fromJson(
                  Map<String, dynamic>.from(item as Map),
                ),
              )
              .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    'entry': entry.toJson(),
    'breadcrumb': breadcrumb.map((part) => part.toJson()).toList(),
  };
}

class KnowledgeSearchFilter {
  const KnowledgeSearchFilter({
    this.query = '',
    this.topicId,
    this.includeChildren = false,
    this.kind = '',
    this.tag = '',
    this.occurredFrom,
    this.occurredTo,
    this.limit = 30,
  });

  final String query;
  final String? topicId;
  final bool includeChildren;
  final String kind;
  final String tag;
  final DateTime? occurredFrom;
  final DateTime? occurredTo;
  final int limit;
}

class KnowledgeDirectorySelection {
  const KnowledgeDirectorySelection({
    required this.breadcrumb,
    required this.children,
    this.topic,
  });

  final KnowledgeTopicNode? topic;
  final List<KnowledgeTopic> breadcrumb;
  final List<KnowledgeTopicNode> children;
}

List<KnowledgeTopicNode> flattenKnowledgeTopics(
  List<KnowledgeTopicNode> roots,
) {
  final result = <KnowledgeTopicNode>[];
  void addNode(KnowledgeTopicNode node) {
    result.add(node);
    for (final child in node.children) {
      addNode(child);
    }
  }

  for (final root in roots) {
    addNode(root);
  }
  return result;
}

KnowledgeTopicNode? findKnowledgeTopic(
  List<KnowledgeTopicNode> roots,
  String topicId,
) {
  for (final node in flattenKnowledgeTopics(roots)) {
    if (node.topic.id == topicId) return node;
  }
  return null;
}

KnowledgeDirectorySelection selectKnowledgeDirectory(
  List<KnowledgeTopicNode> roots,
  String? topicId,
) {
  if (topicId == null) {
    return KnowledgeDirectorySelection(breadcrumb: const [], children: roots);
  }
  final node = findKnowledgeTopic(roots, topicId);
  if (node == null) {
    return const KnowledgeDirectorySelection(breadcrumb: [], children: []);
  }

  final byId = {
    for (final candidate in flattenKnowledgeTopics(roots))
      candidate.topic.id: candidate.topic,
  };
  final reversed = <KnowledgeTopic>[];
  KnowledgeTopic? current = node.topic;
  while (current != null && reversed.length < 5) {
    reversed.add(current);
    current = current.parentId == null ? null : byId[current.parentId!];
  }
  return KnowledgeDirectorySelection(
    topic: node,
    breadcrumb: reversed.reversed.toList(),
    children: node.children,
  );
}

List<KnowledgeTopicNode> rebuildKnowledgeTopicTree(
  Iterable<KnowledgeTopicNode> nodes,
) {
  final all = nodes.toList();
  final byParent = <String?, List<KnowledgeTopicNode>>{};
  for (final node in all) {
    byParent.putIfAbsent(node.topic.parentId, () => []).add(node);
  }

  int compare(KnowledgeTopicNode left, KnowledgeTopicNode right) {
    if (left.topic.isInbox != right.topic.isInbox) {
      return left.topic.isInbox ? -1 : 1;
    }
    return left.topic.name.toLowerCase().compareTo(
      right.topic.name.toLowerCase(),
    );
  }

  KnowledgeTopicNode build(KnowledgeTopicNode source, int depth) {
    final children = [...?byParent[source.topic.id]]..sort(compare);
    final builtChildren =
        children.map((child) => build(child, depth + 1)).toList();
    return source.copyWith(
      topic: source.topic.copyWith(depth: depth),
      childCount: builtChildren.length,
      children: builtChildren,
    );
  }

  final roots = [...?byParent[null]]..sort(compare);
  return roots.map((root) => build(root, 0)).toList();
}

List<KnowledgeTopicNode> replaceKnowledgeTopic(
  List<KnowledgeTopicNode> roots,
  KnowledgeTopic topic,
) {
  final flat =
      flattenKnowledgeTopics(roots)
          .map(
            (node) =>
                node.topic.id == topic.id ? node.copyWith(topic: topic) : node,
          )
          .toList();
  return rebuildKnowledgeTopicTree(flat);
}

List<KnowledgeTopicNode> removeKnowledgeTopic(
  List<KnowledgeTopicNode> roots,
  String topicId,
) {
  final flat =
      flattenKnowledgeTopics(
        roots,
      ).where((node) => node.topic.id != topicId).toList();
  return rebuildKnowledgeTopicTree(flat);
}

List<KnowledgeTopicNode> adjustKnowledgeEntryCount(
  List<KnowledgeTopicNode> roots,
  String topicId,
  int delta,
) {
  return roots
      .map(
        (node) => node.copyWith(
          entryCount:
              node.topic.id == topicId
                  ? (node.entryCount + delta).clamp(0, 1 << 31)
                  : node.entryCount,
          children: adjustKnowledgeEntryCount(node.children, topicId, delta),
        ),
      )
      .toList();
}
