import { describe, expect, it } from 'vitest'
import { applyIncomingMessage } from './snapshot'
import type { GameSnapshot } from './types'

const baseState: GameSnapshot = {
  tick: 1,
  width: 20,
  height: 20,
  snakes: {},
  food: null,
  scores: {},
  gameOver: false,
}

describe('snapshot message parsing', () => {
  it('TestApplyRoomSnapshotUsesDynamicGridSizeAndPlayers', () => {
    const next = applyIncomingMessage(baseState, {
      type: 'room_snapshot',
      tick: 3,
      width: 31,
      height: 17,
      players: ['alice', 'bob'],
      food: { X: 4, Y: 5 },
    })

    expect(next).not.toBeNull()
    expect(next?.width).toBe(31)
    expect(next?.height).toBe(17)
    expect(next?.players).toEqual(['alice', 'bob'])
    expect(next?.snakes.alice).toEqual([])
    expect(next?.scores.bob).toBe(0)
    expect(next?.food).toEqual({ x: 4, y: 5 })
  })

  it('TestApplyTickUpdateMapsSnakeFoodScoresAndGameOver', () => {
    const next = applyIncomingMessage(baseState, {
      type: 'tick_update',
      tick: 9,
      snakes: {
        alice: [
          { X: 2, Y: 3 },
          { x: 1, y: 3 },
        ],
      },
      food: { x: 7, y: 8 },
      scores: { alice: 6 },
      gameOver: true,
    })

    expect(next).not.toBeNull()
    expect(next?.tick).toBe(9)
    expect(next?.snakes.alice).toEqual([
      { x: 2, y: 3 },
      { x: 1, y: 3 },
    ])
    expect(next?.food).toEqual({ x: 7, y: 8 })
    expect(next?.scores.alice).toBe(6)
    expect(next?.gameOver).toBe(true)
  })

  it('TestApplySnapshotIgnoresMalformedPoints', () => {
    const next = applyIncomingMessage(baseState, {
      type: 'tick_update',
      tick: 2,
      snakes: {
        alice: [{ x: 2 }, { x: 2, y: 4 }],
      },
      food: { y: 3 },
      scores: { alice: 1 },
      gameOver: false,
    })

    expect(next).not.toBeNull()
    expect(next?.snakes.alice).toEqual([{ x: 2, y: 4 }])
    expect(next?.food).toBeNull()
  })
})
