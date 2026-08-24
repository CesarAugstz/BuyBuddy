package repository

import (
	"buybuddy-api/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxKnowledgeDepth      = 4
	maxKnowledgeTitleRunes = 200
	maxKnowledgeBodyRunes  = 50000
	maxKnowledgeTags       = 20
	maxKnowledgeTagRunes   = 64
	maxKnowledgeAttributes = 64 * 1024
	inboxFallbackDedupAge  = 10 * time.Minute
)

var (
	ErrKnowledgeNotFound = errors.New("knowledge item not found")
	ErrKnowledgeConflict = errors.New("knowledge item conflict")
	ErrKnowledgeInvalid  = errors.New("invalid knowledge operation")
)

type KnowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

func NormalizeKnowledgeName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func NormalizeKnowledgeTags(tags []string) ([]string, error) {
	if len(tags) > maxKnowledgeTags {
		return nil, fmt.Errorf("%w: at most %d tags are allowed", ErrKnowledgeInvalid, maxKnowledgeTags)
	}

	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > maxKnowledgeTagRunes {
			return nil, fmt.Errorf("%w: tags must be %d characters or fewer", ErrKnowledgeInvalid, maxKnowledgeTagRunes)
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result, nil
}

func validateTopicName(name string) error {
	length := utf8.RuneCountInString(strings.TrimSpace(name))
	if length == 0 || length > 120 {
		return fmt.Errorf("%w: topic name must be between 1 and 120 characters", ErrKnowledgeInvalid)
	}
	return nil
}

func validateEntryFields(kind, title, body string, attributes models.KnowledgeAttributes, tags []string) error {
	kind = strings.TrimSpace(kind)
	if kind == "" || utf8.RuneCountInString(kind) > 64 {
		return fmt.Errorf("%w: kind must be between 1 and 64 characters", ErrKnowledgeInvalid)
	}
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxKnowledgeTitleRunes {
		return fmt.Errorf("%w: title must be between 1 and %d characters", ErrKnowledgeInvalid, maxKnowledgeTitleRunes)
	}
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > maxKnowledgeBodyRunes {
		return fmt.Errorf("%w: body must be between 1 and %d characters", ErrKnowledgeInvalid, maxKnowledgeBodyRunes)
	}
	if _, err := NormalizeKnowledgeTags(tags); err != nil {
		return err
	}
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return fmt.Errorf("%w: attributes must be valid JSON", ErrKnowledgeInvalid)
	}
	if len(encoded) > maxKnowledgeAttributes {
		return fmt.Errorf("%w: attributes are too large", ErrKnowledgeInvalid)
	}
	return nil
}

func cloneKnowledgeAttributes(attributes models.KnowledgeAttributes) models.KnowledgeAttributes {
	if len(attributes) == 0 {
		return models.KnowledgeAttributes{}
	}
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return models.KnowledgeAttributes{}
	}
	var cloned models.KnowledgeAttributes
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return models.KnowledgeAttributes{}
	}
	return cloned
}

func applyKnowledgeAttributeMutation(
	current models.KnowledgeAttributes,
	incoming *models.KnowledgeAttributes,
	replace bool,
) (models.KnowledgeAttributes, error) {
	if replace {
		if incoming == nil {
			return nil, fmt.Errorf("%w: attributes are required when replaceAttributes is true", ErrKnowledgeInvalid)
		}
		return cloneKnowledgeAttributes(*incoming), nil
	}

	attributes := cloneKnowledgeAttributes(current)
	if incoming != nil {
		for key, value := range *incoming {
			attributes[key] = value
		}
	}
	return attributes, nil
}

func (r *KnowledgeRepository) EnsureInbox(userID string) (*models.KnowledgeTopic, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: user ID is required", ErrKnowledgeInvalid)
	}

	var inbox models.KnowledgeTopic
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		inbox, err = ensureInboxTx(tx, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &inbox, nil
}

func lockKnowledgeTopicHierarchyTx(tx *gorm.DB, userID string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "knowledge-topic-hierarchy:"+userID).Error
}

func ensureInboxTx(tx *gorm.DB, userID string) (models.KnowledgeTopic, error) {
	if err := lockKnowledgeTopicHierarchyTx(tx, userID); err != nil {
		return models.KnowledgeTopic{}, err
	}

	var inbox models.KnowledgeTopic
	err := tx.Where("user_id = ? AND parent_id IS NULL AND normalized_name = ?", userID, NormalizeKnowledgeName(models.KnowledgeInboxName)).
		First(&inbox).Error
	if err == nil {
		return inbox, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.KnowledgeTopic{}, err
	}

	inbox = models.KnowledgeTopic{
		UserID:         userID,
		Name:           models.KnowledgeInboxName,
		NormalizedName: NormalizeKnowledgeName(models.KnowledgeInboxName),
		Depth:          0,
	}
	if err := tx.Create(&inbox).Error; err != nil {
		return models.KnowledgeTopic{}, err
	}
	return inbox, nil
}

