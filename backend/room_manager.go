package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

var (
	ErrRoomNotFound        = errors.New("room not found")
	ErrPlayerAlreadyInRoom = errors.New("player already in room")
	ErrPlayerNotInRoom     = errors.New("player not in room")
	ErrPlayerIDRequired    = errors.New("player id is required")
)

type RoomManager struct {
	mu      sync.RWMutex
	rooms   map[string]*Room
	counter uint64
}

type Room struct {
	ID      string
	players map[string]struct{}
}

type RoomSnapshot struct {
	ID      string
	Players []string
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

func (m *RoomManager) CreateRoom(initialPlayers ...string) (string, error) {
	players := make(map[string]struct{}, len(initialPlayers))
	for _, playerID := range initialPlayers {
		if playerID == "" {
			return "", ErrPlayerIDRequired
		}
		players[playerID] = struct{}{}
	}

	roomID := fmt.Sprintf("room-%d", atomic.AddUint64(&m.counter, 1))

	m.mu.Lock()
	m.rooms[roomID] = &Room{
		ID:      roomID,
		players: players,
	}
	m.mu.Unlock()

	return roomID, nil
}

func (m *RoomManager) JoinRoom(roomID, playerID string) error {
	if playerID == "" {
		return ErrPlayerIDRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}

	if _, exists := room.players[playerID]; exists {
		return ErrPlayerAlreadyInRoom
	}

	room.players[playerID] = struct{}{}
	return nil
}

// LeaveRoom removes a player from the room. It returns true if the room was removed because it became empty.
func (m *RoomManager) LeaveRoom(roomID, playerID string) (bool, error) {
	if playerID == "" {
		return false, ErrPlayerIDRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return false, ErrRoomNotFound
	}

	if _, exists := room.players[playerID]; !exists {
		return false, ErrPlayerNotInRoom
	}

	delete(room.players, playerID)
	if len(room.players) == 0 {
		delete(m.rooms, roomID)
		return true, nil
	}

	return false, nil
}

func (m *RoomManager) GetRoomSnapshot(roomID string) (RoomSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return RoomSnapshot{}, ErrRoomNotFound
	}

	players := make([]string, 0, len(room.players))
	for playerID := range room.players {
		players = append(players, playerID)
	}
	sort.Strings(players)

	return RoomSnapshot{
		ID:      room.ID,
		Players: players,
	}, nil
}

func (m *RoomManager) RoomCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}
