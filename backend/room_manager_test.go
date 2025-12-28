package main

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestCreateRoomGeneratesUniqueIDs(t *testing.T) {
	manager := NewRoomManager()

	const roomCount = 100
	ids := make(map[string]struct{}, roomCount)
	var idsMu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < roomCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			roomID, err := manager.CreateRoom()
			if err != nil {
				t.Fatalf("CreateRoom returned error: %v", err)
			}

			idsMu.Lock()
			defer idsMu.Unlock()

			if _, exists := ids[roomID]; exists {
				t.Fatalf("duplicate room id generated: %s", roomID)
			}
			ids[roomID] = struct{}{}
		}()
	}

	wg.Wait()

	if len(ids) != roomCount {
		t.Fatalf("expected %d unique rooms, got %d", roomCount, len(ids))
	}

	if manager.RoomCount() != roomCount {
		t.Fatalf("expected room count %d, got %d", roomCount, manager.RoomCount())
	}
}

func TestJoinRoom(t *testing.T) {
	manager := NewRoomManager()
	roomID, err := manager.CreateRoom("alice")
	if err != nil {
		t.Fatalf("CreateRoom returned error: %v", err)
	}

	if err := manager.JoinRoom(roomID, "bob"); err != nil {
		t.Fatalf("JoinRoom returned error: %v", err)
	}

	snapshot, err := manager.GetRoomSnapshot(roomID)
	if err != nil {
		t.Fatalf("GetRoomSnapshot returned error: %v", err)
	}

	if len(snapshot.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(snapshot.Players))
	}

	if err := manager.JoinRoom("unknown-room", "charlie"); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound, got %v", err)
	}

	if err := manager.JoinRoom(roomID, ""); !errors.Is(err, ErrPlayerIDRequired) {
		t.Fatalf("expected ErrPlayerIDRequired, got %v", err)
	}

	if err := manager.JoinRoom(roomID, "alice"); !errors.Is(err, ErrPlayerAlreadyInRoom) {
		t.Fatalf("expected ErrPlayerAlreadyInRoom, got %v", err)
	}
}

func TestLeaveRoom(t *testing.T) {
	manager := NewRoomManager()
	roomID, err := manager.CreateRoom("alice", "bob")
	if err != nil {
		t.Fatalf("CreateRoom returned error: %v", err)
	}

	if _, err := manager.LeaveRoom(roomID, "charlie"); !errors.Is(err, ErrPlayerNotInRoom) {
		t.Fatalf("expected ErrPlayerNotInRoom, got %v", err)
	}

	removed, err := manager.LeaveRoom(roomID, "alice")
	if err != nil {
		t.Fatalf("LeaveRoom returned error: %v", err)
	}
	if removed {
		t.Fatalf("room should not be removed while players remain")
	}

	snapshot, err := manager.GetRoomSnapshot(roomID)
	if err != nil {
		t.Fatalf("GetRoomSnapshot returned error: %v", err)
	}
	if len(snapshot.Players) != 1 || snapshot.Players[0] != "bob" {
		t.Fatalf("expected only bob to remain, got %#v", snapshot.Players)
	}

	removed, err = manager.LeaveRoom(roomID, "bob")
	if err != nil {
		t.Fatalf("LeaveRoom returned error: %v", err)
	}
	if !removed {
		t.Fatalf("expected room to be removed when last player leaves")
	}

	if manager.RoomCount() != 0 {
		t.Fatalf("expected room manager to be empty, got %d rooms", manager.RoomCount())
	}

	if _, err := manager.LeaveRoom(roomID, "bob"); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound after room cleanup, got %v", err)
	}
}

func TestConcurrentJoinAndLeave(t *testing.T) {
	manager := NewRoomManager()
	roomID, err := manager.CreateRoom("owner")
	if err != nil {
		t.Fatalf("CreateRoom returned error: %v", err)
	}

	const joiners = 50
	var wg sync.WaitGroup

	for i := 0; i < joiners; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			playerID := fmt.Sprintf("player-%d", idx)
			if err := manager.JoinRoom(roomID, playerID); err != nil {
				t.Errorf("JoinRoom error for %s: %v", playerID, err)
			}
		}(i)
	}
	wg.Wait()

	snapshot, err := manager.GetRoomSnapshot(roomID)
	if err != nil {
		t.Fatalf("GetRoomSnapshot returned error: %v", err)
	}
	if len(snapshot.Players) != joiners+1 {
		t.Fatalf("expected %d players after concurrent joins, got %d", joiners+1, len(snapshot.Players))
	}

	wg = sync.WaitGroup{}
	for i := 0; i < joiners; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			playerID := fmt.Sprintf("player-%d", idx)
			removed, err := manager.LeaveRoom(roomID, playerID)
			if err != nil {
				t.Errorf("LeaveRoom error for %s: %v", playerID, err)
			}
			if removed {
				t.Errorf("room removed early while players still present")
			}
		}(i)
	}
	wg.Wait()

	if manager.RoomCount() != 1 {
		t.Fatalf("expected room to remain until owner leaves, got %d rooms", manager.RoomCount())
	}

	removed, err := manager.LeaveRoom(roomID, "owner")
	if err != nil {
		t.Fatalf("LeaveRoom returned error: %v", err)
	}
	if !removed {
		t.Fatalf("expected room to be removed after owner leaves last")
	}

	if manager.RoomCount() != 0 {
		t.Fatalf("expected all rooms cleaned up, got %d", manager.RoomCount())
	}
}