func (r *KnowledgeRepository) CreateInboxFallback(ctx context.Context, userID, body, title string) (*models.KnowledgeEntry, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(userID) == "" {
		return nil, false, fmt.Errorf("%w: user ID is required", ErrKnowledgeInvalid)
	}
	body = strings.TrimSpace(body)
	entry := &models.KnowledgeEntry{
		Kind:  "note",
		Title: strings.TrimSpace(title),
		Body:  body,
		Attributes: models.KnowledgeAttributes{
			"classificationFallback": true,
		},
		Source: models.KnowledgeSourceAssistant,
	}
	if err := validateEntryFields(entry.Kind, entry.Title, entry.Body, entry.Attributes, nil); err != nil {
		return nil, false, err
	}

	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inbox, err := ensureInboxTx(tx, userID)
		if err != nil {
			return err
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "knowledge-inbox-fallback:"+userID).Error; err != nil {
			return err
		}

		var existing models.KnowledgeEntry
		err = tx.Where(
			"user_id = ? AND topic_id = ? AND kind = ? AND source = ? AND status = ? AND body = ? AND created_at >= ? AND attributes ->> 'classificationFallback' = 'true'",
			userID,
			inbox.ID,
			entry.Kind,
			entry.Source,
			models.KnowledgeStatusActive,
			body,
			time.Now().Add(-inboxFallbackDedupAge),
		).Order("created_at DESC").First(&existing).Error
		if err == nil {
			*entry = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		entry.UserID = userID
		entry.TopicID = inbox.ID
		entry.Status = models.KnowledgeStatusActive
		entry.Version = 1
		entry.Tags = pq.StringArray{}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		if err := scheduleKnowledgeTopicWritesTx(tx, userID, []string{inbox.ID}, time.Now()); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return entry, created, nil
}

func (r *KnowledgeRepository) ListTopics(userID string) ([]models.KnowledgeTopic, error) {
	if _, err := r.EnsureInbox(userID); err != nil {
		return nil, err
	}
	var topics []models.KnowledgeTopic
	err := r.db.Where("user_id = ?", userID).
		Order("depth ASC, normalized_name ASC").
		Find(&topics).Error
	return topics, err
}

func (r *KnowledgeRepository) ListTopicTree(userID string) ([]models.KnowledgeTopicNode, error) {
	topics, err := r.ListTopics(userID)
	if err != nil {
		return nil, err
	}

	type countRow struct {
		TopicID string
		Count   int
	}
	var rows []countRow
	if err := r.db.Model(&models.KnowledgeEntry{}).
		Select("topic_id, count(*) AS count").
		Where("user_id = ? AND status = ?", userID, models.KnowledgeStatusActive).
		Group("topic_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.TopicID] = row.Count
	}
	return BuildKnowledgeTopicTree(topics, counts), nil
}

func BuildKnowledgeTopicTree(topics []models.KnowledgeTopic, entryCounts map[string]int) []models.KnowledgeTopicNode {
	children := make(map[string][]models.KnowledgeTopic)
	roots := make([]models.KnowledgeTopic, 0)
	for _, topic := range topics {
		if topic.ParentID == nil {
			roots = append(roots, topic)
			continue
		}
		children[*topic.ParentID] = append(children[*topic.ParentID], topic)
	}

	sortTopics := func(values []models.KnowledgeTopic) {
		sort.SliceStable(values, func(i, j int) bool {
			iInbox := NormalizeKnowledgeName(values[i].Name) == NormalizeKnowledgeName(models.KnowledgeInboxName)
			jInbox := NormalizeKnowledgeName(values[j].Name) == NormalizeKnowledgeName(models.KnowledgeInboxName)
			if iInbox != jInbox {
				return iInbox
			}
			return values[i].NormalizedName < values[j].NormalizedName
		})
	}
	sortTopics(roots)
	for key := range children {
		sortTopics(children[key])
	}

	var build func(models.KnowledgeTopic) models.KnowledgeTopicNode
	build = func(topic models.KnowledgeTopic) models.KnowledgeTopicNode {
		directChildren := children[topic.ID]
		node := models.KnowledgeTopicNode{
			KnowledgeTopic: topic,
			EntryCount:     entryCounts[topic.ID],
			ChildCount:     len(directChildren),
			Children:       make([]models.KnowledgeTopicNode, 0, len(directChildren)),
		}
		for _, child := range directChildren {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	result := make([]models.KnowledgeTopicNode, 0, len(roots))
	for _, root := range roots {
		result = append(result, build(root))
	}
	return result
}

func (r *KnowledgeRepository) GetTopic(userID, topicID string) (*models.KnowledgeTopic, error) {
	var topic models.KnowledgeTopic
	err := r.db.Where("id = ? AND user_id = ?", topicID, userID).First(&topic).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &topic, nil
}

func (r *KnowledgeRepository) GetTopicDetail(userID, topicID string) (*models.KnowledgeTopicDetail, error) {
	topic, err := r.GetTopic(userID, topicID)
	if err != nil {
		return nil, err
	}

	var entryCount, childCount int64
	if err := r.db.Model(&models.KnowledgeEntry{}).
		Where("user_id = ? AND topic_id = ? AND status = ?", userID, topicID, models.KnowledgeStatusActive).
		Count(&entryCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.KnowledgeTopic{}).
		Where("user_id = ? AND parent_id = ?", userID, topicID).
		Count(&childCount).Error; err != nil {
		return nil, err
	}

	topics, err := r.ListTopics(userID)
	if err != nil {
		return nil, err
	}
	return &models.KnowledgeTopicDetail{
		KnowledgeTopic: *topic,
		EntryCount:     int(entryCount),
		ChildCount:     int(childCount),
		Breadcrumb:     knowledgeBreadcrumb(topic.ID, topics),
	}, nil
}

func (r *KnowledgeRepository) CreateTopic(userID string, topic *models.KnowledgeTopic) error {
	if err := validateTopicName(topic.Name); err != nil {
		return err
	}
	if utf8.RuneCountInString(topic.Description) > 1000 {
		return fmt.Errorf("%w: description must be 1000 characters or fewer", ErrKnowledgeInvalid)
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := ensureInboxTx(tx, userID); err != nil {
			return err
		}

		depth := 0
		if topic.ParentID != nil {
			var parent models.KnowledgeTopic
			if err := tx.Where("id = ? AND user_id = ?", *topic.ParentID, userID).First(&parent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrKnowledgeNotFound
				}
				return err
			}
			depth = parent.Depth + 1
		}
		if depth > maxKnowledgeDepth {
			return fmt.Errorf("%w: topic depth cannot exceed %d", ErrKnowledgeInvalid, maxKnowledgeDepth)
		}

		normalized := NormalizeKnowledgeName(topic.Name)
		query := tx.Model(&models.KnowledgeTopic{}).
			Where("user_id = ? AND normalized_name = ?", userID, normalized)
		if topic.ParentID == nil {
			query = query.Where("parent_id IS NULL")
		} else {
			query = query.Where("parent_id = ?", *topic.ParentID)
		}
		var duplicateCount int64
		if err := query.Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("%w: a topic with this name already exists", ErrKnowledgeConflict)
		}

		topic.UserID = userID
		topic.Name = strings.TrimSpace(topic.Name)
		topic.NormalizedName = normalized
		topic.Depth = depth
		topic.PendingWriteCount = 0
		topic.OrganizationDueAt = nil
		topic.OrganizationLeaseUntil = nil
		topic.LastOrganizedAt = nil
		return tx.Create(topic).Error
	})
}

