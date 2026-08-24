package repository

import (
	"buybuddy-api/models"
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
	maxKnowledgeOrganizationOperations = 25
	maxKnowledgeOrganizationSubtopics  = 5
)

type organizerProjectedEntry struct {
	version    int
	topicID    string
	title      string
	tags       []string
	attributes models.KnowledgeAttributes
	status     string
}

func ValidateKnowledgeOrganizationPlan(context *models.KnowledgeOrganizerContext, plan *models.KnowledgeOrganizationPlan) error {
	if context == nil || plan == nil {
		return fmt.Errorf("%w: organizer context and plan are required", ErrKnowledgeInvalid)
	}
	if len(plan.Operations) > maxKnowledgeOrganizationOperations {
		return fmt.Errorf("%w: at most %d organizer operations are allowed", ErrKnowledgeInvalid, maxKnowledgeOrganizationOperations)
	}

	entries := make(map[string]*organizerProjectedEntry)
	addEntry := func(entry models.KnowledgeOrganizerEntry) {
		if _, found := entries[entry.ID]; found {
			return
		}
		entries[entry.ID] = &organizerProjectedEntry{
			version:    entry.Version,
			topicID:    entry.TopicID,
			title:      entry.Title,
			tags:       append([]string(nil), entry.Tags...),
			attributes: cloneKnowledgeAttributes(entry.Attributes),
			status:     models.KnowledgeStatusActive,
		}
	}
	for _, entry := range context.RecentEntries {
		addEntry(entry)
	}
	for _, entry := range context.SimilarEntries {
		addEntry(entry)
	}

	allowedTopics := map[string]struct{}{context.Target.ID: {}}
	existingNames := make(map[string]string, len(context.Children))
	for _, child := range context.Children {
		allowedTopics[child.ID] = struct{}{}
		existingNames[NormalizeKnowledgeName(child.Name)] = child.ID
	}
	temporaryTopics := make(map[string]string)
	newNames := make(map[string]struct{})
	newTopicCount := 0

	for index, operation := range plan.Operations {
		if err := validateOrganizerOperationShape(operation); err != nil {
			return fmt.Errorf("%w: operation %d: %v", ErrKnowledgeInvalid, index+1, err)
		}
		if strings.TrimSpace(operation.Reason) == "" || utf8.RuneCountInString(operation.Reason) > 500 {
			return fmt.Errorf("%w: operation %d reason must be between 1 and 500 characters", ErrKnowledgeInvalid, index+1)
		}
		if operation.Type != "create_subtopic" {
			continue
		}
		if context.Target.Depth+1 > maxKnowledgeDepth {
			return organizerOperationError(index, fmt.Errorf("target topic is already at maximum depth %d", maxKnowledgeDepth))
		}
		if !validTemporaryTopicID(operation.TemporaryID) {
			return organizerOperationError(index, errors.New("temporaryId must start with new-topic- and contain only letters, numbers, dashes, or underscores"))
		}
		if _, exists := temporaryTopics[operation.TemporaryID]; exists {
			return organizerOperationError(index, errors.New("temporaryId must be unique"))
		}
		if err := validateTopicName(operation.Name); err != nil {
			return organizerOperationError(index, err)
		}
		if utf8.RuneCountInString(strings.TrimSpace(operation.Description)) > 1000 {
			return organizerOperationError(index, errors.New("subtopic description must be 1000 characters or fewer"))
		}
		normalizedName := NormalizeKnowledgeName(operation.Name)
		if existingID, found := existingNames[normalizedName]; found {
			temporaryTopics[operation.TemporaryID] = existingID
			continue
		}
		newTopicCount++
		if newTopicCount > maxKnowledgeOrganizationSubtopics {
			return organizerOperationError(index, fmt.Errorf("at most %d subtopics may be created", maxKnowledgeOrganizationSubtopics))
		}
		if _, duplicate := newNames[normalizedName]; duplicate {
			return organizerOperationError(index, errors.New("subtopic name must be unique under the target topic"))
		}
		newNames[normalizedName] = struct{}{}
		temporaryTopics[operation.TemporaryID] = operation.TemporaryID
	}

	for index, operation := range plan.Operations {
		switch operation.Type {
		case "normalize_tags":
			entry, err := projectedOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
			if err != nil {
				return organizerOperationError(index, err)
			}
			normalized, err := normalizeOrganizerTags(operation.Tags)
			if err != nil {
				return organizerOperationError(index, err)
			}
			if !sameNormalizedTagSet(entry.tags, normalized) {
				return organizerOperationError(index, errors.New("tag normalization cannot add or remove tag meanings"))
			}
			if equalStringSlices(entry.tags, normalized) {
				return organizerOperationError(index, errors.New("tag normalization must change the entry"))
			}
			entry.tags = normalized
			entry.version++

		case "create_subtopic":
			continue

		case "move_entry":
			entry, err := projectedOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
			if err != nil {
				return organizerOperationError(index, err)
			}
			targetID := operation.TargetTopicID
			if resolved, found := temporaryTopics[targetID]; found {
				targetID = resolved
			} else if _, found := allowedTopics[targetID]; !found {
				return organizerOperationError(index, errors.New("move target must be the target topic, a direct child, or a subtopic created by this plan"))
			}
			if targetID == entry.topicID {
				return organizerOperationError(index, errors.New("move must change the entry topic"))
			}
			entry.topicID = targetID
			entry.version++

		case "rename_entry":
			entry, err := projectedOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
			if err != nil {
				return organizerOperationError(index, err)
			}
			title := strings.TrimSpace(operation.Title)
			if title == "" || utf8.RuneCountInString(title) > maxKnowledgeTitleRunes {
				return organizerOperationError(index, fmt.Errorf("title must be between 1 and %d characters", maxKnowledgeTitleRunes))
			}
			if title == entry.title {
				return organizerOperationError(index, errors.New("rename must change the entry title"))
			}
			entry.title = title
			entry.version++

		case "merge_duplicate_entries":
			canonical, err := projectedOrganizerEntry(entries, operation.CanonicalEntryID, operation.CanonicalExpectedVersion)
			if err != nil {
				return organizerOperationError(index, fmt.Errorf("canonical entry: %w", err))
			}
			duplicate, err := projectedOrganizerEntry(entries, operation.DuplicateEntryID, operation.DuplicateExpectedVersion)
			if err != nil {
				return organizerOperationError(index, fmt.Errorf("duplicate entry: %w", err))
			}
			if operation.CanonicalEntryID == operation.DuplicateEntryID {
				return organizerOperationError(index, errors.New("canonical and duplicate entries must be different"))
			}
			if canonical.status != models.KnowledgeStatusActive || duplicate.status != models.KnowledgeStatusActive {
				return organizerOperationError(index, errors.New("only active entries may be merged"))
			}
			mergedTags, err := mergeOrganizerTags(canonical.tags, duplicate.tags)
			if err != nil {
				return organizerOperationError(index, err)
			}
			if existing, found := duplicate.attributes["mergedIntoEntryId"]; found && fmt.Sprint(existing) != operation.CanonicalEntryID {
				return organizerOperationError(index, errors.New("duplicate already references a different canonical entry"))
			}
			canonical.tags = mergedTags
			canonical.version++
			duplicate.status = models.KnowledgeStatusMerged
			duplicate.version++

		default:
			return organizerOperationError(index, fmt.Errorf("unsupported operation type %q", operation.Type))
		}
	}
	return nil
}

