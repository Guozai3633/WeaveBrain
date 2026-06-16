package embedding

import "context"

// Provider generates vector embeddings for text.
type Provider interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// Dimension returns the dimensionality of the embedding vectors.
	Dimension() int

	// Close releases any resources held by the provider.
	Close() error
}
