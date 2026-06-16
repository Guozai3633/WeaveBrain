package service

import (
	"context"
	"log"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/embedding"
	"weavebrain/internal/entity"
)

// EmbeddingService manages vector embedding generation and semantic search.
type EmbeddingService struct {
	provider embedding.Provider
	repos    *repository.DBStore
}

// NewEmbeddingService creates a new EmbeddingService.
func NewEmbeddingService(provider embedding.Provider, repos *repository.DBStore) *EmbeddingService {
	return &EmbeddingService{provider: provider, repos: repos}
}

// GenerateAndStore generates an embedding for the given text and stores it.
// Intended to be called as a fire-and-forget goroutine.
func (s *EmbeddingService) GenerateAndStore(ctx context.Context, ideaID int64, text string) {
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		log.Printf("[Embedding] Failed to generate embedding for idea %d: %v", ideaID, err)
		return
	}

	if err := s.repos.Idea.UpdateEmbedding(ctx, ideaID, vec); err != nil {
		log.Printf("[Embedding] Failed to store embedding for idea %d: %v", ideaID, err)
		return
	}

	log.Printf("[Embedding] Stored embedding for idea %d (dim=%d)", ideaID, len(vec))
}

// SearchSimilar finds ideas semantically similar to the query text.
func (s *EmbeddingService) SearchSimilar(ctx context.Context, projectID int64, query string, limit int) ([]*entity.Idea, error) {
	vec, err := s.provider.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	return s.repos.Idea.SearchBySimilarity(ctx, projectID, vec, limit)
}

// BackfillEmbeddings generates embeddings for ideas that don't have one yet.
// Returns the number of ideas processed.
func (s *EmbeddingService) BackfillEmbeddings(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	ideas, err := s.repos.Idea.GetWithoutEmbedding(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, idea := range ideas {
		vec, err := s.provider.Embed(ctx, idea.RawInput)
		if err != nil {
			log.Printf("[Embedding] Backfill: failed to generate embedding for idea %d: %v", idea.ID, err)
			continue
		}

		if err := s.repos.Idea.UpdateEmbedding(ctx, idea.ID, vec); err != nil {
			log.Printf("[Embedding] Backfill: failed to store embedding for idea %d: %v", idea.ID, err)
			continue
		}
		processed++
	}

	return processed, nil
}

// Provider returns the underlying embedding provider.
func (s *EmbeddingService) Provider() embedding.Provider {
	return s.provider
}
