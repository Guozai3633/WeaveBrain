package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"weavebrain/internal/review"
	"weavebrain/internal/service"
	"weavebrain/internal/stt"
	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// VoiceWSHandler handles WebSocket connections for streaming voice input.
type VoiceWSHandler struct {
	sttService    *service.STTService
	reviewService *review.Service
	tokenCfg      auth.TokenConfig
}

// NewVoiceWSHandler creates a new VoiceWSHandler.
func NewVoiceWSHandler(sttService *service.STTService, reviewService *review.Service, tokenCfg auth.TokenConfig) *VoiceWSHandler {
	return &VoiceWSHandler{sttService: sttService, reviewService: reviewService, tokenCfg: tokenCfg}
}

// RegisterRoute registers the voice WebSocket route.
func (h *VoiceWSHandler) RegisterRoute(engine *gin.Engine) {
	engine.GET("/ws/voice", auth.WSJWTMiddleware(h.tokenCfg), h.handleVoiceWebSocket)
}

// clientControlMsg represents a control message from the client.
type clientControlMsg struct {
	Type     string `json:"type"`     // "start" or "stop"
	Language string `json:"language"` // e.g. "zh-CN"
}

// serverTranscriptionMsg represents a transcription result sent to the client.
type serverTranscriptionMsg struct {
	Type       string  `json:"type"` // "partial" or "final"
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence"`
}

// serverReviewedMsg represents a reviewed/cleaned text sent to the client after async LLM review.
type serverReviewedMsg struct {
	Type string `json:"type"` // "reviewed"
	Text string `json:"text"`
}

// handleVoiceWebSocket upgrades the HTTP connection to WebSocket and
// manages the voice streaming lifecycle.
func (h *VoiceWSHandler) handleVoiceWebSocket(c *gin.Context) {
	if h.sttService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "STT service not available"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[VoiceWS] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Println("[VoiceWS] Client connected")

	var (
		audioCh     chan []byte
		sttResults  <-chan stt.TranscriptionResult
		sessionDone = make(chan struct{})
		mu          sync.Mutex // protects audioCh
		writeMu     sync.Mutex // protects concurrent WebSocket writes
	)

	// safeWriteJSON locks writeMu before writing to the WebSocket.
	safeWriteJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	stopSession := func() {
		mu.Lock()
		defer mu.Unlock()
		if audioCh != nil {
			close(audioCh)
			audioCh = nil
		}
	}

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("[VoiceWS] Client disconnected")
			} else {
				log.Printf("[VoiceWS] Read error: %v", err)
			}
			stopSession()
			return
		}

		switch msgType {
		case websocket.TextMessage:
			var ctrl clientControlMsg
			if err := json.Unmarshal(msg, &ctrl); err != nil {
				log.Printf("[VoiceWS] Invalid control message: %v", err)
				continue
			}

			switch ctrl.Type {
			case "start":
				log.Printf("[VoiceWS] Starting recognition (language: %s)", ctrl.Language)

				stopSession() // Stop any existing session

				audioCh = make(chan []byte, 64)
				sttResults, err = h.sttService.RecognizeStream(c.Request.Context(), audioCh)
				if err != nil {
					log.Printf("[VoiceWS] STT stream error: %v", err)
					safeWriteJSON(gin.H{"type": "error", "message": "failed to start recognition"})
					continue
				}

				// Pump transcription results to the WebSocket client
				go func(results <-chan stt.TranscriptionResult, done chan<- struct{}) {
					defer func() { close(done) }()
					for r := range results {
						msgType := "partial"
						if r.IsFinal {
							msgType = "final"
						}
						if err := safeWriteJSON(serverTranscriptionMsg{
							Type:       msgType,
							Text:       r.Text,
							Confidence: r.Confidence,
						}); err != nil {
							log.Printf("[VoiceWS] Write error: %v", err)
							return
						}

						// On final result, trigger async review if available
						if r.IsFinal && h.reviewService != nil && r.Text != "" {
							go func(text string) {
								reviewCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer cancel()

								cleaned, err := h.reviewService.Review(reviewCtx, text)
								if err != nil {
									log.Printf("[VoiceWS] Review error: %v", err)
									return
								}
								if cleaned != text {
									if err := safeWriteJSON(serverReviewedMsg{
										Type: "reviewed",
										Text: cleaned,
									}); err != nil {
										log.Printf("[VoiceWS] Reviewed write error: %v", err)
									}
								}
							}(r.Text)
						}
					}
					log.Println("[VoiceWS] STT stream ended")
				}(sttResults, sessionDone)

			case "stop":
				log.Println("[VoiceWS] Stopping recognition")
				stopSession()
				<-sessionDone
				sessionDone = make(chan struct{})
				safeWriteJSON(gin.H{"type": "stopped"})
			}

		case websocket.BinaryMessage:
			// PCM audio chunk - forward to STT stream
			mu.Lock()
			ch := audioCh
			mu.Unlock()
			if ch != nil {
				select {
				case ch <- msg:
				default:
					// Drop chunk if buffer is full
					log.Println("[VoiceWS] Audio buffer full, dropping chunk")
				}
			}
		}
	}
}
