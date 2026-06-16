package service

import (
	"context"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// UserService handles user business logic.
type UserService struct {
	repos *repository.DBStore
}

func NewUserService(repos *repository.DBStore) *UserService {
	return &UserService{repos: repos}
}

// CreateUser creates a new user.
func (s *UserService) CreateUser(ctx context.Context, displayName, avatarURL *string) (*entity.User, error) {
	u := entity.NewUser()
	u.DisplayName = displayName
	u.AvatarURL = avatarURL
	if err := s.repos.User.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByID retrieves a user by ID.
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return s.repos.User.GetByID(ctx, id)
}

// GetOrCreateByProvider retrieves a user by provider identity, creates if not found.
func (s *UserService) GetOrCreateByProvider(ctx context.Context, provider, providerID string, phone *string) (*entity.User, error) {
	identity, err := s.repos.Identity.GetByProvider(ctx, provider, providerID)
	if err == nil && identity != nil {
		return s.repos.User.GetByID(ctx, identity.UserID)
	}

	// Create new user
	user := entity.NewUser()
	if err := s.repos.User.Create(ctx, user); err != nil {
		return nil, err
	}

	// Create identity
	id := &entity.Identity{
		UserID:     user.ID,
		Provider:   provider,
		ProviderID: providerID,
		Phone:      phone,
		CreatedAt:  time.Now(),
	}
	if err := s.repos.Identity.Create(ctx, id); err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateProfile updates user profile data.
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, avatarURL *string) error {
	u, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	u.DisplayName = displayName
	u.AvatarURL = avatarURL
	return s.repos.User.Update(ctx, u)
}
