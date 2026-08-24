package repository

import (
	"buybuddy-api/models"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	knowledgeOrganizationLease = 10 * time.Minute
	knowledgeOrganizationRetry = 6 * time.Hour
)

func (r *KnowledgeRepository) ClaimNextKnowledgeOrganization(now time.Time) (*models.KnowledgeOrganizationClaim, error) {
	now = now.UTC().Truncate(time.Microsecond)
	var claim *models.KnowledgeOrganizationClaim
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var topic models.KnowledgeTopic
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("organization_due_at <= ? AND pending_write_count > 0 AND (organization_lease_until IS NULL OR organization_lease_until <= ?)", now, now).
			Order("organization_due_at ASC, id ASC").
			Limit(1).
			First(&topic).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		leaseUntil := now.Add(knowledgeOrganizationLease)
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ?", topic.ID, topic.UserID).
			Update("organization_lease_until", leaseUntil).Error; err != nil {
			return err
		}
		topic.OrganizationLeaseUntil = &leaseUntil
		claim = &models.KnowledgeOrganizationClaim{
			Topic:                        topic,
			LeaseUntil:                   leaseUntil,
			PendingWriteCountBeforeClaim: topic.PendingWriteCount,
			OrganizationDueAtBeforeClaim: cloneKnowledgeTime(topic.OrganizationDueAt),
		}
		return nil
	})
	return claim, err
}

func (r *KnowledgeRepository) ClaimKnowledgeOrganization(userID, topicID string, now time.Time) (*models.KnowledgeOrganizationClaim, error) {
	now = now.UTC().Truncate(time.Microsecond)
	var claim models.KnowledgeOrganizationClaim
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var topic models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", topicID, userID).
			First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if topic.OrganizationLeaseUntil != nil && topic.OrganizationLeaseUntil.After(now) {
			return fmt.Errorf("%w: this topic is already being organized", ErrKnowledgeConflict)
		}

		leaseUntil := now.Add(knowledgeOrganizationLease)
		pendingWriteCountBeforeClaim := topic.PendingWriteCount
		organizationDueAtBeforeClaim := cloneKnowledgeTime(topic.OrganizationDueAt)
		synthetic := topic.PendingWriteCount == 0
		if err := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ?", topic.ID, userID).
			Updates(map[string]interface{}{
				"organization_due_at":      now,
				"organization_lease_until": leaseUntil,
			}).Error; err != nil {
			return err
		}
		topic.OrganizationDueAt = cloneKnowledgeTime(&now)
		topic.OrganizationLeaseUntil = cloneKnowledgeTime(&leaseUntil)
		claim = models.KnowledgeOrganizationClaim{
			Topic:                        topic,
			LeaseUntil:                   leaseUntil,
			SyntheticManual:              synthetic,
			PendingWriteCountBeforeClaim: pendingWriteCountBeforeClaim,
			OrganizationDueAtBeforeClaim: organizationDueAtBeforeClaim,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &claim, nil
}

func (r *KnowledgeRepository) FailKnowledgeOrganization(claim *models.KnowledgeOrganizationClaim, now time.Time) error {
	return r.finishKnowledgeOrganizationClaim(context.Background(), claim, cloneKnowledgeTime(&now))
}

func (r *KnowledgeRepository) ReleaseKnowledgeOrganization(ctx context.Context, claim *models.KnowledgeOrganizationClaim) error {
	if ctx == nil {
		return fmt.Errorf("%w: release context is required", ErrKnowledgeInvalid)
	}
	return r.finishKnowledgeOrganizationClaim(ctx, claim, nil)
}

func (r *KnowledgeRepository) finishKnowledgeOrganizationClaim(ctx context.Context, claim *models.KnowledgeOrganizationClaim, failedAt *time.Time) error {
	if claim == nil {
		return fmt.Errorf("%w: organization claim is required", ErrKnowledgeInvalid)
	}
	if failedAt != nil {
		normalized := failedAt.UTC().Truncate(time.Microsecond)
		failedAt = &normalized
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var topic models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", claim.Topic.ID, claim.Topic.UserID).
			First(&topic).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: organization lease is no longer owned", ErrKnowledgeConflict)
			}
			return err
		}
		if topic.OrganizationLeaseUntil == nil || !topic.OrganizationLeaseUntil.Equal(claim.LeaseUntil) {
			return fmt.Errorf("%w: organization lease is no longer owned", ErrKnowledgeConflict)
		}

		pendingWriteCount, dueAt, err := knowledgeOrganizationStateAfterClaim(topic, claim, failedAt)
		if err != nil {
			return err
		}
		result := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ? AND organization_lease_until = ?", claim.Topic.ID, claim.Topic.UserID, claim.LeaseUntil).
			Updates(map[string]interface{}{
				"pending_write_count":      pendingWriteCount,
				"organization_due_at":      dueAt,
				"organization_lease_until": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: organization lease is no longer owned", ErrKnowledgeConflict)
		}
		return nil
	})
}