func (r *KnowledgeRepository) UpdateTopic(userID, topicID string, request models.UpdateKnowledgeTopicRequest) (*models.KnowledgeTopic, error) {
	var updated models.KnowledgeTopic
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockKnowledgeTopicHierarchyTx(tx, userID); err != nil {
			return err
		}
		var topic models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", topicID, userID).
			First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}

		isInbox := topic.ParentID == nil && topic.NormalizedName == NormalizeKnowledgeName(models.KnowledgeInboxName)
		parentChanged := request.ParentID != nil || request.MoveToRoot
		if isInbox && (request.Name != nil || parentChanged) {
			return fmt.Errorf("%w: Inbox cannot be renamed or moved", ErrKnowledgeInvalid)
		}

		name := topic.Name
		if request.Name != nil {
			name = strings.TrimSpace(*request.Name)
			if err := validateTopicName(name); err != nil {
				return err
			}
		}
		description := topic.Description
		if request.Description != nil {
			description = strings.TrimSpace(*request.Description)
			if utf8.RuneCountInString(description) > 1000 {
				return fmt.Errorf("%w: description must be 1000 characters or fewer", ErrKnowledgeInvalid)
			}
		}

		parentID := topic.ParentID
		if request.MoveToRoot {
			parentID = nil
		} else if request.ParentID != nil {
			parentValue := strings.TrimSpace(*request.ParentID)
			parentID = &parentValue
		}
		if parentID != nil && *parentID == topic.ID {
			return fmt.Errorf("%w: a topic cannot be its own parent", ErrKnowledgeInvalid)
		}

		var allTopics []models.KnowledgeTopic
		if err := tx.Where("user_id = ?", userID).Find(&allTopics).Error; err != nil {
			return err
		}
		descendants := knowledgeDescendants(topic.ID, allTopics)
		if parentID != nil {
			if _, found := descendants[*parentID]; found {
				return fmt.Errorf("%w: a topic cannot be moved into its descendant", ErrKnowledgeInvalid)
			}
		}

		newDepth := 0
		if parentID != nil {
			var parent models.KnowledgeTopic
			if err := tx.Where("id = ? AND user_id = ?", *parentID, userID).First(&parent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrKnowledgeNotFound
				}
				return err
			}
			newDepth = parent.Depth + 1
		}
		depthDelta := newDepth - topic.Depth
		for descendantID := range descendants {
			var descendantDepth int
			for _, candidate := range allTopics {
				if candidate.ID == descendantID {
					descendantDepth = candidate.Depth
					break
				}
			}
			if descendantDepth+depthDelta > maxKnowledgeDepth {
				return fmt.Errorf("%w: moving this topic would exceed maximum depth", ErrKnowledgeInvalid)
			}
		}
		if newDepth > maxKnowledgeDepth {
			return fmt.Errorf("%w: topic depth cannot exceed %d", ErrKnowledgeInvalid, maxKnowledgeDepth)
		}

		normalized := NormalizeKnowledgeName(name)
		duplicate := tx.Model(&models.KnowledgeTopic{}).
			Where("user_id = ? AND normalized_name = ? AND id <> ?", userID, normalized, topic.ID)
		if parentID == nil {
			duplicate = duplicate.Where("parent_id IS NULL")
		} else {
			duplicate = duplicate.Where("parent_id = ?", *parentID)
		}
		var duplicateCount int64
		if err := duplicate.Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("%w: a topic with this name already exists", ErrKnowledgeConflict)
		}

		values := map[string]interface{}{
			"name":            name,
			"normalized_name": normalized,
			"description":     description,
			"parent_id":       parentID,
			"depth":           newDepth,
		}
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ?", topic.ID, userID).
			Updates(values).Error; err != nil {
			return err
		}
		if depthDelta != 0 {
			for descendantID := range descendants {
				if err := tx.Model(&models.KnowledgeTopic{}).
					Where("id = ? AND user_id = ?", descendantID, userID).
					Update("depth", gorm.Expr("depth + ?", depthDelta)).Error; err != nil {
					return err
				}
			}
		}
		return tx.Where("id = ? AND user_id = ?", topic.ID, userID).First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func knowledgeDescendants(topicID string, topics []models.KnowledgeTopic) map[string]struct{} {
	result := make(map[string]struct{})
	added := true
	for added {
		added = false
		for _, topic := range topics {
			if topic.ParentID == nil {
				continue
			}
			if *topic.ParentID == topicID {
				if _, exists := result[topic.ID]; !exists {
					result[topic.ID] = struct{}{}
					added = true
				}
				continue
			}
			if _, parentFound := result[*topic.ParentID]; parentFound {
				if _, exists := result[topic.ID]; !exists {
					result[topic.ID] = struct{}{}
					added = true
				}
			}
		}
	}
	return result
}

