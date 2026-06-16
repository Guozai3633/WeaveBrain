package service

import (
	"context"
	"errors"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// IdeaService handles idea capture and retrieval.
type IdeaService struct {
	repos     *repository.DBStore
	embedding *EmbeddingService
}

func NewIdeaService(repos *repository.DBStore) *IdeaService {
	return &IdeaService{repos: repos}
}

// SetEmbeddingService injects the embedding service for async embedding generation.
func (s *IdeaService) SetEmbeddingService(es *EmbeddingService) {
	s.embedding = es
}

// Create creates a new idea.
func (s *IdeaService) Create(ctx context.Context, projectID int64, userID uuid.UUID, rawInput string, structuredData map[string]any, tags []string) (*entity.Idea, error) {
	if rawInput == "" {
		return nil, errors.New("raw input is required")
	}

	if projectID == 0 {
		return nil, errors.New("project_id is required")
	}

	// Validate project belongs to user.
	_, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	i := entity.NewIdea()
	i.ProjectID = projectID
	i.UserID = userID
	i.RawInput = rawInput
	i.StructuredData = structuredData
	i.Tags = tags

	if err := s.repos.Idea.Create(ctx, i); err != nil {
		return nil, err
	}

	// Fire-and-forget: generate embedding in background
	if s.embedding != nil {
		go s.embedding.GenerateAndStore(context.Background(), i.ID, i.RawInput)
	}

	return i, nil
}

// GetByID retrieves an idea.
func (s *IdeaService) GetByID(ctx context.Context, id int64) (*entity.Idea, error) {
	return s.repos.Idea.GetByID(ctx, id)
}

// ListByProject lists ideas for a project with pagination.
func (s *IdeaService) ListByProject(ctx context.Context, projectID int64, page, limit int) ([]*entity.Idea, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repos.Idea.GetByProjectID(ctx, projectID, page, limit)
}

// Update updates an idea.
func (s *IdeaService) Update(ctx context.Context, id int64, rawInput string, structuredData map[string]any, tags []string) error {
	i, err := s.repos.Idea.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if rawInput != "" {
		i.RawInput = rawInput
	}
	if structuredData != nil {
		i.StructuredData = structuredData
	}
	if tags != nil {
		i.Tags = tags
	}
	return s.repos.Idea.Update(ctx, i)
}

// Delete soft-deletes an idea.
func (s *IdeaService) Delete(ctx context.Context, id int64) error {
	return s.repos.Idea.Delete(ctx, id)
}

// SearchByTags finds ideas matching given tags.
func (s *IdeaService) SearchByTags(ctx context.Context, tags []string, projectID int64) ([]*entity.Idea, error) {
	return s.repos.Idea.ListByTags(ctx, tags, projectID)
}

// Search finds ideas by text query and/or tags with pagination.
func (s *IdeaService) Search(ctx context.Context, projectID int64, query string, tags []string, page, limit int) ([]*entity.Idea, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	return s.repos.Idea.Search(ctx, projectID, query, tags, limit, offset)
}
