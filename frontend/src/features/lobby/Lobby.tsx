import { useCallback, useEffect, useState, type FormEvent } from 'react'
import type { ConnectionState } from '../../shared/websocketClient'
import { createRoom, joinRoom, listRooms, type RoomSummary } from './api'

const connectionStateColor: Record<ConnectionState, string> = {
  connecting: '#ca8a04',
  connected: '#15803d',
  disconnected: '#b91c1c',
}

interface LobbyProps {
  connectionState: ConnectionState
  onJoinReady: (roomId: string, playerName: string) => void
}

const toErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message
  }

  return fallback
}

function Lobby({ connectionState, onJoinReady }: LobbyProps) {
  const [rooms, setRooms] = useState<RoomSummary[]>([])
  const [roomsLoading, setRoomsLoading] = useState(false)
  const [roomsError, setRoomsError] = useState<string | null>(null)

  const [joinRoomId, setJoinRoomId] = useState('')
  const [joinPlayerName, setJoinPlayerName] = useState('')
  const [createPlayerName, setCreatePlayerName] = useState('')

  const [statusMessage, setStatusMessage] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [actionPending, setActionPending] = useState(false)

  const loadRooms = useCallback(async () => {
    setRoomsLoading(true)
    setRoomsError(null)

    try {
      const nextRooms = await listRooms()
      setRooms(nextRooms)
    } catch (error) {
      setRoomsError(toErrorMessage(error, 'failed to load rooms'))
    } finally {
      setRoomsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadRooms()
  }, [loadRooms])

  const handleCreateRoom = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    const playerName = createPlayerName.trim()
    if (!playerName) {
      setActionError('player name is required')
      return
    }

    setActionPending(true)
    setActionError(null)
    setStatusMessage(null)

    try {
      const created = await createRoom()
      await joinRoom(created.roomId, playerName)
      setStatusMessage(`joined ${created.roomId}`)
      onJoinReady(created.roomId, playerName)
      await loadRooms()
    } catch (error) {
      setActionError(toErrorMessage(error, 'failed to create room'))
    } finally {
      setActionPending(false)
    }
  }

  const handleJoinRoom = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    const roomId = joinRoomId.trim()
    const playerName = joinPlayerName.trim()

    if (!roomId) {
      setActionError('room id is required')
      return
    }

    if (!playerName) {
      setActionError('player name is required')
      return
    }

    setActionPending(true)
    setActionError(null)
    setStatusMessage(null)

    try {
      await joinRoom(roomId, playerName)
      setStatusMessage(`joined ${roomId}`)
      onJoinReady(roomId, playerName)
      await loadRooms()
    } catch (error) {
      setActionError(toErrorMessage(error, 'failed to join room'))
    } finally {
      setActionPending(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: '1rem' }}>
      <h1 style={{ margin: 0 }}>Snake Arena</h1>

      <div
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: '0.5rem',
          fontWeight: 600,
        }}
      >
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

      <section style={{ border: '1px solid #cbd5e1', borderRadius: 10, padding: '0.9rem', display: 'grid', gap: '0.75rem' }}>
        <h2 style={{ margin: 0, fontSize: '1rem' }}>Create Room</h2>
        <form style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }} onSubmit={handleCreateRoom}>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            <span>Player name</span>
            <input
              name="create-player-name"
              value={createPlayerName}
              onChange={(event) => setCreatePlayerName(event.target.value)}
              placeholder="alice"
              autoComplete="off"
            />
          </label>
          <button type="submit" disabled={actionPending}>
            Create room
          </button>
        </form>
      </section>

      <section style={{ border: '1px solid #cbd5e1', borderRadius: 10, padding: '0.9rem', display: 'grid', gap: '0.75rem' }}>
        <h2 style={{ margin: 0, fontSize: '1rem' }}>Join Room</h2>
        <form style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }} onSubmit={handleJoinRoom}>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            <span>Room ID</span>
            <input
              name="join-room-id"
              value={joinRoomId}
              onChange={(event) => setJoinRoomId(event.target.value)}
              placeholder="room-1"
              autoComplete="off"
            />
          </label>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            <span>Player name</span>
            <input
              name="join-player-name"
              value={joinPlayerName}
              onChange={(event) => setJoinPlayerName(event.target.value)}
              placeholder="alice"
              autoComplete="off"
            />
          </label>
          <button type="submit" disabled={actionPending}>
            Join room
          </button>
        </form>
      </section>

      <section style={{ border: '1px solid #cbd5e1', borderRadius: 10, padding: '0.9rem', display: 'grid', gap: '0.75rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ margin: 0, fontSize: '1rem' }}>Available Rooms</h2>
          <button type="button" onClick={() => void loadRooms()} disabled={roomsLoading || actionPending}>
            Refresh
          </button>
        </div>

        {roomsLoading ? <div>Loading rooms…</div> : null}
        {roomsError ? <div style={{ color: '#b91c1c' }}>{roomsError}</div> : null}

        {!roomsLoading && !roomsError && rooms.length === 0 ? <div>No rooms yet.</div> : null}

        {!roomsLoading && !roomsError && rooms.length > 0 ? (
          <ul style={{ margin: 0, paddingLeft: '1.25rem', display: 'grid', gap: '0.35rem' }}>
            {rooms.map((room) => (
              <li key={room.id} style={{ display: 'flex', justifyContent: 'space-between', gap: '0.75rem' }}>
                <span>
                  <strong>{room.id}</strong> ({room.playerCount} players, {room.status})
                </span>
                <button type="button" onClick={() => setJoinRoomId(room.id)} disabled={actionPending}>
                  Use ID
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </section>

      {actionError ? (
        <div role="alert" style={{ color: '#b91c1c', fontWeight: 600 }}>
          {actionError}
        </div>
      ) : null}

      {statusMessage ? (
        <div role="status" style={{ color: '#166534', fontWeight: 600 }}>
          {statusMessage}
        </div>
      ) : null}
    </div>
  )
}

export default Lobby