func (r *KnowledgeRepository) DeleteTopic(userID, topicID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockKnowledgeTopicHierarchyTx(tx, userID); err != nil {
			return err
		}
		var topic models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", topicID, userID).
			First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if topic.ParentID == nil && topic.NormalizedName == NormalizeKnowledgeName(models.KnowledgeInboxName) {
			return fmt.Errorf("%w: Inbox cannot be deleted", ErrKnowledgeInvalid)
		}

		var childCount, entryCount int64
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("user_id = ? AND parent_id = ?", userID, topicID).
			Count(&childCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.KnowledgeEntry{}).
			Where("user_id = ? AND topic_id = ?", userID, topicID).
			Count(&entryCount).Error; err != nil {
			return err
		}
		if childCount > 0 || entryCount > 0 {
			return fmt.Errorf("%w: only empty topics can be deleted", ErrKnowledgeConflict)
		}
		return tx.Where("id = ? AND user_id = ?", topicID, userID).Delete(&models.KnowledgeTopic{}).Error
	})
}

func (r *KnowledgeRepository) ListEntries(userID, topicID string, limit, offset int) ([]models.KnowledgeEntry, error) {
	if _, err := r.GetTopic(userID, topicID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var entries []models.KnowledgeEntry
	err := r.db.Where("user_id = ? AND topic_id = ? AND status = ?", userID, topicID, models.KnowledgeStatusActive).
		Order("COALESCE(occurred_at, updated_at) DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error
	return entries, err
}

func (r *KnowledgeRepository) GetEntry(userID, entryID string) (*models.KnowledgeEntry, error) {
	var entry models.KnowledgeEntry
	err := r.db.Where("id = ? AND user_id = ?", entryID, userID).
		Preload("Topic", "user_id = ?", userID).
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrKnowledgeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *KnowledgeRepository) CreateEntry(userID string, entry *models.KnowledgeEntry) error {
	if entry.Kind == "" {
		entry.Kind = "note"
	}
	if entry.Source == "" {
		entry.Source = models.KnowledgeSourceManual
	}
	if entry.Attributes == nil {
		entry.Attributes = models.KnowledgeAttributes{}
	}
	tags, err := NormalizeKnowledgeTags(entry.Tags)
	if err != nil {
		return err
	}
	if err := validateEntryFields(entry.Kind, entry.Title, entry.Body, entry.Attributes, tags); err != nil {
		return err
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := ensureInboxTx(tx, userID); err != nil {
			return err
		}
		var topic models.KnowledgeTopic
		if err := tx.Where("id = ? AND user_id = ?", entry.TopicID, userID).First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}

		entry.UserID = userID
		entry.Kind = strings.TrimSpace(entry.Kind)
		entry.Title = strings.TrimSpace(entry.Title)
		entry.Tags = pq.StringArray(tags)
		entry.Status = models.KnowledgeStatusActive
		entry.Version = 1
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		return scheduleKnowledgeTopicWritesTx(tx, userID, []string{entry.TopicID}, time.Now())
	})
}

func NextKnowledgeOrganizationSchedule(pendingWriteCount int, now time.Time) (int, time.Time) {
	nextCount := pendingWriteCount + 1
	if nextCount >= 5 {
		return nextCount, now
	}
	return nextCount, now.Add(48 * time.Hour)
}