func knowledgeOrganizationStateAfterClaim(
	topic models.KnowledgeTopic,
	claim *models.KnowledgeOrganizationClaim,
	failedAt *time.Time,
) (int, *time.Time, error) {
	if topic.PendingWriteCount < claim.PendingWriteCountBeforeClaim {
		return 0, nil, fmt.Errorf("%w: pending organization state moved backwards", ErrKnowledgeConflict)
	}

	pendingWriteCount := topic.PendingWriteCount
	dueAt := cloneKnowledgeTime(topic.OrganizationDueAt)
	if claim.SyntheticManual {
		if claim.PendingWriteCountBeforeClaim != 0 || claim.Topic.PendingWriteCount != claim.PendingWriteCountBeforeClaim {
			return 0, nil, fmt.Errorf("%w: synthetic organization claim is inconsistent", ErrKnowledgeConflict)
		}
		if pendingWriteCount == claim.PendingWriteCountBeforeClaim {
			dueAt = cloneKnowledgeTime(claim.OrganizationDueAtBeforeClaim)
			return pendingWriteCount, dueAt, nil
		}
		if failedAt != nil {
			retryAt := failedAt.Add(knowledgeOrganizationRetry)
			dueAt = &retryAt
		}
		return pendingWriteCount, dueAt, nil
	}
	if claim.Topic.PendingWriteCount != claim.PendingWriteCountBeforeClaim {
		return 0, nil, fmt.Errorf("%w: natural organization claim is inconsistent", ErrKnowledgeConflict)
	}

	if failedAt != nil {
		retryAt := failedAt.Add(knowledgeOrganizationRetry)
		return pendingWriteCount, &retryAt, nil
	}
	if pendingWriteCount == claim.Topic.PendingWriteCount {
		return claim.PendingWriteCountBeforeClaim, cloneKnowledgeTime(claim.OrganizationDueAtBeforeClaim), nil
	}
	return pendingWriteCount, dueAt, nil
}

