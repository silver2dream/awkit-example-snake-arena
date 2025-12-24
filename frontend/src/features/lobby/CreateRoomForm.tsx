import { FormEvent, useEffect, useState } from 'react'

type CreateRoomFormProps = {
  onCreate: () => Promise<string | null>
  isBusy: boolean
  lastRoomId?: string | null
}

const CreateRoomForm = ({ onCreate, isBusy, lastRoomId }: CreateRoomFormProps) => {
  const [roomId, setRoomId] = useState<string | null>(lastRoomId ?? null)
  const [localError, setLocalError] = useState<string | null>(null)

  useEffect(() => {
    setRoomId(lastRoomId ?? null)
  }, [lastRoomId])

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setLocalError(null)

    const createdRoomId = await onCreate()

    if (createdRoomId) {
      setRoomId(createdRoomId)
      return
    }

    setLocalError('Unable to create a room. Please try again.')
  }

  return (
    <section className="panel">
      <div className="panel-header">
        <h2>Create a room</h2>
        <p>Generate a new room ID and share it with friends.</p>
      </div>

      <form onSubmit={handleCreate} className="form-grid">
        <button className="button primary" type="submit" disabled={isBusy}>
          {isBusy ? 'Creating...' : 'Create room'}
        </button>
      </form>

      {roomId ? (
        <div className="callout">
          <div className="callout-title">Room created</div>
          <div className="room-id" aria-live="polite">
            {roomId}
          </div>
          <p className="muted">Share this code so others can join.</p>
        </div>
      ) : null}

      {localError ? <p className="error">{localError}</p> : null}
    </section>
  )
}

export default CreateRoomForm
