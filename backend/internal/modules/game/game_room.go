package game

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
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
	TickInterval    time.Duration
	Grid            Grid
	Seed            int64
	MaxClients      int
	MaxRooms        int
	MaxMessages     int
	MaxMessageBytes int
	MessageWindow   time.Duration
	InputThrottle   time.Duration
}

// RoomManager manages room lifecycle and acts as the HTTP handler.
type RoomManager struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	config RoomConfig
}

var roomIDPattern = regexp.MustCompile(`^[A-Z0-9]{6}$`)

const defaultSnakeID = "snake-1"

type clientLimits struct {
	maxMessages     int
	window          time.Duration
	inputThrottle   time.Duration
	maxPayloadBytes int
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
	if config.MaxClients <= 0 {
		config.MaxClients = 8
	}
	if config.MaxMessages <= 0 {
		config.MaxMessages = 30
	}
	if config.MessageWindow <= 0 {
		config.MessageWindow = time.Second
	}
	if config.InputThrottle <= 0 {
		config.InputThrottle = 50 * time.Millisecond
	}
	if config.MaxRooms <= 0 {
		config.MaxRooms = 128
	}
	if config.MaxMessageBytes <= 0 {
		config.MaxMessageBytes = 2048
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

	roomID, err := normalizeRoomID(strings.TrimPrefix(r.URL.Path, "/ws/rooms/"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if m.roomLimitReached(roomID) {
		http.Error(w, "room limit reached", http.StatusTooManyRequests)
		return
	}

	if m.roomAtCapacity(roomID) {
		http.Error(w, "room is full", http.StatusTooManyRequests)
		return
	}

	conn, err := ws.Upgrade(w, r)
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}

	room, err := m.getOrCreateRoom(roomID)
	if err != nil {
		_ = conn.WriteClose()
		return
	}
	if room.isAtCapacity() {
		_ = conn.WriteClose()
		return
	}
	room.addClient(conn)
}

func normalizeRoomID(raw string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", errors.New("room id required")
	}

	normalized := strings.ToUpper(trimmed)
	if !roomIDPattern.MatchString(normalized) {
		return "", errors.New("invalid room id format")
	}
	return normalized, nil
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

func (m *RoomManager) roomLimitReached(roomID string) bool {
	if m.config.MaxRooms <= 0 {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[roomID]; exists {
		return false
	}

	return len(m.rooms) >= m.config.MaxRooms
}

func (m *RoomManager) roomAtCapacity(roomID string) bool {
	m.mu.Lock()
	room := m.rooms[roomID]
	m.mu.Unlock()
	if room == nil {
		return false
	}
	return room.isAtCapacity()
}

// RoomClientCount returns the number of clients in a room (primarily for tests).
func (m *RoomManager) RoomClientCount(roomID string) int {
	roomID, err := normalizeRoomID(roomID)
	if err != nil {
		return 0
	}
	m.mu.Lock()
	room := m.rooms[roomID]
	m.mu.Unlock()
	if room == nil {
		return 0
	}
	return room.ClientCount()
}

func (m *RoomManager) getOrCreateRoom(roomID string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, ok := m.rooms[roomID]; ok {
		return room, nil
	}

	if m.config.MaxRooms > 0 && len(m.rooms) >= m.config.MaxRooms {
		return nil, errors.New("room limit reached")
	}

	room := newRoom(roomID, m.config, m.handleRoomEmpty)
	m.rooms[roomID] = room
	return room, nil
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
	maxClients   int
	limits       clientLimits
	mu           sync.RWMutex
	stop         chan struct{}
	stopOnce     sync.Once
	onEmpty      func(string)
}

func newRoom(id string, config RoomConfig, onEmpty func(string)) *Room {
	limits := clientLimits{
		maxMessages:     config.MaxMessages,
		window:          config.MessageWindow,
		inputThrottle:   config.InputThrottle,
		maxPayloadBytes: config.MaxMessageBytes,
	}

	room := &Room{
		id:           id,
		engine:       NewTickEngine(config.Seed + int64(len(id))),
		state:        defaultState(config.Grid),
		tickInterval: config.TickInterval,
		clients:      make(map[*client]struct{}),
		maxClients:   config.MaxClients,
		limits:       limits,
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
	if r.isAtCapacity() {
		_ = conn.WriteClose()
		return
	}

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

func (r *Room) isAtCapacity() bool {
	if r.maxClients <= 0 {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients) >= r.maxClients
}

func (r *Room) applyDirectionChange(direction Direction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == nil {
		return errors.New("room not ready")
	}

	snake, ok := r.state.Snakes[defaultSnakeID]
	if !ok || snake == nil {
		return errors.New("snake not found")
	}

	if !snake.Alive {
		return errors.New("snake is not alive")
	}

	snake.Direction = direction
	return nil
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

	return &GameState{
		Grid: grid,
		Snakes: map[string]*Snake{
			defaultSnakeID: {
				ID:        defaultSnakeID,
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