func (r *KnowledgeRepository) LoadKnowledgeOrganizerContext(userID, topicID string) (*models.KnowledgeOrganizerContext, error) {
	topic, err := r.GetTopic(userID, topicID)
	if err != nil {
		return nil, err
	}

	var directCount int64
	if err := r.db.Model(&models.KnowledgeEntry{}).
		Where("user_id = ? AND topic_id = ? AND status = ?", userID, topicID, models.KnowledgeStatusActive).
		Count(&directCount).Error; err != nil {
		return nil, err
	}
	ancestors, err := r.knowledgeTopicAncestors(userID, []string{topicID}, maxKnowledgeDepth+1)
	if err != nil {
		return nil, err
	}
	parentNames := make([]string, 0, maxKnowledgeDepth)
	for _, ancestor := range knowledgeBreadcrumb(topicID, ancestors) {
		if ancestor.ID != topicID {
			parentNames = append(parentNames, ancestor.Name)
		}
	}

	context := &models.KnowledgeOrganizerContext{
		Target: models.KnowledgeOrganizerTarget{
			ID:                topic.ID,
			Name:              topic.Name,
			ParentPath:        strings.Join(parentNames, " / "),
			Description:       truncateRunes(topic.Description, 1000),
			DirectEntryCount:  int(directCount),
			PendingWriteCount: topic.PendingWriteCount,
			Depth:             topic.Depth,
		},
		Children:        []models.KnowledgeOrganizerChild{},
		RecentEntries:   []models.KnowledgeOrganizerEntry{},
		Tags:            []string{},
		SimilarEntries:  []models.KnowledgeOrganizerEntry{},
		RecentRevisions: []models.KnowledgeOrganizerRevision{},
	}

	type childRow struct {
		ID          string
		Name        string
		Description string
		Depth       int
		EntryCount  int
	}
	var childRows []childRow
	if err := r.db.Table("knowledge_topics AS topics").
		Select("topics.id, topics.name, topics.description, topics.depth, count(entries.id) AS entry_count").
		Joins("LEFT JOIN knowledge_entries AS entries ON entries.topic_id = topics.id AND entries.user_id = ? AND entries.status = ? AND entries.deleted_at IS NULL", userID, models.KnowledgeStatusActive).
		Where("topics.user_id = ? AND topics.parent_id = ? AND topics.deleted_at IS NULL", userID, topicID).
		Group("topics.id, topics.name, topics.description, topics.depth, topics.normalized_name").
		Order("topics.normalized_name ASC").
		Limit(20).
		Scan(&childRows).Error; err != nil {
		return nil, err
	}
	scopeTopicIDs := []string{topicID}
	for _, child := range childRows {
		scopeTopicIDs = append(scopeTopicIDs, child.ID)
		context.Children = append(context.Children, models.KnowledgeOrganizerChild{
			ID:          child.ID,
			Name:        child.Name,
			Description: truncateRunes(child.Description, 1000),
			EntryCount:  child.EntryCount,
			Depth:       child.Depth,
		})
	}

	recentQuery := r.db.Where("user_id = ? AND topic_id IN ? AND status = ?", userID, scopeTopicIDs, models.KnowledgeStatusActive)
	if topic.LastOrganizedAt != nil {
		recentQuery = recentQuery.Where("updated_at > ?", *topic.LastOrganizedAt)
	}
	var recent []models.KnowledgeEntry
	if err := recentQuery.Order("updated_at DESC, id DESC").Limit(20).Find(&recent).Error; err != nil {
		return nil, err
	}
	for _, entry := range recent {
		context.RecentEntries = append(context.RecentEntries, organizerEntry(entry))
	}

	type tagRow struct {
		Tag string
	}
	var tagRows []tagRow
	if err := r.db.Raw(`
		SELECT tag
		FROM knowledge_entries AS entries
		CROSS JOIN LATERAL unnest(entries.tags) AS tag
		WHERE entries.user_id = ?
		  AND entries.topic_id IN ?
		  AND entries.status = ?
		  AND entries.deleted_at IS NULL
		GROUP BY tag
		ORDER BY count(*) DESC, lower(tag) ASC
		LIMIT 30`,
		userID, scopeTopicIDs, models.KnowledgeStatusActive,
	).Scan(&tagRows).Error; err != nil {
		return nil, err
	}
	for _, row := range tagRows {
		context.Tags = append(context.Tags, row.Tag)
	}

	var olderBefore *time.Time
	if topic.LastOrganizedAt != nil {
		olderBefore = cloneKnowledgeTime(topic.LastOrganizedAt)
	} else if len(recent) > 0 {
		cutoff := recent[len(recent)-1].UpdatedAt
		olderBefore = &cutoff
	}
	context.SimilarEntries, err = r.loadSimilarOrganizerEntries(userID, scopeTopicIDs, recent, olderBefore)
	if err != nil {
		return nil, err
	}

	var revisions []models.KnowledgeEntryRevision
	if err := r.db.Where("user_id = ? AND topic_id IN ? AND changed_by = ?", userID, scopeTopicIDs, models.KnowledgeSourceOrganizer).
		Order("created_at DESC, id DESC").
		Limit(10).
		Find(&revisions).Error; err != nil {
		return nil, err
	}
	for _, revision := range revisions {
		context.RecentRevisions = append(context.RecentRevisions, models.KnowledgeOrganizerRevision{
			EntryID:   revision.EntryID,
			TopicID:   revision.TopicID,
			Title:     truncateRunes(revision.Title, maxKnowledgeTitleRunes),
			Operation: truncateRunes(revision.Operation, 200),
			CreatedAt: revision.CreatedAt,
		})
	}
	return context, nil
}

