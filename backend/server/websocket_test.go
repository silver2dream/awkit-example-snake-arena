package server

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"awkit-example-snake-arena-backend/game"
	"awkit-example-snake-arena-backend/ws"
)

func TestWebSocketJoinInputTickAndDisconnectCleanup(t *testing.T) {
	manager := NewRoomManager(200 * time.Millisecond)
	room := manager.CreateRoom()

	server := NewHTTPServer(manager)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	req := httptest.NewRequest(http.MethodGet, "/ws/room/"+room.ID(), nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("test-websocket-key")))
	req.Header.Set("Sec-WebSocket-Version", "13")

	writer := newHijackResponseWriter(serverConn)
	go func() {
		server.handleRoomWebSocket(writer, req)
	}()

	clientRW := bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
	respReq := httptest.NewRequest(http.MethodGet, "http://example.com/ws/room/"+room.ID(), nil)
	resp, err := http.ReadResponse(clientRW.Reader, respReq)
	if err != nil {
		t.Fatalf("failed to read websocket handshake response: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusSwitchingProtocols; got != want {
		t.Fatalf("websocket handshake status mismatch: got %d want %d", got, want)
	}

	conn := ws.NewClientConnWithRW(clientConn, clientRW)

	join := clientMessage{
		Type:       clientMessageJoinRoom,
		RoomID:     room.ID(),
		PlayerName: "alice",
	}
	if err := conn.WriteJSON(join); err != nil {
		t.Fatalf("write join message failed: %v", err)
	}

	snapshotMsg, err := readUntilType(conn, serverMessageRoomSnapshot, 2*time.Second)
	if err != nil {
		t.Fatalf("did not receive room snapshot: %v", err)
	}
	if snapshotMsg.Snapshot == nil || len(snapshotMsg.Snapshot.Players) != 1 {
		t.Fatalf("unexpected snapshot payload: %#v", snapshotMsg)
	}

	if err := conn.WriteJSON(clientMessage{
		Type:      clientMessageInput,
		Direction: game.DirectionUp,
		Seq:       1,
	}); err != nil {
		t.Fatalf("write input message failed: %v", err)
	}

	tickMsg, err := readUntilType(conn, serverMessageTickUpdate, 2*time.Second)
	if err != nil {
		t.Fatalf("did not receive tick update: %v", err)
	}
	if tickMsg.Snapshot == nil || len(tickMsg.Snapshot.Players) != 1 {
		t.Fatalf("unexpected tick payload: %#v", tickMsg)
	}

	player := tickMsg.Snapshot.Players[0]
	if got, want := player.Direction, game.DirectionUp; got != want {
		t.Fatalf("direction mismatch after buffered input: got %s want %s", got, want)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, exists := manager.GetRoom(room.ID()); !exists {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("room should be cleaned up when all players disconnect")
}

func readUntilType(conn *ws.Conn, messageType string, timeout time.Duration) (serverMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
		_ = conn.SetReadDeadline(deadline)
		var msg serverMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return serverMessage{}, err
		}
		if msg.Type == messageType {
			return msg, nil
		}
		if time.Now().After(deadline) {
			return serverMessage{}, fmt.Errorf("timeout waiting for message type %s", messageType)
		}
	}
}

type hijackResponseWriter struct {
	headers http.Header
	conn    net.Conn
	rw      *bufio.ReadWriter
}

func newHijackResponseWriter(conn net.Conn) *hijackResponseWriter {
	return &hijackResponseWriter{
		headers: make(http.Header),
		conn:    conn,
		rw:      bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

func (w *hijackResponseWriter) Header() http.Header {
	return w.headers
}

func (w *hijackResponseWriter) Write(body []byte) (int, error) {
	if _, err := w.rw.Write(body); err != nil {
		return 0, err
	}
	return len(body), w.rw.Flush()
}

func (w *hijackResponseWriter) WriteHeader(_ int) {}

func (w *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}