func scheduleKnowledgeTopicWritesTx(tx *gorm.DB, userID string, topicIDs []string, now time.Time) error {
	unique := make(map[string]struct{}, len(topicIDs))
	for _, topicID := range topicIDs {
		if topicID = strings.TrimSpace(topicID); topicID != "" {
			unique[topicID] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(unique))
	for topicID := range unique {
		ordered = append(ordered, topicID)
	}
	sort.Strings(ordered)

	for _, topicID := range ordered {
		var topic models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", topicID, userID).
			First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		nextCount, dueAt := NextKnowledgeOrganizationSchedule(topic.PendingWriteCount, now)
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ?", topicID, userID).
			Updates(map[string]interface{}{
				"pending_write_count": nextCount,
				"organization_due_at": dueAt,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func createEntryRevision(tx *gorm.DB, entry *models.KnowledgeEntry, changedBy, operation string) error {
	revision := &models.KnowledgeEntryRevision{
		EntryID:        entry.ID,
		UserID:         entry.UserID,
		Version:        entry.Version,
		TopicID:        entry.TopicID,
		Kind:           entry.Kind,
		Title:          entry.Title,
		Body:           entry.Body,
		Attributes:     cloneKnowledgeAttributes(entry.Attributes),
		Tags:           append(pq.StringArray{}, entry.Tags...),
		OccurredAt:     cloneKnowledgeTime(entry.OccurredAt),
		Status:         entry.Status,
		EntryDeletedAt: cloneKnowledgeTime(entryDeletedAt(entry)),
		ChangedBy:      changedBy,
		Operation:      operation,
	}
	return tx.Create(revision).Error
}

func cloneKnowledgeTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func entryDeletedAt(entry *models.KnowledgeEntry) *time.Time {
	if !entry.DeletedAt.Valid {
		return nil
	}
	return &entry.DeletedAt.Time
}

func (r *KnowledgeRepository) UpdateEntry(userID, entryID string, expectedVersion int, mutation models.KnowledgeEntryMutation, changedBy string) (*models.KnowledgeEntry, error) {
	if expectedVersion <= 0 {
		return nil, fmt.Errorf("%w: expectedVersion is required", ErrKnowledgeInvalid)
	}
	if mutation.ReplaceAttributes && mutation.Attributes == nil {
		return nil, fmt.Errorf("%w: attributes are required when replaceAttributes is true", ErrKnowledgeInvalid)
	}
	if changedBy == "" {
		changedBy = models.KnowledgeSourceManual
	}

	var updated models.KnowledgeEntry
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if mutation.TopicID != nil {
			if err := lockKnowledgeTopicHierarchyTx(tx, userID); err != nil {
				return err
			}
		}
		var entry models.KnowledgeEntry
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", entryID, userID).
			First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if entry.Version != expectedVersion {
			return fmt.Errorf("%w: entry version changed", ErrKnowledgeConflict)
		}

		topicID := entry.TopicID
		if mutation.TopicID != nil {
			topicID = strings.TrimSpace(*mutation.TopicID)
			var topic models.KnowledgeTopic
			if err := tx.Where("id = ? AND user_id = ?", topicID, userID).First(&topic).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrKnowledgeNotFound
				}
				return err
			}
		}
		kind := entry.Kind
		if mutation.Kind != nil {
			kind = strings.TrimSpace(*mutation.Kind)
		}
		title := entry.Title
		if mutation.Title != nil {
			title = strings.TrimSpace(*mutation.Title)
		}
		body := entry.Body
		if mutation.Body != nil {
			body = *mutation.Body
		}
		attributes, err := applyKnowledgeAttributeMutation(
			entry.Attributes,
			mutation.Attributes,
			mutation.ReplaceAttributes,
		)
		if err != nil {
			return err
		}
		tags := append([]string(nil), entry.Tags...)
		if mutation.Tags != nil {
			tags = append([]string(nil), (*mutation.Tags)...)
		}
		normalizedTags, err := NormalizeKnowledgeTags(tags)
		if err != nil {
			return err
		}
		if err := validateEntryFields(kind, title, body, attributes, normalizedTags); err != nil {
			return err
		}

		occurredAt := entry.OccurredAt
		if mutation.ClearOccurredAt {
			occurredAt = nil
		} else if mutation.OccurredAt != nil {
			occurredAt = mutation.OccurredAt
		}

		if err := createEntryRevision(tx, &entry, changedBy, "update"); err != nil {
			return err
		}
		values := map[string]interface{}{
			"topic_id":    topicID,
			"kind":        kind,
			"title":       title,
			"body":        body,
			"attributes":  attributes,
			"tags":        pq.StringArray(normalizedTags),
			"occurred_at": occurredAt,
			"version":     entry.Version + 1,
		}
		if err := tx.Model(&models.KnowledgeEntry{}).
			Where("id = ? AND user_id = ? AND version = ?", entry.ID, userID, expectedVersion).
			Updates(values).Error; err != nil {
			return err
		}
		if err := scheduleKnowledgeTopicWritesTx(tx, userID, []string{entry.TopicID, topicID}, time.Now()); err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ?", entry.ID, userID).Preload("Topic", "user_id = ?", userID).First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *KnowledgeRepository) DeleteEntry(userID, entryID string, expectedVersion int, changedBy string) error {
	if changedBy == "" {
		changedBy = models.KnowledgeSourceManual
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var entry models.KnowledgeEntry
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", entryID, userID).
			First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if expectedVersion > 0 && entry.Version != expectedVersion {
			return fmt.Errorf("%w: entry version changed", ErrKnowledgeConflict)
		}
		if err := createEntryRevision(tx, &entry, changedBy, "delete"); err != nil {
			return err
		}
		if err := tx.Model(&models.KnowledgeEntry{}).
			Where("id = ? AND user_id = ? AND version = ?", entry.ID, userID, entry.Version).
			Updates(map[string]interface{}{
				"status":  "deleted",
				"version": entry.Version + 1,
			}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ?", entry.ID, userID).Delete(&models.KnowledgeEntry{}).Error
	})
}

func (r *KnowledgeRepository) UndoEntry(userID, entryID string, expectedVersion int) (*models.KnowledgeEntry, error) {
	if expectedVersion <= 0 {
		return nil, fmt.Errorf("%w: expectedVersion is required", ErrKnowledgeInvalid)
	}
	var restored models.KnowledgeEntry
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockKnowledgeTopicHierarchyTx(tx, userID); err != nil {
			return err
		}
		var entry models.KnowledgeEntry
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", entryID, userID).
			First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if entry.Version != expectedVersion {
			return fmt.Errorf("%w: entry version changed", ErrKnowledgeConflict)
		}

		var revision models.KnowledgeEntryRevision
		if err := tx.Where("entry_id = ? AND user_id = ? AND version = ?", entryID, userID, entry.Version-1).
			First(&revision).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: no change is available to undo", ErrKnowledgeConflict)
			}
			return err
		}
		if canonicalID, duplicateID, isMerge := parseKnowledgeMergeOperation(revision.Operation); isMerge {
			mergeRestored, mergeErr := undoKnowledgeMergeTx(
				tx,
				userID,
				&entry,
				&revision,
				canonicalID,
				duplicateID,
			)
			if mergeErr != nil {
				return mergeErr
			}
			restored = *mergeRestored
			return nil
		}
		var topicCount int64
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ?", revision.TopicID, userID).
			Count(&topicCount).Error; err != nil {
			return err
		}
		if topicCount == 0 {
			return fmt.Errorf("%w: the previous topic no longer exists", ErrKnowledgeConflict)
		}

		if err := createEntryRevision(tx, &entry, models.KnowledgeSourceManual, "undo"); err != nil {
			return err
		}
		values := map[string]interface{}{
			"topic_id":    revision.TopicID,
			"kind":        revision.Kind,
			"title":       revision.Title,
			"body":        revision.Body,
			"attributes":  cloneKnowledgeAttributes(revision.Attributes),
			"tags":        append(pq.StringArray{}, revision.Tags...),
			"occurred_at": cloneKnowledgeTime(revision.OccurredAt),
			"status":      revision.Status,
			"version":     entry.Version + 1,
			"deleted_at":  cloneKnowledgeTime(revision.EntryDeletedAt),
		}
		if err := tx.Unscoped().Model(&models.KnowledgeEntry{}).
			Where("id = ? AND user_id = ? AND version = ?", entry.ID, userID, expectedVersion).
			Updates(values).Error; err != nil {
			return err
		}
		if err := scheduleKnowledgeTopicWritesTx(tx, userID, []string{entry.TopicID, revision.TopicID}, time.Now()); err != nil {
			return err
		}
		return tx.Unscoped().Where("id = ? AND user_id = ?", entry.ID, userID).
			Preload("Topic", "user_id = ?", userID).
			First(&restored).Error
	})
	if err != nil {
		return nil, err
	}
	return &restored, nil
}

