package server

import (
	"testing"
	"time"
)

func TestRoomManagerCreateJoinLeaveCleanup(t *testing.T) {
	manager := NewRoomManager(50 * time.Millisecond)

	room := manager.CreateRoom()
	if room.ID() == "" {
		t.Fatalf("expected room id to be set")
	}

	if got, want := len(manager.ListRooms()), 1; got != want {
		t.Fatalf("room list size mismatch: got %d want %d", got, want)
	}

	playerA, err := manager.JoinRoom(room.ID(), "alice")
	if err != nil {
		t.Fatalf("join room failed: %v", err)
	}
	if playerA == "" {
		t.Fatalf("expected player id")
	}
	if !room.tickerActive() {
		t.Fatalf("tick loop should start when first player joins")
	}

	playerB, err := manager.JoinRoom(room.ID(), "bob")
	if err != nil {
		t.Fatalf("join room failed: %v", err)
	}

	if got, want := room.PlayerCount(), 2; got != want {
		t.Fatalf("player count mismatch: got %d want %d", got, want)
	}

	manager.LeaveRoom(room.ID(), playerA)
	if _, exists := manager.GetRoom(room.ID()); !exists {
		t.Fatalf("room should remain while players are still connected")
	}

	manager.LeaveRoom(room.ID(), playerB)
	if _, exists := manager.GetRoom(room.ID()); exists {
		t.Fatalf("room should be removed when empty")
	}
	if room.tickerActive() {
		t.Fatalf("tick loop should stop once room is empty")
	}
}

func TestJoinRoomRejectsBlankName(t *testing.T) {
	manager := NewRoomManager(50 * time.Millisecond)
	room := manager.CreateRoom()

	if _, err := manager.JoinRoom(room.ID(), "   "); err != ErrPlayerNameRequired {
		t.Fatalf("expected ErrPlayerNameRequired, got %v", err)
	}
}
