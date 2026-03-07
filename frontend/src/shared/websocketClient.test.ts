import { describe, expect, it, vi } from 'vitest'
import { WebSocketClient, buildWebSocketURL, parseIncomingMessage, type ConnectionState, type IncomingMessage } from './websocketClient'

class MockWebSocket {
  readonly url: string
  readyState = 0
  sent: string[] = []
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(url: string) {
    this.url = url
  }

  send(payload: string) {
    this.sent.push(payload)
  }

  close() {
    this.readyState = 3
    this.onclose?.({} as CloseEvent)
  }

  triggerOpen() {
    this.readyState = 1
    this.onopen?.({} as Event)
  }

  triggerMessage(data: unknown) {
    this.onmessage?.({ data } as MessageEvent)
  }

  triggerError() {
    this.onerror?.({} as Event)
  }

  triggerClose() {
    this.readyState = 3
    this.onclose?.({} as CloseEvent)
  }
}

describe('buildWebSocketURL', () => {
  it('builds ws://host/ws/room/{roomId} for http origins', () => {
    expect(buildWebSocketURL('room alpha', { protocol: 'http:', host: 'host' })).toBe('ws://host/ws/room/room%20alpha')
  })

  it('builds wss://host/ws/room/{roomId} for https origins', () => {
    expect(buildWebSocketURL('room-1', { protocol: 'https:', host: 'example.com' })).toBe('wss://example.com/ws/room/room-1')
  })
})

describe('parseIncomingMessage', () => {
  it('parses room snapshot messages with snapshot payload', () => {
    const parsed = parseIncomingMessage({
      type: 'room_snapshot',
      roomId: 'room-1',
      snapshot: {
        tick: 1,
      },
    })

    expect(parsed).toEqual({
      type: 'room_snapshot',
      roomId: 'room-1',
      snapshot: {
        tick: 1,
      },
    })
  })

  it('parses server error messages', () => {
    const parsed = parseIncomingMessage({
      type: 'error',
      message: 'room not found',
    })

    expect(parsed).toEqual({
      type: 'error',
      message: 'room not found',
    })
  })

  it('returns null for unknown message type', () => {
    const parsed = parseIncomingMessage({
      type: 'unknown',
      snapshot: {},
    })

    expect(parsed).toBeNull()
  })

  it('returns null when required snapshot payload is missing', () => {
    const parsed = parseIncomingMessage({
      type: 'tick_update',
      roomId: 'room-2',
    })

    expect(parsed).toBeNull()
  })
})

describe('WebSocketClient', () => {
  const createClientHarness = () => {
    const sockets: MockWebSocket[] = []
    const states: ConnectionState[] = []
    const messages: IncomingMessage[] = []

    const client = new WebSocketClient({
      roomId: 'room-1',
      onMessage: (message) => messages.push(message),
      onConnectionStateChange: (state) => states.push(state),
      webSocketFactory: (url) => {
        const socket = new MockWebSocket(url)
        sockets.push(socket)
        return socket as unknown as WebSocket
      },
      reconnectBaseDelayMs: 100,
      reconnectMaxDelayMs: 200,
    })

    return { client, sockets, states, messages }
  }

  it('opens ws://localhost/ws/room/{roomId} and parses incoming messages', () => {
    const harness = createClientHarness()
    harness.client.connect()

    expect(harness.sockets).toHaveLength(1)
    expect(harness.sockets[0].url).toBe('ws://localhost/ws/room/room-1')
    expect(harness.states).toEqual(['connecting'])

    harness.sockets[0].triggerOpen()
    expect(harness.states).toEqual(['connecting', 'connected'])

    harness.sockets[0].triggerMessage(JSON.stringify({ type: 'tick_update', snapshot: { tick: 7 } }))
    expect(harness.messages).toEqual([
      {
        type: 'tick_update',
        roomId: undefined,
        snapshot: { tick: 7 },
      },
    ])
  })

  it('reports invalid websocket payloads as error messages', () => {
    const harness = createClientHarness()
    harness.client.connect()
    harness.sockets[0].triggerMessage('{broken json')
    harness.sockets[0].triggerMessage(JSON.stringify({ type: 'tick_update' }))

    expect(harness.messages).toEqual([
      {
        type: 'error',
        message: 'invalid JSON message received from server',
      },
      {
        type: 'error',
        message: 'invalid message payload received from server',
      },
    ])
  })

  it('reconnects with exponential backoff after close', () => {
    vi.useFakeTimers()
    try {
      const harness = createClientHarness()
      harness.client.connect()

      harness.sockets[0].triggerClose()
      expect(harness.states).toEqual(['connecting', 'disconnected'])

      vi.advanceTimersByTime(99)
      expect(harness.sockets).toHaveLength(1)

      vi.advanceTimersByTime(1)
      expect(harness.sockets).toHaveLength(2)

      harness.sockets[1].triggerClose()
      vi.advanceTimersByTime(199)
      expect(harness.sockets).toHaveLength(2)

      vi.advanceTimersByTime(1)
      expect(harness.sockets).toHaveLength(3)
    } finally {
      vi.useRealTimers()
    }
  })
})
