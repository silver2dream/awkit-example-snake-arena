package game

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"awkit-example-snake-arena-backend/internal/ws"
)

func TestWebSocketRoomLifecycle(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval: 15 * time.Millisecond,
		Grid:         Grid{Width: 6, Height: 6},
		Seed:         99,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/test-room")
	defer conn.Close()

	first := readEnvelope(t, conn, reader)
	if first.Type != "snapshot" {
		t.Fatalf("expected first message to be snapshot, got %s", first.Type)
	}

	tick := waitForType(t, conn, reader, "tick")
	var payload TickPayload
	decodeData(t, tick.Data, &payload)

	if payload.RoomID != "test-room" {
		t.Fatalf("expected room id test-room, got %s", payload.RoomID)
	}

	if payload.State == nil || len(payload.State.Snakes) == 0 {
		t.Fatalf("expected a populated state snapshot, got %+v", payload.State)
	}

	if got := manager.RoomClientCount("test-room"); got != 1 {
		t.Fatalf("expected 1 client registered, got %d", got)
	}

	_ = conn.Close()
	time.Sleep(50 * time.Millisecond)

	if got := manager.RoomClientCount("test-room"); got != 0 {
		t.Fatalf("expected client to be removed after close, got %d", got)
	}
}

func TestBroadcastTickToMultipleClients(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval: 10 * time.Millisecond,
		Grid:         Grid{Width: 5, Height: 5},
		Seed:         7,
	})
	defer manager.Shutdown()

	connA, readerA := openWebSocket(t, manager, "/ws/rooms/shared")
	defer connA.Close()
	connB, readerB := openWebSocket(t, manager, "/ws/rooms/shared")
	defer connB.Close()

	_ = readEnvelope(t, connA, readerA) // snapshot
	_ = readEnvelope(t, connB, readerB) // snapshot

	tickA := waitForType(t, connA, readerA, "tick")
	tickB := waitForType(t, connB, readerB, "tick")

	var payloadA, payloadB TickPayload
	decodeData(t, tickA.Data, &payloadA)
	decodeData(t, tickB.Data, &payloadB)

	if payloadA.RoomID != "shared" || payloadB.RoomID != "shared" {
		t.Fatalf("expected room id shared, got %s and %s", payloadA.RoomID, payloadB.RoomID)
	}

	if len(payloadA.State.Snakes) == 0 || len(payloadB.State.Snakes) == 0 {
		t.Fatalf("expected state to include snakes")
	}
}

type pipeResponseWriter struct {
	header http.Header
	conn   net.Conn
}

func (w *pipeResponseWriter) Header() http.Header {
	return w.header
}

func (w *pipeResponseWriter) Write(p []byte) (int, error) {
	return w.conn.Write(p)
}

func (w *pipeResponseWriter) WriteHeader(statusCode int) {}

func (w *pipeResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn))
	return w.conn, rw, nil
}

func openWebSocket(t *testing.T, manager *RoomManager, path string) (net.Conn, *bufio.Reader) {
	t.Helper()
	clientConn, serverConn := net.Pipe()

	req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("client-key")))

	writer := &pipeResponseWriter{
		header: http.Header{},
		conn:   serverConn,
	}

	done := make(chan struct{})
	go func() {
		manager.ServeHTTP(writer, req)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	verifyHandshake(t, reader)
	_ = clientConn.SetReadDeadline(time.Time{})
	<-done
	return clientConn, reader
}

func verifyHandshake(t *testing.T, reader *bufio.Reader) {
	t.Helper()
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("expected status 101, got %s", strings.TrimSpace(status))
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake headers: %v", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
}

func readEnvelope(t *testing.T, conn net.Conn, reader *bufio.Reader) MessageEnvelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, payload := readFrame(t, reader)
	if opcode != ws.OpText {
		t.Fatalf("expected text frame opcode, got %d", opcode)
	}
	return decodeEnvelope(t, payload)
}

func waitForType(t *testing.T, conn net.Conn, reader *bufio.Reader, desired string) MessageEnvelope {
	t.Helper()
	for attempts := 0; attempts < 10; attempts++ {
		env := readEnvelope(t, conn, reader)
		if env.Type == desired {
			return env
		}
	}
	t.Fatalf("did not receive message type %s", desired)
	return MessageEnvelope{}
}

func decodeEnvelope(t *testing.T, payload []byte) MessageEnvelope {
	t.Helper()
	var env MessageEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func decodeData[T any](t *testing.T, data interface{}, target *T) {
	t.Helper()
	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
}

func readFrame(t *testing.T, r *bufio.Reader) (byte, []byte) {
	t.Helper()
	first, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read frame first byte: %v", err)
	}
	second, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read frame second byte: %v", err)
	}

	opcode := first & 0x0F
	masked := second&0x80 != 0
	length := int64(second & 0x7F)

	switch length {
	case 126:
		var ext [2]byte
		if _, err := r.Read(ext[:]); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		length = int64(ext[0])<<8 | int64(ext[1])
	case 127:
		var ext [8]byte
		if _, err := r.Read(ext[:]); err != nil {
			t.Fatalf("read extended length 64: %v", err)
		}
		length = int64(ext[0])<<56 | int64(ext[1])<<48 | int64(ext[2])<<40 | int64(ext[3])<<32 |
			int64(ext[4])<<24 | int64(ext[5])<<16 | int64(ext[6])<<8 | int64(ext[7])
	}

	var maskKey [4]byte
	if masked {
		if _, err := r.Read(maskKey[:]); err != nil {
			t.Fatalf("read mask key: %v", err)
		}
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := r.Read(payload); err != nil {
			t.Fatalf("read payload: %v", err)
		}
	}

	if masked {
		for i := int64(0); i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload
}
