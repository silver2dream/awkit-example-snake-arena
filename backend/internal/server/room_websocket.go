package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"awkit-example-snake-arena-backend/internal/game"
	"awkit-example-snake-arena-backend/internal/rooms"
)

const (
	defaultGridWidth  = 15
	defaultGridHeight = 15
	defaultTickRate   = time.Millisecond * 100
)

type RoomServerOption func(*RoomWebSocketServer)

func WithTickInterval(interval time.Duration) RoomServerOption {
	return func(s *RoomWebSocketServer) {
		if interval > 0 {
			s.tickInterval = interval
		}
	}
}

func WithGridSize(width, height int) RoomServerOption {
	return func(s *RoomWebSocketServer) {
		if width > 0 && height > 0 {
			s.gridWidth = width
			s.gridHeight = height
		}
	}
}

func WithSeedSource(seed func() int64) RoomServerOption {
	return func(s *RoomWebSocketServer) {
		if seed != nil {
			s.seedSource = seed
		}
	}
}

type RoomWebSocketServer struct {
	manager      *rooms.RoomManager
	tickInterval time.Duration
	gridWidth    int
	gridHeight   int
	seedSource   func() int64

	mu    sync.Mutex
	rooms map[string]*roomState
}

func NewRoomWebSocketServer(manager *rooms.RoomManager, opts ...RoomServerOption) *RoomWebSocketServer {
	server := &RoomWebSocketServer{
		manager:      manager,
		tickInterval: defaultTickRate,
		gridWidth:    defaultGridWidth,
		gridHeight:   defaultGridHeight,
		seedSource: func() int64 {
			return time.Now().UnixNano()
		},
		rooms: make(map[string]*roomState),
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *RoomWebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	roomID, ok := extractRoomID(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	conn, err := upgradeToWebSocket(w, r)
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}

	client := newWSClient(conn)
	go s.handleConnection(r.Context(), roomID, client)
}

func (s *RoomWebSocketServer) handleConnection(ctx context.Context, roomID string, client *wsClient) {
	defer client.close()

	var state *roomState
	var playerName string
	var joined bool

	for {
		opcode, payload, err := client.conn.ReadFrame()
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
				client.sendError(fmt.Sprintf("read error: %v", err))
			}
			break
		}

		switch opcode {
		case 0x8:
			return
		case 0x9:
			_ = client.conn.WriteFrame(0xA, payload)
		case 0x1:
			var envelope messageEnvelope
			if err := json.Unmarshal(payload, &envelope); err != nil {
				client.sendError("invalid json payload")
				continue
			}
			switch envelope.Type {
			case "join_room":
				if joined {
					client.sendError("already joined")
					continue
				}
				var msg joinRoomMessage
				if err := json.Unmarshal(payload, &msg); err != nil {
					client.sendError("invalid join payload")
					continue
				}
				if msg.RoomID != "" && msg.RoomID != roomID {
					client.sendError("room mismatch")
					continue
				}
				if msg.PlayerName == "" {
					client.sendError("playerName is required")
					continue
				}

				snapshot, err := s.manager.JoinRoom(roomID, msg.PlayerName)
				if err != nil {
					client.sendError(err.Error())
					continue
				}

				state, err = s.ensureRoomState(roomID)
				if err != nil {
					client.sendError("unable to start room")
					return
				}

				playerName = msg.PlayerName
				client.player = msg.PlayerName
				state.addClient(client)

				joined = true
				s.broadcastRoomSnapshot(state, snapshot)
			case "input":
				if !joined || state == nil {
					client.sendError("join_room required before sending input")
					continue
				}
				var msg inputMessage
				if err := json.Unmarshal(payload, &msg); err != nil {
					client.sendError("invalid input payload")
					continue
				}
				dir, ok := parseDirection(msg.Direction)
				if !ok {
					client.sendError("invalid direction")
					continue
				}
				if !state.queueDirection(dir) {
					client.sendError("unable to queue direction")
					continue
				}
			default:
				client.sendError("unknown message type")
			}
		default:
			client.sendError("unsupported opcode")
		}
	}

	if joined {
		_ = s.manager.LeaveRoom(roomID, playerName)
	}
	if state != nil {
		state.removeClient(client)
		if snapshot, ok := s.manager.GetRoom(roomID); ok {
			s.broadcastRoomSnapshot(state, snapshot)
		}
	}
}

func (s *RoomWebSocketServer) ensureRoomState(roomID string) (*roomState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.rooms[roomID]; ok {
		return state, nil
	}

	engine, err := game.NewEngine(s.gridWidth, s.gridHeight, s.seedSource())
	if err != nil {
		return nil, err
	}

	state := &roomState{
		id:       roomID,
		server:   s,
		engine:   engine,
		clients:  make(map[*wsClient]struct{}),
		stopTick: make(chan struct{}),
	}
	state.startTicker(s.tickInterval)

	s.rooms[roomID] = state
	return state, nil
}

func (s *RoomWebSocketServer) broadcastRoomSnapshot(state *roomState, snapshot rooms.RoomSnapshot) {
	gameSnapshot := state.snapshot()
	message := roomSnapshotMessage{
		Type:    "room_snapshot",
		RoomID:  state.id,
		Tick:    gameSnapshot.Tick,
		Players: snapshot.Players,
		Food:    gameSnapshot.Food,
		Width:   gameSnapshot.Width,
		Height:  gameSnapshot.Height,
	}
	s.broadcast(state, message)
}

