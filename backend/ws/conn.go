package ws

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Conn is a minimal WebSocket connection supporting JSON text messages.
type Conn struct {
	conn     net.Conn
	rw       *bufio.ReadWriter
	isClient bool
	writeMu  sync.Mutex
}

// NewClientConn wraps an already-established network connection for websocket
// client frames. The caller is responsible for performing the HTTP upgrade.
func NewClientConn(conn net.Conn) *Conn {
	return &Conn{
		conn:     conn,
		rw:       bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		isClient: true,
	}
}

// NewClientConnWithRW wraps an already-established connection using caller-managed buffers.
func NewClientConnWithRW(conn net.Conn, rw *bufio.ReadWriter) *Conn {
	return &Conn{
		conn:     conn,
		rw:       rw,
		isClient: true,
	}
}

// Upgrade upgrades an incoming HTTP request to a WebSocket connection.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !headerContainsToken(r.Header, "Connection", "Upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("not a websocket upgrade request")
	}

	secKey := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if secKey == "" {
		return nil, errors.New("missing sec-websocket-key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptForKey(secKey) + "\r\n\r\n"

	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &Conn{
		conn:     conn,
		rw:       rw,
		isClient: false,
	}, nil
}

// Dial opens a client websocket connection for ws:// URLs.
func Dial(rawURL string) (*Conn, *http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	if u.Scheme != "ws" {
		return nil, nil, fmt.Errorf("unsupported websocket scheme: %s", u.Scheme)
	}

	address := u.Host
	if !strings.Contains(address, ":") {
		address += ":80"
	}

	netConn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, nil, err
	}

	reader := bufio.NewReader(netConn)
	writer := bufio.NewWriter(netConn)
	rw := bufio.NewReadWriter(reader, writer)

	secKey, err := randomSecKey()
	if err != nil {
		_ = netConn.Close()
		return nil, nil, err
	}

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, u.Host, secKey)

	if _, err := rw.WriteString(request); err != nil {
		_ = netConn.Close()
		return nil, nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = netConn.Close()
		return nil, nil, err
	}

	httpReq, err := http.NewRequest(http.MethodGet, "http://"+u.Host+path, nil)
	if err != nil {
		_ = netConn.Close()
		return nil, nil, err
	}

	resp, err := http.ReadResponse(reader, httpReq)
	if err != nil {
		_ = netConn.Close()
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = netConn.Close()
		return nil, resp, fmt.Errorf("unexpected websocket status: %d", resp.StatusCode)
	}
	if !strings.EqualFold(strings.TrimSpace(resp.Header.Get("Sec-WebSocket-Accept")), acceptForKey(secKey)) {
		_ = netConn.Close()
		return nil, resp, errors.New("invalid sec-websocket-accept")
	}

	return &Conn{
		conn:     netConn,
		rw:       rw,
		isClient: true,
	}, resp, nil
}

// ReadJSON reads one text websocket message and unmarshals it from JSON.
func (c *Conn) ReadJSON(target any) error {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return err
		}

		switch opcode {
		case 0x1: // text
			return json.Unmarshal(payload, target)
		case 0x8: // close
			return io.EOF
		case 0x9: // ping
			_ = c.writeControl(0xA, payload)
			continue
		case 0xA: // pong
			continue
		default:
			return fmt.Errorf("unsupported websocket opcode: %d", opcode)
		}
	}
}

// WriteJSON serializes and writes a text websocket message.
func (c *Conn) WriteJSON(message any) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return c.writeFrame(0x1, payload)
}

// Close closes the underlying network connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// SetReadDeadline forwards to the underlying connection.
func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline forwards to the underlying connection.
func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

func (c *Conn) readFrame() (byte, []byte, error) {
	first, err := c.rw.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := c.rw.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	fin := (first & 0x80) != 0
	opcode := first & 0x0F
	if !fin {
		return 0, nil, errors.New("fragmented frames are not supported")
	}

	masked := (second & 0x80) != 0
	payloadLen, err := c.readPayloadLength(second & 0x7F)
	if err != nil {
		return 0, nil, err
	}

	var maskingKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.rw, maskingKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.rw, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskingKey[i%4]
		}
	}
	return opcode, payload, nil
}

func (c *Conn) readPayloadLength(shortLen byte) (int, error) {
	switch shortLen {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(c.rw, buf[:]); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint16(buf[:])), nil
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(c.rw, buf[:]); err != nil {
			return 0, err
		}
		length := binary.BigEndian.Uint64(buf[:])
		if length > uint64(^uint(0)>>1) {
			return 0, errors.New("payload too large")
		}
		return int(length), nil
	default:
		return int(shortLen), nil
	}
}

func (c *Conn) writeControl(opcode byte, payload []byte) error {
	return c.writeFrame(opcode, payload)
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|(opcode&0x0F))

	maskBit := byte(0)
	if c.isClient {
		maskBit = 0x80
	}

	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		frame = append(frame, maskBit|byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, maskBit|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(payloadLen))
		frame = append(frame, ext[:]...)
	default:
		frame = append(frame, maskBit|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(payloadLen))
		frame = append(frame, ext[:]...)
	}

	if c.isClient {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		frame = append(frame, mask[:]...)

		maskedPayload := make([]byte, len(payload))
		copy(maskedPayload, payload)
		for i := range maskedPayload {
			maskedPayload[i] ^= mask[i%4]
		}
		frame = append(frame, maskedPayload...)
	} else {
		frame = append(frame, payload...)
	}

	if _, err := c.rw.Write(frame); err != nil {
		return err
	}
	return c.rw.Flush()
}

func randomSecKey() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(random[:]), nil
}

func acceptForKey(secKey string) string {
	sum := sha1.Sum([]byte(secKey + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContainsToken(headers http.Header, key, token string) bool {
	values := headers.Values(key)
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}
