package organizer

import (
	"buybuddy-api/models"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeOrganizationRepository struct {
	explicitClaim *models.KnowledgeOrganizationClaim
	nextClaim     *models.KnowledgeOrganizationClaim
	context       *models.KnowledgeOrganizerContext
	failedClaim   *models.KnowledgeOrganizationClaim
	releasedClaim *models.KnowledgeOrganizationClaim
	releaseErr    error
	releaseCtxErr error
	releaseHasTTL bool
	failCalls     int
	releaseCalls  int
}

func (f *fakeOrganizationRepository) ClaimKnowledgeOrganization(_, _ string, _ time.Time) (*models.KnowledgeOrganizationClaim, error) {
	return f.explicitClaim, nil
}

func (f *fakeOrganizationRepository) ClaimNextKnowledgeOrganization(_ time.Time) (*models.KnowledgeOrganizationClaim, error) {
	return f.nextClaim, nil
}

func (f *fakeOrganizationRepository) LoadKnowledgeOrganizerContext(_, _ string) (*models.KnowledgeOrganizerContext, error) {
	return f.context, nil
}

func (f *fakeOrganizationRepository) ApplyKnowledgeOrganizationPlan(
	_ *models.KnowledgeOrganizationClaim,
	_ *models.KnowledgeOrganizerContext,
	_ *models.KnowledgeOrganizationPlan,
	_ time.Time,
) (*models.KnowledgeOrganizationApplyResult, *models.KnowledgeTopic, error) {
	return &models.KnowledgeOrganizationApplyResult{}, &models.KnowledgeTopic{}, nil
}

func (f *fakeOrganizationRepository) FailKnowledgeOrganization(claim *models.KnowledgeOrganizationClaim, _ time.Time) error {
	f.failCalls++
	f.failedClaim = claim
	return nil
}

func (f *fakeOrganizationRepository) ReleaseKnowledgeOrganization(ctx context.Context, claim *models.KnowledgeOrganizationClaim) error {
	f.releaseCalls++
	f.releasedClaim = claim
	f.releaseCtxErr = ctx.Err()
	_, f.releaseHasTTL = ctx.Deadline()
	return f.releaseErr
}

func TestOrganizeTopicCancellationReleasesLeaseWithFreshBoundedContext(t *testing.T) {
	claim := organizerServiceClaim(true)
	repo := &fakeOrganizationRepository{
		explicitClaim: claim,
		context:       organizerServiceContext(claim),
	}
	service := &Service{
		repo: repo,
		generate: func(ctx context.Context, _ *models.KnowledgeOrganizerContext, _ string) (*models.KnowledgeOrganizationPlan, error) {
			return nil, ctx.Err()
		},
		now: time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := service.OrganizeTopic(ctx, claim.Topic.UserID, claim.Topic.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("OrganizeTopic() error = %v, want context.Canceled", err)
	}
	if repo.releaseCalls != 1 || repo.releasedClaim != claim {
		t.Fatalf("release calls/claim = %d/%p, want 1/%p", repo.releaseCalls, repo.releasedClaim, claim)
	}
	if repo.releaseCtxErr != nil || !repo.releaseHasTTL {
		t.Fatalf("release context error/deadline = %v/%t, want active bounded context", repo.releaseCtxErr, repo.releaseHasTTL)
	}
	if repo.failCalls != 0 {
		t.Fatalf("failure calls = %d, want cancellation release only", repo.failCalls)
	}
}

func TestFailedExplicitAndScheduledClaimsRetainClaimKind(t *testing.T) {
	generationErr := errors.New("generation failed")
	tests := []struct {
		name      string
		synthetic bool
		run       func(*Service) error
	}{
		{
			name:      "synthetic explicit claim",
			synthetic: true,
			run: func(service *Service) error {
				_, err := service.OrganizeTopic(context.Background(), "user", "topic")
				return err
			},
		},
		{
			name:      "natural scheduled claim",
			synthetic: false,
			run: func(service *Service) error {
				_, err := service.RunNext(context.Background())
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claim := organizerServiceClaim(test.synthetic)
			repo := &fakeOrganizationRepository{
				explicitClaim: claim,
				nextClaim:     claim,
				context:       organizerServiceContext(claim),
			}
			service := &Service{
				repo: repo,
				generate: func(context.Context, *models.KnowledgeOrganizerContext, string) (*models.KnowledgeOrganizationPlan, error) {
					return nil, generationErr
				},
				now: time.Now,
			}

			if err := test.run(service); !errors.Is(err, generationErr) {
				t.Fatalf("run error = %v, want generation failure", err)
			}
			if repo.failCalls != 1 || repo.failedClaim != claim || repo.failedClaim.SyntheticManual != test.synthetic {
				t.Fatalf("failed claim = %#v after %d calls", repo.failedClaim, repo.failCalls)
			}
			if repo.releaseCalls != 0 {
				t.Fatalf("release calls = %d, want failure path", repo.releaseCalls)
			}
		})
	}
}

func organizerServiceClaim(synthetic bool) *models.KnowledgeOrganizationClaim {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	pending := 3
	if synthetic {
		pending = 0
	}
	return &models.KnowledgeOrganizationClaim{
		Topic: models.KnowledgeTopic{
			ID:                "topic",
			UserID:            "user",
			PendingWriteCount: pending,
			OrganizationDueAt: &now,
		},
		LeaseUntil:                   now.Add(10 * time.Minute),
		SyntheticManual:              synthetic,
		PendingWriteCountBeforeClaim: pending,
	}
}

func organizerServiceContext(claim *models.KnowledgeOrganizationClaim) *models.KnowledgeOrganizerContext {
	return &models.KnowledgeOrganizerContext{
		Target: models.KnowledgeOrganizerTarget{
			ID:                claim.Topic.ID,
			PendingWriteCount: claim.Topic.PendingWriteCount,
		},
	}
}
