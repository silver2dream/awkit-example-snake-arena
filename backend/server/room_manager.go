package server

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"awkit-example-snake-arena-backend/game"
	"awkit-example-snake-arena-backend/ws"
)

const (
	defaultBoardWidth  = 20
	defaultBoardHeight = 20
)

var (
	ErrRoomNotFound        = errors.New("room not found")
	ErrPlayerNameRequired  = errors.New("player name is required")
	ErrPlayerAlreadyJoined = errors.New("player is already connected")
	ErrRoomFull            = errors.New("room is full")
)

// RoomSummary is a compact room representation for lobby listing.
type RoomSummary struct {
	ID          string `json:"id"`
	PlayerCount int    `json:"playerCount"`
	Status      string `json:"status"`
}

// PlayerSnapshot is a compact player view sent to clients.
type PlayerSnapshot struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Score     int             `json:"score"`
	Alive     bool            `json:"alive"`
	Direction game.Direction  `json:"direction"`
	Body      []game.Position `json:"body"`
}

// RoomSnapshot is a serialized room game state sent over WebSocket.
type RoomSnapshot struct {
	RoomID  string           `json:"roomId"`
	Width   int              `json:"width"`
	Height  int              `json:"height"`
	Tick    uint64           `json:"tick"`
	Players []PlayerSnapshot `json:"players"`
	Food    *game.Position   `json:"food,omitempty"`
}

// RoomManager keeps all active in-memory rooms.
type RoomManager struct {
	mu           sync.RWMutex
	rooms        map[string]*Room
	nextRoomID   atomic.Uint64
	nextPlayerID atomic.Uint64
	tickInterval time.Duration
}

// NewRoomManager creates an in-memory room manager.
func NewRoomManager(tickInterval time.Duration) *RoomManager {
	if tickInterval <= 0 {
		tickInterval = 120 * time.Millisecond
	}
	return &RoomManager{
		rooms:        make(map[string]*Room),
		tickInterval: tickInterval,
	}
}

// CreateRoom allocates and registers a new room.
func (m *RoomManager) CreateRoom() *Room {
	roomID := "room-" + strconv.FormatUint(m.nextRoomID.Add(1), 10)
	room := newRoom(roomID, m.tickInterval)

	m.mu.Lock()
	m.rooms[roomID] = room
	m.mu.Unlock()

	return room
}

// GetRoom returns a room if it exists.
func (m *RoomManager) GetRoom(roomID string) (*Room, bool) {
	m.mu.RLock()
	room, ok := m.rooms[roomID]
	m.mu.RUnlock()
	return room, ok
}

// ListRooms returns active room summaries.
func (m *RoomManager) ListRooms() []RoomSummary {
	m.mu.RLock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.mu.RUnlock()

	summaries := make([]RoomSummary, 0, len(rooms))
	for _, room := range rooms {
		summaries = append(summaries, room.Summary())
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})
	return summaries
}

// JoinRoom creates or reuses a player by name in the given room.
func (m *RoomManager) JoinRoom(roomID, playerName string) (string, error) {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return "", ErrPlayerNameRequired
	}

	playerID := "player-" + strconv.FormatUint(m.nextPlayerID.Add(1), 10)

	m.mu.RLock()
	room, ok := m.rooms[roomID]
	if !ok {
		m.mu.RUnlock()
		return "", ErrRoomNotFound
	}

	id, _, err := room.AddOrGetPlayer(playerID, playerName)
	m.mu.RUnlock()
	if err != nil {
		return "", err
	}
	return id, nil
}

