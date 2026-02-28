export type ConnectionState = 'connecting' | 'connected' | 'disconnected'

export type Direction = 'up' | 'down' | 'left' | 'right'

export interface JoinRoomMessage {
  type: 'join_room'
  roomId?: string
  playerName: string
}

export interface InputMessage {
  type: 'input'
  direction: Direction
  seq?: number
}

export type OutgoingMessage = JoinRoomMessage | InputMessage

export interface RoomSnapshotMessage {
  type: 'room_snapshot'
  roomId?: string
  snapshot: Record<string, unknown>
}

export interface TickUpdateMessage {
  type: 'tick_update'
  roomId?: string
  snapshot: Record<string, unknown>
}

export interface ErrorMessage {
  type: 'error'
  message: string
}

export type IncomingMessage = RoomSnapshotMessage | TickUpdateMessage | ErrorMessage

const asObject = (value: unknown): Record<string, unknown> | null =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null

const toString = (value: unknown): string | null => (typeof value === 'string' ? value : null)

const isIncomingSnapshotType = (value: unknown): value is RoomSnapshotMessage['type'] | TickUpdateMessage['type'] =>
  value === 'room_snapshot' || value === 'tick_update'

export function parseIncomingMessage(raw: unknown): IncomingMessage | null {
  const message = asObject(raw)
  if (!message) {
    return null
  }

  const messageType = toString(message.type)
  if (!messageType) {
    return null
  }

  if (messageType === 'error') {
    const errorMessage = toString(message.message)
    if (!errorMessage) {
      return null
    }

    return {
      type: 'error',
      message: errorMessage,
    }
  }

  if (!isIncomingSnapshotType(messageType)) {
    return null
  }

  const snapshot = asObject(message.snapshot)
  if (!snapshot) {
    return null
  }

  const roomId = toString(message.roomId) ?? undefined

  if (messageType === 'room_snapshot') {
    return {
      type: 'room_snapshot',
      roomId,
      snapshot,
    }
  }

  return {
    type: 'tick_update',
    roomId,
    snapshot,
  }
}

interface WebSocketClientOptions {
  roomId: string
  onMessage: (message: IncomingMessage) => void
  onConnectionStateChange?: (state: ConnectionState) => void
  webSocketFactory?: (url: string) => WebSocket
  reconnectBaseDelayMs?: number
  reconnectMaxDelayMs?: number
}

const defaultWebSocketFactory = (url: string): WebSocket => new WebSocket(url)

const toWebSocketURL = (roomId: string) => {
  const encodedRoomId = encodeURIComponent(roomId)

  if (typeof window === 'undefined' || !window.location) {
    return `ws://localhost/ws/room/${encodedRoomId}`
  }

  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${protocol}://${window.location.host}/ws/room/${encodedRoomId}`
}

export class WebSocketClient {
  private readonly roomId: string
  private readonly onMessage: (message: IncomingMessage) => void
  private readonly onConnectionStateChange?: (state: ConnectionState) => void
  private readonly webSocketFactory: (url: string) => WebSocket
  private readonly reconnectBaseDelayMs: number
  private readonly reconnectMaxDelayMs: number

  private socket: WebSocket | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  private shouldReconnect = false
  private connectionState: ConnectionState = 'disconnected'
  private joinMessage: JoinRoomMessage | null = null

  constructor(options: WebSocketClientOptions) {
    this.roomId = options.roomId
    this.onMessage = options.onMessage
    this.onConnectionStateChange = options.onConnectionStateChange
    this.webSocketFactory = options.webSocketFactory ?? defaultWebSocketFactory
    this.reconnectBaseDelayMs = options.reconnectBaseDelayMs ?? 250
    this.reconnectMaxDelayMs = options.reconnectMaxDelayMs ?? 5_000
  }

  getConnectionState(): ConnectionState {
    return this.connectionState
  }

  connect() {
    if (this.shouldReconnect && this.socket) {
      return
    }

    this.shouldReconnect = true
    this.clearReconnectTimer()
    this.openSocket()
  }

  disconnect() {
    this.shouldReconnect = false
    this.clearReconnectTimer()

    if (this.socket) {
      this.socket.close()
      this.socket = null
    }

    this.setConnectionState('disconnected')
  }

  joinRoom(playerName: string): boolean {
    const trimmedPlayerName = playerName.trim()
    if (!trimmedPlayerName) {
      return false
    }

    this.joinMessage = {
      type: 'join_room',
      roomId: this.roomId,
      playerName: trimmedPlayerName,
    }

    return this.send(this.joinMessage)
  }

  sendInput(direction: Direction, seq?: number): boolean {
    return this.send({
      type: 'input',
      direction,
      seq,
    })
  }

  send(message: OutgoingMessage): boolean {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return false
    }

    this.socket.send(JSON.stringify(message))
    return true
  }

  private openSocket() {
    if (!this.shouldReconnect) {
      return
    }

    this.setConnectionState('connecting')

    const socket = this.webSocketFactory(toWebSocketURL(this.roomId))
    this.socket = socket

    socket.onopen = () => {
      if (this.socket !== socket) {
        return
      }

      this.reconnectAttempts = 0
      this.clearReconnectTimer()
      this.setConnectionState('connected')

      if (this.joinMessage) {
        this.send(this.joinMessage)
      }
    }

    socket.onmessage = (event) => {
      if (this.socket !== socket) {
        return
      }

      let rawData: unknown
      if (typeof event.data === 'string') {
        try {
          rawData = JSON.parse(event.data)
        } catch {
          this.onMessage({
            type: 'error',
            message: 'invalid JSON message received from server',
          })
          return
        }
      } else {
        rawData = event.data
      }

      const parsed = parseIncomingMessage(rawData)
      if (!parsed) {
        this.onMessage({
          type: 'error',
          message: 'invalid message payload received from server',
        })
        return
      }

      this.onMessage(parsed)
    }

    socket.onerror = () => {
      if (this.socket !== socket) {
        return
      }

      this.onMessage({
        type: 'error',
        message: 'websocket connection error',
      })
    }

    socket.onclose = () => {
      if (this.socket !== socket) {
        return
      }

      this.socket = null
      this.setConnectionState('disconnected')

      if (this.shouldReconnect) {
        this.scheduleReconnect()
      }
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer || !this.shouldReconnect) {
      return
    }

    this.reconnectAttempts += 1
    const delay = Math.min(this.reconnectBaseDelayMs * 2 ** (this.reconnectAttempts - 1), this.reconnectMaxDelayMs)

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.openSocket()
    }, delay)
  }

  private clearReconnectTimer() {
    if (!this.reconnectTimer) {
      return
    }

    clearTimeout(this.reconnectTimer)
    this.reconnectTimer = null
  }

  private setConnectionState(state: ConnectionState) {
    if (this.connectionState === state) {
      return
    }

    this.connectionState = state
    this.onConnectionStateChange?.(state)
  }
}
