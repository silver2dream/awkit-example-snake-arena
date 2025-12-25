package game

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"awkit-example-snake-arena-backend/internal/ws"
)

type client struct {
	room          *Room
	conn          *ws.Conn
	send          chan []byte
	done          chan struct{}
	once          sync.Once
	limiter       *rateLimiter
	lastInput     time.Time
	inputThrottle time.Duration
}

func newClient(room *Room, conn *ws.Conn) *client {
	limiter := newRateLimiter(room.limits.maxMessages, room.limits.window)
	return &client{
		room:          room,
		conn:          conn,
		send:          make(chan []byte, 16),
		done:          make(chan struct{}),
		limiter:       limiter,
		inputThrottle: room.limits.inputThrottle,
	}
}

func (c *client) start() {
	go c.readLoop()
	go c.writeLoop()
}

func (c *client) readLoop() {
	for {
		opcode, payload, err := c.conn.Read()
		if err != nil {
			break
		}
		switch opcode {
		case ws.OpClose:
			_ = c.conn.WriteClose()
			return
		case ws.OpPing:
			_ = c.conn.WritePong(payload)
		case ws.OpText:
			if c.limiter != nil && !c.limiter.Allow(time.Now()) {
				c.sendError("rate_limit", "too many messages, slow down")
				continue
			}
			if err := c.handleTextMessage(payload); err != nil {
				c.sendError("bad_request", err.Error())
			}
		}
	}

	c.room.removeClient(c)
}

func (c *client) writeLoop() {
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteText(msg); err != nil {
				c.room.removeClient(c)
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *client) close() {
	c.once.Do(func() {
		close(c.done)
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *client) handleTextMessage(payload []byte) error {
	var msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(payload, &msg); err != nil {
		return errors.New("invalid message format")
	}

	switch strings.ToLower(msg.Type) {
	case "input":
		return c.handleInputCommand(msg.Data)
	default:
		return errors.New("unsupported message type")
	}
}

func (c *client) handleInputCommand(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("input payload required")
	}

	var cmd struct {
		Direction string `json:"direction"`
	}

	if err := json.Unmarshal(raw, &cmd); err != nil {
		return errors.New("invalid input payload")
	}

	direction, err := parseDirection(cmd.Direction)
	if err != nil {
		return err
	}

	if c.inputThrottle > 0 && !c.allowInput(time.Now()) {
		c.sendError("throttled", "input commands are too frequent")
		return nil
	}

	return c.room.applyDirectionChange(direction)
}

func (c *client) allowInput(now time.Time) bool {
	if now.Sub(c.lastInput) < c.inputThrottle {
		return false
	}
	c.lastInput = now
	return true
}

func (c *client) sendError(code, message string) {
	c.room.sendToClient(c, MessageEnvelope{
		Type: "error",
		Data: map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func parseDirection(value string) (Direction, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "up":
		return DirectionUp, nil
	case "right":
		return DirectionRight, nil
	case "down":
		return DirectionDown, nil
	case "left":
		return DirectionLeft, nil
	default:
		return Direction(0), errors.New("invalid direction")
	}
}

type rateLimiter struct {
	limit       int
	window      time.Duration
	count       int
	windowStart time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	if limit <= 0 {
		return nil
	}
	if window <= 0 {
		window = time.Second
	}
	return &rateLimiter{
		limit:       limit,
		window:      window,
		windowStart: time.Now(),
	}
}

func (r *rateLimiter) Allow(now time.Time) bool {
	if r == nil {
		return true
	}

	if now.Sub(r.windowStart) > r.window {
		r.windowStart = now
		r.count = 0
	}

	if r.count >= r.limit {
		return false
	}

	r.count++
	return true
}