func parseKnowledgeMergeOperation(operation string) (string, string, bool) {
	parts := strings.Split(operation, ":")
	if len(parts) != 3 || parts[0] != "merge_duplicate" || parts[1] == "" || parts[2] == "" || parts[1] == parts[2] {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func undoKnowledgeMergeTx(
	tx *gorm.DB,
	userID string,
	requested *models.KnowledgeEntry,
	requestedRevision *models.KnowledgeEntryRevision,
	canonicalID string,
	duplicateID string,
) (*models.KnowledgeEntry, error) {
	operation := requestedRevision.Operation
	entryIDs := []string{canonicalID, duplicateID}
	sort.Strings(entryIDs)
	entries := make(map[string]*models.KnowledgeEntry, 2)
	for _, entryID := range entryIDs {
		if entryID == requested.ID {
			entries[entryID] = requested
			continue
		}
		var entry models.KnowledgeEntry
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", entryID, userID).
			First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: the other merged entry no longer exists", ErrKnowledgeConflict)
			}
			return nil, err
		}
		entries[entryID] = &entry
	}

	revisions := make(map[string]models.KnowledgeEntryRevision, 2)
	for _, entryID := range entryIDs {
		entry := entries[entryID]
		var revision models.KnowledgeEntryRevision
		if entryID == requested.ID {
			revision = *requestedRevision
		} else if err := tx.Where(
			"entry_id = ? AND user_id = ? AND version = ? AND operation = ?",
			entryID,
			userID,
			entry.Version-1,
			operation,
		).First(&revision).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: the merged entries changed after organization", ErrKnowledgeConflict)
			}
			return nil, err
		}
		revisions[entryID] = revision
	}

	for _, entryID := range entryIDs {
		entry := entries[entryID]
		revision := revisions[entryID]
		if err := createEntryRevision(tx, entry, models.KnowledgeSourceManual, "undo_merge"); err != nil {
			return nil, err
		}
		values := map[string]interface{}{
			"topic_id":    revision.TopicID,
			"kind":        revision.Kind,
			"title":       revision.Title,
			"body":        revision.Body,
			"attributes":  cloneKnowledgeAttributes(revision.Attributes),
			"tags":        append(pq.StringArray{}, revision.Tags...),
			"occurred_at": cloneKnowledgeTime(revision.OccurredAt),
			"status":      revision.Status,
			"version":     entry.Version + 1,
			"deleted_at":  cloneKnowledgeTime(revision.EntryDeletedAt),
		}
		update := tx.Unscoped().Model(&models.KnowledgeEntry{}).
			Where("id = ? AND user_id = ? AND version = ?", entry.ID, userID, entry.Version).
			Updates(values)
		if update.Error != nil {
			return nil, update.Error
		}
		if update.RowsAffected != 1 {
			return nil, fmt.Errorf("%w: a merged entry version changed", ErrKnowledgeConflict)
		}
		entry.TopicID = revision.TopicID
		entry.Kind = revision.Kind
		entry.Title = revision.Title
		entry.Body = revision.Body
		entry.Attributes = cloneKnowledgeAttributes(revision.Attributes)
		entry.Tags = append(pq.StringArray{}, revision.Tags...)
		entry.OccurredAt = cloneKnowledgeTime(revision.OccurredAt)
		entry.Status = revision.Status
		entry.Version++
		if revision.EntryDeletedAt == nil {
			entry.DeletedAt = gorm.DeletedAt{}
		} else {
			entry.DeletedAt = gorm.DeletedAt{Time: *revision.EntryDeletedAt, Valid: true}
		}
	}
	if err := scheduleKnowledgeTopicWritesTx(
		tx,
		userID,
		[]string{
			revisions[canonicalID].TopicID,
			revisions[duplicateID].TopicID,
		},
		time.Now(),
	); err != nil {
		return nil, err
	}
	var restored models.KnowledgeEntry
	if err := tx.Unscoped().Where("id = ? AND user_id = ?", requested.ID, userID).
		Preload("Topic", "user_id = ?", userID).
		First(&restored).Error; err != nil {
		return nil, err
	}
	return &restored, nil
}

