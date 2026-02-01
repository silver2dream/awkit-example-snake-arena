import { describe, expect, it, vi } from 'vitest'
import type { ConnectionState } from './websocketClient'
import { createRoomWebSocketClient } from './websocketClient'

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  sent: string[] = []

  constructor(public url: string) {}

  open() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.({} as Event)
  }

  emitMessage(data: unknown) {
    this.onmessage?.({ data } as MessageEvent)
  }

  failClose(code = 1006) {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code, wasClean: false } as CloseEvent)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code: 1000, wasClean: true } as CloseEvent)
  }

  send(payload: string) {
    this.sent.push(payload)
  }
}

// Ensure the global constant exists for the module under test.
globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket

describe('createRoomWebSocketClient', () => {
  it('emits connection state changes', () => {
    const states: ConnectionState[] = []
    const socket = new MockWebSocket('ws://example.test')
    const client = createRoomWebSocketClient({
      endpoint: 'ws://example.test',
      roomId: 'room-1',
      playerId: 'player-1',
      createSocket: () => socket as unknown as WebSocket,
    })

    client.subscribeState((next) => states.push(next))

    client.connect()
    socket.open()
    client.disconnect()

    expect(states).toEqual(['disconnected', 'connecting', 'connected', 'disconnected'])
    expect(socket.sent[0]).toBeDefined() // join_room is sent on connect
  })

  it('reconnects with exponential backoff until max attempts', () => {
    vi.useFakeTimers()
    const sockets: MockWebSocket[] = []
    const errors: string[] = []
    const client = createRoomWebSocketClient({
      endpoint: 'ws://example.test',
      roomId: 'room-2',
      playerId: 'player-2',
      initialBackoffMs: 1000,
      maxBackoffMs: 8000,
      maxReconnectAttempts: 2,
      createSocket: () => {
        const sock = new MockWebSocket('ws://example.test')
        sockets.push(sock)
        return sock as unknown as WebSocket
      },
    })

    client.subscribeErrors((err) => errors.push(err.message))
    client.connect()

    // First attempt fails before opening.
    sockets[0].failClose()
    expect(sockets).toHaveLength(1)

    vi.advanceTimersByTime(1000)
    expect(sockets).toHaveLength(2)

    sockets[1].failClose()
    vi.advanceTimersByTime(2000)
    expect(sockets).toHaveLength(3)

    // Third failure hits the max and stops.
    sockets[2].failClose()
    expect(errors[0]).toMatch(/maximum reconnect attempts/i)
    vi.useRealTimers()
  })

  it('serializes outgoing messages and parses incoming payloads', () => {
    const socket = new MockWebSocket('ws://example.test')
    const messages: string[] = []
    const incoming: string[] = []

    const client = createRoomWebSocketClient({
      endpoint: 'ws://example.test',
      roomId: 'room-3',
      playerId: 'player-3',
      createSocket: () => socket as unknown as WebSocket,
    })

    client.subscribeMessages((msg) => incoming.push(msg.type))

    client.connect()
    socket.open()

    // join_room should be the first send
    messages.push(...socket.sent)

    client.sendInput({ direction: 'up' })
    messages.push(...socket.sent.slice(messages.length))

    socket.emitMessage('{"type":"room_snapshot","snapshot":{"players":1}}')

    expect(messages.map((payload) => JSON.parse(payload).type)).toEqual(['join_room', 'input'])
    expect(incoming).toEqual(['room_snapshot'])
  })

  it('routes server error messages to error subscribers', () => {
    const socket = new MockWebSocket('ws://example.test')
    const errors: string[] = []
    const types: string[] = []

    const client = createRoomWebSocketClient({
      endpoint: 'ws://example.test',
      roomId: 'room-5',
      playerId: 'player-5',
      createSocket: () => socket as unknown as WebSocket,
    })

    client.subscribeMessages((msg) => types.push(msg.type))
    client.subscribeErrors((err) => errors.push(err.message))

    client.connect()
    socket.open()

    socket.emitMessage('{"type":"error","message":"room full","code":"room_full"}')

    expect(types).toEqual(['error'])
    expect(errors[0]).toMatch(/room full/i)
    expect(errors[0]).toMatch(/room_full/i)
  })

  it('surfaces JSON parsing errors to subscribers', () => {
    const socket = new MockWebSocket('ws://example.test')
    const errors: string[] = []
    const client = createRoomWebSocketClient({
      endpoint: 'ws://example.test',
      roomId: 'room-4',
      playerId: 'player-4',
      createSocket: () => socket as unknown as WebSocket,
    })

    client.subscribeErrors((err) => errors.push(err.message))
    client.connect()
    socket.open()

    socket.emitMessage('not-json')
    expect(errors.some((msg) => msg.includes('parse'))).toBe(true)
  })
})
