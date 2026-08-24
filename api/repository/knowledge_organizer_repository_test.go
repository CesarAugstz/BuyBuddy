package repository

import (
	"buybuddy-api/models"
	"errors"
	"testing"
	"time"
)

func TestKnowledgeOrganizationStateAfterClaimRestoresSyntheticManualSchedule(t *testing.T) {
	claimTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	claim := &models.KnowledgeOrganizationClaim{
		Topic: models.KnowledgeTopic{
			PendingWriteCount: 0,
			OrganizationDueAt: &claimTime,
		},
		SyntheticManual:              true,
		PendingWriteCountBeforeClaim: 0,
	}
	current := claim.Topic
	failedAt := claimTime.Add(time.Hour)

	pending, dueAt, err := knowledgeOrganizationStateAfterClaim(current, claim, &failedAt)
	if err != nil {
		t.Fatalf("knowledgeOrganizationStateAfterClaim() error = %v", err)
	}
	if pending != 0 || dueAt != nil {
		t.Fatalf("failed synthetic state = %d/%v, want pre-claim 0/nil", pending, dueAt)
	}

	pending, dueAt, err = knowledgeOrganizationStateAfterClaim(current, claim, nil)
	if err != nil {
		t.Fatalf("release synthetic claim error = %v", err)
	}
	if pending != 0 || dueAt != nil {
		t.Fatalf("released synthetic state = %d/%v, want pre-claim 0/nil", pending, dueAt)
	}
}

func TestKnowledgeOrganizationStateAfterClaimPreservesNewWrites(t *testing.T) {
	claimTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	naturalDueAt := claimTime.Add(48 * time.Hour)
	claim := &models.KnowledgeOrganizationClaim{
		Topic: models.KnowledgeTopic{
			PendingWriteCount: 0,
			OrganizationDueAt: &claimTime,
		},
		SyntheticManual:              true,
		PendingWriteCountBeforeClaim: 0,
	}
	current := models.KnowledgeTopic{
		PendingWriteCount: 1,
		OrganizationDueAt: &naturalDueAt,
	}

	pending, dueAt, err := knowledgeOrganizationStateAfterClaim(current, claim, nil)
	if err != nil {
		t.Fatalf("release with new write error = %v", err)
	}
	if pending != 1 || dueAt == nil || !dueAt.Equal(naturalDueAt) {
		t.Fatalf("released state = %d/%v, want natural write schedule 1/%v", pending, dueAt, naturalDueAt)
	}

	failedAt := claimTime.Add(time.Hour)
	pending, dueAt, err = knowledgeOrganizationStateAfterClaim(current, claim, &failedAt)
	if err != nil {
		t.Fatalf("failure with new write error = %v", err)
	}
	if pending != 1 || dueAt == nil || !dueAt.Equal(failedAt.Add(knowledgeOrganizationRetry)) {
		t.Fatalf("failed state = %d/%v, want retained write and six-hour retry", pending, dueAt)
	}
}

func TestKnowledgeOrganizationStateAfterClaimReschedulesNaturalFailure(t *testing.T) {
	claimTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	originalDueAt := claimTime.Add(-time.Hour)
	claim := &models.KnowledgeOrganizationClaim{
		Topic: models.KnowledgeTopic{
			PendingWriteCount: 3,
			OrganizationDueAt: &originalDueAt,
		},
		PendingWriteCountBeforeClaim: 3,
		OrganizationDueAtBeforeClaim: &originalDueAt,
	}
	failedAt := claimTime.Add(time.Hour)

	pending, dueAt, err := knowledgeOrganizationStateAfterClaim(claim.Topic, claim, &failedAt)
	if err != nil {
		t.Fatalf("natural failure error = %v", err)
	}
	if pending != 3 || dueAt == nil || !dueAt.Equal(failedAt.Add(knowledgeOrganizationRetry)) {
		t.Fatalf("natural failure state = %d/%v, want 3 and six-hour retry", pending, dueAt)
	}

	claimDueAt := claimTime
	claim.Topic.OrganizationDueAt = &claimDueAt
	pending, dueAt, err = knowledgeOrganizationStateAfterClaim(claim.Topic, claim, nil)
	if err != nil {
		t.Fatalf("natural release error = %v", err)
	}
	if pending != 3 || dueAt == nil || !dueAt.Equal(originalDueAt) {
		t.Fatalf("natural release state = %d/%v, want restored pre-claim due %v", pending, dueAt, originalDueAt)
	}

	inconsistent := *claim
	inconsistent.PendingWriteCountBeforeClaim = 4
	if _, _, err := knowledgeOrganizationStateAfterClaim(claim.Topic, &inconsistent, nil); !errors.Is(err, ErrKnowledgeConflict) {
		t.Fatalf("inconsistent claim error = %v, want ErrKnowledgeConflict", err)
	}
}