func validateOrganizerOperationShape(operation models.KnowledgeOrganizationOperation) error {
	typeName := strings.TrimSpace(operation.Type)
	if typeName != operation.Type {
		return errors.New("operation type may not contain surrounding whitespace")
	}
	commonEmpty := func(values ...bool) bool {
		for _, value := range values {
			if !value {
				return false
			}
		}
		return true
	}
	switch typeName {
	case "normalize_tags":
		if operation.EntryID == "" || operation.ExpectedVersion <= 0 || operation.Tags == nil {
			return errors.New("normalize_tags requires entryId, expectedVersion, and tags")
		}
		if !commonEmpty(operation.TemporaryID == "", operation.Name == "", operation.Description == "", operation.TargetTopicID == "", operation.Title == "", operation.CanonicalEntryID == "", operation.DuplicateEntryID == "", operation.CanonicalExpectedVersion == 0, operation.DuplicateExpectedVersion == 0) {
			return errors.New("normalize_tags contains fields that are not allowed")
		}
	case "create_subtopic":
		if operation.TemporaryID == "" || strings.TrimSpace(operation.Name) == "" {
			return errors.New("create_subtopic requires temporaryId and name")
		}
		if !commonEmpty(operation.EntryID == "", operation.ExpectedVersion == 0, operation.Tags == nil, operation.TargetTopicID == "", operation.Title == "", operation.CanonicalEntryID == "", operation.DuplicateEntryID == "", operation.CanonicalExpectedVersion == 0, operation.DuplicateExpectedVersion == 0) {
			return errors.New("create_subtopic contains fields that are not allowed")
		}
	case "move_entry":
		if operation.EntryID == "" || operation.ExpectedVersion <= 0 || operation.TargetTopicID == "" {
			return errors.New("move_entry requires entryId, expectedVersion, and targetTopicId")
		}
		if !commonEmpty(operation.Tags == nil, operation.TemporaryID == "", operation.Name == "", operation.Description == "", operation.Title == "", operation.CanonicalEntryID == "", operation.DuplicateEntryID == "", operation.CanonicalExpectedVersion == 0, operation.DuplicateExpectedVersion == 0) {
			return errors.New("move_entry contains fields that are not allowed")
		}
	case "rename_entry":
		if operation.EntryID == "" || operation.ExpectedVersion <= 0 || strings.TrimSpace(operation.Title) == "" {
			return errors.New("rename_entry requires entryId, expectedVersion, and title")
		}
		if !commonEmpty(operation.Tags == nil, operation.TemporaryID == "", operation.Name == "", operation.Description == "", operation.TargetTopicID == "", operation.CanonicalEntryID == "", operation.DuplicateEntryID == "", operation.CanonicalExpectedVersion == 0, operation.DuplicateExpectedVersion == 0) {
			return errors.New("rename_entry contains fields that are not allowed")
		}
	case "merge_duplicate_entries":
		if operation.CanonicalEntryID == "" || operation.DuplicateEntryID == "" || operation.CanonicalExpectedVersion <= 0 || operation.DuplicateExpectedVersion <= 0 {
			return errors.New("merge_duplicate_entries requires both entry IDs and expected versions")
		}
		if !commonEmpty(operation.EntryID == "", operation.ExpectedVersion == 0, operation.Tags == nil, operation.TemporaryID == "", operation.Name == "", operation.Description == "", operation.TargetTopicID == "", operation.Title == "") {
			return errors.New("merge_duplicate_entries contains fields that are not allowed")
		}
	default:
		return fmt.Errorf("unsupported operation type %q", operation.Type)
	}
	return nil
}

