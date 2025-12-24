package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const (
	OpText  byte = 0x1
	OpClose byte = 0x8
	OpPing  byte = 0x9
	OpPong  byte = 0xA
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Conn is a minimal WebSocket connection that supports text frames,
// ping/pong, and close frames. It intentionally avoids third-party
// dependencies to keep the build offline-friendly.
type Conn struct {
	conn   net.Conn
	rw     *bufio.ReadWriter
	writeM sync.Mutex
}

// Upgrade upgrades an HTTP request to a WebSocket connection.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		return nil, errors.New("missing websocket upgrade header")
	}

	if !headerContainsToken(r.Header.Values("Connection"), "upgrade") {
		return nil, errors.New("missing connection upgrade token")
	}

	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("unsupported websocket version")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing websocket key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("hijacking not supported")
	}

	netConn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	acceptKey := computeAcceptKey(key)
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptKey)

	if _, err := rw.WriteString(response); err != nil {
		netConn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		netConn.Close()
		return nil, err
	}

	return &Conn{conn: netConn, rw: rw}, nil
}

// WriteText writes a text frame to the connection.
func (c *Conn) WriteText(payload []byte) error {
	return c.writeFrame(OpText, payload)
}

// WritePong writes a pong frame to respond to a ping.
func (c *Conn) WritePong(payload []byte) error {
	return c.writeFrame(OpPong, payload)
}

// WriteClose writes a close frame and closes the connection.
func (c *Conn) WriteClose() error {
	_ = c.writeFrame(OpClose, nil)
	return c.conn.Close()
}

// Read reads the next WebSocket frame opcode and payload.
func (c *Conn) Read() (byte, []byte, error) {
	return readFrame(c.rw.Reader)
}

// Close closes the underlying network connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.writeM.Lock()
	defer c.writeM.Unlock()

	if err := writeFrame(c.rw.Writer, opcode, payload); err != nil {
		return err
	}
	return c.rw.Flush()
}

func computeAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func headerContainsToken(values []string, token string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func writeFrame(w io.Writer, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	length := len(payload)
	length64 := uint64(length)

	switch {
	case length <= 125:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		header = append(header, 127,
			byte(length64>>56), byte(length64>>48), byte(length64>>40), byte(length64>>32),
			byte(length64>>24), byte(length64>>16), byte(length64>>8), byte(length64))
	}

	if _, err := w.Write(header); err != nil {
		return err
	}
	if length > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}

	return nil
}

func readFrame(r *bufio.Reader) (byte, []byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	opcode := first & 0x0F
	masked := second&0x80 != 0
	length := int64(second & 0x7F)

	switch length {
	case 126:
		var extended [2]byte
		if _, err := io.ReadFull(r, extended[:]); err != nil {
			return 0, nil, err
		}
		length = int64(extended[0])<<8 | int64(extended[1])
	case 127:
		var extended [8]byte
		if _, err := io.ReadFull(r, extended[:]); err != nil {
			return 0, nil, err
		}
		length = int64(extended[0])<<56 | int64(extended[1])<<48 | int64(extended[2])<<40 | int64(extended[3])<<32 |
			int64(extended[4])<<24 | int64(extended[5])<<16 | int64(extended[6])<<8 | int64(extended[7])
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}

	if masked {
		for i := int64(0); i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}
