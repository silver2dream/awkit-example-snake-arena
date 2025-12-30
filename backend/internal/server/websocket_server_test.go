package server

import (
	"strings"
	"testing"
	"time"

	"awkit-example-snake-arena-backend/internal/game"
)

func TestHandleMessageValidDirection(t *testing.T) {
	server := NewWebSocketServer(5, time.Second)
	now := time.Unix(0, 0)

	msg := []byte(`{"type":"direction","direction":"up","tick":3}`)
	validated, errResp := server.HandleMessage("player-1", msg, now)

	if errResp != nil {
		t.Fatalf("expected no error response, got %+v", errResp)
	}
	if validated.Type != "direction" {
		t.Fatalf("expected message type direction, got %q", validated.Type)
	}
	if validated.Direction != game.DirectionUp {
		t.Fatalf("expected direction up, got %v", validated.Direction)
	}
	if validated.Tick != 3 {
		t.Fatalf("expected tick 3, got %d", validated.Tick)
	}
}

func TestHandleMessageValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		payload    []byte
		expectCode string
		expectMsg  string
	}{
		{
			name:       "invalid json",
			payload:    []byte(`{`),
			expectCode: "invalid_message",
			expectMsg:  "invalid json payload",
		},
		{
			name:       "missing type",
			payload:    []byte(`{"direction":"up"}`),
			expectCode: "invalid_message",
			expectMsg:  "message type is required",
		},
		{
			name:       "unknown type",
			payload:    []byte(`{"type":"ping"}`),
			expectCode: "invalid_message",
			expectMsg:  "unsupported message type",
		},
		{
			name:       "missing direction",
			payload:    []byte(`{"type":"direction"}`),
			expectCode: "invalid_message",
			expectMsg:  "direction is required",
		},
		{
			name:       "invalid direction",
			payload:    []byte(`{"type":"direction","direction":"north"}`),
			expectCode: "invalid_message",
			expectMsg:  "invalid direction",
		},
		{
			name:       "negative tick",
			payload:    []byte(`{"type":"direction","direction":"up","tick":-1}`),
			expectCode: "invalid_message",
			expectMsg:  "tick must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewWebSocketServer(10, time.Second)
			now := time.Unix(0, 0)

			_, errResp := server.HandleMessage("client", tt.payload, now)
			if errResp == nil {
				t.Fatalf("expected error response")
			}
			if errResp.Code != tt.expectCode {
				t.Fatalf("expected code %q, got %q", tt.expectCode, errResp.Code)
			}
			if !strings.Contains(errResp.Message, tt.expectMsg) {
				t.Fatalf("expected message to contain %q, got %q", tt.expectMsg, errResp.Message)
			}
		})
	}
}

func TestHandleMessageRateLimiting(t *testing.T) {
	server := NewWebSocketServer(1, time.Second)
	now := time.Unix(0, 0)
	payload := []byte(`{"type":"direction","direction":"left"}`)

	if _, errResp := server.HandleMessage("fast-client", payload, now); errResp != nil {
		t.Fatalf("expected first message allowed, got error: %+v", errResp)
	}

	_, errResp := server.HandleMessage("fast-client", payload, now.Add(100*time.Millisecond))
	if errResp == nil || errResp.Code != "rate_limited" {
		t.Fatalf("expected rate limited response, got %+v", errResp)
	}
	if errResp.RetryAfterMs <= 0 {
		t.Fatalf("expected retryAfterMs to be positive, got %d", errResp.RetryAfterMs)
	}

	// After the window, the client should be allowed again.
	if _, errResp = server.HandleMessage("fast-client", payload, now.Add(time.Second+time.Millisecond)); errResp != nil {
		t.Fatalf("expected message after cooldown to be allowed, got %+v", errResp)
	}
}

func TestHandleMessageEmptyClient(t *testing.T) {
	server := NewWebSocketServer(1, time.Second)
	payload := []byte(`{"type":"direction","direction":"right"}`)

	_, errResp := server.HandleMessage("   ", payload, time.Unix(0, 0))
	if errResp == nil {
		t.Fatalf("expected error for empty client id")
	}
	if errResp.Code != "invalid_client" {
		t.Fatalf("expected invalid_client code, got %q", errResp.Code)
	}
}