func organizerOperationError(index int, err error) error {
	return fmt.Errorf("%w: operation %d: %v", ErrKnowledgeInvalid, index+1, err)
}

func projectedOrganizerEntry(entries map[string]*organizerProjectedEntry, entryID string, expectedVersion int) (*organizerProjectedEntry, error) {
	entry, found := entries[entryID]
	if !found {
		return nil, errors.New("entry ID was not supplied in organizer context")
	}
	if entry.status != models.KnowledgeStatusActive {
		return nil, errors.New("entry is no longer active")
	}
	if expectedVersion <= 0 || entry.version != expectedVersion {
		return nil, errors.New("expectedVersion does not match the supplied entry version")
	}
	return entry, nil
}

func validTemporaryTopicID(value string) bool {
	if !strings.HasPrefix(value, "new-topic-") || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func normalizeOrganizerTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(tag)), " "))
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > maxKnowledgeTagRunes {
			return nil, fmt.Errorf("tags must be %d characters or fewer", maxKnowledgeTagRunes)
		}
		if _, found := seen[tag]; found {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	if len(normalized) > maxKnowledgeTags {
		return nil, fmt.Errorf("at most %d tags are allowed", maxKnowledgeTags)
	}
	return normalized, nil
}

func sameNormalizedTagSet(left, right []string) bool {
	set := func(tags []string) map[string]struct{} {
		result := make(map[string]struct{}, len(tags))
		for _, tag := range tags {
			key := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(tag)), " "))
			if key != "" {
				result[key] = struct{}{}
			}
		}
		return result
	}
	leftSet := set(left)
	rightSet := set(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for key := range leftSet {
		if _, found := rightSet[key]; !found {
			return false
		}
	}
	return true
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mergeOrganizerTags(canonical, duplicate []string) ([]string, error) {
	result := append([]string(nil), canonical...)
	seen := make(map[string]struct{}, len(canonical))
	for _, tag := range canonical {
		seen[strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(tag)), " "))] = struct{}{}
	}
	for _, tag := range duplicate {
		key := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(tag)), " "))
		if key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	if len(result) > maxKnowledgeTags {
		return nil, fmt.Errorf("merged entry would exceed the %d tag limit", maxKnowledgeTags)
	}
	return result, nil
}

