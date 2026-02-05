import { useEffect, useRef } from 'react'
import type { RoomWebSocketClient } from '../../shared/websocketClient'

export type Direction = 'up' | 'down' | 'left' | 'right'

const KEY_DIRECTION_MAP: Record<string, Direction> = {
  arrowup: 'up',
  w: 'up',
  arrowdown: 'down',
  s: 'down',
  arrowleft: 'left',
  a: 'left',
  arrowright: 'right',
  d: 'right',
}

const mapKeyToDirection = (key: string): Direction | null => {
  const normalized = key.toLowerCase()
  return KEY_DIRECTION_MAP[normalized] ?? null
}

type KeyboardListenerTarget = Pick<EventTarget, 'addEventListener' | 'removeEventListener'>

export interface AttachKeyboardInputOptions {
  target: KeyboardListenerTarget
  throttleMs: number
  sendInput: (input: unknown) => void
  now?: () => number
}

export function attachKeyboardInputHandler(options: AttachKeyboardInputOptions) {
  const { target, throttleMs } = options
  const now = options.now ?? Date.now
  let lastSentAt: number | null = null

  const handleKeyDown = (event: Event) => {
    const keyboardEvent = event as KeyboardEvent
    const direction = keyboardEvent.key ? mapKeyToDirection(keyboardEvent.key) : null
    if (!direction) return

    const timestamp = now()
    if (lastSentAt !== null && timestamp - lastSentAt < throttleMs) {
      return
    }

    lastSentAt = timestamp
    options.sendInput({ direction })
  }

  target.addEventListener('keydown', handleKeyDown as EventListener)
  return () => {
    target.removeEventListener('keydown', handleKeyDown as EventListener)
  }
}

export interface UseKeyboardInputOptions {
  throttleMs?: number
  target?: EventTarget
}

export function useKeyboardInput(client: RoomWebSocketClient | null | undefined, options?: UseKeyboardInputOptions) {
  const throttleMs = options?.throttleMs ?? 75
  const target = (options?.target ?? (typeof window !== 'undefined' ? window : null)) as KeyboardListenerTarget | null
  const clientRef = useRef(client)

  useEffect(() => {
    clientRef.current = client
  }, [client])

  useEffect(() => {
    if (!client || !target) return

    return attachKeyboardInputHandler({
      target,
      throttleMs,
      sendInput: (input) => {
        clientRef.current?.sendInput(input)
      },
    })
  }, [client, target, throttleMs])
}

export const keyboardInputTestUtils = {
  mapKeyToDirection,
  attachKeyboardInputHandler,
}
