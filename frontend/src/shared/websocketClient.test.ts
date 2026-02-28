import { describe, expect, it } from 'vitest'
import { parseIncomingMessage } from './websocketClient'

describe('parseIncomingMessage', () => {
  it('parses room snapshot messages with snapshot payload', () => {
    const parsed = parseIncomingMessage({
      type: 'room_snapshot',
      roomId: 'room-1',
      snapshot: {
        tick: 1,
      },
    })

    expect(parsed).toEqual({
      type: 'room_snapshot',
      roomId: 'room-1',
      snapshot: {
        tick: 1,
      },
    })
  })

  it('parses server error messages', () => {
    const parsed = parseIncomingMessage({
      type: 'error',
      message: 'room not found',
    })

    expect(parsed).toEqual({
      type: 'error',
      message: 'room not found',
    })
  })

  it('returns null for unknown message type', () => {
    const parsed = parseIncomingMessage({
      type: 'unknown',
      snapshot: {},
    })

    expect(parsed).toBeNull()
  })

  it('returns null when required snapshot payload is missing', () => {
    const parsed = parseIncomingMessage({
      type: 'tick_update',
      roomId: 'room-2',
    })

    expect(parsed).toBeNull()
  })
})
