package service

import (
	"context"
	"errors"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// ProjectService handles project business logic.
type ProjectService struct {
	repos *repository.DBStore
}

func NewProjectService(repos *repository.DBStore) *ProjectService {
	return &ProjectService{repos: repos}
}

// CreateProject creates a new project for a user.
func (s *ProjectService) CreateProject(ctx context.Context, userID uuid.UUID, name string, isDefault bool) (*entity.Project, error) {
	if name == "" {
		return nil, errors.New("project name is required")
	}

	// Ensure user has exactly one default project.
	existing, err := s.repos.Project.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if isDefault {
		// Set all existing defaults to false.
		for _, p := range existing {
			p.DefaultProject = false
			s.repos.Project.Update(ctx, p)
		}
	} else if len(existing) == 0 {
		// First project for this user must be default.
		isDefault = true
	}

	p := &entity.Project{
		UserID:         userID,
		Name:           name,
		DefaultProject: isDefault,
	}
	if err := s.repos.Project.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetByID retrieves a project.
func (s *ProjectService) GetByID(ctx context.Context, id int64) (*entity.Project, error) {
	return s.repos.Project.GetByID(ctx, id)
}

// ListByUser lists all projects for a user.
func (s *ProjectService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*entity.Project, error) {
	return s.repos.Project.GetByUserID(ctx, userID)
}

// Update renames or modifies a project.
func (s *ProjectService) Update(ctx context.Context, projectID int64, name string, isDefault bool) error {
	p, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return err
	}
	if name != "" {
		p.Name = name
	}
	p.DefaultProject = isDefault
	return s.repos.Project.Update(ctx, p)
}

// Delete removes a project (soft delete).
func (s *ProjectService) Delete(ctx context.Context, projectID int64) error {
	return s.repos.Project.Delete(ctx, projectID)
}
