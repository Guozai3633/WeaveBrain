package mock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"weavebrain/internal/stt"
)

// Provider is a mock STT provider for development and testing.
// It simulates streaming transcription by returning pre-scripted responses
// with configurable delays.
type Provider struct {
	script []string
	delay  time.Duration
}

// Config configures the mock STT provider.
type Config struct {
	// Script is a list of transcription responses to return.
	// Each entry is split into partial results (one word at a time)
	// followed by a final result.
	Script []string

	// Delay between simulated partial results. Defaults to 200ms.
	Delay time.Duration
}

// NewProvider creates a new mock STT provider.
func NewProvider(cfg Config) *Provider {
	if cfg.Delay == 0 {
		cfg.Delay = 200 * time.Millisecond
	}
	if len(cfg.Script) == 0 {
		cfg.Script = []string{
			"这是一个测试语音识别结果",
			"今天我想到了一个好主意",
		}
	}
	return &Provider{
		script: cfg.Script,
		delay:  cfg.Delay,
	}
}

// StreamRecognize simulates streaming speech recognition.
// It consumes audio chunks from the input stream and produces
// pre-scripted transcription results with delays.
func (p *Provider) StreamRecognize(ctx context.Context, audioStream <-chan []byte) (<-chan stt.TranscriptionResult, error) {
	out := make(chan stt.TranscriptionResult, 16)

	go func() {
		defer close(out)

		scriptIdx := 0

		// Wait for first audio chunk to start "recognizing"
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-audioStream:
				if !ok {
					return
				}
				// Got first chunk, start producing results
				goto produce
			}
		}

	produce:
		for {
			if scriptIdx >= len(p.script) {
				// Drain remaining audio
				for range audioStream {
				}
				return
			}

			sentence := p.script[scriptIdx]
			words := strings.Split(sentence, "")

			// Send partial results (character by character for Chinese)
			partial := ""
			for i, w := range words {
				partial += w

				select {
				case <-ctx.Done():
					return
				case <-time.After(p.delay):
				}

				isLast := i == len(words)-1
				out <- stt.TranscriptionResult{
					Text:       partial,
					IsFinal:    isLast,
					Confidence: 0.85 + float32(i)*0.01,
				}

				// Drain any audio that arrived during delay
				drained := false
				for !drained {
					select {
					case _, ok := <-audioStream:
						if !ok {
							// Stream closed, send final if not already
							if !isLast {
								out <- stt.TranscriptionResult{
									Text:       partial,
									IsFinal:    true,
									Confidence: 0.95,
								}
							}
							return
						}
					default:
						drained = true
					}
				}
			}

			scriptIdx++
		}
	}()

	return out, nil
}

// RecognizeFile transcribes a complete audio file.
// The mock verifies the file exists and is non-empty, then returns the first
// scripted sentence as the final result. It simulates STT failure when the
// file content contains the marker "FAIL" (used by adversarial tests).
func (p *Provider) RecognizeFile(_ context.Context, audioPath string) (stt.TranscriptionResult, error) {
	info, err := os.Stat(audioPath)
	if err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("mock stt: read audio file: %w", err)
	}
	if info.Size() == 0 {
		return stt.TranscriptionResult{}, fmt.Errorf("mock stt: audio file is empty")
	}

	data, err := os.ReadFile(audioPath)
	if err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("mock stt: read audio file: %w", err)
	}
	if strings.Contains(string(data), "FAIL") {
		return stt.TranscriptionResult{}, fmt.Errorf("mock stt: simulated recognition failure")
	}

	if len(p.script) == 0 {
		return stt.TranscriptionResult{}, fmt.Errorf("mock stt: no script configured")
	}
	return stt.TranscriptionResult{
		Text:       p.script[0],
		IsFinal:    true,
		Confidence: 0.95,
	}, nil
}

// Close is a no-op for the mock provider.
func (p *Provider) Close() error {
	return nil
}
