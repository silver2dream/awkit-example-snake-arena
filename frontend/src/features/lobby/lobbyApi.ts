const ROOM_ID_LENGTH = 6
const ROOM_ID_PATTERN = /^[A-Z0-9]{6}$/
const ROOM_ID_CHARS = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'

const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected'

export const normalizeRoomId = (value: string) => value.trim().toUpperCase()

export const isValidRoomId = (value: string) => ROOM_ID_PATTERN.test(normalizeRoomId(value))

export const generateRoomId = () => {
  let output = ''

  for (let i = 0; i < ROOM_ID_LENGTH; i += 1) {
    const index = Math.floor(Math.random() * ROOM_ID_CHARS.length)
    output += ROOM_ID_CHARS[index]
  }

  return output
}

export const mockCreateRoom = async () => {
  await wait(300)

  return { roomId: generateRoomId() }
}

export const mockJoinRoom = async (roomId: string) => {
  const normalized = normalizeRoomId(roomId)

  if (!isValidRoomId(normalized)) {
    throw new Error('Room ID must be 6 characters (letters or numbers).')
  }

  await wait(200)

  return { roomId: normalized }
}
