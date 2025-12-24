package game

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"awkit-example-snake-arena-backend/internal/ws"
)

// MessageEnvelope wraps outbound WebSocket messages with a type discriminator.
type MessageEnvelope struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// SnapshotPayload is sent when a client first connects.
type SnapshotPayload struct {
	RoomID string     `json:"roomId"`
	State  *GameState `json:"state"`
}

// TickPayload delivers tick outcomes alongside the latest state snapshot.
type TickPayload struct {
	RoomID string     `json:"roomId"`
	State  *GameState `json:"state"`
	Tick   TickResult `json:"tick"`
}

// RoomConfig defines defaults for new rooms.
type RoomConfig struct {
	TickInterval time.Duration
	Grid         Grid
	Seed         int64
}

// RoomManager manages room lifecycle and acts as the HTTP handler.
type RoomManager struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	config RoomConfig
}

// NewRoomManager constructs a manager with sane defaults.
func NewRoomManager(config RoomConfig) *RoomManager {
	if config.TickInterval <= 0 {
		config.TickInterval = 200 * time.Millisecond
	}
	if config.Grid.Width == 0 || config.Grid.Height == 0 {
		config.Grid = Grid{Width: 15, Height: 15}
	}
	if config.Seed == 0 {
		config.Seed = 1
	}

	return &RoomManager{
		rooms:  make(map[string]*Room),
		config: config,
	}
}

// ServeHTTP upgrades the connection and attaches the client to a room.
func (m *RoomManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/ws/rooms/") {
		http.NotFound(w, r)
		return
	}

	roomID := strings.TrimPrefix(r.URL.Path, "/ws/rooms/")
	roomID = strings.Trim(roomID, "/")
	if roomID == "" {
		http.Error(w, "room id required", http.StatusBadRequest)
		return
	}

	conn, err := ws.Upgrade(w, r)
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}

	room := m.getOrCreateRoom(roomID)
	room.addClient(conn)
}

// Shutdown stops all active rooms.
func (m *RoomManager) Shutdown() {
	m.mu.Lock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.rooms = make(map[string]*Room)
	m.mu.Unlock()

	for _, room := range rooms {
		room.Stop()
	}
}

// RoomClientCount returns the number of clients in a room (primarily for tests).
func (m *RoomManager) RoomClientCount(roomID string) int {
	m.mu.Lock()
	room := m.rooms[roomID]
	m.mu.Unlock()
	if room == nil {
		return 0
	}
	return room.ClientCount()
}

func (m *RoomManager) getOrCreateRoom(roomID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, ok := m.rooms[roomID]; ok {
		return room
	}

	room := newRoom(roomID, m.config, m.handleRoomEmpty)
	m.rooms[roomID] = room
	return room
}

func (m *RoomManager) handleRoomEmpty(roomID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if ok {
		delete(m.rooms, roomID)
	}
	m.mu.Unlock()

	if ok {
		room.Stop()
	}
}

type Room struct {
	id           string
	engine       *TickEngine
	state        *GameState
	tickInterval time.Duration
	clients      map[*client]struct{}
	mu           sync.RWMutex
	stop         chan struct{}
	stopOnce     sync.Once
	onEmpty      func(string)
}

func newRoom(id string, config RoomConfig, onEmpty func(string)) *Room {
	room := &Room{
		id:           id,
		engine:       NewTickEngine(config.Seed + int64(len(id))),
		state:        defaultState(config.Grid),
		tickInterval: config.TickInterval,
		clients:      make(map[*client]struct{}),
		stop:         make(chan struct{}),
		onEmpty:      onEmpty,
	}

	go room.run()
	return room
}

func (r *Room) run() {
	ticker := time.NewTicker(r.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.handleTick()
		case <-r.stop:
			return
		}
	}
}

func (r *Room) addClient(conn *ws.Conn) {
	client := newClient(r, conn)

	r.mu.Lock()
	r.clients[client] = struct{}{}
	snapshot := CloneState(r.state)
	r.mu.Unlock()

	client.start()
	r.sendToClient(client, MessageEnvelope{
		Type: "snapshot",
		Data: SnapshotPayload{RoomID: r.id, State: snapshot},
	})
}

func (r *Room) handleTick() {
	r.mu.Lock()
	result := r.engine.Tick(r.state)
	snapshot := CloneState(r.state)
	clients := r.copyClientsLocked()
	r.mu.Unlock()

	if len(clients) == 0 || snapshot == nil {
		return
	}

	payload := MessageEnvelope{
		Type: "tick",
		Data: TickPayload{
			RoomID: r.id,
			State:  snapshot,
			Tick:   result,
		},
	}
	r.broadcast(payload, clients)
}

func (r *Room) broadcast(message MessageEnvelope, targets []*client) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	for _, c := range targets {
		r.queueMessage(c, data)
	}
}

func (r *Room) sendToClient(c *client, message MessageEnvelope) {
	r.broadcast(message, []*client{c})
}

func (r *Room) queueMessage(c *client, data []byte) {
	select {
	case c.send <- data:
	default:
		r.removeClient(c)
	}
}

func (r *Room) removeClient(c *client) {
	r.mu.Lock()
	if _, ok := r.clients[c]; !ok {
		r.mu.Unlock()
		return
	}
	delete(r.clients, c)
	empty := len(r.clients) == 0
	r.mu.Unlock()

	c.close()
	if empty && r.onEmpty != nil {
		r.onEmpty(r.id)
	}
}

// Stop halts ticking and closes client connections.
func (r *Room) Stop() {
	r.stopOnce.Do(func() {
		close(r.stop)

		r.mu.Lock()
		clients := make([]*client, 0, len(r.clients))
		for c := range r.clients {
			clients = append(clients, c)
		}
		r.clients = make(map[*client]struct{})
		r.mu.Unlock()

		for _, c := range clients {
			c.close()
		}
	})
}

// ClientCount exposes the current number of clients attached to the room.
func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) copyClientsLocked() []*client {
	clients := make([]*client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	return clients
}

func defaultState(grid Grid) *GameState {
	if grid.Width == 0 || grid.Height == 0 {
		grid = Grid{Width: 15, Height: 15}
	}

	snakeID := "snake-1"
	return &GameState{
		Grid: grid,
		Snakes: map[string]*Snake{
			snakeID: {
				ID:        snakeID,
				Body:      []Position{{X: 1, Y: 1}},
				Direction: DirectionRight,
				Alive:     true,
			},
		},
		Food: []Position{
			{X: grid.Width / 2, Y: grid.Height / 2},
		},
	}
}
