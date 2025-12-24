package game

import (
	"sync"

	"awkit-example-snake-arena-backend/internal/ws"
)

type client struct {
	room *Room
	conn *ws.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

func newClient(room *Room, conn *ws.Conn) *client {
	return &client{
		room: room,
		conn: conn,
		send: make(chan []byte, 16),
		done: make(chan struct{}),
	}
}

func (c *client) start() {
	go c.readLoop()
	go c.writeLoop()
}

func (c *client) readLoop() {
	for {
		opcode, payload, err := c.conn.Read()
		if err != nil {
			break
		}
		switch opcode {
		case ws.OpClose:
			_ = c.conn.WriteClose()
			return
		case ws.OpPing:
			_ = c.conn.WritePong(payload)
		}
	}

	c.room.removeClient(c)
}

func (c *client) writeLoop() {
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteText(msg); err != nil {
				c.room.removeClient(c)
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *client) close() {
	c.once.Do(func() {
		close(c.done)
		close(c.send)
		_ = c.conn.Close()
	})
}
