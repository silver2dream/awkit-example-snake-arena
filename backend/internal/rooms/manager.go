package rooms

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomFull          = errors.New("room is full")
	ErrPlayerExists      = errors.New("player already in room")
	ErrPlayerNotInRoom   = errors.New("player not in room")
	ErrInvalidPlayerName = errors.New("invalid player name")
)

const DefaultRoomCapacity = 4

type RoomSnapshot struct {
	ID       string
	Capacity int
	Players  []string
}

type room struct {
	id       string
	capacity int
	players  map[string]struct{}
}

func (r *room) snapshot() RoomSnapshot {
	players := make([]string, 0, len(r.players))
	for name := range r.players {
		players = append(players, name)
	}
	sort.Strings(players)

	return RoomSnapshot{
		ID:       r.id,
		Capacity: r.capacity,
		Players:  players,
	}
}

type RoomManager struct {
	mu       sync.Mutex
	rooms    map[string]*room
	capacity int
	nextID   uint64
}

func NewRoomManager(capacity int) *RoomManager {
	if capacity <= 0 {
		capacity = DefaultRoomCapacity
	}

	return &RoomManager{
		rooms:    make(map[string]*room),
		capacity: capacity,
	}
}

func (m *RoomManager) CreateRoom() RoomSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("room-%d", m.nextID)
	room := &room{
		id:       id,
		capacity: m.capacity,
		players:  make(map[string]struct{}),
	}
	m.rooms[id] = room

	return room.snapshot()
}

func (m *RoomManager) JoinRoom(roomID, playerName string) (RoomSnapshot, error) {
	name := strings.TrimSpace(playerName)
	if name == "" {
		return RoomSnapshot{}, ErrInvalidPlayerName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return RoomSnapshot{}, ErrRoomNotFound
	}
	if _, exists := room.players[name]; exists {
		return RoomSnapshot{}, ErrPlayerExists
	}
	if len(room.players) >= room.capacity {
		return RoomSnapshot{}, ErrRoomFull
	}

	room.players[name] = struct{}{}
	return room.snapshot(), nil
}

func (m *RoomManager) LeaveRoom(roomID, playerName string) error {
	name := strings.TrimSpace(playerName)
	if name == "" {
		return ErrInvalidPlayerName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}
	if _, exists := room.players[name]; !exists {
		return ErrPlayerNotInRoom
	}

	delete(room.players, name)
	if len(room.players) == 0 {
		delete(m.rooms, roomID)
	}

	return nil
}

func (m *RoomManager) GetRoom(roomID string) (RoomSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return RoomSnapshot{}, false
	}

	return room.snapshot(), true
}
