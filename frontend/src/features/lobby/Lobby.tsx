import type { ReactNode } from 'react'
import GameCanvas from '../game/GameCanvas'
import type { LobbyApi } from './types'
import { useLobby } from './useLobby'

type LobbyProps = {
  api?: LobbyApi
}

const Section = ({ title, children }: { title: string; children: ReactNode }) => (
  <section style={{ border: '1px solid #e5e7eb', padding: '1rem', borderRadius: 8, background: '#f9fafb' }}>
    <h2 style={{ marginTop: 0 }}>{title}</h2>
    {children}
  </section>
)

function Lobby({ api }: LobbyProps) {
  const {
    rooms,
    roomName,
    setRoomName,
    joinRoomId,
    setJoinRoomId,
    currentRoomId,
    statusMessage,
    errorMessage,
    isLoadingRooms,
    isSubmitting,
    connectionState,
    gameSnapshot,
    createRoom,
    joinRoom,
    refreshRooms,
  } = useLobby({ api })

  return (
    <main style={{ maxWidth: 960, margin: '0 auto', padding: '2rem', display: 'grid', gap: '1rem' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem' }}>
        <div>
          <h1 style={{ margin: 0 }}>Snake Arena Lobby</h1>
          <p style={{ margin: '0.25rem 0', color: '#4b5563' }}>
            Create a room or join an existing one to start playing.
          </p>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontWeight: 600 }}>Connection</div>
          <div aria-live="polite">{connectionState}</div>
          {currentRoomId ? <div style={{ fontSize: 12 }}>Current room: {currentRoomId}</div> : <div style={{ fontSize: 12 }}>Not in a room</div>}
        </div>
      </header>

      {errorMessage ? (
        <div role="alert" style={{ background: '#fef2f2', border: '1px solid #fecaca', padding: '0.75rem', borderRadius: 8 }}>
          {errorMessage}
        </div>
      ) : null}

      {statusMessage ? (
        <div role="status" aria-live="polite" style={{ background: '#ecfdf3', border: '1px solid #bbf7d0', padding: '0.75rem', borderRadius: 8 }}>
          {statusMessage}
        </div>
      ) : null}

      <Section title="Create a room">
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void createRoom()
          }}
          style={{ display: 'flex', alignItems: 'flex-end', gap: '0.75rem' }}
        >
          <label style={{ flex: 1 }}>
            <span style={{ display: 'block', fontWeight: 600, marginBottom: 4 }}>Room name</span>
            <input
              id="room-name-input"
              placeholder="e.g. Friday Night Squad"
              value={roomName}
              onChange={(event) => setRoomName(event.target.value)}
              minLength={3}
              maxLength={30}
              required
              style={{
                width: '100%',
                padding: '0.65rem 0.75rem',
                borderRadius: 6,
                border: '1px solid #d1d5db',
              }}
            />
          </label>
          <button
            id="create-room-button"
            type="submit"
            disabled={isSubmitting}
            style={{
              padding: '0.7rem 1rem',
              borderRadius: 6,
              border: 'none',
              background: '#0ea5e9',
              color: '#fff',
              fontWeight: 600,
              cursor: 'pointer',
              opacity: isSubmitting ? 0.7 : 1,
            }}
          >
            {isSubmitting ? 'Working…' : 'Create room'}
          </button>
        </form>
      </Section>

      <Section title="Join a room by ID">
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void joinRoom()
          }}
          style={{ display: 'flex', alignItems: 'flex-end', gap: '0.75rem' }}
        >
          <label style={{ flex: 1 }}>
            <span style={{ display: 'block', fontWeight: 600, marginBottom: 4 }}>Room ID</span>
            <input
              id="room-id-input"
              placeholder="room-123"
              value={joinRoomId}
              onChange={(event) => setJoinRoomId(event.target.value)}
              required
              style={{
                width: '100%',
                padding: '0.65rem 0.75rem',
                borderRadius: 6,
                border: '1px solid #d1d5db',
              }}
            />
          </label>
          <button
            id="join-room-button"
            type="submit"
            disabled={isSubmitting}
            style={{
              padding: '0.7rem 1rem',
              borderRadius: 6,
              border: 'none',
              background: '#10b981',
              color: '#fff',
              fontWeight: 600,
              cursor: 'pointer',
              opacity: isSubmitting ? 0.7 : 1,
            }}
          >
            {isSubmitting ? 'Joining…' : 'Join room'}
          </button>
        </form>
      </Section>

      <Section title="Game">
        <GameCanvas snapshot={gameSnapshot} />
      </Section>

      <Section title="Available rooms">
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem', alignItems: 'center' }}>
          <div style={{ color: '#4b5563' }}>
            {isLoadingRooms ? 'Loading rooms…' : `${rooms.length} open room${rooms.length === 1 ? '' : 's'}`}
          </div>
          <button
            type="button"
            onClick={() => void refreshRooms()}
            disabled={isLoadingRooms}
            style={{
              padding: '0.35rem 0.75rem',
              borderRadius: 6,
              border: '1px solid #d1d5db',
              background: '#fff',
              cursor: 'pointer',
            }}
          >
            Refresh
          </button>
        </div>
        {rooms.length === 0 ? (
          <p style={{ margin: 0, color: '#6b7280' }}>No rooms yet. Be the first to create one!</p>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: '0.5rem' }}>
            {rooms.map((room) => (
              <li
                key={room.id}
                style={{
                  border: '1px solid #e5e7eb',
                  borderRadius: 6,
                  padding: '0.75rem',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  background: '#fff',
                }}
              >
                <div>
                  <div style={{ fontWeight: 600 }}>{room.name ?? room.id}</div>
                  <div style={{ color: '#6b7280', fontSize: 12 }}>
                    ID: {room.id} · Players: {room.players ?? 0}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => void joinRoom(room.id)}
                  disabled={isSubmitting}
                  style={{
                    padding: '0.5rem 0.9rem',
                    borderRadius: 6,
                    border: 'none',
                  background: '#f59e0b',
                  color: '#fff',
                  fontWeight: 600,
                  cursor: 'pointer',
                  opacity: isSubmitting ? 0.75 : 1,
                }}
                  aria-label={`Join room ${room.name ?? room.id}`}
                >
                  Join
                </button>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </main>
  )
}

export default Lobby
