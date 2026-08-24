package repository

import (
	"buybuddy-api/models"
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNormalizeKnowledgeName(t *testing.T) {
	if got := NormalizeKnowledgeName("  Project   NOTES "); got != "project notes" {
		t.Fatalf("NormalizeKnowledgeName() = %q, want %q", got, "project notes")
	}
}

func TestNormalizeKnowledgeTags(t *testing.T) {
	got, err := NormalizeKnowledgeTags([]string{" Milk ", "favorite", "milk", "", "Favorite"})
	if err != nil {
		t.Fatalf("NormalizeKnowledgeTags() error = %v", err)
	}
	want := []string{"Milk", "favorite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeKnowledgeTags() = %#v, want %#v", got, want)
	}
}

func TestApplyKnowledgeAttributeMutationReplacesCompleteObject(t *testing.T) {
	current := models.KnowledgeAttributes{
		"kept":    "old value",
		"removed": true,
	}
	replacement := models.KnowledgeAttributes{"kept": "new value"}

	got, err := applyKnowledgeAttributeMutation(current, &replacement, true)
	if err != nil {
		t.Fatalf("applyKnowledgeAttributeMutation() error = %v", err)
	}
	if !reflect.DeepEqual(got, replacement) {
		t.Fatalf("replacement = %#v, want %#v", got, replacement)
	}
	if _, exists := got["removed"]; exists {
		t.Fatalf("replacement retained removed key: %#v", got)
	}
}

func TestApplyKnowledgeAttributeMutationRetainsExplicitEmptyObject(t *testing.T) {
	current := models.KnowledgeAttributes{"removed": true}
	empty := models.KnowledgeAttributes{}

	got, err := applyKnowledgeAttributeMutation(current, &empty, true)
	if err != nil {
		t.Fatalf("applyKnowledgeAttributeMutation() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty replacement = %#v, want non-nil empty object", got)
	}
}

func TestApplyKnowledgeAttributeMutationStillMergesByDefault(t *testing.T) {
	current := models.KnowledgeAttributes{"existing": true}
	incoming := models.KnowledgeAttributes{"added": true}

	got, err := applyKnowledgeAttributeMutation(current, &incoming, false)
	if err != nil {
		t.Fatalf("applyKnowledgeAttributeMutation() error = %v", err)
	}
	if got["existing"] != true || got["added"] != true {
		t.Fatalf("merged attributes = %#v, want existing and added keys", got)
	}
}

func TestBuildKnowledgeTopicTreePinsInboxAndCountsChildren(t *testing.T) {
	inbox := models.KnowledgeTopic{ID: "inbox", Name: "Inbox", NormalizedName: "inbox"}
	projects := models.KnowledgeTopic{ID: "projects", Name: "Projects", NormalizedName: "projects"}
	parentID := projects.ID
	buyBuddy := models.KnowledgeTopic{ID: "buybuddy", ParentID: &parentID, Name: "BuyBuddy", NormalizedName: "buybuddy", Depth: 1}

	tree := BuildKnowledgeTopicTree([]models.KnowledgeTopic{projects, buyBuddy, inbox}, map[string]int{
		"inbox":    2,
		"buybuddy": 3,
	})
	if len(tree) != 2 {
		t.Fatalf("len(tree) = %d, want 2", len(tree))
	}
	if tree[0].ID != inbox.ID || tree[0].EntryCount != 2 {
		t.Fatalf("first root = %#v, want pinned Inbox with count 2", tree[0])
	}
	if tree[1].ChildCount != 1 || len(tree[1].Children) != 1 || tree[1].Children[0].EntryCount != 3 {
		t.Fatalf("projects node = %#v, want one counted child", tree[1])
	}
}

