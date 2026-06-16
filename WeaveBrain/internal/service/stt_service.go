package service

import (
	"context"

	"weavebrain/internal/stt"
)

// STTService manages STT provider lifecycle and streaming recognition.
type STTService struct {
	provider stt.STTProvider
}

// NewSTTService creates a new STTService with the given provider.
func NewSTTService(provider stt.STTProvider) *STTService {
	return &STTService{provider: provider}
}

// RecognizeStream delegates streaming recognition to the underlying provider.
func (s *STTService) RecognizeStream(ctx context.Context, audioCh <-chan []byte) (<-chan stt.TranscriptionResult, error) {
	return s.provider.StreamRecognize(ctx, audioCh)
}

// Close releases the underlying provider resources.
func (s *STTService) Close() error {
	return s.provider.Close()
}
