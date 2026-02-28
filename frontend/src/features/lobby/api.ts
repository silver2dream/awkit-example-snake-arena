export interface RoomSummary {
  id: string
  playerCount: number
  status: string
}

interface CreateRoomResponse {
  roomId: string
}

interface JoinRoomResponse {
  roomId: string
  playerId: string
}

interface APIErrorResponse {
  type?: string
  message?: string
}

const asErrorMessage = (payload: unknown): string | null => {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return null
  }

  const errorPayload = payload as APIErrorResponse
  if (typeof errorPayload.message !== 'string' || errorPayload.message.trim().length === 0) {
    return null
  }

  return errorPayload.message
}

const readJSON = async <T>(response: Response): Promise<T | null> => {
  try {
    return (await response.json()) as T
  } catch {
    return null
  }
}

const parseError = async (response: Response, fallback: string) => {
  const parsed = await readJSON<APIErrorResponse>(response)
  return asErrorMessage(parsed) ?? fallback
}

export async function listRooms(): Promise<RoomSummary[]> {
  const response = await fetch('/api/rooms')
  if (!response.ok) {
    throw new Error(await parseError(response, 'failed to load rooms'))
  }

  const payload = await readJSON<RoomSummary[]>(response)
  return Array.isArray(payload) ? payload : []
}

export async function createRoom(): Promise<CreateRoomResponse> {
  const response = await fetch('/api/rooms', {
    method: 'POST',
  })

  if (!response.ok) {
    throw new Error(await parseError(response, 'failed to create room'))
  }

  const payload = await readJSON<CreateRoomResponse>(response)
  if (!payload || typeof payload.roomId !== 'string' || payload.roomId.length === 0) {
    throw new Error('invalid create room response')
  }

  return payload
}

export async function joinRoom(roomId: string, playerName: string): Promise<JoinRoomResponse> {
  const response = await fetch(`/api/rooms/${encodeURIComponent(roomId)}/join`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ playerName }),
  })

  if (!response.ok) {
    throw new Error(await parseError(response, 'failed to join room'))
  }

  const payload = await readJSON<JoinRoomResponse>(response)
  if (!payload || typeof payload.roomId !== 'string' || payload.roomId.length === 0) {
    throw new Error('invalid join room response')
  }

  return payload
}
