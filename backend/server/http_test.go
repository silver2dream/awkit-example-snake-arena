package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	manager := NewRoomManager(100 * time.Millisecond)
	server := NewHTTPServer(manager)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if got, want := rec.Code, 200; got != want {
		t.Fatalf("status mismatch: got %d want %d", got, want)
	}

	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected health payload: %#v", payload)
	}
}

func TestRoomsAPIFlow(t *testing.T) {
	manager := NewRoomManager(100 * time.Millisecond)
	server := NewHTTPServer(manager)

	createReq := httptest.NewRequest("POST", "/api/rooms", nil)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	if got, want := createRec.Code, 201; got != want {
		t.Fatalf("create status mismatch: got %d want %d", got, want)
	}

	var created createRoomResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.RoomID == "" {
		t.Fatalf("expected room id")
	}

	listReq := httptest.NewRequest("GET", "/api/rooms", nil)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)

	if got, want := listRec.Code, 200; got != want {
		t.Fatalf("list status mismatch: got %d want %d", got, want)
	}

	var rooms []RoomSummary
	if err := json.NewDecoder(listRec.Body).Decode(&rooms); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(rooms) != 1 || rooms[0].ID != created.RoomID {
		t.Fatalf("unexpected room listing: %#v", rooms)
	}

	joinBody, err := json.Marshal(joinRoomRequest{PlayerName: "alice"})
	if err != nil {
		t.Fatalf("marshal join request failed: %v", err)
	}
	joinReq := httptest.NewRequest("POST", "/api/rooms/"+created.RoomID+"/join", bytes.NewReader(joinBody))
	joinReq.Header.Set("Content-Type", "application/json")
	joinRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(joinRec, joinReq)

	if got, want := joinRec.Code, 200; got != want {
		t.Fatalf("join status mismatch: got %d want %d", got, want)
	}

	var joined joinRoomResponse
	if err := json.NewDecoder(joinRec.Body).Decode(&joined); err != nil {
		t.Fatalf("decode join response failed: %v", err)
	}
	if joined.PlayerID == "" {
		t.Fatalf("expected player id in join response")
	}

	listAfterReq := httptest.NewRequest("GET", "/api/rooms", nil)
	listAfterRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listAfterRec, listAfterReq)

	var roomsAfterJoin []RoomSummary
	if err := json.NewDecoder(listAfterRec.Body).Decode(&roomsAfterJoin); err != nil {
		t.Fatalf("decode list after join failed: %v", err)
	}
	if len(roomsAfterJoin) != 1 || roomsAfterJoin[0].PlayerCount != 1 {
		t.Fatalf("unexpected room list after join: %#v", roomsAfterJoin)
	}
}
