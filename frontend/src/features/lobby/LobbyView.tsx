import { useMemo, useState } from 'react'
import CreateRoomForm from './CreateRoomForm'
import JoinRoomForm from './JoinRoomForm'
import {
  ConnectionStatus,
  isValidRoomId,
  mockCreateRoom,
  mockJoinRoom,
  normalizeRoomId,
} from './lobbyApi'

type LobbyMode = 'create' | 'join'

const LobbyView = () => {
  const [mode, setMode] = useState<LobbyMode>('create')
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected')
  const [activeRoomId, setActiveRoomId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [statusMessage, setStatusMessage] = useState<string | null>(null)
  const [isBusy, setIsBusy] = useState(false)

  const connectionLabel = useMemo(() => {
    if (connectionStatus === 'connecting') return 'Connecting...'
    if (connectionStatus === 'connected') return 'Connected'
    return 'Disconnected'
  }, [connectionStatus])

  const setConnected = (roomId: string, message: string) => {
    setActiveRoomId(roomId)
    setStatusMessage(message)
    setConnectionStatus('connected')
  }

  const handleCreateRoom = async () => {
    setStatusMessage(null)
    setError(null)
    setIsBusy(true)
    setConnectionStatus('connecting')

    try {
      const { roomId } = await mockCreateRoom()
      setConnected(roomId, `Room ${roomId} created. Share the ID to invite friends.`)
      return roomId
    } catch (err) {
      setConnectionStatus('disconnected')
      setError('Unable to create a room right now. Please try again.')
      return null
    } finally {
      setIsBusy(false)
    }
  }

  const handleJoinRoom = async (roomIdInput: string) => {
    const roomId = normalizeRoomId(roomIdInput)
    setStatusMessage(null)
    setError(null)

    if (!isValidRoomId(roomId)) {
      setError('Room ID must be 6 characters (letters or numbers).')
      return
    }

    setIsBusy(true)
    setConnectionStatus('connecting')

    try {
      const { roomId: joinedRoomId } = await mockJoinRoom(roomId)
      setConnected(joinedRoomId, `Joined room ${joinedRoomId}. Waiting for game start.`)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to join this room.'
      setConnectionStatus('disconnected')
      setError(message)
    } finally {
      setIsBusy(false)
    }
  }

  const handleEnterGame = () => {
    setStatusMessage('Game navigation placeholder: gameplay view will load here next.')
  }

  return (
    <div className="lobby">
      <header className="page-header">
        <div>
          <p className="eyebrow">Snake Arena</p>
          <h1>Lobby</h1>
          <p className="muted">Create a room or join an existing one to start a match.</p>
        </div>

        <div className={`status-badge ${connectionStatus}`} aria-live="polite">
          <span className="dot" />
          {connectionLabel}
        </div>
      </header>

      <div className="tabs" role="tablist" aria-label="Lobby mode">
        <button
          className={`tab ${mode === 'create' ? 'active' : ''}`}
          onClick={() => setMode('create')}
          role="tab"
          aria-selected={mode === 'create'}
          aria-controls="create-panel"
          id="create-tab"
        >
          Create room
        </button>
        <button
          className={`tab ${mode === 'join' ? 'active' : ''}`}
          onClick={() => setMode('join')}
          role="tab"
          aria-selected={mode === 'join'}
          aria-controls="join-panel"
          id="join-tab"
        >
          Join room
        </button>
      </div>

      <div className="panel-grid">
        {mode === 'create' ? (
          <div role="tabpanel" id="create-panel" aria-labelledby="create-tab">
            <CreateRoomForm onCreate={handleCreateRoom} isBusy={isBusy} lastRoomId={activeRoomId} />
          </div>
        ) : (
          <div role="tabpanel" id="join-panel" aria-labelledby="join-tab">
            <JoinRoomForm onJoin={handleJoinRoom} isBusy={isBusy} />
          </div>
        )}
      </div>

      {statusMessage ? (
        <div className="callout neutral" aria-live="polite">
          {statusMessage}
        </div>
      ) : null}

      {error ? (
        <div className="callout error" aria-live="assertive">
          {error}
        </div>
      ) : null}

      <div className="next-step">
        <div>
          <h3>Enter the arena</h3>
          <p className="muted">
            Connected room: {activeRoomId ? <strong>{activeRoomId}</strong> : 'none yet'}.
            Gameplay view will be wired up in the next task.
          </p>
        </div>

        <button
          className="button ghost"
          type="button"
          onClick={handleEnterGame}
          disabled={!activeRoomId || connectionStatus !== 'connected'}
        >
          Go to game (placeholder)
        </button>
      </div>
    </div>
  )
}

export default LobbyView
