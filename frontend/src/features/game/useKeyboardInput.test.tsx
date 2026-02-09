import { describe, expect, it, vi } from 'vitest'
import { keyboardInputTestUtils } from './useKeyboardInput'
import type { RoomWebSocketClient } from '../../shared/websocketClient'

const createClient = () => {
  const noop = () => {}
  return {
    roomId: 'room-1',
    getState: () => 'connected',
    connect: noop,
    disconnect: noop,
    sendJoinRoom: vi.fn().mockReturnValue(true),
    sendInput: vi.fn().mockReturnValue(true),
    subscribeState: vi.fn().mockReturnValue(noop),
    subscribeMessages: vi.fn().mockReturnValue(noop),
    subscribeErrors: vi.fn().mockReturnValue(noop),
  } satisfies RoomWebSocketClient
}

class MockTarget implements EventTarget {
  private listeners = new Set<EventListener>()

  addEventListener(
    type: string,
    listener: EventListenerOrEventListenerObject | null,
    _options?: boolean | AddEventListenerOptions,
  ) {
    if (type === 'keydown' && typeof listener === 'function') {
      this.listeners.add(listener)
    }
  }

  removeEventListener(
    _type: string,
    listener: EventListenerOrEventListenerObject | null,
    _options?: boolean | EventListenerOptions,
  ) {
    if (typeof listener === 'function') {
      this.listeners.delete(listener)
    }
  }

  dispatchEvent(event: Event): boolean {
    this.listeners.forEach((listener) => listener(event))
    return true
  }

  dispatch(key: string) {
    const event = { key } as unknown as Event
    this.dispatchEvent(event)
  }

  listenerCount() {
    return this.listeners.size
  }
}

describe('useKeyboardInput', () => {
  it('TestKeyboardInputMapsArrowKeysAndWasdToDirections', () => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    const client = createClient()
    const target = new MockTarget()
    const cleanup = keyboardInputTestUtils.attachKeyboardInputHandler({
      target,
      throttleMs: 50,
      sendInput: client.sendInput,
    })

    try {
      target.dispatch('ArrowUp')
      vi.advanceTimersByTime(60)
      target.dispatch('a')
      vi.advanceTimersByTime(60)
      target.dispatch('ArrowRight')

      expect(client.sendInput.mock.calls.length).toBe(3)
      expect(client.sendInput).toHaveBeenCalledWith({ direction: 'up' })
      expect(client.sendInput).toHaveBeenCalledWith({ direction: 'left' })
      expect(client.sendInput).toHaveBeenCalledWith({ direction: 'right' })
      expect(client.sendInput).toHaveBeenCalledTimes(3)
    } finally {
      cleanup()
      vi.useRealTimers()
    }
  })

  it('TestKeyboardInputThrottlesRapidInputs', () => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    const client = createClient()
    const target = new MockTarget()
    const cleanup = keyboardInputTestUtils.attachKeyboardInputHandler({
      target,
      throttleMs: 100,
      sendInput: client.sendInput,
    })

    try {
      target.dispatch('ArrowDown')
      target.dispatch('s')

      expect(client.sendInput.mock.calls.length).toBe(1)
      expect(client.sendInput).toHaveBeenCalledTimes(1)

      vi.advanceTimersByTime(110)
      target.dispatch('s')
      expect(client.sendInput).toHaveBeenCalledTimes(2)
    } finally {
      cleanup()
      vi.useRealTimers()
    }
  })

  it('TestKeyboardInputCleansUpOnUnmount', () => {
    const client = createClient()
    const target = new MockTarget()
    const cleanup = keyboardInputTestUtils.attachKeyboardInputHandler({
      target,
      throttleMs: 0,
      sendInput: client.sendInput,
    })

    target.dispatch('ArrowUp')
    expect(client.sendInput.mock.calls.length).toBe(1)
    expect(client.sendInput).toHaveBeenCalledTimes(1)
    expect(target.listenerCount()).toBe(1)

    cleanup()

    target.dispatch('ArrowLeft')
    expect(client.sendInput).toHaveBeenCalledTimes(1)
    expect(target.listenerCount()).toBe(0)
  })
})
