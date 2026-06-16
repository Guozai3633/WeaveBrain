package review

import (
	"context"
	"log"
)

// Service wraps a ReviewProvider into a service layer for use by API handlers.
type Service struct {
	provider ReviewProvider
}

// NewService creates a new review service.
func NewService(provider ReviewProvider) *Service {
	return &Service{provider: provider}
}

// Review cleans up raw STT text using the configured provider.
func (s *Service) Review(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}

	result, err := s.provider.Review(ctx, text)
	if err != nil {
		// On error, return original text (graceful degradation)
		log.Printf("[Review] Provider error, returning original text: %v", err)
		return text, nil
	}

	if result.Changed {
		log.Printf("[Review] Text cleaned: %q -> %q", text, result.Cleaned)
	}

	return result.Cleaned, nil
}
