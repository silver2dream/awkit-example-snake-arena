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
      roomId: 'room-1',
      snapshot: {
        tick: 3,
        width: 31,
        height: 17,
        players: [
          {
            id: 'player-1',
            name: 'alice',
            score: 0,
            alive: true,
            body: [{ X: 1, Y: 1 }],
          },
          {
            id: 'player-2',
            name: 'bob',
            score: 0,
            alive: true,
            body: [{ X: 2, Y: 2 }],
          },
        ],
        food: { X: 4, Y: 5 },
      },
    })

    expect(next).not.toBeNull()
    expect(next?.width).toBe(31)
    expect(next?.height).toBe(17)
    expect(next?.players).toEqual(['alice', 'bob'])
    expect(next?.snakes.alice).toEqual([{ x: 1, y: 1 }])
    expect(next?.scores.bob).toBe(0)
    expect(next?.food).toEqual({ x: 4, y: 5 })
  })

  it('TestApplyTickUpdateMapsSnakeFoodScoresAndGameOver', () => {
    const next = applyIncomingMessage(baseState, {
      type: 'tick_update',
      roomId: 'room-1',
      snapshot: {
        tick: 9,
        players: [
          {
            id: 'player-1',
            name: 'alice',
            score: 6,
            alive: false,
            body: [
              { X: 2, Y: 3 },
              { x: 1, y: 3 },
            ],
          },
        ],
        food: { x: 7, y: 8 },
      },
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
      roomId: 'room-1',
      snapshot: {
        tick: 2,
        players: [
          {
            id: 'player-1',
            name: 'alice',
            score: 1,
            alive: true,
            body: [{ x: 2 }, { x: 2, y: 4 }],
          },
        ],
        food: { y: 3 },
      },
    })

    expect(next).not.toBeNull()
    expect(next?.snakes.alice).toEqual([{ x: 2, y: 4 }])
    expect(next?.food).toBeNull()
  })
})
