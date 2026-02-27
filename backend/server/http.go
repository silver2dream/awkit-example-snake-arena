package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"awkit-example-snake-arena-backend/game"
	"awkit-example-snake-arena-backend/ws"
)

const (
	clientMessageJoinRoom = "join_room"
	clientMessageInput    = "input"

	serverMessageRoomSnapshot = "room_snapshot"
	serverMessageTickUpdate   = "tick_update"
	serverMessageError        = "error"
)

type serverMessage struct {
	Type     string        `json:"type"`
	RoomID   string        `json:"roomId,omitempty"`
	Snapshot *RoomSnapshot `json:"snapshot,omitempty"`
	Message  string        `json:"message,omitempty"`
}

type clientMessage struct {
	Type       string         `json:"type"`
	RoomID     string         `json:"roomId,omitempty"`
	PlayerName string         `json:"playerName,omitempty"`
	Direction  game.Direction `json:"direction,omitempty"`
	Seq        uint64         `json:"seq,omitempty"`
}

type createRoomResponse struct {
	RoomID string `json:"roomId"`
}

type joinRoomRequest struct {
	PlayerName string `json:"playerName"`
}

type joinRoomResponse struct {
	RoomID   string `json:"roomId"`
	PlayerID string `json:"playerId"`
}

// HTTPServer wires lobby REST and room websocket endpoints.
type HTTPServer struct {
	manager *RoomManager
	mux     *http.ServeMux
}

// NewHTTPServer constructs a fully wired HTTP server.
func NewHTTPServer(manager *RoomManager) *HTTPServer {
	if manager == nil {
		manager = NewRoomManager(120 * time.Millisecond)
	}

	s := &HTTPServer{
		manager: manager,
		mux:     http.NewServeMux(),
	}

	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/rooms", s.handleRooms)
	s.mux.HandleFunc("/api/rooms/", s.handleRoomJoin)
	s.mux.HandleFunc("/ws/room/", s.handleRoomWebSocket)

	return s
}

// Handler returns the server http handler.
func (s *HTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleRooms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.manager.ListRooms())
	case http.MethodPost:
		room := s.manager.CreateRoom()
		writeJSON(w, http.StatusCreated, createRoomResponse{RoomID: room.ID()})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *HTTPServer) handleRoomJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	roomID, ok := parseJoinPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	var req joinRoomRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, serverMessage{Type: serverMessageError, Message: "invalid request body"})
		return
	}

	playerID, err := s.manager.JoinRoom(roomID, req.PlayerName)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			writeJSON(w, http.StatusNotFound, serverMessage{Type: serverMessageError, Message: err.Error()})
		case errors.Is(err, ErrPlayerNameRequired):
			writeJSON(w, http.StatusBadRequest, serverMessage{Type: serverMessageError, Message: err.Error()})
		case errors.Is(err, ErrPlayerAlreadyJoined), errors.Is(err, ErrRoomFull):
			writeJSON(w, http.StatusConflict, serverMessage{Type: serverMessageError, Message: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, serverMessage{Type: serverMessageError, Message: "failed to join room"})
		}
		return
	}

	writeJSON(w, http.StatusOK, joinRoomResponse{
		RoomID:   roomID,
		PlayerID: playerID,
	})
}

func (s *HTTPServer) handleRoomWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	roomID, ok := parseWSPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if _, exists := s.manager.GetRoom(roomID); !exists {
		http.NotFound(w, r)
		return
	}

	conn, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	var joinedPlayerID string
	sendError := func(message string) {
		if joinedPlayerID != "" {
			room, ok := s.manager.GetRoom(roomID)
			if ok {
				_ = room.SendToPlayer(joinedPlayerID, serverMessage{
					Type:    serverMessageError,
					Message: message,
				})
				return
			}
		}
		sendConnError(conn, message)
	}

	defer func() {
		if joinedPlayerID != "" {
			s.manager.LeaveRoom(roomID, joinedPlayerID)
		}
	}()

	for {
		var incoming clientMessage
		if err := conn.ReadJSON(&incoming); err != nil {
			return
		}

		switch incoming.Type {
		case clientMessageJoinRoom:
			if incoming.RoomID != "" && incoming.RoomID != roomID {
				sendError("room id mismatch")
				continue
			}
			playerID, err := s.manager.JoinRoom(roomID, incoming.PlayerName)
			if err != nil {
				sendError(err.Error())
				continue
			}

			room, ok := s.manager.GetRoom(roomID)
			if !ok {
				sendError(ErrRoomNotFound.Error())
				continue
			}
			if err := room.AttachClient(playerID, conn); err != nil {
				sendError(err.Error())
				continue
			}

			joinedPlayerID = playerID
			room.BroadcastSnapshot()

		case clientMessageInput:
			if joinedPlayerID == "" {
				sendError("must join room before sending input")
				continue
			}
			room, ok := s.manager.GetRoom(roomID)
			if !ok {
				sendError(ErrRoomNotFound.Error())
				continue
			}
			room.BufferInput(joinedPlayerID, incoming.Direction)

		default:
			sendError("unknown message type")
		}
	}
}

func parseJoinPath(path string) (string, bool) {
	const prefix = "/api/rooms/"
	const suffix = "/join"

	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}

	roomID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if roomID == "" || strings.Contains(roomID, "/") {
		return "", false
	}
	return roomID, true
}

func parseWSPath(path string) (string, bool) {
	const prefix = "/ws/room/"

	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	roomID := strings.TrimPrefix(path, prefix)
	if roomID == "" || strings.Contains(roomID, "/") {
		return "", false
	}
	return roomID, true
}

func sendConnError(conn *ws.Conn, message string) {
	_ = conn.WriteJSON(serverMessage{
		Type:    serverMessageError,
		Message: message,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
