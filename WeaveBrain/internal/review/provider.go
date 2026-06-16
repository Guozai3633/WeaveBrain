package review

import "context"

// ReviewResult holds the cleaned-up text returned by the review provider.
type ReviewResult struct {
	// Cleaned is the post-processed text after review.
	Cleaned string `json:"cleaned"`
	// Changed indicates whether the text was modified.
	Changed bool `json:"changed"`
}

// ReviewProvider defines the interface for text post-processing after STT.
// Implementations include LLM-based review and passthrough mocks.
type ReviewProvider interface {
	// Review takes raw STT text and returns cleaned-up text.
	Review(ctx context.Context, text string) (ReviewResult, error)
}
