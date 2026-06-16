package mock

import (
	"context"
	"math"
)

// Provider is a mock embedding provider for development.
// Returns deterministic vectors based on text content.
type Provider struct {
	dim int
}

// Config holds configuration for the mock embedding provider.
type Config struct {
	Dim int
}

// NewProvider creates a new mock embedding provider.
func NewProvider(cfg Config) *Provider {
	dim := cfg.Dim
	if dim <= 0 {
		dim = 768
	}
	return &Provider{dim: dim}
}

// Embed returns a deterministic pseudo-vector based on text content.
func (p *Provider) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, p.dim)
	for i := range vec {
		// Simple deterministic generation based on text length and position
		vec[i] = float32(math.Sin(float64(len(text)*137+i*31))) * 0.1
	}
	return vec, nil
}

// Dimension returns the configured dimension.
func (p *Provider) Dimension() int {
	return p.dim
}

// Close is a no-op for the mock provider.
func (p *Provider) Close() error {
	return nil
}
