package server

import (
	"testing"
	"time"
)

func TestRateLimiterHonorsLimitWithinWindow(t *testing.T) {
	limiter := NewRateLimiter(2, time.Second)
	now := time.Unix(0, 0)

	if allowed, _ := limiter.Allow("client-1", now); !allowed {
		t.Fatalf("expected first message to be allowed")
	}
	if allowed, _ := limiter.Allow("client-1", now.Add(100*time.Millisecond)); !allowed {
		t.Fatalf("expected second message within limit to be allowed")
	}

	allowed, retryAfter := limiter.Allow("client-1", now.Add(200*time.Millisecond))
	if allowed {
		t.Fatalf("expected third message to be rate limited")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %v", retryAfter)
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	limiter := NewRateLimiter(1, time.Second)
	start := time.Unix(0, 0)

	if allowed, _ := limiter.Allow("player", start); !allowed {
		t.Fatalf("expected first message to be allowed")
	}
	if allowed, _ := limiter.Allow("player", start.Add(500*time.Millisecond)); allowed {
		t.Fatalf("expected second message within window to be blocked")
	}

	allowed, _ := limiter.Allow("player", start.Add(time.Second+time.Millisecond))
	if !allowed {
		t.Fatalf("expected limiter to reset after window")
	}
}

func TestRateLimiterIsPerClient(t *testing.T) {
	limiter := NewRateLimiter(1, time.Second)
	now := time.Unix(0, 0)

	if allowed, _ := limiter.Allow("client-a", now); !allowed {
		t.Fatalf("expected client-a first message allowed")
	}
	if allowed, _ := limiter.Allow("client-b", now); !allowed {
		t.Fatalf("expected client-b first message allowed independently")
	}
	if allowed, _ := limiter.Allow("client-a", now); allowed {
		t.Fatalf("expected second client-a message to be blocked within window")
	}
}
