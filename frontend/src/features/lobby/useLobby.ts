import { useEffect, useMemo, useRef, useState } from 'react'
import { createLobbyApi, defaultLobbyApi } from './api'
import { LobbyApi, RoomSummary } from './types'

const PLAYER_ID_STORAGE_KEY = 'snake-arena:player-id'
const MIN_ROOM_NAME_LENGTH = 3
const MAX_ROOM_NAME_LENGTH = 30

export interface UseLobbyOptions {
  api?: LobbyApi
}

const generatePlayerId = () => {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `player-${Math.random().toString(36).slice(2, 10)}`
}

const getOrCreatePlayerId = () => {
  try {
    const existing = localStorage.getItem(PLAYER_ID_STORAGE_KEY)
    if (existing) return existing
    const next = generatePlayerId()
    localStorage.setItem(PLAYER_ID_STORAGE_KEY, next)
    return next
  } catch {
    // Fallback for non-browser/test environments
    return generatePlayerId()
  }
}

const validateRoomName = (value: string): string | null => {
  const name = value.trim()
  if (!name) return 'Please enter a room name.'
  if (name.length < MIN_ROOM_NAME_LENGTH) return `Room name must be at least ${MIN_ROOM_NAME_LENGTH} characters.`
  if (name.length > MAX_ROOM_NAME_LENGTH) return `Room name must be at most ${MAX_ROOM_NAME_LENGTH} characters.`
  if (!/^[a-zA-Z0-9 _-]+$/.test(name)) return 'Room name may only contain letters, numbers, spaces, dashes, and underscores.'
  return null
}

const validateRoomId = (value: string): string | null => {
  const roomId = value.trim()
  if (!roomId) return 'Please provide a room ID.'
  if (!/^[a-zA-Z0-9_-]+$/.test(roomId)) return 'Room ID may only contain letters, numbers, dashes, and underscores.'
  return null
}

export function useLobby(options?: UseLobbyOptions) {
  const api = useMemo(() => options?.api ?? defaultLobbyApi, [options?.api])
  const [rooms, setRooms] = useState<RoomSummary[]>([])
  const [roomName, setRoomName] = useState('')
  const [joinRoomId, setJoinRoomId] = useState('')
  const [currentRoomId, setCurrentRoomId] = useState<string | null>(null)
  const [statusMessage, setStatusMessage] = useState<string | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [isLoadingRooms, setIsLoadingRooms] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [connectionState, setConnectionState] = useState<'disconnected' | 'connecting' | 'connected'>('disconnected')
  const socketRef = useRef<WebSocket | null>(null)
  const playerIdRef = useRef<string>(getOrCreatePlayerId())

  const refreshRooms = async () => {
    setIsLoadingRooms(true)
    setErrorMessage(null)
    try {
      const response = await api.listRooms()
      setRooms(response)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to load rooms.'
      setErrorMessage(message)
    } finally {
      setIsLoadingRooms(false)
    }
  }

  useEffect(() => {
    void refreshRooms()
    return () => {
      socketRef.current?.close()
    }
  }, [api])

  const attachSocket = (roomId: string) => {
    socketRef.current?.close()
    const socket = api.connectToRoom(roomId, playerIdRef.current)
    if (!socket) {
      setConnectionState('disconnected')
      return
    }
    setConnectionState('connecting')
    socket.onopen = () => setConnectionState('connected')
    socket.onclose = () => setConnectionState('disconnected')
    socket.onerror = () => {
      setConnectionState('disconnected')
      setErrorMessage('Lost connection to the room socket.')
    }
    socketRef.current = socket
  }

  const createRoom = async () => {
    const validation = validateRoomName(roomName)
    if (validation) {
      setErrorMessage(validation)
      return false
    }

    setIsSubmitting(true)
    setErrorMessage(null)
    setStatusMessage(null)

    try {
      const { roomId } = await api.createRoom({
        name: roomName.trim(),
        playerId: playerIdRef.current,
      })
      setCurrentRoomId(roomId)
      setStatusMessage(`Created room ${roomId}.`)
      setRoomName('')
      attachSocket(roomId)
      void refreshRooms()
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to create room.'
      setErrorMessage(message)
      return false
    } finally {
      setIsSubmitting(false)
    }
  }

  const joinRoom = async (targetRoomId?: string) => {
    const desiredRoomId = (targetRoomId ?? joinRoomId).trim()
    const validation = validateRoomId(desiredRoomId)
    if (validation) {
      setErrorMessage(validation)
      return false
    }

    setIsSubmitting(true)
    setErrorMessage(null)
    setStatusMessage(null)

    try {
      const { roomId } = await api.joinRoom({
        roomId: desiredRoomId,
        playerId: playerIdRef.current,
      })
      setCurrentRoomId(roomId)
      setStatusMessage(`Joined room ${roomId}.`)
      setJoinRoomId('')
      attachSocket(roomId)
      void refreshRooms()
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unable to join room.'
      setErrorMessage(message)
      return false
    } finally {
      setIsSubmitting(false)
    }
  }

  return {
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
    createRoom,
    joinRoom,
    refreshRooms,
  }
}

export const lobbyTestUtils = {
  validateRoomName,
  validateRoomId,
  createLobbyApi,
}
