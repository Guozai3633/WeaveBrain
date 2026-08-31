package stt

import "context"

// TranscriptionResult represents a single transcription output from STT.
type TranscriptionResult struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"is_final"`
	Confidence float32 `json:"confidence"`
}

// STTProvider defines the interface for speech-to-text providers.
// Implementations include cloud providers (Alibaba Cloud, iFlytek) and mocks.
type STTProvider interface {
	// StreamRecognize starts a streaming recognition session.
	// It reads PCM audio chunks from audioStream and sends transcription
	// results to the returned channel. The channel is closed when the
	// audio stream is exhausted or the context is cancelled.
	StreamRecognize(ctx context.Context, audioStream <-chan []byte) (<-chan TranscriptionResult, error)

	// RecognizeFile transcribes a complete audio file (e.g. an uploaded WAV/AAC).
	// The final transcript text is returned; implementations must not lose the
	// tail of the utterance just because a streaming session ended.
	RecognizeFile(ctx context.Context, audioPath string) (TranscriptionResult, error)

	// Close releases any resources held by the provider.
	Close() error
}
