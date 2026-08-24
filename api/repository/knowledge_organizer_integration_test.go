package repository

import (
	"buybuddy-api/models"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestKnowledgeOrganizerPostgreSQLSchedulingClaimsContextAndMergeUndo(t *testing.T) {
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run PostgreSQL organizer integration coverage")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.KnowledgeTopic{}, &models.KnowledgeEntry{}, &models.KnowledgeEntryRevision{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	user := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Organizer User", ClientID: uuid.NewString()}
	otherUser := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Other User", ClientID: uuid.NewString()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create organizer user: %v", err)
	}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}
	t.Cleanup(func() {
		userIDs := []string{user.ID, otherUser.ID}
		db.Unscoped().Where("user_id IN ?", userIDs).Delete(&models.KnowledgeEntryRevision{})
		db.Unscoped().Where("user_id IN ?", userIDs).Delete(&models.KnowledgeEntry{})
		db.Unscoped().Where("user_id IN ?", userIDs).Delete(&models.KnowledgeTopic{})
		db.Unscoped().Where("id IN ?", userIDs).Delete(&models.User{})
	})

	repo := NewKnowledgeRepository(db)
	schedulingTopic := &models.KnowledgeTopic{Name: "Scheduling"}
	destination := &models.KnowledgeTopic{Name: "Destination"}
	if err := repo.CreateTopic(user.ID, schedulingTopic); err != nil {
		t.Fatalf("CreateTopic(scheduling) error = %v", err)
	}
	if err := repo.CreateTopic(user.ID, destination); err != nil {
		t.Fatalf("CreateTopic(destination) error = %v", err)
	}
	var firstEntry *models.KnowledgeEntry
	for index := 0; index < 5; index++ {
		entry := &models.KnowledgeEntry{
			TopicID: schedulingTopic.ID,
			Kind:    "note",
			Title:   fmt.Sprintf("Scheduled note %d", index),
			Body:    "Schedule this entry.",
		}
		if err := repo.CreateEntry(user.ID, entry); err != nil {
			t.Fatalf("CreateEntry(%d) error = %v", index, err)
		}
		if firstEntry == nil {
			firstEntry = entry
		}
	}
	storedTopic, err := repo.GetTopic(user.ID, schedulingTopic.ID)
	if err != nil {
		t.Fatalf("GetTopic(scheduling) error = %v", err)
	}
	if storedTopic.PendingWriteCount != 5 || storedTopic.OrganizationDueAt == nil || storedTopic.OrganizationDueAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("fifth-write schedule = %#v, want immediate due with count 5", storedTopic)
	}

	destinationID := destination.ID
	if _, err := repo.UpdateEntry(
		user.ID,
		firstEntry.ID,
		firstEntry.Version,
		models.KnowledgeEntryMutation{TopicID: &destinationID},
		models.KnowledgeSourceManual,
	); err != nil {
		t.Fatalf("move scheduled entry: %v", err)
	}
	storedSource, _ := repo.GetTopic(user.ID, schedulingTopic.ID)
	storedDestination, _ := repo.GetTopic(user.ID, destination.ID)
	if storedSource.PendingWriteCount != 6 || storedDestination.PendingWriteCount != 1 {
		t.Fatalf("move counts source/destination = %d/%d, want 6/1", storedSource.PendingWriteCount, storedDestination.PendingWriteCount)
	}
	if storedDestination.OrganizationDueAt == nil || storedDestination.OrganizationDueAt.Before(time.Now().Add(47*time.Hour)) {
		t.Fatalf("destination idle deadline = %v, want about two days", storedDestination.OrganizationDueAt)
	}

	dueNow := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&models.KnowledgeTopic{}).Where("id = ?", schedulingTopic.ID).Updates(map[string]interface{}{
		"pending_write_count":      1,
		"organization_due_at":      dueNow,
		"organization_lease_until": nil,
	}).Error; err != nil {
		t.Fatalf("make topic claimable: %v", err)
	}
	start := make(chan struct{})
	claims := make(chan *models.KnowledgeOrganizationClaim, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			claim, claimErr := repo.ClaimNextKnowledgeOrganization(time.Now())
			claims <- claim
			errs <- claimErr
		}()
	}
	close(start)
	wait.Wait()
	close(claims)
	close(errs)
	for claimErr := range errs {
		if claimErr != nil {
			t.Fatalf("ClaimNextKnowledgeOrganization() error = %v", claimErr)
		}
	}
	claimCount := 0
	var claimed *models.KnowledgeOrganizationClaim
	for claim := range claims {
		if claim != nil && claim.Topic.ID == schedulingTopic.ID {
			claimCount++
			claimed = claim
		}
	}
	if claimCount != 1 {
		t.Fatalf("concurrent claims = %d, want exactly one", claimCount)
	}
	if _, err := repo.ClaimKnowledgeOrganization(user.ID, schedulingTopic.ID, time.Now()); !errors.Is(err, ErrKnowledgeConflict) {
		t.Fatalf("duplicate explicit claim error = %v, want ErrKnowledgeConflict", err)
	}
	failureTime := time.Now().UTC()
	if err := repo.FailKnowledgeOrganization(claimed, failureTime); err != nil {
		t.Fatalf("FailKnowledgeOrganization() error = %v", err)
	}
	failedTopic, _ := repo.GetTopic(user.ID, schedulingTopic.ID)
	if failedTopic.OrganizationLeaseUntil != nil ||
		failedTopic.OrganizationDueAt == nil ||
		failedTopic.OrganizationDueAt.Before(failureTime.Add(6*time.Hour-time.Minute)) {
		t.Fatalf("failure state = %#v, want cleared lease and six-hour retry", failedTopic)
	}

	cleanTopic := &models.KnowledgeTopic{Name: "Clean explicit topic"}
	if err := repo.CreateTopic(user.ID, cleanTopic); err != nil {
		t.Fatalf("CreateTopic(clean explicit) error = %v", err)
	}
	cleanClaim, err := repo.ClaimKnowledgeOrganization(user.ID, cleanTopic.ID, time.Now())
	if err != nil {
		t.Fatalf("claim clean explicit topic: %v", err)
	}
	if !cleanClaim.SyntheticManual || cleanClaim.Topic.PendingWriteCount != 0 {
		t.Fatalf("clean explicit claim = %#v, want synthetic without pending writes", cleanClaim)
	}
	if err := repo.FailKnowledgeOrganization(cleanClaim, failureTime); err != nil {
		t.Fatalf("fail clean explicit claim: %v", err)
	}
	cleanAfterFailure, _ := repo.GetTopic(user.ID, cleanTopic.ID)
	if cleanAfterFailure.PendingWriteCount != 0 || cleanAfterFailure.OrganizationDueAt != nil || cleanAfterFailure.OrganizationLeaseUntil != nil {
		t.Fatalf("clean explicit failure state = %#v, want restored clean schedule", cleanAfterFailure)
	}

	releasedClaim, err := repo.ClaimKnowledgeOrganization(user.ID, cleanTopic.ID, time.Now())
	if err != nil {
		t.Fatalf("claim clean topic for release: %v", err)
	}
	if err := repo.ReleaseKnowledgeOrganization(context.Background(), releasedClaim); err != nil {
		t.Fatalf("ReleaseKnowledgeOrganization() error = %v", err)
	}
	releasedTopic, _ := repo.GetTopic(user.ID, cleanTopic.ID)
	if releasedTopic.PendingWriteCount != 0 || releasedTopic.OrganizationDueAt != nil || releasedTopic.OrganizationLeaseUntil != nil {
		t.Fatalf("released clean state = %#v, want restored clean schedule", releasedTopic)
	}
	replacementClaim, err := repo.ClaimKnowledgeOrganization(user.ID, cleanTopic.ID, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("claim replacement lease: %v", err)
	}
	if err := repo.ReleaseKnowledgeOrganization(context.Background(), releasedClaim); !errors.Is(err, ErrKnowledgeConflict) {
		t.Fatalf("stale release error = %v, want ErrKnowledgeConflict", err)
	}
	stillLeased, _ := repo.GetTopic(user.ID, cleanTopic.ID)
	if stillLeased.OrganizationLeaseUntil == nil || !stillLeased.OrganizationLeaseUntil.Equal(replacementClaim.LeaseUntil) {
		t.Fatalf("stale release changed replacement lease: %#v", stillLeased)
	}
	if err := repo.ReleaseKnowledgeOrganization(context.Background(), replacementClaim); err != nil {
		t.Fatalf("release replacement lease: %v", err)
	}

	contextTopic := &models.KnowledgeTopic{Name: "Context Caps"}
	if err := repo.CreateTopic(user.ID, contextTopic); err != nil {
		t.Fatalf("CreateTopic(context) error = %v", err)
	}
	if err := db.Model(&models.KnowledgeTopic{}).Where("id = ?", contextTopic.ID).
		Update("last_organized_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("set last organized: %v", err)
	}
	for index := 0; index < 25; index++ {
		child := &models.KnowledgeTopic{
			UserID:         user.ID,
			ParentID:       &contextTopic.ID,
			Name:           fmt.Sprintf("Child %02d", index),
			NormalizedName: fmt.Sprintf("child %02d", index),
			Depth:          1,
		}
		if err := db.Create(child).Error; err != nil {
			t.Fatalf("create context child %d: %v", index, err)
		}
	}
	for index := 0; index < 12; index++ {
		older := &models.KnowledgeEntry{
			UserID:     user.ID,
			TopicID:    contextTopic.ID,
			Kind:       "note",
			Title:      fmt.Sprintf("Milk history %02d", index),
			Body:       "An older milk note.",
			Attributes: models.KnowledgeAttributes{},
			Tags:       pq.StringArray{fmt.Sprintf("older-%02d", index)},
			Source:     models.KnowledgeSourceManual,
			Status:     models.KnowledgeStatusActive,
			Version:    1,
			CreatedAt:  time.Now().Add(-2 * time.Hour),
			UpdatedAt:  time.Now().Add(-2 * time.Hour),
		}
		if err := db.Create(older).Error; err != nil {
			t.Fatalf("create older entry %d: %v", index, err)
		}
	}
	recentEntries := make([]models.KnowledgeEntry, 0, 25)
	for index := 0; index < 25; index++ {
		recent := models.KnowledgeEntry{
			UserID:     user.ID,
			TopicID:    contextTopic.ID,
			Kind:       "note",
			Title:      fmt.Sprintf("Milk note %02d", index),
			Body:       strings.Repeat("é", 700),
			Attributes: models.KnowledgeAttributes{},
			Tags:       pq.StringArray{fmt.Sprintf("recent-%02d", index)},
			Source:     models.KnowledgeSourceManual,
			Status:     models.KnowledgeStatusActive,
			Version:    1,
		}
		if err := db.Create(&recent).Error; err != nil {
			t.Fatalf("create recent entry %d: %v", index, err)
		}
		recentEntries = append(recentEntries, recent)
	}
	for index := 0; index < 15; index++ {
		entry := recentEntries[index]
		revision := models.KnowledgeEntryRevision{
			EntryID:    entry.ID,
			UserID:     user.ID,
			Version:    entry.Version,
			TopicID:    entry.TopicID,
			Kind:       entry.Kind,
			Title:      entry.Title,
			Body:       entry.Body,
			Attributes: models.KnowledgeAttributes{},
			Tags:       entry.Tags,
			Status:     entry.Status,
			ChangedBy:  models.KnowledgeSourceOrganizer,
			Operation:  "rename_entry",
		}
		if err := db.Create(&revision).Error; err != nil {
			t.Fatalf("create organizer revision %d: %v", index, err)
		}
	}
	otherTopic := &models.KnowledgeTopic{UserID: otherUser.ID, Name: "Other", NormalizedName: "other", Depth: 0}
	if err := db.Create(otherTopic).Error; err != nil {
		t.Fatalf("create other-user topic: %v", err)
	}
	if err := db.Create(&models.KnowledgeEntry{
		UserID:     otherUser.ID,
		TopicID:    otherTopic.ID,
		Kind:       "note",
		Title:      "Milk secret",
		Body:       "Must not leak.",
		Attributes: models.KnowledgeAttributes{},
		Tags:       pq.StringArray{"private"},
		Source:     models.KnowledgeSourceManual,
		Status:     models.KnowledgeStatusActive,
		Version:    1,
	}).Error; err != nil {
		t.Fatalf("create other-user entry: %v", err)
	}

	focused, err := repo.LoadKnowledgeOrganizerContext(user.ID, contextTopic.ID)
	if err != nil {
		t.Fatalf("LoadKnowledgeOrganizerContext() error = %v", err)
	}
	if len(focused.Children) != 20 || len(focused.RecentEntries) != 20 || len(focused.Tags) != 30 ||
		len(focused.SimilarEntries) != 10 || len(focused.RecentRevisions) != 10 {
		t.Fatalf(
			"context caps children/recent/tags/similar/revisions = %d/%d/%d/%d/%d",
			len(focused.Children),
			len(focused.RecentEntries),
			len(focused.Tags),
			len(focused.SimilarEntries),
			len(focused.RecentRevisions),
		)
	}
	for _, entry := range append(append([]models.KnowledgeOrganizerEntry{}, focused.RecentEntries...), focused.SimilarEntries...) {
		if len([]rune(entry.Body)) > 500 || strings.Contains(entry.Body, "Must not leak") {
			t.Fatalf("unbounded or cross-user organizer entry: %#v", entry)
		}
	}

	mergeTopic := &models.KnowledgeTopic{Name: "Merge"}
	if err := repo.CreateTopic(user.ID, mergeTopic); err != nil {
		t.Fatalf("CreateTopic(merge) error = %v", err)
	}
	canonical := &models.KnowledgeEntry{TopicID: mergeTopic.ID, Kind: "preference", Title: "Milk", Body: "Brand X is best.", Tags: pq.StringArray{"milk"}}
	duplicate := &models.KnowledgeEntry{TopicID: mergeTopic.ID, Kind: "preference", Title: "Milk duplicate", Body: "Brand X is best.", Tags: pq.StringArray{"favorite"}}
	if err := repo.CreateEntry(user.ID, canonical); err != nil {
		t.Fatalf("create canonical: %v", err)
	}
	if err := repo.CreateEntry(user.ID, duplicate); err != nil {
		t.Fatalf("create duplicate: %v", err)
	}
	mergeClaim, err := repo.ClaimKnowledgeOrganization(user.ID, mergeTopic.ID, time.Now())
	if err != nil {
		t.Fatalf("claim merge topic: %v", err)
	}
	mergeContext, err := repo.LoadKnowledgeOrganizerContext(user.ID, mergeTopic.ID)
	if err != nil {
		t.Fatalf("load merge context: %v", err)
	}
	mergePlan := &models.KnowledgeOrganizationPlan{Operations: []models.KnowledgeOrganizationOperation{{
		Type:                     "merge_duplicate_entries",
		CanonicalEntryID:         canonical.ID,
		DuplicateEntryID:         duplicate.ID,
		CanonicalExpectedVersion: 1,
		DuplicateExpectedVersion: 1,
		Reason:                   "These entries are exact duplicates.",
	}}}
	mergeResult, organized, err := repo.ApplyKnowledgeOrganizationPlan(mergeClaim, mergeContext, mergePlan, time.Now())
	if err != nil {
		t.Fatalf("ApplyKnowledgeOrganizationPlan(merge) error = %v", err)
	}
	if mergeResult.OperationsApplied != 1 || organized.PendingWriteCount != 0 || organized.OrganizationDueAt != nil || organized.OrganizationLeaseUntil != nil || organized.LastOrganizedAt == nil {
		t.Fatalf("merge result/topic = %#v / %#v", mergeResult, organized)
	}
	storedCanonical, err := repo.GetEntry(user.ID, canonical.ID)
	if err != nil {
		t.Fatalf("load canonical after merge: %v", err)
	}
	var storedDuplicate models.KnowledgeEntry
	if err := db.Unscoped().Where("id = ? AND user_id = ?", duplicate.ID, user.ID).First(&storedDuplicate).Error; err != nil {
		t.Fatalf("load duplicate after merge: %v", err)
	}
	if storedCanonical.Body != canonical.Body || storedDuplicate.Body != duplicate.Body {
		t.Fatal("organizer merge rewrote an entry body")
	}
	if storedCanonical.UpdatedAt.After(*organized.LastOrganizedAt) {
		t.Fatalf("canonical updated_at %s is after last_organized_at %s", storedCanonical.UpdatedAt, *organized.LastOrganizedAt)
	}
	postOrganizationContext, err := repo.LoadKnowledgeOrganizerContext(user.ID, mergeTopic.ID)
	if err != nil {
		t.Fatalf("load post-organization context: %v", err)
	}
	for _, entry := range postOrganizationContext.RecentEntries {
		if entry.ID == canonical.ID || entry.ID == duplicate.ID {
			t.Fatalf("organizer-touched entry %s leaked into the next recent-additions window", entry.ID)
		}
	}
	if storedDuplicate.Status != models.KnowledgeStatusMerged || !storedDuplicate.DeletedAt.Valid || fmt.Sprint(storedDuplicate.Attributes["mergedIntoEntryId"]) != canonical.ID {
		t.Fatalf("soft-merged duplicate = %#v", storedDuplicate)
	}
	var mergeRevisionCount int64
	if err := db.Model(&models.KnowledgeEntryRevision{}).
		Where("user_id = ? AND operation = ?", user.ID, "merge_duplicate:"+canonical.ID+":"+duplicate.ID).
		Count(&mergeRevisionCount).Error; err != nil || mergeRevisionCount != 2 {
		t.Fatalf("merge revision count = %d, %v; want 2", mergeRevisionCount, err)
	}
	if _, err := repo.UndoEntry(user.ID, duplicate.ID, 2); err != nil {
		t.Fatalf("UndoEntry(merge) error = %v", err)
	}
	restoredCanonical, _ := repo.GetEntry(user.ID, canonical.ID)
	restoredDuplicate, err := repo.GetEntry(user.ID, duplicate.ID)
	if err != nil {
		t.Fatalf("duplicate was not restored: %v", err)
	}
	if restoredCanonical.Version != 3 || restoredDuplicate.Version != 3 ||
		len(restoredCanonical.Tags) != 1 || restoredCanonical.Tags[0] != "milk" ||
		restoredDuplicate.Status != models.KnowledgeStatusActive || restoredDuplicate.DeletedAt.Valid {
		t.Fatalf("merge undo canonical/duplicate = %#v / %#v", restoredCanonical, restoredDuplicate)
	}

	rollbackClaim, err := repo.ClaimKnowledgeOrganization(user.ID, mergeTopic.ID, time.Now().Add(11*time.Minute))
	if err != nil {
		t.Fatalf("claim rollback topic: %v", err)
	}
	rollbackContext, err := repo.LoadKnowledgeOrganizerContext(user.ID, mergeTopic.ID)
	if err != nil {
		t.Fatalf("load rollback context: %v", err)
	}
	rollbackVersion := restoredCanonical.Version
	rollbackPlan := &models.KnowledgeOrganizationPlan{Operations: []models.KnowledgeOrganizationOperation{
		{
			Type:        "create_subtopic",
			TemporaryID: "new-topic-rollback",
			Name:        "Must Roll Back",
			Reason:      "Test transactional rollback.",
		},
		{
			Type:            "rename_entry",
			EntryID:         canonical.ID,
			ExpectedVersion: rollbackVersion,
			Title:           "Should not persist",
			Reason:          "Test transactional rollback.",
		},
	}}
	if err := db.Model(&models.KnowledgeEntry{}).Where("id = ?", canonical.ID).Update("version", rollbackVersion+1).Error; err != nil {
		t.Fatalf("make rollback plan stale: %v", err)
	}
	if _, _, err := repo.ApplyKnowledgeOrganizationPlan(rollbackClaim, rollbackContext, rollbackPlan, time.Now()); !errors.Is(err, ErrKnowledgeConflict) {
		t.Fatalf("rollback plan error = %v, want ErrKnowledgeConflict", err)
	}
	var rolledBackTopicCount int64
	if err := db.Model(&models.KnowledgeTopic{}).
		Where("user_id = ? AND parent_id = ? AND normalized_name = ?", user.ID, mergeTopic.ID, "must roll back").
		Count(&rolledBackTopicCount).Error; err != nil || rolledBackTopicCount != 0 {
		t.Fatalf("rolled-back subtopic count = %d, %v; want 0", rolledBackTopicCount, err)
	}
}