func (r *KnowledgeRepository) ApplyKnowledgeOrganizationPlan(
	claim *models.KnowledgeOrganizationClaim,
	context *models.KnowledgeOrganizerContext,
	plan *models.KnowledgeOrganizationPlan,
	now time.Time,
) (*models.KnowledgeOrganizationApplyResult, *models.KnowledgeTopic, error) {
	if claim == nil || claim.Topic.UserID == "" {
		return nil, nil, fmt.Errorf("%w: organization claim is required", ErrKnowledgeInvalid)
	}
	if context == nil || context.Target.ID != claim.Topic.ID {
		return nil, nil, fmt.Errorf("%w: organizer context does not match the claimed topic", ErrKnowledgeInvalid)
	}
	if err := ValidateKnowledgeOrganizationPlan(context, plan); err != nil {
		return nil, nil, err
	}
	now = now.UTC().Truncate(time.Microsecond)

	result := &models.KnowledgeOrganizationApplyResult{
		ChangedEntryIDs:  []string{},
		CreatedTopicIDs:  []string{},
		AffectedTopicIDs: []string{claim.Topic.ID},
	}
	var organizedTopic models.KnowledgeTopic
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockKnowledgeTopicHierarchyTx(tx, claim.Topic.UserID); err != nil {
			return err
		}

		referencedIDs := organizationReferencedEntryIDs(plan)
		entries := make(map[string]*models.KnowledgeEntry, len(referencedIDs))
		if len(referencedIDs) > 0 {
			var rows []models.KnowledgeEntry
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ? AND id IN ? AND status = ?", claim.Topic.UserID, referencedIDs, models.KnowledgeStatusActive).
				Find(&rows).Error; err != nil {
				return err
			}
			for index := range rows {
				entries[rows[index].ID] = &rows[index]
			}
			if len(entries) != len(referencedIDs) {
				return fmt.Errorf("%w: a referenced entry is missing or no longer active", ErrKnowledgeConflict)
			}
		}

		var target models.KnowledgeTopic
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", claim.Topic.ID, claim.Topic.UserID).
			First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeNotFound
			}
			return err
		}
		if target.OrganizationLeaseUntil == nil || !target.OrganizationLeaseUntil.Equal(claim.LeaseUntil) {
			return fmt.Errorf("%w: organization lease is no longer owned", ErrKnowledgeConflict)
		}
		if target.PendingWriteCount != context.Target.PendingWriteCount {
			return fmt.Errorf("%w: the target topic received new writes during organization", ErrKnowledgeConflict)
		}
		for _, operation := range plan.Operations {
			if operation.Type == "create_subtopic" && target.Depth+1 > maxKnowledgeDepth {
				return fmt.Errorf("%w: target topic moved to the maximum depth", ErrKnowledgeConflict)
			}
		}

		var children []models.KnowledgeTopic
		if err := tx.Where("user_id = ? AND parent_id = ?", claim.Topic.UserID, target.ID).
			Order("normalized_name ASC").
			Limit(1000).
			Find(&children).Error; err != nil {
			return err
		}
		currentChildren := make(map[string]models.KnowledgeTopic, len(children))
		existingByName := make(map[string]models.KnowledgeTopic, len(children))
		for _, child := range children {
			currentChildren[child.ID] = child
			existingByName[child.NormalizedName] = child
		}
		allowedTopics := map[string]struct{}{target.ID: {}}
		for _, suppliedChild := range context.Children {
			if _, stillDirect := currentChildren[suppliedChild.ID]; stillDirect {
				allowedTopics[suppliedChild.ID] = struct{}{}
			}
		}
		if len(entries) > 0 {
			for _, entry := range entries {
				if _, allowed := allowedTopics[entry.TopicID]; !allowed {
					return fmt.Errorf("%w: a referenced entry moved outside the organizer scope", ErrKnowledgeConflict)
				}
			}
		}

		temporaryTopics := make(map[string]string)
		for _, operation := range plan.Operations {
			if operation.Type != "create_subtopic" {
				continue
			}
			normalizedName := NormalizeKnowledgeName(operation.Name)
			if existing, found := existingByName[normalizedName]; found {
				if _, supplied := allowedTopics[existing.ID]; !supplied {
					return fmt.Errorf("%w: a matching subtopic exists outside the supplied context", ErrKnowledgeConflict)
				}
				temporaryTopics[operation.TemporaryID] = existing.ID
				continue
			}
			child := models.KnowledgeTopic{
				UserID:         claim.Topic.UserID,
				ParentID:       &target.ID,
				Name:           strings.TrimSpace(operation.Name),
				NormalizedName: normalizedName,
				Description:    strings.TrimSpace(operation.Description),
				Depth:          target.Depth + 1,
			}
			if err := tx.Create(&child).Error; err != nil {
				return err
			}
			temporaryTopics[operation.TemporaryID] = child.ID
			allowedTopics[child.ID] = struct{}{}
			existingByName[normalizedName] = child
			result.CreatedTopicIDs = append(result.CreatedTopicIDs, child.ID)
			addUniqueString(&result.AffectedTopicIDs, child.ID)
		}

		for _, operation := range plan.Operations {
			switch operation.Type {
			case "create_subtopic":
				result.OperationsApplied++
				continue
			case "normalize_tags":
				entry, err := currentOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
				if err != nil {
					return err
				}
				tags, err := normalizeOrganizerTags(operation.Tags)
				if err != nil || !sameNormalizedTagSet(entry.Tags, tags) {
					return fmt.Errorf("%w: tag normalization became invalid", ErrKnowledgeConflict)
				}
				if err := applyOrganizerEntryUpdate(tx, entry, operation.Type, map[string]interface{}{"tags": pq.StringArray(tags)}); err != nil {
					return err
				}
				entry.Tags = pq.StringArray(tags)
				addUniqueString(&result.AffectedTopicIDs, entry.TopicID)
			case "move_entry":
				entry, err := currentOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
				if err != nil {
					return err
				}
				targetID := operation.TargetTopicID
				if resolved, found := temporaryTopics[targetID]; found {
					targetID = resolved
				}
				if _, found := allowedTopics[targetID]; !found || targetID == entry.TopicID {
					return fmt.Errorf("%w: move destination became invalid", ErrKnowledgeConflict)
				}
				previousTopicID := entry.TopicID
				if err := applyOrganizerEntryUpdate(tx, entry, operation.Type, map[string]interface{}{"topic_id": targetID}); err != nil {
					return err
				}
				entry.TopicID = targetID
				addUniqueString(&result.AffectedTopicIDs, previousTopicID)
				addUniqueString(&result.AffectedTopicIDs, targetID)
			case "rename_entry":
				entry, err := currentOrganizerEntry(entries, operation.EntryID, operation.ExpectedVersion)
				if err != nil {
					return err
				}
				title := strings.TrimSpace(operation.Title)
				if title == "" || utf8.RuneCountInString(title) > maxKnowledgeTitleRunes || title == entry.Title {
					return fmt.Errorf("%w: entry title became invalid", ErrKnowledgeConflict)
				}
				if err := applyOrganizerEntryUpdate(tx, entry, operation.Type, map[string]interface{}{"title": title}); err != nil {
					return err
				}
				entry.Title = title
				addUniqueString(&result.AffectedTopicIDs, entry.TopicID)
			case "merge_duplicate_entries":
				canonical, err := currentOrganizerEntry(entries, operation.CanonicalEntryID, operation.CanonicalExpectedVersion)
				if err != nil {
					return err
				}
				duplicate, err := currentOrganizerEntry(entries, operation.DuplicateEntryID, operation.DuplicateExpectedVersion)
				if err != nil {
					return err
				}
				if canonical.ID == duplicate.ID {
					return fmt.Errorf("%w: an entry cannot be merged with itself", ErrKnowledgeInvalid)
				}
				tags, err := mergeOrganizerTags(canonical.Tags, duplicate.Tags)
				if err != nil {
					return fmt.Errorf("%w: %v", ErrKnowledgeInvalid, err)
				}
				groupOperation := "merge_duplicate:" + canonical.ID + ":" + duplicate.ID
				if err := applyOrganizerEntryUpdate(tx, canonical, groupOperation, map[string]interface{}{"tags": pq.StringArray(tags)}); err != nil {
					return err
				}
				canonical.Tags = pq.StringArray(tags)

				attributes := cloneKnowledgeAttributes(duplicate.Attributes)
				if existing, found := attributes["mergedIntoEntryId"]; found && fmt.Sprint(existing) != canonical.ID {
					return fmt.Errorf("%w: duplicate merge target changed", ErrKnowledgeConflict)
				}
				attributes["mergedIntoEntryId"] = canonical.ID
				if err := createEntryRevision(tx, duplicate, models.KnowledgeSourceOrganizer, groupOperation); err != nil {
					return err
				}
				deletedAt := now
				update := tx.Model(&models.KnowledgeEntry{}).
					Where("id = ? AND user_id = ? AND version = ?", duplicate.ID, duplicate.UserID, duplicate.Version).
					Updates(map[string]interface{}{
						"attributes": attributes,
						"status":     models.KnowledgeStatusMerged,
						"version":    duplicate.Version + 1,
						"deleted_at": deletedAt,
					})
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected != 1 {
					return fmt.Errorf("%w: duplicate entry version changed", ErrKnowledgeConflict)
				}
				duplicate.Attributes = attributes
				duplicate.Status = models.KnowledgeStatusMerged
				duplicate.Version++
				duplicate.DeletedAt = gorm.DeletedAt{Time: deletedAt, Valid: true}
				addUniqueString(&result.AffectedTopicIDs, canonical.TopicID)
				addUniqueString(&result.AffectedTopicIDs, duplicate.TopicID)
				addUniqueString(&result.ChangedEntryIDs, duplicate.ID)
			default:
				return fmt.Errorf("%w: unsupported operation %q", ErrKnowledgeInvalid, operation.Type)
			}
			result.OperationsApplied++
			if operation.EntryID != "" {
				addUniqueString(&result.ChangedEntryIDs, operation.EntryID)
			}
			if operation.CanonicalEntryID != "" {
				addUniqueString(&result.ChangedEntryIDs, operation.CanonicalEntryID)
			}
		}

		var completedAt time.Time
		if err := tx.Raw("SELECT clock_timestamp()").Scan(&completedAt).Error; err != nil {
			return err
		}
		completedAt = completedAt.UTC().Truncate(time.Microsecond)
		successValues := map[string]interface{}{
			"pending_write_count":      0,
			"organization_due_at":      nil,
			"organization_lease_until": nil,
			"last_organized_at":        completedAt,
		}
		update := tx.Model(&models.KnowledgeTopic{}).
			Where("id = ? AND user_id = ? AND organization_lease_until = ?", target.ID, target.UserID, claim.LeaseUntil).
			Updates(successValues)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: organization lease is no longer owned", ErrKnowledgeConflict)
		}
		return tx.Where("id = ? AND user_id = ?", target.ID, target.UserID).First(&organizedTopic).Error
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(result.ChangedEntryIDs)
	sort.Strings(result.CreatedTopicIDs)
	sort.Strings(result.AffectedTopicIDs)
	return result, &organizedTopic, nil
}

