package mock

import (
	"context"

	"weavebrain/internal/review"
)

// Provider is a passthrough review provider for development and testing.
// It returns the input text unchanged.
type Provider struct{}

// NewProvider creates a new mock review provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Review returns the input text unchanged.
func (p *Provider) Review(ctx context.Context, text string) (review.ReviewResult, error) {
	return review.ReviewResult{
		Cleaned: text,
		Changed: false,
	}, nil
}
