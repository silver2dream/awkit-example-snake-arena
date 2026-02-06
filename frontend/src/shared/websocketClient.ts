type Listener<T> = (value: T) => void

export type ConnectionState = 'disconnected' | 'connecting' | 'connected'

export type OutgoingMessage =
  | { type: 'join_room'; roomId: string; playerId: string }
  | { type: 'input'; roomId: string; playerId: string; input: unknown }

export type IncomingMessage =
  | {
      type: 'room_snapshot'
      snapshot?: unknown
      roomId?: unknown
      tick?: unknown
      players?: unknown
      food?: unknown
      width?: unknown
      height?: unknown
    }
  | {
      type: 'tick_update'
      delta?: unknown
      roomId?: unknown
      tick?: unknown
      snakes?: unknown
      food?: unknown
      scores?: unknown
      gameOver?: unknown
      direction?: unknown
    }
  | { type: 'error'; message?: string; code?: string }

export interface RoomWebSocketClientOptions {
  endpoint: string
  roomId: string
  playerId: string
  initialBackoffMs?: number
  maxBackoffMs?: number
  maxReconnectAttempts?: number
  createSocket?: (url: string) => WebSocket
}

export interface RoomWebSocketClient {
  readonly roomId: string
  getState(): ConnectionState
  connect(): void
  disconnect(): void
  sendJoinRoom(): boolean
  sendInput(input: unknown): boolean
  subscribeState(listener: Listener<ConnectionState>): () => void
  subscribeMessages(listener: Listener<IncomingMessage>): () => void
  subscribeErrors(listener: Listener<Error>): () => void
}

export function createRoomWebSocketClient(options: RoomWebSocketClientOptions): RoomWebSocketClient {
  const initialBackoff = options.initialBackoffMs ?? 1000
  const maxBackoff = options.maxBackoffMs ?? 30000
  const maxAttempts = options.maxReconnectAttempts ?? 5
  const socketFactory = options.createSocket ?? ((url: string) => new WebSocket(url))

  let socket: WebSocket | null = null
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let shouldReconnect = false
  let state: ConnectionState = 'disconnected'

  const stateListeners = new Set<Listener<ConnectionState>>()
  const messageListeners = new Set<Listener<IncomingMessage>>()
  const errorListeners = new Set<Listener<Error>>()

  const emitState = (next: ConnectionState) => {
    state = next
    stateListeners.forEach((listener) => listener(state))
  }

  const emitError = (err: Error) => {
    errorListeners.forEach((listener) => listener(err))
  }

  const emitMessage = (msg: IncomingMessage) => {
    messageListeners.forEach((listener) => listener(msg))
  }

  const clearTimer = () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  const closeSocket = () => {
    if (socket) {
      socket.onclose = null
      socket.onerror = null
      socket.onopen = null
      socket.onmessage = null
      socket.close()
    }
    socket = null
  }

  const scheduleReconnect = () => {
    if (!shouldReconnect) return
    if (reconnectAttempts >= maxAttempts) {
      shouldReconnect = false
      emitState('disconnected')
      emitError(new Error('Reached maximum reconnect attempts.'))
      return
    }

    const delay = Math.min(maxBackoff, initialBackoff * 2 ** reconnectAttempts)
    reconnectAttempts += 1
    emitState('connecting')

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      openSocket()
    }, delay)
  }

  const handleMessage = (event: MessageEvent) => {
    try {
      const parsed = typeof event.data === 'string' ? JSON.parse(event.data) : event.data
      if (!parsed || typeof parsed.type !== 'string') {
        emitError(new Error('Received message without a type.'))
        return
      }

      switch (parsed.type) {
        case 'room_snapshot':
        case 'tick_update':
          emitMessage(parsed as IncomingMessage)
          break
        case 'error': {
          const details = parsed as IncomingMessage & { message?: string; code?: string }
          const summary = details.message ?? 'Server reported an error.'
          const decorated = details.code ? `${summary} (code: ${details.code})` : summary
          emitMessage(details)
          emitError(new Error(decorated))
          break
        }
        default:
          emitError(new Error(`Unknown message type: ${parsed.type}`))
      }
    } catch (err) {
      const detail = err instanceof Error ? err.message : 'Failed to parse server message.'
      emitError(new Error(`Failed to parse server message: ${detail}`))
    }
  }

  const sendMessage = (payload: OutgoingMessage): boolean => {
    const openState = typeof WebSocket !== 'undefined' ? WebSocket.OPEN : 1
    if (!socket || socket.readyState !== openState) {
      return false
    }
    socket.send(JSON.stringify(payload))
    return true
  }

  const openSocket = () => {
    clearTimer()

    try {
      socket = socketFactory(options.endpoint)
    } catch (err) {
      emitError(err instanceof Error ? err : new Error('Unable to open WebSocket connection.'))
      emitState('disconnected')
      scheduleReconnect()
      return
    }

    emitState('connecting')

    socket.onopen = () => {
      reconnectAttempts = 0
      emitState('connected')
      sendMessage({ type: 'join_room', roomId: options.roomId, playerId: options.playerId })
    }

    socket.onmessage = handleMessage

    socket.onerror = () => {
      emitError(new Error('WebSocket encountered an error.'))
    }

    socket.onclose = () => {
      socket = null
      emitState('disconnected')
      if (shouldReconnect) {
        scheduleReconnect()
      }
    }
  }

  return {
    roomId: options.roomId,
    getState: () => state,
    connect: () => {
      shouldReconnect = true
      reconnectAttempts = 0
      closeSocket()
      openSocket()
    },
    disconnect: () => {
      shouldReconnect = false
      clearTimer()
      closeSocket()
      emitState('disconnected')
    },
    sendJoinRoom: () => sendMessage({ type: 'join_room', roomId: options.roomId, playerId: options.playerId }),
    sendInput: (input: unknown) =>
      sendMessage({ type: 'input', roomId: options.roomId, playerId: options.playerId, input }),
    subscribeState: (listener: Listener<ConnectionState>) => {
      stateListeners.add(listener)
      listener(state)
      return () => stateListeners.delete(listener)
    },
    subscribeMessages: (listener: Listener<IncomingMessage>) => {
      messageListeners.add(listener)
      return () => messageListeners.delete(listener)
    },
    subscribeErrors: (listener: Listener<Error>) => {
      errorListeners.add(listener)
      return () => errorListeners.delete(listener)
    },
  }
}
