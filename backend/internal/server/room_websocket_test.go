package server

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"awkit-example-snake-arena-backend/internal/game"
	"awkit-example-snake-arena-backend/internal/rooms"
)

func TestJoinSendsRoomSnapshot(t *testing.T) {
	manager := rooms.NewRoomManager(2)
	room := manager.CreateRoom()

	server := NewRoomWebSocketServer(manager, WithTickInterval(time.Millisecond*20), WithSeedSource(func() int64 { return 1 }))
	client := startPipeWebSocket(t, server, room.ID)
	defer client.close()

	client.sendJSON(t, map[string]any{
		"type":       "join_room",
		"playerName": "alice",
		"roomId":     room.ID,
	})

	msg := client.readJSON(t)
	if msg["type"] != "room_snapshot" {
		t.Fatalf("expected room_snapshot, got %v", msg["type"])
	}
	players, ok := msg["players"].([]any)
	if !ok {
		t.Fatalf("expected players array in snapshot")
	}
	if len(players) != 1 || players[0].(string) != "alice" {
		t.Fatalf("expected alice in snapshot players, got %v", players)
	}
}

func TestInvalidJSONReturnsError(t *testing.T) {
	manager := rooms.NewRoomManager(2)
	room := manager.CreateRoom()

	server := NewRoomWebSocketServer(manager, WithTickInterval(time.Millisecond*20), WithSeedSource(func() int64 { return 1 }))
	client := startPipeWebSocket(t, server, room.ID)
	defer client.close()

	client.writeFrame(0x1, []byte("not-json"))

	msg := client.readJSON(t)
	if msg["type"] != "error" {
		t.Fatalf("expected error message, got %v", msg)
	}
}

func TestInputQueuesForNextTick(t *testing.T) {
	manager := rooms.NewRoomManager(2)
	room := manager.CreateRoom()

	server := NewRoomWebSocketServer(manager, WithTickInterval(time.Millisecond*20), WithGridSize(8, 8), WithSeedSource(func() int64 { return 5 }))
	client := startPipeWebSocket(t, server, room.ID)
	defer client.close()

	client.sendJSON(t, map[string]any{
		"type":       "join_room",
		"playerName": "alice",
		"roomId":     room.ID,
	})

	var firstTick map[string]any
	for {
		msg := client.readJSON(t)
		if msg["type"] == "tick_update" {
			firstTick = msg
			break
		}
	}

	headBefore := extractHead(t, firstTick)

	client.sendJSON(t, map[string]any{
		"type":      "input",
		"direction": "down",
	})

	var nextTick map[string]any
	for {
		msg := client.readJSON(t)
		if msg["type"] == "tick_update" {
			tick := int(msg["tick"].(float64))
			if tick > int(firstTick["tick"].(float64)) {
				nextTick = msg
				break
			}
		}
	}

	headAfter := extractHead(t, nextTick)

	if headAfter.X != headBefore.X || headAfter.Y != headBefore.Y+1 {
		t.Fatalf("expected head to move down from %+v to %+v", headBefore, headAfter)
	}
}

func TestDisconnectRemovesPlayer(t *testing.T) {
	manager := rooms.NewRoomManager(2)
	room := manager.CreateRoom()

	server := NewRoomWebSocketServer(manager, WithTickInterval(time.Millisecond*10), WithSeedSource(func() int64 { return 1 }))
	client := startPipeWebSocket(t, server, room.ID)

	client.sendJSON(t, map[string]any{
		"type":       "join_room",
		"playerName": "alice",
		"roomId":     room.ID,
	})

	client.close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := manager.GetRoom(room.ID); !ok {
			return
		}
		time.Sleep(time.Millisecond * 10)
	}

	t.Fatalf("expected room to be removed after disconnect")
}

type testWSClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

type pipeResponseWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
	rw   *bufio.ReadWriter
}

func (p *pipeResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return p.conn, p.rw, nil
}

func startPipeWebSocket(t *testing.T, handler http.Handler, roomID string) *testWSClient {
	t.Helper()

	serverConn, clientConn := net.Pipe()

	req := httptest.NewRequest("GET", "/ws/room/"+roomID, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("test-key-"+roomID)))

	rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn))
	w := &pipeResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             serverConn,
		rw:               rw,
	}

	go handler.ServeHTTP(w, req)

	reader := bufio.NewReader(clientConn)
	status, err := reader.ReadString('\n')
	if err != nil {
		clientConn.Close()
		t.Fatalf("failed to read status line: %v", err)
	}
	if !strings.Contains(status, "101") {
		clientConn.Close()
		t.Fatalf("expected 101 response, got %q", status)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			clientConn.Close()
			t.Fatalf("failed to read handshake headers: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	return &testWSClient{
		conn:   clientConn,
		reader: reader,
	}
}

func (c *testWSClient) writeFrame(opcode byte, payload []byte) {
	maskKey := []byte{0x12, 0x34, 0x56, 0x78}
	header := []byte{0x80 | opcode}

	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		header = append(header, byte(payloadLen)|0x80)
	case payloadLen < 65536:
		header = append(header, 126|0x80)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(payloadLen))
		header = append(header, ext...)
	default:
		header = append(header, 127|0x80)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(payloadLen))
		header = append(header, ext...)
	}

	header = append(header, maskKey...)

	masked := make([]byte, payloadLen)
	for i := 0; i < payloadLen; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}

	if _, err := c.conn.Write(append(header, masked...)); err != nil {
		panic("failed to write frame: " + err.Error())
	}
}

func (c *testWSClient) sendJSON(t *testing.T, payload any) {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	c.writeFrame(0x1, data)
}

func (c *testWSClient) readJSON(t *testing.T) map[string]any {
	t.Helper()

	opcode, payload, err := c.readFrame()
	if err != nil {
		t.Fatalf("failed to read frame: %v", err)
	}
	if opcode != 0x1 {
		t.Fatalf("expected text frame, got opcode %d", opcode)
	}

	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("failed to unmarshal json: %v", err)
	}
	return msg
}

func (c *testWSClient) readFrame() (byte, []byte, error) {
	header1, err := c.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	opcode := header1 & 0x0F

	header2, err := c.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	payloadLen := int64(header2 & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}

	return opcode, payload, nil
}

func (c *testWSClient) close() {
	_ = c.conn.Close()
}

func extractHead(t *testing.T, message map[string]any) game.Point {
	t.Helper()

	snakes, ok := message["snakes"].(map[string]any)
	if !ok {
		t.Fatalf("expected snakes map in tick_update")
	}

	for _, body := range snakes {
		segments := body.([]any)
		if len(segments) == 0 {
			continue
		}
		head := segments[0].(map[string]any)
		return game.Point{
			X: int(head["X"].(float64)),
			Y: int(head["Y"].(float64)),
		}
	}

	t.Fatalf("no snake segments found")
	return game.Point{}
}
