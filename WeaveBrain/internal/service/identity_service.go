package service

import (
	"context"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// IdentityService handles identity OAuth operations.
type IdentityService struct {
	repos *repository.DBStore
}

func NewIdentityService(repos *repository.DBStore) *IdentityService {
	return &IdentityService{repos: repos}
}

// Link links a provider identity to a user.
func (s *IdentityService) Link(ctx context.Context, userID uuid.UUID, provider, providerID, phone string) error {
	id := &entity.Identity{
		UserID:     userID,
		Provider:   provider,
		ProviderID: providerID,
		Phone:      &phone,
	}
	return s.repos.Identity.Create(ctx, id)
}

// GetByUser returns all identities for a user.
func (s *IdentityService) GetByUser(ctx context.Context, userID uuid.UUID) ([]*entity.Identity, error) {
	return s.repos.Identity.GetByUserID(ctx, userID)
}

// GetByProvider looks up identity by provider credentials.
func (s *IdentityService) GetByProvider(ctx context.Context, provider, providerID string) (*entity.Identity, error) {
	return s.repos.Identity.GetByProvider(ctx, provider, providerID)
}
