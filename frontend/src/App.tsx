import { useEffect, useRef, useState } from 'react'
import GameCanvas from './features/game/GameCanvas'
import { applyIncomingMessage } from './features/game/snapshot'
import type { GameSnapshot } from './features/game/types'
import { Lobby } from './features/lobby'
import { WebSocketClient, type ConnectionState, type Direction } from './shared/websocketClient'

type AppPhase = 'lobby' | 'connecting' | 'game' | 'game_over'

interface ActiveSession {
  roomId: string
  playerName: string
}

const INPUT_THROTTLE_MS = 85

const keyToDirection: Record<string, Direction> = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
}

const connectionStateColor: Record<ConnectionState, string> = {
  connecting: '#ca8a04',
  connected: '#15803d',
  disconnected: '#b91c1c',
}

function App() {
  const [phase, setPhase] = useState<AppPhase>('lobby')
  const [activeSession, setActiveSession] = useState<ActiveSession | null>(null)
  const [snapshot, setSnapshot] = useState<GameSnapshot | null>(null)
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected')
  const [appError, setAppError] = useState<string | null>(null)

  const clientRef = useRef<WebSocketClient | null>(null)
  const inputSequenceRef = useRef(0)
  const lastInputAtRef = useRef(0)
  const lastDirectionRef = useRef<Direction | null>(null)

  useEffect(() => {
    if (!activeSession) {
      return
    }

    let disposed = false

    const client = new WebSocketClient({
      roomId: activeSession.roomId,
      onConnectionStateChange: (nextConnectionState) => {
        if (disposed) {
          return
        }

        setConnectionState(nextConnectionState)
      },
      onMessage: (message) => {
        if (disposed) {
          return
        }

        if (message.type === 'error') {
          setAppError(message.message)
          return
        }

        setAppError(null)
        setSnapshot((previous) => {
          const next = applyIncomingMessage(previous, message)
          if (next) {
            setPhase(next.gameOver ? 'game_over' : 'game')
          }
          return next
        })
      },
    })

    clientRef.current = client
    inputSequenceRef.current = 0
    lastInputAtRef.current = 0
    lastDirectionRef.current = null

    setSnapshot(null)
    setAppError(null)
    setConnectionState('connecting')
    setPhase('connecting')

    client.connect()
    client.joinRoom(activeSession.playerName)

    return () => {
      disposed = true
      client.disconnect()
      if (clientRef.current === client) {
        clientRef.current = null
      }
    }
  }, [activeSession])

  useEffect(() => {
    if (phase !== 'game' || connectionState !== 'connected') {
      return
    }

    const onKeyDown = (event: KeyboardEvent) => {
      const direction = keyToDirection[event.key]
      if (!direction) {
        return
      }

      event.preventDefault()

      const now = Date.now()
      if (now - lastInputAtRef.current < INPUT_THROTTLE_MS) {
        return
      }

      if (lastDirectionRef.current === direction) {
        return
      }

      const nextSequence = inputSequenceRef.current + 1
      const didSend = clientRef.current?.sendInput(direction, nextSequence) ?? false
      if (!didSend) {
        return
      }

      inputSequenceRef.current = nextSequence
      lastInputAtRef.current = now
      lastDirectionRef.current = direction
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [phase, connectionState])

  const leaveRoom = () => {
    clientRef.current?.disconnect()
    clientRef.current = null

    setActiveSession(null)
    setSnapshot(null)
    setConnectionState('disconnected')
    setAppError(null)
    setPhase('lobby')
  }

  const handleJoinReady = (roomId: string, playerName: string) => {
    setAppError(null)
    setActiveSession({
      roomId,
      playerName,
    })
  }

  return (
    <main
      style={{
        maxWidth: 900,
        margin: '0 auto',
        padding: '1rem',
        display: 'grid',
        gap: '1rem',
      }}
    >
      {phase === 'lobby' ? <Lobby connectionState={connectionState} onJoinReady={handleJoinReady} /> : null}

      {phase !== 'lobby' ? (
        <>
          <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
            <div>
              <h1 style={{ margin: 0 }}>Snake Arena</h1>
              <div style={{ color: '#475569' }}>
                Room <strong>{activeSession?.roomId}</strong> as <strong>{activeSession?.playerName}</strong>
              </div>
            </div>

            <div style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem', fontWeight: 600 }}>
              <span
                aria-hidden
                style={{
                  display: 'inline-block',
                  width: 10,
                  height: 10,
                  borderRadius: '50%',
                  background: connectionStateColor[connectionState],
                }}
              />
              <span>Connection: {connectionState}</span>
            </div>
          </header>

          {phase === 'connecting' && !snapshot ? <div>Connecting to room…</div> : null}

          {snapshot ? <GameCanvas snapshot={snapshot} /> : null}

          {phase === 'game' ? <div>Use arrow keys to move your snake.</div> : null}

          {phase === 'game_over' ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', flexWrap: 'wrap' }}>
              <strong style={{ color: '#b91c1c' }}>Game over</strong>
              <button type="button" onClick={leaveRoom}>
                Back to lobby
              </button>
            </div>
          ) : null}

          {phase !== 'game_over' ? (
            <button type="button" onClick={leaveRoom} style={{ justifySelf: 'start' }}>
              Leave room
            </button>
          ) : null}
        </>
      ) : null}

      {appError ? (
        <div role="alert" style={{ color: '#b91c1c', fontWeight: 600 }}>
          {appError}
        </div>
      ) : null}
    </main>
  )
}

export default App
