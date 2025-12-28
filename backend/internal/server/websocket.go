package server

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var (
	errFragmentedFrame = errors.New("fragmented frames are not supported")
	errUnmaskedClient  = errors.New("client frames must be masked")
)

type wsConn struct {
	conn    net.Conn
	rw      *bufio.ReadWriter
	writeMu sync.Mutex
}

func upgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, fmt.Errorf("missing connection upgrade header")
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("missing websocket upgrade")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("websocket upgrade not supported")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeAcceptKey(key)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if _, err := rw.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	return &wsConn{
		conn: conn,
		rw:   rw,
	}, nil
}

func computeAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (c *wsConn) ReadFrame() (byte, []byte, error) {
	header1, err := c.rw.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	if header1&0x80 == 0 {
		return 0, nil, errFragmentedFrame
	}

	opcode := header1 & 0x0F

	header2, err := c.rw.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	masked := header2&0x80 != 0
	if !masked {
		return 0, nil, errUnmaskedClient
	}

	payloadLen := int64(header2 & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.rw, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.rw, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	maskKey := make([]byte, 4)
	if _, err := io.ReadFull(c.rw, maskKey); err != nil {
		return 0, nil, err
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.rw, payload); err != nil {
		return 0, nil, err
	}

	for i := int64(0); i < payloadLen; i++ {
		payload[i] ^= maskKey[i%4]
	}

	return opcode, payload, nil
}

func (c *wsConn) WriteFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	header := []byte{0x80 | opcode}
	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		header = append(header, byte(payloadLen))
	case payloadLen < 65536:
		header = append(header, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(payloadLen))
		header = append(header, ext...)
	default:
		header = append(header, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(payloadLen))
		header = append(header, ext...)
	}

	if _, err := c.rw.Write(header); err != nil {
		return err
	}
	if payloadLen > 0 {
		if _, err := c.rw.Write(payload); err != nil {
			return err
		}
	}
	return c.rw.Flush()
}

func (c *wsConn) Close() error {
	return c.conn.Close()
}
