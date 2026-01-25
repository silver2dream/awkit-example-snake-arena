import { CreateRoomRequest, JoinRoomRequest, LobbyApi, RoomSummary } from './types'

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

const DEFAULT_BASE_URL = '/api'

const cleanBase = (base: string) => base.replace(/\/+$/, '')

async function readError(res: Response): Promise<never> {
  try {
    const data = await res.clone().json()
    const message = typeof data?.error === 'string' ? data.error : undefined
    throw new Error((message ?? res.statusText) || `Request failed with ${res.status}`)
  } catch {
    const text = await res.text()
    throw new Error(text || res.statusText || `Request failed with ${res.status}`)
  }
}

async function parseJsonOrThrow<T>(res: Response): Promise<T> {
  if (!res.ok) {
    return readError(res)
  }
  return res.json() as Promise<T>
}

export interface LobbyApiOptions {
  baseUrl?: string
  wsBaseUrl?: string
  fetchImpl?: FetchLike
}

export function createLobbyApi(options?: LobbyApiOptions): LobbyApi {
  const baseUrl = cleanBase(options?.baseUrl ?? DEFAULT_BASE_URL)
  const wsBaseUrl = cleanBase(options?.wsBaseUrl ?? baseUrl)
  const fetcher: FetchLike = options?.fetchImpl ?? fetch

  const buildUrl = (path: string) => `${baseUrl}${path}`

  const listRooms = async (): Promise<RoomSummary[]> => {
    const res = await fetcher(buildUrl('/rooms'))
    if (res.status === 204) return []
    return parseJsonOrThrow<RoomSummary[]>(res)
  }

  const createRoom = async (request: CreateRoomRequest) => {
    const res = await fetcher(buildUrl('/rooms'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    })
    return parseJsonOrThrow<{ roomId: string }>(res)
  }

  const joinRoom = async (request: JoinRoomRequest) => {
    const res = await fetcher(buildUrl(`/rooms/${encodeURIComponent(request.roomId)}/join`), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ playerId: request.playerId }),
    })
    return parseJsonOrThrow<{ roomId: string }>(res)
  }

  const connectToRoom = (roomId: string, playerId: string): WebSocket | null => {
    if (typeof WebSocket === 'undefined') return null
    const wsBase = wsBaseUrl.replace(/^http/i, 'ws')
    const url = `${wsBase}/ws/rooms/${encodeURIComponent(roomId)}?playerId=${encodeURIComponent(playerId)}`
    return new WebSocket(url)
  }

  return {
    listRooms,
    createRoom,
    joinRoom,
    connectToRoom,
  }
}

export const defaultLobbyApi = createLobbyApi()
