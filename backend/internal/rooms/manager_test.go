package rooms

import (
	"errors"
	"testing"
)

func TestCreateRoomGeneratesUniqueIDs(t *testing.T) {
	manager := NewRoomManager(3)

	first := manager.CreateRoom()
	second := manager.CreateRoom()

	if first.ID == "" || second.ID == "" {
		t.Fatalf("expected non-empty room IDs")
	}
	if first.ID == second.ID {
		t.Fatalf("expected unique room IDs, got %q twice", first.ID)
	}
	if first.Capacity != 3 || second.Capacity != 3 {
		t.Fatalf("expected capacity to be preserved on room creation")
	}
}

func TestJoinRoomHappyPath(t *testing.T) {
	manager := NewRoomManager(2)
	room := manager.CreateRoom()

	snapshot, err := manager.JoinRoom(room.ID, "Alice")
	if err != nil {
		t.Fatalf("unexpected error joining room: %v", err)
	}
	if len(snapshot.Players) != 1 || snapshot.Players[0] != "Alice" {
		t.Fatalf("expected player to be added, got %+v", snapshot.Players)
	}

	current, ok := manager.GetRoom(room.ID)
	if !ok {
		t.Fatalf("expected room to exist after join")
	}
	if len(current.Players) != 1 || current.Players[0] != "Alice" {
		t.Fatalf("expected manager state to reflect joined player, got %+v", current.Players)
	}
}

func TestJoinRoomTrimsAndValidatesName(t *testing.T) {
	manager := NewRoomManager(1)
	room := manager.CreateRoom()

	snapshot, err := manager.JoinRoom(room.ID, "  Bob  ")
	if err != nil {
		t.Fatalf("unexpected error for trimmed name: %v", err)
	}
	if snapshot.Players[0] != "Bob" {
		t.Fatalf("expected trimmed name to be stored, got %q", snapshot.Players[0])
	}

	if _, err := manager.JoinRoom(room.ID, "   "); !errors.Is(err, ErrInvalidPlayerName) {
		t.Fatalf("expected invalid name error, got %v", err)
	}
}

func TestJoinRoomErrors(t *testing.T) {
	manager := NewRoomManager(1)
	room := manager.CreateRoom()

	if _, err := manager.JoinRoom("missing", "Alice"); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected room not found error, got %v", err)
	}

	if _, err := manager.JoinRoom(room.ID, "Alice"); err != nil {
		t.Fatalf("unexpected error joining room: %v", err)
	}
	if _, err := manager.JoinRoom(room.ID, "Alice"); !errors.Is(err, ErrPlayerExists) {
		t.Fatalf("expected duplicate player error, got %v", err)
	}

	if _, err := manager.JoinRoom(room.ID, "Charlie"); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("expected room full error, got %v", err)
	}
}

func TestLeaveRoomRemovesPlayerAndCleansRoom(t *testing.T) {
	manager := NewRoomManager(2)
	room := manager.CreateRoom()

	if _, err := manager.JoinRoom(room.ID, "Alice"); err != nil {
		t.Fatalf("unexpected join error: %v", err)
	}
	if _, err := manager.JoinRoom(room.ID, "Bob"); err != nil {
		t.Fatalf("unexpected join error: %v", err)
	}

	if err := manager.LeaveRoom(room.ID, "Alice"); err != nil {
		t.Fatalf("unexpected leave error: %v", err)
	}
	snapshot, ok := manager.GetRoom(room.ID)
	if !ok {
		t.Fatalf("expected room to persist while players remain")
	}
	if len(snapshot.Players) != 1 || snapshot.Players[0] != "Bob" {
		t.Fatalf("expected only Bob to remain, got %+v", snapshot.Players)
	}

	if err := manager.LeaveRoom(room.ID, "Bob"); err != nil {
		t.Fatalf("unexpected leave error: %v", err)
	}
	if _, ok := manager.GetRoom(room.ID); ok {
		t.Fatalf("expected room to be cleaned up when empty")
	}
}

func TestLeaveRoomErrors(t *testing.T) {
	manager := NewRoomManager(1)
	room := manager.CreateRoom()

	if err := manager.LeaveRoom("missing", "Alice"); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected room not found error, got %v", err)
	}

	if err := manager.LeaveRoom(room.ID, "Alice"); !errors.Is(err, ErrPlayerNotInRoom) {
		t.Fatalf("expected player not in room error, got %v", err)
	}
}
