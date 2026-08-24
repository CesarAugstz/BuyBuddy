package repository

import (
	"buybuddy-api/models"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNextKnowledgeOrganizationScheduleThresholdAndIdleDeadline(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	count, due := NextKnowledgeOrganizationSchedule(0, now)
	if count != 1 || !due.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("first write = count %d due %s, want count 1 due in two days", count, due)
	}

	later := now.Add(6 * time.Hour)
	count, due = NextKnowledgeOrganizationSchedule(count, later)
	if count != 2 || !due.Equal(later.Add(48*time.Hour)) {
		t.Fatalf("later write = count %d due %s, want idle deadline from latest write", count, due)
	}

	count, due = NextKnowledgeOrganizationSchedule(4, later)
	if count != 5 || !due.Equal(later) {
		t.Fatalf("fifth write = count %d due %s, want immediate deadline", count, due)
	}
}

func TestValidateKnowledgeOrganizationPlanAllowsOnlyBoundedSuppliedOperations(t *testing.T) {
	context := organizerValidationContext()
	plan := &models.KnowledgeOrganizationPlan{Operations: []models.KnowledgeOrganizationOperation{
		{
			Type:            "normalize_tags",
			EntryID:         "entry-1",
			ExpectedVersion: 1,
			Tags:            []string{" Milk ", "FAVORITE"},
			Reason:          "Normalize casing.",
		},
		{
			Type:        "create_subtopic",
			TemporaryID: "new-topic-food",
			Name:        "Food",
			Description: "Food notes.",
			Reason:      "Several notes concern food.",
		},
		{
			Type:            "move_entry",
			EntryID:         "entry-1",
			ExpectedVersion: 2,
			TargetTopicID:   "new-topic-food",
			Reason:          "Move the food note.",
		},
		{
			Type:            "rename_entry",
			EntryID:         "entry-1",
			ExpectedVersion: 3,
			Title:           "Preferred milk brand",
			Reason:          "Clarify the title.",
		},
		{
			Type:                     "merge_duplicate_entries",
			CanonicalEntryID:         "entry-1",
			DuplicateEntryID:         "entry-2",
			CanonicalExpectedVersion: 4,
			DuplicateExpectedVersion: 1,
			Reason:                   "The notes are duplicates.",
		},
	}}
	if err := ValidateKnowledgeOrganizationPlan(context, plan); err != nil {
		t.Fatalf("valid organization plan rejected: %v", err)
	}

	plan.Operations[2].TargetTopicID = "outside-topic"
	if err := ValidateKnowledgeOrganizationPlan(context, plan); !errors.Is(err, ErrKnowledgeInvalid) {
		t.Fatalf("outside topic error = %v, want ErrKnowledgeInvalid", err)
	}
	plan.Operations[2].TargetTopicID = "new-topic-food"
	plan.Operations[3].ExpectedVersion = 2
	if err := ValidateKnowledgeOrganizationPlan(context, plan); !errors.Is(err, ErrKnowledgeInvalid) {
		t.Fatalf("stale expectedVersion error = %v, want ErrKnowledgeInvalid", err)
	}
}

func TestValidateKnowledgeOrganizationPlanResolvesCreateSubtopicsBeforeOperationOrder(t *testing.T) {
	context := organizerValidationContext()
	plan := &models.KnowledgeOrganizationPlan{Operations: []models.KnowledgeOrganizationOperation{
		{
			Type:            "move_entry",
			EntryID:         "entry-1",
			ExpectedVersion: 1,
			TargetTopicID:   "new-topic-food",
			Reason:          "Move the food note.",
		},
		{
			Type:        "create_subtopic",
			TemporaryID: "new-topic-food",
			Name:        "Food",
			Reason:      "Group food notes.",
		},
	}}
	if err := ValidateKnowledgeOrganizationPlan(context, plan); err != nil {
		t.Fatalf("move-before-create plan rejected: %v", err)
	}

	plan.Operations = append(plan.Operations, models.KnowledgeOrganizationOperation{
		Type:        "create_subtopic",
		TemporaryID: "new-topic-food",
		Name:        "Other",
		Reason:      "Duplicate temporary ID.",
	})
	if err := ValidateKnowledgeOrganizationPlan(context, plan); !errors.Is(err, ErrKnowledgeInvalid) {
		t.Fatalf("duplicate temporary ID error = %v, want ErrKnowledgeInvalid", err)
	}
}

func TestValidateKnowledgeOrganizationPlanRejectsTagMeaningChangesAndUnknownIDs(t *testing.T) {
	context := organizerValidationContext()
	tests := []models.KnowledgeOrganizationOperation{
		{
			Type:            "normalize_tags",
			EntryID:         "entry-1",
			ExpectedVersion: 1,
			Tags:            []string{"milk", "invented"},
			Reason:          "Change meanings.",
		},
		{
			Type:            "rename_entry",
			EntryID:         "not-supplied",
			ExpectedVersion: 1,
			Title:           "Unknown",
			Reason:          "Unknown ID.",
		},
		{
			Type:            "move_entry",
			EntryID:         "entry-1",
			ExpectedVersion: 1,
			TargetTopicID:   "target",
			Title:           "Forbidden extra field",
			Reason:          "Contains an extra field.",
		},
	}
	for _, operation := range tests {
		err := ValidateKnowledgeOrganizationPlan(
			context,
			&models.KnowledgeOrganizationPlan{Operations: []models.KnowledgeOrganizationOperation{operation}},
		)
		if !errors.Is(err, ErrKnowledgeInvalid) {
			t.Errorf("%s error = %v, want ErrKnowledgeInvalid", operation.Type, err)
		}
	}
}

func TestOrganizerEntryExcerptDoesNotMutateStoredBody(t *testing.T) {
	body := strings.Repeat("é", 700)
	entry := models.KnowledgeEntry{
		ID:      "entry",
		TopicID: "target",
		Body:    body,
	}
	bounded := organizerEntry(entry)
	if got := len([]rune(bounded.Body)); got != 500 {
		t.Fatalf("excerpt rune count = %d, want 500", got)
	}
	if entry.Body != body {
		t.Fatal("building organizer context mutated the stored entry body")
	}
}

func organizerValidationContext() *models.KnowledgeOrganizerContext {
	return &models.KnowledgeOrganizerContext{
		Target: models.KnowledgeOrganizerTarget{
			ID:    "target",
			Name:  "Recommendations",
			Depth: 1,
		},
		Children: []models.KnowledgeOrganizerChild{{
			ID:    "child",
			Name:  "Existing",
			Depth: 2,
		}},
		RecentEntries: []models.KnowledgeOrganizerEntry{
			{
				ID:         "entry-1",
				TopicID:    "target",
				Title:      "Milk",
				Tags:       []string{"Milk", "favorite"},
				Attributes: models.KnowledgeAttributes{},
				Version:    1,
			},
			{
				ID:         "entry-2",
				TopicID:    "target",
				Title:      "Milk duplicate",
				Tags:       []string{"dairy"},
				Attributes: models.KnowledgeAttributes{},
				Version:    1,
			},
		},
	}
}