func organizerEntry(entry models.KnowledgeEntry) models.KnowledgeOrganizerEntry {
	attributes := cloneKnowledgeAttributes(entry.Attributes)
	if encoded, err := attributes.Value(); err != nil || len(fmt.Sprint(encoded)) > 2000 {
		attributes = models.KnowledgeAttributes{}
	}
	return models.KnowledgeOrganizerEntry{
		ID:         entry.ID,
		TopicID:    entry.TopicID,
		Kind:       truncateRunes(entry.Kind, 64),
		Title:      truncateRunes(entry.Title, maxKnowledgeTitleRunes),
		Body:       truncateRunes(entry.Body, 500),
		Tags:       append([]string(nil), entry.Tags...),
		Attributes: attributes,
		OccurredAt: cloneKnowledgeTime(entry.OccurredAt),
		Version:    entry.Version,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func (r *KnowledgeRepository) loadSimilarOrganizerEntries(userID string, scopeTopicIDs []string, recent []models.KnowledgeEntry, olderBefore *time.Time) ([]models.KnowledgeOrganizerEntry, error) {
	if len(recent) == 0 {
		return []models.KnowledgeOrganizerEntry{}, nil
	}
	recentIDs := make([]string, 0, len(recent))
	searchParts := make([]string, 0, 20)
	recentTitle := strings.Builder{}
	for _, entry := range recent {
		recentIDs = append(recentIDs, entry.ID)
		if recentTitle.Len() < 2000 {
			recentTitle.WriteString(" ")
			recentTitle.WriteString(entry.Title)
		}
		for _, term := range boundedSearchTerms(entry.Title+" "+strings.Join(entry.Tags, " "), 5) {
			if len(searchParts) == 20 {
				break
			}
			searchParts = append(searchParts, term)
		}
	}
	if len(searchParts) == 0 {
		return []models.KnowledgeOrganizerEntry{}, nil
	}

	var candidates []models.KnowledgeEntry
	textCondition := r.db.Where("1 = 0")
	for _, term := range searchParts {
		like := "%" + term + "%"
		textCondition = textCondition.Or(
			"to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(body, '')) @@ plainto_tsquery('simple', ?) OR title ILIKE ?",
			term, like,
		)
	}
	query := r.db.Where("user_id = ? AND topic_id IN ? AND status = ? AND id NOT IN ?", userID, scopeTopicIDs, models.KnowledgeStatusActive, recentIDs).
		Where(textCondition).
		Order("updated_at DESC").
		Limit(50)
	if olderBefore != nil {
		query = query.Where("updated_at <= ?", *olderBefore)
	}
	if err := query.Find(&candidates).Error; err != nil {
		return nil, err
	}

	type scoredEntry struct {
		entry models.KnowledgeEntry
		score float64
	}
	scored := make([]scoredEntry, 0, len(candidates))
	for _, candidate := range candidates {
		score := titleTokenSimilarity(recentTitle.String(), candidate.Title)
		scored = append(scored, scoredEntry{entry: candidate, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].entry.UpdatedAt.After(scored[j].entry.UpdatedAt)
	})
	result := make([]models.KnowledgeOrganizerEntry, 0, min(10, len(scored)))
	for _, candidate := range scored {
		if len(result) == 10 {
			break
		}
		result = append(result, organizerEntry(candidate.entry))
	}
	return result, nil
}

func titleTokenSimilarity(left, right string) float64 {
	tokenSet := func(value string) map[string]struct{} {
		result := make(map[string]struct{})
		for _, term := range boundedSearchTerms(value, 100) {
			result[strings.ToLower(term)] = struct{}{}
		}
		return result
	}
	leftTokens := tokenSet(left)
	rightTokens := tokenSet(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range rightTokens {
		if _, found := leftTokens[token]; found {
			intersection++
		}
	}
	return float64(intersection) / float64(len(leftTokens)+len(rightTokens)-intersection)
}
