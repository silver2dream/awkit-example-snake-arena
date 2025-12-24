import { ChangeEvent, FormEvent, useState } from 'react'
import { isValidRoomId, normalizeRoomId } from './lobbyApi'

type JoinRoomFormProps = {
  onJoin: (roomId: string) => Promise<void>
  isBusy: boolean
}

const JoinRoomForm = ({ onJoin, isBusy }: JoinRoomFormProps) => {
  const [roomId, setRoomId] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    setRoomId(normalizeRoomId(event.target.value))
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    if (!roomId) {
      setLocalError('Room ID is required.')
      return
    }

    if (!isValidRoomId(roomId)) {
      setLocalError('Room ID must be 6 letters or numbers.')
      return
    }

    setLocalError(null)
    await onJoin(roomId)
  }

  return (
    <section className="panel">
      <div className="panel-header">
        <h2>Join a room</h2>
        <p>Enter a room ID shared by your friends.</p>
      </div>

      <form onSubmit={handleSubmit} className="form-grid">
        <label className="field">
          <span className="field-label">Room ID</span>
          <input
            type="text"
            inputMode="text"
            autoComplete="off"
            placeholder="ABC123"
            value={roomId}
            onChange={handleChange}
            maxLength={6}
            className="input"
            disabled={isBusy}
          />
        </label>

        <button className="button primary" type="submit" disabled={isBusy}>
          {isBusy ? 'Joining...' : 'Join room'}
        </button>
      </form>

      {localError ? <p className="error">{localError}</p> : null}
    </section>
  )
}

export default JoinRoomForm