func TestKnowledgeOwnershipRevisionUndoAndSoftDelete(t *testing.T) {
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run PostgreSQL integration coverage")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.KnowledgeTopic{}, &models.KnowledgeEntry{}, &models.KnowledgeEntryRevision{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	userOne := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Knowledge One", ClientID: uuid.NewString()}
	userTwo := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Knowledge Two", ClientID: uuid.NewString()}
	if err := db.Create(&userOne).Error; err != nil {
		t.Fatalf("create user one: %v", err)
	}
	if err := db.Create(&userTwo).Error; err != nil {
		t.Fatalf("create user two: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("user_id IN ?", []string{userOne.ID, userTwo.ID}).Delete(&models.KnowledgeEntryRevision{})
		db.Unscoped().Where("user_id IN ?", []string{userOne.ID, userTwo.ID}).Delete(&models.KnowledgeEntry{})
		db.Unscoped().Where("user_id IN ?", []string{userOne.ID, userTwo.ID}).Delete(&models.KnowledgeTopic{})
		db.Unscoped().Where("id IN ?", []string{userOne.ID, userTwo.ID}).Delete(&models.User{})
	})

	repo := NewKnowledgeRepository(db)
	inbox, err := repo.EnsureInbox(userOne.ID)
	if err != nil {
		t.Fatalf("EnsureInbox() error = %v", err)
	}
	secondInbox, err := repo.EnsureInbox(userOne.ID)
	if err != nil || secondInbox.ID != inbox.ID {
		t.Fatalf("second EnsureInbox() = %#v, %v; want same Inbox", secondInbox, err)
	}

	entry := &models.KnowledgeEntry{
		UserID:  userTwo.ID,
		TopicID: inbox.ID,
		Kind:    "diary",
		Title:   "Productive Monday",
		Body:    "Today was productive.",
		Attributes: models.KnowledgeAttributes{
			"mood": "focused",
		},
		Tags:       []string{"diary", "productive"},
		OccurredAt: knowledgeTimePointer(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)),
		Source:     models.KnowledgeSourceManual,
	}
	if err := repo.CreateEntry(userOne.ID, entry); err != nil {
		t.Fatalf("CreateEntry() error = %v", err)
	}
	if entry.UserID != userOne.ID {
		t.Fatalf("CreateEntry() retained model-provided user ID %q, want authenticated user %q", entry.UserID, userOne.ID)
	}
	if _, err := repo.GetEntry(userTwo.ID, entry.ID); !errors.Is(err, ErrKnowledgeNotFound) {
		t.Fatalf("cross-user GetEntry() error = %v, want ErrKnowledgeNotFound", err)
	}
	results, err := repo.Search(userOne.ID, models.KnowledgeSearchFilter{Query: "productive", Tag: "DIARY", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 || len(results[0].Breadcrumb) != 1 || results[0].Entry.ID != entry.ID {
		t.Fatalf("Search() = %#v, want created entry with Inbox breadcrumb", results)
	}

	newTitle := "A productive Monday"
	newKind := "preference"
	newAttributes := models.KnowledgeAttributes{"rating": float64(5)}
	newOccurredAt := time.Date(2026, 8, 19, 10, 30, 0, 0, time.UTC)
	mutation := models.KnowledgeEntryMutation{
		Kind:       &newKind,
		Title:      &newTitle,
		Attributes: &newAttributes,
		OccurredAt: &newOccurredAt,
	}
	if _, err := repo.UpdateEntry(userTwo.ID, entry.ID, 1, mutation, models.KnowledgeSourceManual); !errors.Is(err, ErrKnowledgeNotFound) {
		t.Fatalf("cross-user UpdateEntry() error = %v, want ErrKnowledgeNotFound", err)
	}
	updated, err := repo.UpdateEntry(userOne.ID, entry.ID, 1, mutation, models.KnowledgeSourceManual)
	if err != nil {
		t.Fatalf("UpdateEntry() error = %v", err)
	}
	if updated.Version != 2 || updated.Title != newTitle || updated.Kind != newKind ||
		updated.OccurredAt == nil || !updated.OccurredAt.Equal(newOccurredAt) {
		t.Fatalf("updated = %#v, want changed kind, title, and occurredAt at version 2", updated)
	}
	if updated.Attributes["mood"] != "focused" || updated.Attributes["rating"] != float64(5) {
		t.Fatalf("updated attributes = %#v, want retained and added keys", updated.Attributes)
	}
	var revisionCount int64
	if err := db.Model(&models.KnowledgeEntryRevision{}).
		Where("entry_id = ? AND user_id = ? AND version = 1", entry.ID, userOne.ID).
		Count(&revisionCount).Error; err != nil || revisionCount != 1 {
		t.Fatalf("revision count = %d, error = %v; want 1", revisionCount, err)
	}
	var firstRevision models.KnowledgeEntryRevision
	if err := db.Where("entry_id = ? AND user_id = ? AND version = 1", entry.ID, userOne.ID).
		First(&firstRevision).Error; err != nil {
		t.Fatalf("load first revision: %v", err)
	}
	if firstRevision.Kind != entry.Kind || firstRevision.OccurredAt == nil ||
		!firstRevision.OccurredAt.Equal(*entry.OccurredAt) {
		t.Fatalf("first revision = %#v, want original kind and occurredAt", firstRevision)
	}

	restored, err := repo.UndoEntry(userOne.ID, entry.ID, 2)
	if err != nil {
		t.Fatalf("UndoEntry() error = %v", err)
	}
	if restored.Title != entry.Title || restored.Kind != entry.Kind || restored.Version != 3 ||
		restored.OccurredAt == nil || !restored.OccurredAt.Equal(*entry.OccurredAt) {
		t.Fatalf("restored = %#v, want complete original state at version 3", restored)
	}

	restored, err = repo.UndoEntry(userOne.ID, entry.ID, 3)
	if err != nil {
		t.Fatalf("second consecutive UndoEntry() error = %v", err)
	}
	if restored.Title != newTitle || restored.Kind != newKind || restored.Version != 4 ||
		restored.OccurredAt == nil || !restored.OccurredAt.Equal(newOccurredAt) {
		t.Fatalf("second undo = %#v, want reversal to updated state at version 4", restored)
	}

	if err := repo.DeleteEntry(userOne.ID, entry.ID, 4, models.KnowledgeSourceManual); err != nil {
		t.Fatalf("DeleteEntry() error = %v", err)
	}
	if _, err := repo.GetEntry(userOne.ID, entry.ID); !errors.Is(err, ErrKnowledgeNotFound) {
		t.Fatalf("GetEntry() after delete error = %v, want ErrKnowledgeNotFound", err)
	}
	restored, err = repo.UndoEntry(userOne.ID, entry.ID, 5)
	if err != nil {
		t.Fatalf("UndoEntry() after delete error = %v", err)
	}
	if restored.Version != 6 || restored.Status != models.KnowledgeStatusActive || restored.DeletedAt.Valid {
		t.Fatalf("restored after delete = %#v, want active version 6", restored)
	}

	restored, err = repo.UndoEntry(userOne.ID, entry.ID, 6)
	if err != nil {
		t.Fatalf("UndoEntry() reversing restore error = %v", err)
	}
	if restored.Version != 7 || restored.Status != "deleted" || !restored.DeletedAt.Valid {
		t.Fatalf("reversed restore = %#v, want deleted version 7", restored)
	}
	if _, err := repo.GetEntry(userOne.ID, entry.ID); !errors.Is(err, ErrKnowledgeNotFound) {
		t.Fatalf("GetEntry() after reversing restore error = %v, want ErrKnowledgeNotFound", err)
	}

	restored, err = repo.UndoEntry(userOne.ID, entry.ID, 7)
	if err != nil {
		t.Fatalf("UndoEntry() restoring reversed deletion error = %v", err)
	}
	if restored.Version != 8 || restored.Status != models.KnowledgeStatusActive || restored.DeletedAt.Valid {
		t.Fatalf("restored reversed deletion = %#v, want active version 8", restored)
	}

	replacementAttributes := models.KnowledgeAttributes{"replacement": "only"}
	restored, err = repo.UpdateEntry(
		userOne.ID,
		entry.ID,
		8,
		models.KnowledgeEntryMutation{
			Attributes:        &replacementAttributes,
			ReplaceAttributes: true,
		},
		models.KnowledgeSourceManual,
	)
	if err != nil {
		t.Fatalf("UpdateEntry() attribute replacement error = %v", err)
	}
	if !reflect.DeepEqual(restored.Attributes, replacementAttributes) {
		t.Fatalf("replaced attributes = %#v, want %#v", restored.Attributes, replacementAttributes)
	}

	emptyAttributes := models.KnowledgeAttributes{}
	restored, err = repo.UpdateEntry(
		userOne.ID,
		entry.ID,
		9,
		models.KnowledgeEntryMutation{
			Attributes:        &emptyAttributes,
			ReplaceAttributes: true,
		},
		models.KnowledgeSourceManual,
	)
	if err != nil {
		t.Fatalf("UpdateEntry() empty attribute replacement error = %v", err)
	}
	if restored.Version != 10 || restored.Attributes == nil || len(restored.Attributes) != 0 {
		t.Fatalf("empty replacement = %#v at version %d, want non-nil empty object at version 10", restored.Attributes, restored.Version)
	}
	var replacementRevision models.KnowledgeEntryRevision
	if err := db.Where("entry_id = ? AND user_id = ? AND version = ?", entry.ID, userOne.ID, 9).
		First(&replacementRevision).Error; err != nil {
		t.Fatalf("load pre-empty-replacement revision: %v", err)
	}
	if !reflect.DeepEqual(replacementRevision.Attributes, replacementAttributes) {
		t.Fatalf("pre-mutation revision attributes = %#v, want %#v", replacementRevision.Attributes, replacementAttributes)
	}

	fallbackOne, created, err := repo.CreateInboxFallback(context.Background(), userOne.ID, "remember this exact fallback", "remember this exact fallback")
	if err != nil || !created {
		t.Fatalf("first CreateInboxFallback() = %#v, %t, %v; want created", fallbackOne, created, err)
	}
	fallbackTwo, created, err := repo.CreateInboxFallback(context.Background(), userOne.ID, "remember this exact fallback", "remember this exact fallback")
	if err != nil || created || fallbackTwo.ID != fallbackOne.ID {
		t.Fatalf("second CreateInboxFallback() = %#v, %t, %v; want existing %s", fallbackTwo, created, err, fallbackOne.ID)
	}
}

func knowledgeTimePointer(value time.Time) *time.Time {
	return &value
}

func TestKnowledgeTopicConcurrentMovesPreserveHierarchy(t *testing.T) {
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run PostgreSQL integration coverage")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.KnowledgeTopic{}, &models.KnowledgeEntry{}, &models.KnowledgeEntryRevision{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	user := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Hierarchy User", ClientID: uuid.NewString()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeEntryRevision{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeEntry{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeTopic{})
		db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{})
	})

	repo := NewKnowledgeRepository(db)
	left := &models.KnowledgeTopic{Name: "Concurrent Left"}
	right := &models.KnowledgeTopic{Name: "Concurrent Right"}
	if err := repo.CreateTopic(user.ID, left); err != nil {
		t.Fatalf("CreateTopic(left) error = %v", err)
	}
	if err := repo.CreateTopic(user.ID, right); err != nil {
		t.Fatalf("CreateTopic(right) error = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		parentID := right.ID
		_, err := repo.UpdateTopic(user.ID, left.ID, models.UpdateKnowledgeTopicRequest{ParentID: &parentID})
		results <- err
	}()
	go func() {
		defer wait.Done()
		<-start
		parentID := left.ID
		_, err := repo.UpdateTopic(user.ID, right.ID, models.UpdateKnowledgeTopicRequest{ParentID: &parentID})
		results <- err
	}()
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	invalidMoves := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrKnowledgeInvalid):
			invalidMoves++
		default:
			t.Fatalf("concurrent UpdateTopic() unexpected error = %v", result)
		}
	}
	if successes != 1 || invalidMoves != 1 {
		t.Fatalf("concurrent moves produced %d successes and %d invalid moves; want one of each", successes, invalidMoves)
	}

	topics, err := repo.ListTopics(user.ID)
	if err != nil {
		t.Fatalf("ListTopics() error = %v", err)
	}
	byID := make(map[string]models.KnowledgeTopic, len(topics))
	for _, topic := range topics {
		byID[topic.ID] = topic
	}
	for _, topic := range topics {
		depth := 0
		current := topic
		seen := map[string]struct{}{current.ID: {}}
		for current.ParentID != nil {
			parent, ok := byID[*current.ParentID]
			if !ok {
				t.Fatalf("topic %s references missing parent %s", current.ID, *current.ParentID)
			}
			if _, duplicate := seen[parent.ID]; duplicate {
				t.Fatalf("topic hierarchy contains a cycle involving %s", parent.ID)
			}
			seen[parent.ID] = struct{}{}
			depth++
			current = parent
		}
		if topic.Depth != depth {
			t.Fatalf("topic %s depth = %d, want %d from ancestry", topic.ID, topic.Depth, depth)
		}
	}
}