// LeaveRoom removes a player and cleans up the room when empty.
func (m *RoomManager) LeaveRoom(roomID, playerID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok {
		m.mu.Unlock()
		return
	}

	remaining := room.RemovePlayer(playerID)
	if remaining == 0 {
		room.Stop()
		delete(m.rooms, roomID)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	room.BroadcastSnapshot()
}

type roomClient struct {
	conn    *ws.Conn
	writeMu sync.Mutex
}

func (c *roomClient) writeJSON(message serverMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(message)
}

func (c *roomClient) close() {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.Close()
}

// Room represents a game room and its active simulation.
type Room struct {
	id           string
	state        *game.GameState
	engine       *game.Engine
	tickInterval time.Duration

	mu            sync.RWMutex
	playersByName map[string]string
	clients       map[string]*roomClient
	tickStop      chan struct{}
	tickRunning   bool
}

func newRoom(id string, tickInterval time.Duration) *Room {
	seed := time.Now().UnixNano()
	for _, ch := range id {
		seed += int64(ch)
	}

	return &Room{
		id:            id,
		state:         game.NewGameState(defaultBoardWidth, defaultBoardHeight),
		engine:        game.NewEngine(seed),
		tickInterval:  tickInterval,
		playersByName: make(map[string]string),
		clients:       make(map[string]*roomClient),
	}
}

// ID returns the room id.
func (r *Room) ID() string {
	return r.id
}

// Summary returns room metadata for lobby listing.
func (r *Room) Summary() RoomSummary {
	r.mu.RLock()
	playerCount := len(r.state.Players)
	r.mu.RUnlock()

	status := "waiting"
	if playerCount >= 2 {
		status = "active"
	}

	return RoomSummary{
		ID:          r.id,
		PlayerCount: playerCount,
		Status:      status,
	}
}

// PlayerCount returns the number of players in the room.
func (r *Room) PlayerCount() int {
	r.mu.RLock()
	count := len(r.state.Players)
	r.mu.RUnlock()
	return count
}

// AddOrGetPlayer creates a player by name, or returns existing player id.
func (r *Room) AddOrGetPlayer(playerID, playerName string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingID, exists := r.playersByName[playerName]; exists {
		if _, hasClient := r.clients[existingID]; hasClient {
			return "", false, ErrPlayerAlreadyJoined
		}
		return existingID, false, nil
	}

	spawn, ok := r.nextSpawnPositionLocked()
	if !ok {
		return "", false, ErrRoomFull
	}

	r.state.Players[playerID] = &game.Player{
		ID:    playerID,
		Name:  playerName,
		Score: 0,
	}
	r.state.Snakes[playerID] = &game.Snake{
		PlayerID:  playerID,
		Body:      []game.Position{spawn},
		Direction: game.DirectionRight,
		Alive:     true,
	}
	r.playersByName[playerName] = playerID

	if r.state.Food == nil {
		if foodPos, ok := r.nextFoodPositionLocked(); ok {
			r.state.Food = &game.Food{Position: foodPos}
		}
	}

	if len(r.state.Players) == 1 {
		r.startTickLoopLocked()
	}
	return playerID, true, nil
}

// AttachClient associates a websocket connection with a joined player.
func (r *Room) AttachClient(playerID string, conn *ws.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.state.Players[playerID]; !exists {
		return fmt.Errorf("player %s does not exist", playerID)
	}
	if _, exists := r.clients[playerID]; exists {
		return ErrPlayerAlreadyJoined
	}
	r.clients[playerID] = &roomClient{conn: conn}
	return nil
}

// RemovePlayer removes a player and any active websocket connection.
func (r *Room) RemovePlayer(playerID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, exists := r.state.Players[playerID]
	if !exists {
		return len(r.state.Players)
	}

	delete(r.state.Players, playerID)
	delete(r.state.Snakes, playerID)
	delete(r.state.BufferedInputs, playerID)
	delete(r.playersByName, player.Name)

	if client, hasClient := r.clients[playerID]; hasClient {
		client.close()
		delete(r.clients, playerID)
	}

	if len(r.state.Players) == 0 {
		r.stopTickLoopLocked()
	}

	return len(r.state.Players)
}

// BufferInput stores a direction input to apply on the next tick.
func (r *Room) BufferInput(playerID string, direction game.Direction) {
	r.mu.Lock()
	r.engine.BufferInput(r.state, playerID, direction)
	r.mu.Unlock()
}

// Snapshot returns a serializable copy of room state.
func (r *Room) Snapshot() RoomSnapshot {
	r.mu.RLock()
	snapshot := r.snapshotLocked()
	r.mu.RUnlock()
	return snapshot
}

// BroadcastSnapshot sends a room_snapshot message to all connected clients.
func (r *Room) BroadcastSnapshot() {
	snapshot := r.Snapshot()
	r.broadcast(serverMessage{
		Type:     serverMessageRoomSnapshot,
		RoomID:   r.id,
		Snapshot: &snapshot,
	})
}

// SendToPlayer sends a single websocket message to one connected player.
func (r *Room) SendToPlayer(playerID string, message serverMessage) error {
	r.mu.RLock()
	client, exists := r.clients[playerID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("player %s has no active websocket", playerID)
	}
	return client.writeJSON(message)
}

// Stop halts the room tick loop.
func (r *Room) Stop() {
	r.mu.Lock()
	r.stopTickLoopLocked()
	r.mu.Unlock()
}

func (r *Room) tickerActive() bool {
	r.mu.RLock()
	active := r.tickRunning
	r.mu.RUnlock()
	return active
}

func (r *Room) startTickLoopLocked() {
	if r.tickRunning {
		return
	}

	r.tickStop = make(chan struct{})
	r.tickRunning = true
	stop := r.tickStop
	interval := r.tickInterval
	if interval <= 0 {
		interval = 120 * time.Millisecond
	}

	go r.runTickLoop(stop, interval)
}

func (r *Room) stopTickLoopLocked() {
	if !r.tickRunning {
		return
	}
	close(r.tickStop)
	r.tickRunning = false
	r.tickStop = nil
}

func (r *Room) runTickLoop(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			snapshot := r.advanceTickAndSnapshot()
			if snapshot == nil {
				continue
			}

			r.broadcast(serverMessage{
				Type:     serverMessageTickUpdate,
				RoomID:   r.id,
				Snapshot: snapshot,
			})
		}
	}
}

