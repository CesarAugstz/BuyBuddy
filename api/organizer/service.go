package organizer

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

const tickerInterval = 15 * time.Minute
const leaseCleanupTimeout = 5 * time.Second

type PlanGenerator func(context.Context, *models.KnowledgeOrganizerContext, string) (*models.KnowledgeOrganizationPlan, error)

type knowledgeOrganizationRepository interface {
	ClaimKnowledgeOrganization(string, string, time.Time) (*models.KnowledgeOrganizationClaim, error)
	ClaimNextKnowledgeOrganization(time.Time) (*models.KnowledgeOrganizationClaim, error)
	LoadKnowledgeOrganizerContext(string, string) (*models.KnowledgeOrganizerContext, error)
	ApplyKnowledgeOrganizationPlan(*models.KnowledgeOrganizationClaim, *models.KnowledgeOrganizerContext, *models.KnowledgeOrganizationPlan, time.Time) (*models.KnowledgeOrganizationApplyResult, *models.KnowledgeTopic, error)
	FailKnowledgeOrganization(*models.KnowledgeOrganizationClaim, time.Time) error
	ReleaseKnowledgeOrganization(context.Context, *models.KnowledgeOrganizationClaim) error
}

type Service struct {
	repo     knowledgeOrganizationRepository
	apiKey   string
	generate PlanGenerator
	now      func() time.Time
	interval time.Duration
}

func NewService(repo *repository.KnowledgeRepository, apiKey string) *Service {
	return &Service{
		repo:     repo,
		apiKey:   apiKey,
		generate: utils.GenerateKnowledgeOrganizationPlan,
		now:      time.Now,
		interval: tickerInterval,
	}
}

func (s *Service) OrganizeTopic(ctx context.Context, userID, topicID string) (*models.KnowledgeOrganizationResponse, error) {
	claim, err := s.repo.ClaimKnowledgeOrganization(userID, topicID, s.now())
	if err != nil {
		return nil, err
	}
	result, topic, err := s.runClaim(ctx, claim)
	if err != nil {
		return nil, err
	}
	return &models.KnowledgeOrganizationResponse{
		Status: "organized",
		Topic:  *topic,
		Result: *result,
	}, nil
}

func (s *Service) RunNext(ctx context.Context) (bool, error) {
	claim, err := s.repo.ClaimNextKnowledgeOrganization(s.now())
	if err != nil || claim == nil {
		return false, err
	}
	_, _, err = s.runClaim(ctx, claim)
	return true, err
}

func (s *Service) runClaim(ctx context.Context, claim *models.KnowledgeOrganizationClaim) (*models.KnowledgeOrganizationApplyResult, *models.KnowledgeTopic, error) {
	contextPayload, err := s.repo.LoadKnowledgeOrganizerContext(claim.Topic.UserID, claim.Topic.ID)
	if err == nil {
		var plan *models.KnowledgeOrganizationPlan
		plan, err = s.generate(ctx, contextPayload, s.apiKey)
		if err == nil {
			err = repository.ValidateKnowledgeOrganizationPlan(contextPayload, plan)
		}
		if err == nil {
			var result *models.KnowledgeOrganizationApplyResult
			var topic *models.KnowledgeTopic
			result, topic, err = s.repo.ApplyKnowledgeOrganizationPlan(claim, contextPayload, plan, s.now())
			if err == nil {
				return result, topic, nil
			}
		}
	}

	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		cleanupContext, cancel := context.WithTimeout(context.Background(), leaseCleanupTimeout)
		defer cancel()
		if releaseErr := s.repo.ReleaseKnowledgeOrganization(cleanupContext, claim); releaseErr != nil &&
			!errors.Is(releaseErr, repository.ErrKnowledgeConflict) {
			return nil, nil, fmt.Errorf("organizer interrupted: %w; release lease: %v", err, releaseErr)
		}
		return nil, nil, err
	}
	if failureErr := s.repo.FailKnowledgeOrganization(claim, s.now()); failureErr != nil && !errors.Is(failureErr, repository.ErrKnowledgeConflict) {
		return nil, nil, fmt.Errorf("organizer failed: %v; reschedule failed: %w", err, failureErr)
	}
	return nil, nil, err
}

func (s *Service) RunTicker(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claimed, err := s.RunNext(ctx)
			if err != nil {
				log.Printf("Knowledge organizer tick failed: %v", err)
			} else if claimed {
				log.Print("Knowledge organizer completed one topic")
			}
		}
	}
}