func TestKnowledgeEntryMoveSerializesWithTopicDelete(t *testing.T) {
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run PostgreSQL integration coverage")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.KnowledgeTopic{}, &models.KnowledgeEntry{}, &models.KnowledgeEntryRevision{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	user := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", Name: "Entry Move User", ClientID: uuid.NewString()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeEntryRevision{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeEntry{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.KnowledgeTopic{})
		db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{})
	})

	repo := NewKnowledgeRepository(db)
	source := &models.KnowledgeTopic{Name: "Move Source"}
	destination := &models.KnowledgeTopic{Name: "Move Destination"}
	if err := repo.CreateTopic(user.ID, source); err != nil {
		t.Fatalf("CreateTopic(source) error = %v", err)
	}
	if err := repo.CreateTopic(user.ID, destination); err != nil {
		t.Fatalf("CreateTopic(destination) error = %v", err)
	}
	entry := &models.KnowledgeEntry{
		TopicID: source.ID,
		Kind:    "note",
		Title:   "Move me",
		Body:    "Concurrent move test",
	}
	if err := repo.CreateEntry(user.ID, entry); err != nil {
		t.Fatalf("CreateEntry() error = %v", err)
	}

	updateEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	var blockUpdate sync.Once
	callbackName := "test:block-knowledge-entry-move:" + uuid.NewString()
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "knowledge_entries" {
			return
		}
		blockUpdate.Do(func() {
			close(updateEntered)
			<-releaseUpdate
		})
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

	moveResult := make(chan error, 1)
	go func() {
		topicID := destination.ID
		_, err := repo.UpdateEntry(user.ID, entry.ID, 1, models.KnowledgeEntryMutation{TopicID: &topicID}, models.KnowledgeSourceManual)
		moveResult <- err
	}()
	select {
	case <-updateEntered:
	case <-time.After(5 * time.Second):
		close(releaseUpdate)
		t.Fatal("entry move did not reach the blocked update")
	}

	deleteResult := make(chan error, 1)
	go func() {
		deleteResult <- repo.DeleteTopic(user.ID, destination.ID)
	}()
	select {
	case err := <-deleteResult:
		close(releaseUpdate)
		t.Fatalf("DeleteTopic() completed before entry move committed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(releaseUpdate)

	select {
	case err := <-moveResult:
		if err != nil {
			t.Fatalf("UpdateEntry() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateEntry() did not complete")
	}
	select {
	case err := <-deleteResult:
		if !errors.Is(err, ErrKnowledgeConflict) {
			t.Fatalf("DeleteTopic() error = %v, want ErrKnowledgeConflict", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DeleteTopic() did not complete")
	}

	stored, err := repo.GetEntry(user.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetEntry() error = %v", err)
	}
	if stored.TopicID != destination.ID {
		t.Fatalf("entry topic = %s, want destination %s", stored.TopicID, destination.ID)
	}
}