func organizationReferencedEntryIDs(plan *models.KnowledgeOrganizationPlan) []string {
	seen := make(map[string]struct{})
	for _, operation := range plan.Operations {
		for _, entryID := range []string{operation.EntryID, operation.CanonicalEntryID, operation.DuplicateEntryID} {
			if entryID != "" {
				seen[entryID] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for entryID := range seen {
		result = append(result, entryID)
	}
	sort.Strings(result)
	return result
}

func currentOrganizerEntry(entries map[string]*models.KnowledgeEntry, entryID string, expectedVersion int) (*models.KnowledgeEntry, error) {
	entry, found := entries[entryID]
	if !found {
		return nil, fmt.Errorf("%w: referenced entry is unavailable", ErrKnowledgeConflict)
	}
	if entry.Status != models.KnowledgeStatusActive || entry.DeletedAt.Valid {
		return nil, fmt.Errorf("%w: referenced entry is no longer active", ErrKnowledgeConflict)
	}
	if entry.Version != expectedVersion {
		return nil, fmt.Errorf("%w: entry version changed", ErrKnowledgeConflict)
	}
	return entry, nil
}

func applyOrganizerEntryUpdate(tx *gorm.DB, entry *models.KnowledgeEntry, operation string, values map[string]interface{}) error {
	if err := createEntryRevision(tx, entry, models.KnowledgeSourceOrganizer, operation); err != nil {
		return err
	}
	values["version"] = entry.Version + 1
	update := tx.Model(&models.KnowledgeEntry{}).
		Where("id = ? AND user_id = ? AND version = ?", entry.ID, entry.UserID, entry.Version).
		Updates(values)
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return fmt.Errorf("%w: entry version changed", ErrKnowledgeConflict)
	}
	entry.Version++
	return nil
}

func addUniqueString(values *[]string, value string) {
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	*values = append(*values, value)
}