func (r *KnowledgeRepository) Search(userID string, filter models.KnowledgeSearchFilter) ([]models.KnowledgeSearchResult, error) {
	if _, err := r.EnsureInbox(userID); err != nil {
		return nil, err
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(filter.Query) > 500 {
		return nil, fmt.Errorf("%w: search query must be 500 characters or fewer", ErrKnowledgeInvalid)
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 30
	}

	query := r.db.Model(&models.KnowledgeEntry{}).
		Where("knowledge_entries.user_id = ? AND knowledge_entries.status = ?", userID, models.KnowledgeStatusActive).
		Preload("Topic", "user_id = ?", userID)
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		textCondition := r.db.Where(
			"(to_tsvector('simple', coalesce(knowledge_entries.title, '') || ' ' || coalesce(knowledge_entries.body, '')) @@ plainto_tsquery('simple', ?) OR knowledge_entries.title ILIKE ? OR knowledge_entries.body ILIKE ?)",
			filter.Query, like, like,
		)
		for _, term := range boundedSearchTerms(filter.Query, 8) {
			termLike := "%" + term + "%"
			textCondition = textCondition.Or("knowledge_entries.title ILIKE ? OR knowledge_entries.body ILIKE ?", termLike, termLike)
		}
		query = query.Where(textCondition)
	}
	if filter.TopicID != "" {
		topicIDs := []string{filter.TopicID}
		if filter.IncludeChildren {
			var err error
			topicIDs, err = r.knowledgeTopicDescendantIDs(userID, filter.TopicID, 500)
			if err != nil {
				return nil, err
			}
			if len(topicIDs) == 0 {
				return nil, ErrKnowledgeNotFound
			}
		} else if _, err := r.GetTopic(userID, filter.TopicID); err != nil {
			return nil, err
		}
		query = query.Where("knowledge_entries.topic_id IN ?", topicIDs)
	}
	if filter.Kind != "" {
		query = query.Where("lower(knowledge_entries.kind) = lower(?)", strings.TrimSpace(filter.Kind))
	}
	if filter.Tag != "" {
		query = query.Where("EXISTS (SELECT 1 FROM unnest(knowledge_entries.tags) AS tag WHERE lower(tag) = lower(?))", strings.TrimSpace(filter.Tag))
	}
	if filter.OccurredFrom != nil {
		query = query.Where("knowledge_entries.occurred_at >= ?", filter.OccurredFrom)
	}
	if filter.OccurredTo != nil {
		query = query.Where("knowledge_entries.occurred_at <= ?", filter.OccurredTo)
	}

	var entries []models.KnowledgeEntry
	if err := query.Order("COALESCE(knowledge_entries.occurred_at, knowledge_entries.updated_at) DESC").
		Limit(filter.Limit).
		Find(&entries).Error; err != nil {
		return nil, err
	}
	entryTopicIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryTopicIDs = append(entryTopicIDs, entry.TopicID)
	}
	topics, err := r.knowledgeTopicAncestors(userID, entryTopicIDs, filter.Limit*(maxKnowledgeDepth+1))
	if err != nil {
		return nil, err
	}
	results := make([]models.KnowledgeSearchResult, 0, len(entries))
	for _, entry := range entries {
		results = append(results, models.KnowledgeSearchResult{
			Entry:      entry,
			Breadcrumb: knowledgeBreadcrumb(entry.TopicID, topics),
		})
	}
	return results, nil
}

func (r *KnowledgeRepository) knowledgeTopicDescendantIDs(userID, topicID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	var topicIDs []string
	err := r.db.Raw(`
		WITH RECURSIVE topic_descendants AS (
			SELECT id
			FROM knowledge_topics
			WHERE user_id = ? AND id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT child.id
			FROM knowledge_topics child
			JOIN topic_descendants parent ON child.parent_id = parent.id
			WHERE child.user_id = ? AND child.deleted_at IS NULL
		)
		SELECT id
		FROM topic_descendants
		LIMIT ?`,
		userID, topicID, userID, limit,
	).Scan(&topicIDs).Error
	return topicIDs, err
}

func boundedSearchTerms(value string, maximum int) []string {
	fields := strings.Fields(value)
	result := make([]string, 0, min(len(fields), maximum))
	seen := make(map[string]struct{}, len(fields))
	stopWords := map[string]struct{}{
		"about": {}, "change": {}, "delete": {}, "entry": {}, "find": {}, "forget": {},
		"from": {}, "that": {}, "the": {}, "this": {}, "what": {}, "where": {},
		"remember": {}, "update": {}, "with": {}, "your": {},
		"alterar": {}, "apagar": {}, "buscar": {}, "encontrar": {}, "esquecer": {},
		"lembrar": {}, "meu": {}, "minha": {}, "mudar": {}, "que": {}, "qual": {},
		"sobre": {},
	}
	for _, field := range fields {
		field = strings.Trim(field, ".,!?;:()[]{}\"'")
		if utf8.RuneCountInString(field) < 2 {
			continue
		}
		key := strings.ToLower(field)
		if _, stopped := stopWords[key]; stopped {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, field)
		if len(result) == maximum {
			break
		}
	}
	return result
}

func (r *KnowledgeRepository) knowledgeTopicAncestors(userID string, topicIDs []string, limit int) ([]models.KnowledgeTopic, error) {
	if len(topicIDs) == 0 {
		return []models.KnowledgeTopic{}, nil
	}
	if limit <= 0 || limit > 250 {
		limit = 250
	}
	var topics []models.KnowledgeTopic
	err := r.db.Raw(`
		WITH RECURSIVE topic_ancestors AS (
			SELECT *
			FROM knowledge_topics
			WHERE user_id = ? AND id IN ? AND deleted_at IS NULL
			UNION
			SELECT parent.*
			FROM knowledge_topics parent
			JOIN topic_ancestors child ON child.parent_id = parent.id
			WHERE parent.user_id = ? AND parent.deleted_at IS NULL
		)
		SELECT DISTINCT *
		FROM topic_ancestors
		LIMIT ?`,
		userID, topicIDs, userID, limit,
	).Scan(&topics).Error
	return topics, err
}