func (r *Room) advanceTickAndSnapshot() *RoomSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.state.Players) == 0 {
		return nil
	}

	r.engine.AdvanceTick(r.state)
	snapshot := r.snapshotLocked()
	return &snapshot
}

func (r *Room) snapshotLocked() RoomSnapshot {
	players := make([]PlayerSnapshot, 0, len(r.state.Players))
	playerIDs := make([]string, 0, len(r.state.Players))
	for playerID := range r.state.Players {
		playerIDs = append(playerIDs, playerID)
	}
	sort.Strings(playerIDs)

	for _, playerID := range playerIDs {
		player := r.state.Players[playerID]
		if player == nil {
			continue
		}

		snake := r.state.Snakes[playerID]
		body := make([]game.Position, 0)
		alive := false
		direction := game.Direction("")
		if snake != nil {
			alive = snake.Alive
			direction = snake.Direction
			body = append(body, snake.Body...)
		}

		players = append(players, PlayerSnapshot{
			ID:        player.ID,
			Name:      player.Name,
			Score:     player.Score,
			Alive:     alive,
			Direction: direction,
			Body:      body,
		})
	}

	var food *game.Position
	if r.state.Food != nil {
		copyPos := r.state.Food.Position
		food = &copyPos
	}

	return RoomSnapshot{
		RoomID:  r.id,
		Width:   r.state.Width,
		Height:  r.state.Height,
		Tick:    r.state.Tick,
		Players: players,
		Food:    food,
	}
}

func (r *Room) broadcast(message serverMessage) {
	r.mu.RLock()
	clients := make([]*roomClient, 0, len(r.clients))
	for _, client := range r.clients {
		clients = append(clients, client)
	}
	r.mu.RUnlock()

	for _, client := range clients {
		if err := client.writeJSON(message); err != nil {
			client.close()
		}
	}
}

func (r *Room) nextSpawnPositionLocked() (game.Position, bool) {
	occupied := r.occupiedPositionsLocked()

	for y := 0; y < r.state.Height; y++ {
		for x := 0; x < r.state.Width; x++ {
			pos := game.Position{X: x, Y: y}
			if _, exists := occupied[pos]; exists {
				continue
			}
			return pos, true
		}
	}
	return game.Position{}, false
}

func (r *Room) nextFoodPositionLocked() (game.Position, bool) {
	occupied := r.occupiedPositionsLocked()

	for y := 0; y < r.state.Height; y++ {
		for x := 0; x < r.state.Width; x++ {
			pos := game.Position{X: x, Y: y}
			if _, exists := occupied[pos]; exists {
				continue
			}
			return pos, true
		}
	}
	return game.Position{}, false
}

func (r *Room) occupiedPositionsLocked() map[game.Position]struct{} {
	occupied := make(map[game.Position]struct{})
	for _, snake := range r.state.Snakes {
		if snake == nil || !snake.Alive {
			continue
		}
		for _, segment := range snake.Body {
			occupied[segment] = struct{}{}
		}
	}
	return occupied
}
