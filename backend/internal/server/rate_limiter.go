package server

import (
	"sync"
	"time"
)

// RateLimiter enforces a fixed number of messages per client within a window.
type RateLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	clients map[string]*clientRate
}

type clientRate struct {
	windowStart time.Time
	count       int
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Second
	}

	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]*clientRate),
	}
}

// Allow returns whether the client is under the rate limit. If not allowed,
// the returned duration indicates how long until the window resets.
func (r *RateLimiter) Allow(clientID string, now time.Time) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.clients[clientID]
	if !ok {
		state = &clientRate{windowStart: now}
		r.clients[clientID] = state
	} else if now.Before(state.windowStart) || now.Sub(state.windowStart) >= r.window {
		state.windowStart = now
		state.count = 0
	}

	if state.count < r.limit {
		state.count++
		return true, 0
	}

	retryAfter := r.window - now.Sub(state.windowStart)
	if retryAfter < 0 {
		retryAfter = 0
	}

	return false, retryAfter
}