func knowledgeBreadcrumb(topicID string, topics []models.KnowledgeTopic) []models.KnowledgeTopic {
	byID := make(map[string]models.KnowledgeTopic, len(topics))
	for _, topic := range topics {
		byID[topic.ID] = topic
	}
	reversed := make([]models.KnowledgeTopic, 0, maxKnowledgeDepth+1)
	currentID := topicID
	for len(reversed) <= maxKnowledgeDepth {
		topic, found := byID[currentID]
		if !found {
			break
		}
		reversed = append(reversed, topic)
		if topic.ParentID == nil {
			break
		}
		currentID = *topic.ParentID
	}
	result := make([]models.KnowledgeTopic, len(reversed))
	for i := range reversed {
		result[len(reversed)-1-i] = reversed[i]
	}
	return result
}

func (r *KnowledgeRepository) AssistantContext(userID, searchText string, topicLimit, entryLimit int) (*models.KnowledgeAssistantContext, error) {
	if topicLimit <= 0 || topicLimit > 50 {
		topicLimit = 50
	}
	if entryLimit <= 0 || entryLimit > 20 {
		entryLimit = 20
	}
	if _, err := r.EnsureInbox(userID); err != nil {
		return nil, err
	}
	var topics []models.KnowledgeTopic
	err := r.db.Where("user_id = ?", userID).
		Order(clause.Expr{
			SQL:  "CASE WHEN parent_id IS NULL AND normalized_name = ? THEN 0 ELSE 1 END, depth ASC, normalized_name ASC",
			Vars: []interface{}{NormalizeKnowledgeName(models.KnowledgeInboxName)},
		}).
		Limit(topicLimit).
		Find(&topics).Error
	if err != nil {
		return nil, err
	}

	entries := make([]models.KnowledgeEntry, 0, entryLimit)
	seenEntries := make(map[string]struct{}, entryLimit)
	searchText = truncateRunes(strings.TrimSpace(searchText), 500)
	if searchText != "" {
		results, searchErr := r.Search(userID, models.KnowledgeSearchFilter{Query: searchText, Limit: min(10, entryLimit)})
		if searchErr != nil {
			return nil, searchErr
		}
		for _, result := range results {
			entries = append(entries, result.Entry)
			seenEntries[result.Entry.ID] = struct{}{}
		}
	}
	var recentEntries []models.KnowledgeEntry
	if err := r.db.Where("user_id = ? AND status = ?", userID, models.KnowledgeStatusActive).
		Order("updated_at DESC").
		Limit(entryLimit).
		Find(&recentEntries).Error; err != nil {
		return nil, err
	}
	for _, entry := range recentEntries {
		if len(entries) == entryLimit {
			break
		}
		if _, exists := seenEntries[entry.ID]; exists {
			continue
		}
		seenEntries[entry.ID] = struct{}{}
		entries = append(entries, entry)
	}

	entryTopicIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryTopicIDs = append(entryTopicIDs, entry.TopicID)
	}
	requiredTopics, err := r.knowledgeTopicAncestors(userID, entryTopicIDs, topicLimit)
	if err != nil {
		return nil, err
	}
	mergedTopics := make([]models.KnowledgeTopic, 0, topicLimit)
	seenTopics := make(map[string]struct{}, topicLimit)
	addTopic := func(topic models.KnowledgeTopic) {
		if len(mergedTopics) == topicLimit {
			return
		}
		if _, exists := seenTopics[topic.ID]; exists {
			return
		}
		seenTopics[topic.ID] = struct{}{}
		mergedTopics = append(mergedTopics, topic)
	}
	for _, topic := range topics {
		if topic.ParentID == nil && topic.NormalizedName == NormalizeKnowledgeName(models.KnowledgeInboxName) {
			addTopic(topic)
			break
		}
	}
	for _, topic := range requiredTopics {
		addTopic(topic)
	}
	for _, topic := range topics {
		addTopic(topic)
	}
	topics = mergedTopics

	pathByID := make(map[string]string, len(topics))
	for _, topic := range topics {
		breadcrumb := knowledgeBreadcrumb(topic.ID, topics)
		names := make([]string, 0, len(breadcrumb))
		for _, part := range breadcrumb {
			names = append(names, part.Name)
		}
		pathByID[topic.ID] = strings.Join(names, " / ")
	}

	context := &models.KnowledgeAssistantContext{
		Topics:  make([]models.KnowledgeAssistantTopic, 0, len(topics)),
		Entries: make([]models.KnowledgeAssistantEntry, 0, len(entries)),
	}
	for _, topic := range topics {
		context.Topics = append(context.Topics, models.KnowledgeAssistantTopic{
			ID:          topic.ID,
			Path:        pathByID[topic.ID],
			Description: truncateRunes(topic.Description, 300),
		})
	}
	for _, entry := range entries {
		attributes := cloneKnowledgeAttributes(entry.Attributes)
		if encoded, marshalErr := json.Marshal(attributes); marshalErr != nil || len(encoded) > 1000 {
			attributes = models.KnowledgeAttributes{}
		}
		context.Entries = append(context.Entries, models.KnowledgeAssistantEntry{
			ID:         entry.ID,
			TopicID:    entry.TopicID,
			TopicPath:  pathByID[entry.TopicID],
			Kind:       entry.Kind,
			Title:      entry.Title,
			Body:       truncateRunes(entry.Body, 500),
			Attributes: attributes,
			Tags:       append([]string(nil), entry.Tags...),
			OccurredAt: entry.OccurredAt,
			Version:    entry.Version,
		})
	}
	return context, nil
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}
