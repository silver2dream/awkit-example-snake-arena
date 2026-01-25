import { renderToString } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import Lobby from './Lobby'
import { createLobbyApi } from './api'
import { lobbyTestUtils } from './useLobby'
import type { LobbyApi } from './types'

const buildApi = (overrides?: Partial<LobbyApi>): LobbyApi => ({
  listRooms: vi.fn().mockResolvedValue([]),
  createRoom: vi.fn().mockResolvedValue({ roomId: 'room-1' }),
  joinRoom: vi.fn().mockResolvedValue({ roomId: 'room-1' }),
  connectToRoom: vi.fn().mockReturnValue(null),
  ...overrides,
})

describe('Lobby', () => {
  it('TestLobbySmokeRendersCoreControls', () => {
    const html = renderToString(<Lobby api={buildApi()} />)

    expect(html).toContain('Snake Arena Lobby')
    expect(html).toContain('Create a room')
    expect(html).toContain('Room name')
    expect(html).toContain('Join a room by ID')
    expect(html).toContain('Room ID')
    expect(html).toContain('Available rooms')
    expect(html).toContain('id="room-name-input"')
    expect(html).toContain('id="create-room-button"')
    expect(html).toContain('id="room-id-input"')
    expect(html).toContain('id="join-room-button"')
  })

  it('TestLobbyValidatesRoomNamesAndIds', () => {
    expect(lobbyTestUtils.validateRoomName('')).toMatch(/enter a room name/i)
    expect(lobbyTestUtils.validateRoomName('ab')).toMatch(/at least/i)
    expect(lobbyTestUtils.validateRoomName('***')).toMatch(/may only contain/i)
    expect(lobbyTestUtils.validateRoomName('Valid Room')).toBeNull()

    expect(lobbyTestUtils.validateRoomId('')).toMatch(/provide a room id/i)
    expect(lobbyTestUtils.validateRoomId('room 123')).toMatch(/letters, numbers, dashes, and underscores/i)
    expect(lobbyTestUtils.validateRoomId('room-123')).toBeNull()
  })

  it('TestLobbySurfacesBackendErrors', async () => {
    const fetchMock = vi.fn(async () => {
      const body = JSON.stringify({ error: 'boom' })
      return new Response(body, { status: 400, headers: { 'Content-Type': 'application/json' } })
    })

    const api = createLobbyApi({ baseUrl: '', fetchImpl: fetchMock })

    await expect(api.createRoom({ name: 'Test', playerId: 'p1' })).rejects.toThrow(/boom/)
    expect(fetchMock).toHaveBeenCalled()
  })
})
