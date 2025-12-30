package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"awkit-example-snake-arena-backend/internal/game"
)

const (
	messageTypeDirection = "direction"
)

// ValidatedMessage represents a parsed and validated client payload.
type ValidatedMessage struct {
	Type      string
	Direction game.Direction
	Tick      int
}

// ErrorResponse is what the server would send back to the WebSocket client.
type ErrorResponse struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RetryAfterMs int64  `json:"retryAfterMs,omitempty"`
}

// WebSocketServer processes client messages with validation and throttling.
type WebSocketServer struct {
	limiter *RateLimiter
}

func NewWebSocketServer(limit int, window time.Duration) *WebSocketServer {
	return &WebSocketServer{
		limiter: NewRateLimiter(limit, window),
	}
}

// HandleMessage validates the message schema, direction, and rate limit for a client.
// It returns either a validated message or an error response describing what went wrong.
func (s *WebSocketServer) HandleMessage(clientID string, raw []byte, now time.Time) (ValidatedMessage, *ErrorResponse) {
	client := strings.TrimSpace(clientID)
	if client == "" {
		return ValidatedMessage{}, &ErrorResponse{
			Code:    "invalid_client",
			Message: "client id is required",
		}
	}

	if allowed, retryAfter := s.limiter.Allow(client, now); !allowed {
		return ValidatedMessage{}, &ErrorResponse{
			Code:         "rate_limited",
			Message:      fmt.Sprintf("too many messages, retry after %dms", retryAfter.Milliseconds()),
			RetryAfterMs: retryAfter.Milliseconds(),
		}
	}

	validated, err := validateMessage(raw)
	if err != nil {
		return ValidatedMessage{}, &ErrorResponse{
			Code:    "invalid_message",
			Message: err.Error(),
		}
	}

	return validated, nil
}

func validateMessage(raw []byte) (ValidatedMessage, error) {
	var incoming struct {
		Type      string `json:"type"`
		Direction string `json:"direction"`
		Tick      *int   `json:"tick,omitempty"`
	}

	if err := json.Unmarshal(raw, &incoming); err != nil {
		return ValidatedMessage{}, fmt.Errorf("invalid json payload: %w", err)
	}

	messageType := strings.TrimSpace(incoming.Type)
	if messageType == "" {
		return ValidatedMessage{}, errors.New("message type is required")
	}

	switch messageType {
	case messageTypeDirection:
		return validateDirectionMessage(incoming.Direction, incoming.Tick)
	default:
		return ValidatedMessage{}, fmt.Errorf("unsupported message type %q", messageType)
	}
}

func validateDirectionMessage(directionValue string, tick *int) (ValidatedMessage, error) {
	normalized := strings.TrimSpace(directionValue)
	if normalized == "" {
		return ValidatedMessage{}, errors.New("direction is required")
	}

	dir, ok := game.ParseDirection(normalized)
	if !ok {
		return ValidatedMessage{}, fmt.Errorf("invalid direction %q: must be up, down, left, or right", directionValue)
	}

	tickValue := 0
	if tick != nil {
		if *tick < 0 {
			return ValidatedMessage{}, errors.New("tick must be non-negative")
		}
		tickValue = *tick
	}

	return ValidatedMessage{
		Type:      messageTypeDirection,
		Direction: dir,
		Tick:      tickValue,
	}, nil
}
