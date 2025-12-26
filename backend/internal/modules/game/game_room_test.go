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

	conn, reader := openWebSocket(t, manager, "/ws/rooms/ROOM01")
	defer conn.Close()

	first := readEnvelope(t, conn, reader)
	if first.Type != "snapshot" {
		t.Fatalf("expected first message to be snapshot, got %s", first.Type)
	}

	tick := waitForType(t, conn, reader, "tick")
	var payload TickPayload
	decodeData(t, tick.Data, &payload)

	if payload.RoomID != "ROOM01" {
		t.Fatalf("expected room id ROOM01, got %s", payload.RoomID)
	}

	if payload.State == nil || len(payload.State.Snakes) == 0 {
		t.Fatalf("expected a populated state snapshot, got %+v", payload.State)
	}

	if got := manager.RoomClientCount("ROOM01"); got != 1 {
		t.Fatalf("expected 1 client registered, got %d", got)
	}

	_ = conn.Close()
	time.Sleep(50 * time.Millisecond)

	if got := manager.RoomClientCount("ROOM01"); got != 0 {
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

	connA, readerA := openWebSocket(t, manager, "/ws/rooms/SHARED")
	defer connA.Close()
	connB, readerB := openWebSocket(t, manager, "/ws/rooms/SHARED")
	defer connB.Close()

	_ = readEnvelope(t, connA, readerA) // snapshot
	_ = readEnvelope(t, connB, readerB) // snapshot

	tickA := waitForType(t, connA, readerA, "tick")
	tickB := waitForType(t, connB, readerB, "tick")

	var payloadA, payloadB TickPayload
	decodeData(t, tickA.Data, &payloadA)
	decodeData(t, tickB.Data, &payloadB)

	if payloadA.RoomID != "SHARED" || payloadB.RoomID != "SHARED" {
		t.Fatalf("expected room id SHARED, got %s and %s", payloadA.RoomID, payloadB.RoomID)
	}

	if len(payloadA.State.Snakes) == 0 || len(payloadB.State.Snakes) == 0 {
		t.Fatalf("expected state to include snakes")
	}
}

func TestInvalidRoomIDIsRejected(t *testing.T) {
	manager := NewRoomManager(RoomConfig{})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws/rooms/short", nil)
	rr := httptest.NewRecorder()

	manager.ServeHTTP(rr, req)

	if rr.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid room id, got %d", rr.Result().StatusCode)
	}
}

func TestRoomCapacityLimit(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		MaxClients:   1,
		TickInterval: time.Hour,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/FULL01")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws/rooms/FULL01", nil)
	rr := httptest.NewRecorder()
	manager.ServeHTTP(rr, req)

	if rr.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 when room is full, got %d", rr.Result().StatusCode)
	}
}

func TestInvalidInputYieldsError(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval: time.Hour,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/INPUT1")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	writeClientTextFrame(t, conn, []byte("not-json"))

	env := readEnvelope(t, conn, reader)
	if env.Type != "error" {
		t.Fatalf("expected error envelope, got %s", env.Type)
	}

	var payload map[string]string
	decodeData(t, env.Data, &payload)

	if payload["code"] != "bad_request" {
		t.Fatalf("expected bad_request code, got %+v", payload)
	}
}

func TestRateLimitingPreventsFlood(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval:  time.Hour,
		MaxMessages:   2,
		MessageWindow: 100 * time.Millisecond,
		InputThrottle: time.Nanosecond,
		MaxClients:    2,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/RATE01")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	payload := []byte(`{"type":"input","data":{"direction":"up"}}`)
	writeClientTextFrame(t, conn, payload)
	writeClientTextFrame(t, conn, payload)
	writeClientTextFrame(t, conn, payload)

	foundRateLimit := false
	for i := 0; i < 2; i++ {
		env := readEnvelope(t, conn, reader)
		if env.Type != "error" {
			t.Fatalf("expected error envelope, got %s", env.Type)
		}

		var payloadData map[string]string
		decodeData(t, env.Data, &payloadData)

		if payloadData["code"] == "rate_limit" {
			foundRateLimit = true
			break
		}
	}

	if !foundRateLimit {
		t.Fatalf("expected to receive a rate_limit error after flooding")
	}
}

func TestInputThrottleBlocksRapidCommands(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval:  time.Hour,
		InputThrottle: 50 * time.Millisecond,
		MaxMessages:   5,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/THROT1")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	payload := []byte(`{"type":"input","data":{"direction":"left"}}`)
	writeClientTextFrame(t, conn, payload)
	writeClientTextFrame(t, conn, payload)
	writeClientTextFrame(t, conn, payload)

	env := readEnvelope(t, conn, reader)
	if env.Type != "error" {
		t.Fatalf("expected error envelope, got %s", env.Type)
	}

	var payloadData map[string]string
	decodeData(t, env.Data, &payloadData)

	if payloadData["code"] != "throttled" {
		t.Fatalf("expected throttled code, got %+v", payloadData)
	}
}

func TestRoomLimitPreventsNewRooms(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval: time.Hour,
		MaxRooms:     1,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/LIMIT1")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws/rooms/LIMIT2", nil)
	rr := httptest.NewRecorder()

	manager.ServeHTTP(rr, req)

	if rr.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 when room limit exceeded, got %d", rr.Result().StatusCode)
	}
}

func TestInputPayloadTooLargeIsRejected(t *testing.T) {
	manager := NewRoomManager(RoomConfig{
		TickInterval:    time.Hour,
		MaxMessageBytes: 32,
	})
	defer manager.Shutdown()

	conn, reader := openWebSocket(t, manager, "/ws/rooms/SIZE01")
	defer conn.Close()
	_ = readEnvelope(t, conn, reader)

	oversized := `{"type":"input","data":{"direction":"up","padding":"` + strings.Repeat("x", 64) + `"}}`
	writeClientTextFrame(t, conn, []byte(oversized))

	env := readEnvelope(t, conn, reader)
	if env.Type != "error" {
		t.Fatalf("expected error envelope for oversized payload, got %s", env.Type)
	}

	var payloadData map[string]string
	decodeData(t, env.Data, &payloadData)

	if payloadData["code"] != "bad_request" {
		t.Fatalf("expected bad_request code, got %+v", payloadData)
	}

	if !strings.Contains(payloadData["message"], "payload too large") {
		t.Fatalf("expected payload too large message, got %+v", payloadData)
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

func writeClientTextFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()

	maskKey := [4]byte{0x11, 0x22, 0x33, 0x44}
	header := []byte{0x81}
	length := len(payload)
	length64 := uint64(length)

	switch {
	case length <= 125:
		header = append(header, byte(0x80|byte(length)))
	case length <= 65535:
		header = append(header, 0x80|126, byte(length>>8), byte(length))
	default:
		header = append(header, 0x80|127,
			byte(length64>>56), byte(length64>>48), byte(length64>>40), byte(length64>>32),
			byte(length64>>24), byte(length64>>16), byte(length64>>8), byte(length64))
	}

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}

	frame := append(header, maskKey[:]...)
	frame = append(frame, masked...)

	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	_ = conn.SetWriteDeadline(time.Time{})
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