func (s *RoomWebSocketServer) broadcastTickUpdate(state *roomState, snapshot game.Snapshot) {
	roomSnapshot, ok := s.manager.GetRoom(state.id)
	if !ok {
		return
	}

	snakes := make(map[string][]game.Point, len(roomSnapshot.Players))
	scores := make(map[string]int, len(roomSnapshot.Players))
	for _, player := range roomSnapshot.Players {
		snakes[player] = snapshot.Snake
		scores[player] = snapshot.Score
	}

	message := tickUpdateMessage{
		Type:      "tick_update",
		RoomID:    state.id,
		Tick:      snapshot.Tick,
		Snakes:    snakes,
		Food:      snapshot.Food,
		Scores:    scores,
		GameOver:  snapshot.GameOver,
		Direction: directionToString(snapshot.Direction),
	}

	s.broadcast(state, message)
}

func (s *RoomWebSocketServer) broadcast(state *roomState, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	clients := state.clientsSnapshot()
	for _, client := range clients {
		client.enqueue(data)
	}
}

func extractRoomID(path string) (string, bool) {
	const prefix = "/ws/room/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	roomID := strings.TrimPrefix(path, prefix)
	if roomID == "" {
		return "", false
	}
	return roomID, true
}

type roomState struct {
	id       string
	server   *RoomWebSocketServer
	engine   *game.Engine
	clients  map[*wsClient]struct{}
	stopTick chan struct{}

	mu       sync.Mutex
	stopping bool
}

func (r *roomState) startTicker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				snapshot := r.advanceTick()
				r.server.broadcastTickUpdate(r, snapshot)
			case <-r.stopTick:
				ticker.Stop()
				return
			}
		}
	}()
}

func (r *roomState) advanceTick() game.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.engine.AdvanceTick()
	return r.engine.Snapshot()
}

func (r *roomState) snapshot() game.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.engine.Snapshot()
}

func (r *roomState) queueDirection(direction game.Direction) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.engine.QueueDirection(direction)
}

func (r *roomState) addClient(client *wsClient) {
	r.mu.Lock()
	r.clients[client] = struct{}{}
	r.mu.Unlock()
}

func (r *roomState) removeClient(client *wsClient) {
	r.mu.Lock()
	delete(r.clients, client)
	remaining := len(r.clients)
	r.mu.Unlock()

	if remaining == 0 {
		r.stop()
	}
}

func (r *roomState) stop() {
	r.mu.Lock()
	if r.stopping {
		r.mu.Unlock()
		return
	}
	r.stopping = true
	close(r.stopTick)
	r.mu.Unlock()

	r.server.mu.Lock()
	delete(r.server.rooms, r.id)
	r.server.mu.Unlock()
}

func (r *roomState) clientsSnapshot() []*wsClient {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]*wsClient, 0, len(r.clients))
	for client := range r.clients {
		result = append(result, client)
	}
	return result
}

type wsClient struct {
	conn      *wsConn
	player    string
	outgoing  chan []byte
	closeOnce sync.Once
	closed    chan struct{}
}

func newWSClient(conn *wsConn) *wsClient {
	client := &wsClient{
		conn:     conn,
		outgoing: make(chan []byte, 16),
		closed:   make(chan struct{}),
	}
	go client.writeLoop()
	return client
}

func (c *wsClient) writeLoop() {
	for msg := range c.outgoing {
		if err := c.conn.WriteFrame(0x1, msg); err != nil {
			break
		}
	}
	close(c.closed)
}

func (c *wsClient) enqueue(message []byte) {
	select {
	case <-c.closed:
		return
	case c.outgoing <- message:
	default:
		c.close()
	}
}

func (c *wsClient) sendError(message string) {
	payload := errorMessage{
		Type:    "error",
		Message: message,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.enqueue(data)
}

func (c *wsClient) close() {
	c.closeOnce.Do(func() {
		close(c.outgoing)
		<-c.closed
		_ = c.conn.Close()
	})
}

type messageEnvelope struct {
	Type string `json:"type"`
}

type joinRoomMessage struct {
	Type       string `json:"type"`
	RoomID     string `json:"roomId"`
	PlayerName string `json:"playerName"`
}

type inputMessage struct {
	Type      string `json:"type"`
	Direction string `json:"direction"`
	Seq       int    `json:"seq"`
}

type roomSnapshotMessage struct {
	Type    string     `json:"type"`
	RoomID  string     `json:"roomId"`
	Tick    int        `json:"tick"`
	Players []string   `json:"players"`
	Food    game.Point `json:"food"`
	Width   int        `json:"width"`
	Height  int        `json:"height"`
}

type tickUpdateMessage struct {
	Type      string                  `json:"type"`
	RoomID    string                  `json:"roomId"`
	Tick      int                     `json:"tick"`
	Snakes    map[string][]game.Point `json:"snakes"`
	Food      game.Point              `json:"food"`
	Scores    map[string]int          `json:"scores"`
	GameOver  bool                    `json:"gameOver"`
	Direction string                  `json:"direction"`
}

type errorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func parseDirection(direction string) (game.Direction, bool) {
	switch strings.ToLower(direction) {
	case "up":
		return game.DirectionUp, true
	case "down":
		return game.DirectionDown, true
	case "left":
		return game.DirectionLeft, true
	case "right":
		return game.DirectionRight, true
	default:
		return 0, false
	}
}

func directionToString(direction game.Direction) string {
	switch direction {
	case game.DirectionUp:
		return "up"
	case game.DirectionDown:
		return "down"
	case game.DirectionLeft:
		return "left"
	default:
		return "right"
	}
}
