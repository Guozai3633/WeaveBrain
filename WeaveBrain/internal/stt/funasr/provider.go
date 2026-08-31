package funasr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"

	"weavebrain/internal/stt"

	"github.com/gorilla/websocket"
)

// Config holds FunASR connection settings.
type Config struct {
	// ServerAddr is the FunASR WebSocket server address (e.g. "localhost:10095").
	ServerAddr string

	// SampleRate of the audio stream. Defaults to 16000.
	SampleRate int

	// HotWords for boosting recognition of specific terms.
	HotWords string
}

// Provider implements stt.STTProvider using a local FunASR server.
type Provider struct {
	cfg Config
}

// NewProvider creates a FunASR-backed STT provider.
func NewProvider(cfg Config) *Provider {
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 16000
	}
	return &Provider{cfg: cfg}
}

// StreamRecognize connects to FunASR and streams audio for recognition.
func (p *Provider) StreamRecognize(ctx context.Context, audioStream <-chan []byte) (<-chan stt.TranscriptionResult, error) {
	u := url.URL{
		Scheme: "ws",
		Host:   p.cfg.ServerAddr,
		Path:   "/",
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("funasr: dial failed: %w", err)
	}

	// Send initial configuration frame.
	firstMsg := map[string]any{
		"mode":        "online",
		"chunk_size":  []int{5, 10, 5},
		"wav_name":    "stream",
		"is_speaking": true,
		"itn":         true,
		"hotwords":    p.cfg.HotWords,
	}
	if err := conn.WriteJSON(firstMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("funasr: send config failed: %w", err)
	}

	out := make(chan stt.TranscriptionResult, 32)

	var wg sync.WaitGroup

	// Sender: forward audio chunks to FunASR.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-audioStream:
				if !ok {
					// Audio stream closed — signal end of speech.
					endMsg := map[string]any{"is_speaking": false}
					if err := conn.WriteJSON(endMsg); err != nil {
						log.Printf("funasr: send end-of-speech failed: %v", err)
					}
					return
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
					log.Printf("funasr: send audio failed: %v", err)
					return
				}
			}
		}
	}()

	// Receiver: read FunASR transcription results.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(out)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("funasr: read message failed: %v", err)
				}
				return
			}

			var resp funasrResponse
			if err := json.Unmarshal(message, &resp); err != nil {
				log.Printf("funasr: unmarshal response failed: %v", err)
				continue
			}

			if resp.Text == "" {
				continue
			}

			isFinal := resp.Mode == "2pass-offline" || resp.Mode == "offline"

			select {
			case <-ctx.Done():
				return
			case out <- stt.TranscriptionResult{
				Text:       resp.Text,
				IsFinal:    isFinal,
				Confidence: 0.9,
			}:
			}
		}
	}()

	// Cleanup goroutine: wait for sender to finish, then close connection.
	go func() {
		wg.Wait()
		conn.Close()
	}()

	return out, nil
}

// RecognizeFile transcribes a complete audio file using FunASR offline mode.
// The whole file is sent, then end-of-speech is signalled, so the tail of the
// utterance is not lost when the audio stream stops.
func (p *Provider) RecognizeFile(ctx context.Context, audioPath string) (stt.TranscriptionResult, error) {
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: read audio file: %w", err)
	}
	if len(data) == 0 {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: audio file is empty")
	}

	u := url.URL{Scheme: "ws", Host: p.cfg.ServerAddr, Path: "/"}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: dial failed: %w", err)
	}
	defer conn.Close()

	config := map[string]any{
		"mode":        "offline",
		"wav_name":    "file",
		"is_speaking": true,
		"itn":         true,
		"hotwords":    p.cfg.HotWords,
	}
	if err := conn.WriteJSON(config); err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: send config failed: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: send audio failed: %w", err)
	}
	endMsg := map[string]any{"mode": "offline", "wav_name": "file", "is_speaking": false}
	if err := conn.WriteJSON(endMsg); err != nil {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: send end-of-speech failed: %w", err)
	}

	var last stt.TranscriptionResult
	for {
		select {
		case <-ctx.Done():
			return stt.TranscriptionResult{}, ctx.Err()
		default:
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var resp funasrResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}
		if resp.Text != "" {
			last = stt.TranscriptionResult{Text: resp.Text, IsFinal: true, Confidence: 0.9}
			if resp.Mode != "2pass-offline" && resp.Mode != "offline" {
				// 2pass partials may arrive before the offline final.
				last.IsFinal = false
			}
		}
	}
	if last.Text == "" {
		return stt.TranscriptionResult{}, fmt.Errorf("funasr: no transcription received")
	}
	return last, nil
}

// Close is a no-op; each StreamRecognize session manages its own connection.
func (p *Provider) Close() error {
	return nil
}

// funasrResponse is the JSON structure returned by the FunASR server.
type funasrResponse struct {
	Text string `json:"text"`
	Mode string `json:"mode"`
}
