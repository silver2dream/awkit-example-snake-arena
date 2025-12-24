import { describe, expect, it } from 'vitest'
import { generateRoomId, isValidRoomId, normalizeRoomId } from './lobbyApi'

describe('lobbyApi helpers', () => {
  it('generates a valid room id', () => {
    const roomId = generateRoomId()

    expect(roomId).toHaveLength(6)
    expect(isValidRoomId(roomId)).toBe(true)
  })

  it('validates and normalizes room ids', () => {
    expect(isValidRoomId('ABC123')).toBe(true)
    expect(isValidRoomId('abc123')).toBe(true)
    expect(normalizeRoomId('abc123')).toBe('ABC123')
    expect(isValidRoomId('ABC!23')).toBe(false)
    expect(isValidRoomId('12')).toBe(false)
  })
})
