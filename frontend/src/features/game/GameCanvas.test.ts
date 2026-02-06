import { describe, expect, it } from 'vitest'
import { render } from './GameCanvas'
import type { GameSnapshot } from './types'

class MockCanvasRenderingContext2D {
  canvas: { width: number; height: number }
  fillStyle = ''
  strokeStyle = ''
  lineWidth = 1
  font = ''
  textAlign: CanvasTextAlign = 'start'
  textBaseline: CanvasTextBaseline = 'alphabetic'
  calls: Array<{ method: string; args: unknown[] }> = []

  constructor(width: number, height: number) {
    this.canvas = { width, height }
  }

  beginPath() {
    this.calls.push({ method: 'beginPath', args: [] })
  }

  clearRect(x: number, y: number, width: number, height: number) {
    this.calls.push({ method: 'clearRect', args: [x, y, width, height] })
  }

  fillRect(x: number, y: number, width: number, height: number) {
    this.calls.push({ method: 'fillRect', args: [x, y, width, height, this.fillStyle] })
  }

  moveTo(x: number, y: number) {
    this.calls.push({ method: 'moveTo', args: [x, y] })
  }

  lineTo(x: number, y: number) {
    this.calls.push({ method: 'lineTo', args: [x, y] })
  }

  stroke() {
    this.calls.push({ method: 'stroke', args: [this.strokeStyle, this.lineWidth] })
  }

  strokeRect(x: number, y: number, width: number, height: number) {
    this.calls.push({ method: 'strokeRect', args: [x, y, width, height, this.strokeStyle, this.lineWidth] })
  }

  fillText(text: string, x: number, y: number) {
    this.calls.push({ method: 'fillText', args: [text, x, y, this.fillStyle, this.font] })
  }
}

const createSnapshot = (overrides?: Partial<GameSnapshot>): GameSnapshot => ({
  tick: 12,
  width: 10,
  height: 8,
  snakes: {
    alice: [
      { x: 2, y: 3 },
      { x: 1, y: 3 },
    ],
    bob: [{ x: 7, y: 1 }],
  },
  food: { x: 5, y: 4 },
  scores: { alice: 4, bob: 1 },
  gameOver: false,
  players: ['alice', 'bob'],
  ...overrides,
})

const readFillRects = (ctx: MockCanvasRenderingContext2D) =>
  ctx.calls
    .filter((call) => call.method === 'fillRect')
    .map((call) => call.args as [number, number, number, number, string])

describe('GameCanvas render', () => {
  it('TestRenderDrawsGridAndBoundaryUsingSnapshotDimensions', () => {
    const snapshot = createSnapshot({ width: 12, height: 9 })
    const ctx = new MockCanvasRenderingContext2D(240, 180) as unknown as CanvasRenderingContext2D

    render(snapshot, ctx)

    const calls = (ctx as unknown as MockCanvasRenderingContext2D).calls
    const moveCalls = calls.filter((call) => call.method === 'moveTo')
    const lineCalls = calls.filter((call) => call.method === 'lineTo')
    const boundaryCall = calls.find((call) => call.method === 'strokeRect')

    expect(moveCalls.length).toBe(23)
    expect(lineCalls.length).toBe(23)
    expect(boundaryCall).toBeDefined()
    expect(boundaryCall?.args.slice(0, 4)).toEqual([0, 0, 240, 180])
  })

  it('TestRenderDrawsSnakeSegmentsAtExpectedGridPositions', () => {
    const snapshot = createSnapshot()
    const ctx = new MockCanvasRenderingContext2D(200, 160) as unknown as CanvasRenderingContext2D

    render(snapshot, ctx)

    const fillRects = readFillRects(ctx as unknown as MockCanvasRenderingContext2D)
    const [background, aliceHead, aliceBody, bobHead] = fillRects

    expect(background.slice(0, 4)).toEqual([0, 0, 200, 160])
    expect(aliceHead.slice(0, 4)).toEqual([41, 61, 18, 18])
    expect(aliceBody.slice(0, 4)).toEqual([21, 61, 18, 18])
    expect(bobHead.slice(0, 4)).toEqual([141, 21, 18, 18])
  })

  it('TestRenderDrawsFoodAtExpectedGridPosition', () => {
    const snapshot = createSnapshot()
    const ctx = new MockCanvasRenderingContext2D(200, 160) as unknown as CanvasRenderingContext2D

    render(snapshot, ctx)

    const fillRects = readFillRects(ctx as unknown as MockCanvasRenderingContext2D)
    const foodRect = fillRects[4]

    expect(foodRect.slice(0, 4)).toEqual([101, 81, 18, 18])
    expect(foodRect[4]).toBe('#ef4444')
  })

  it('TestRenderDrawsGameOverOverlayAndText', () => {
    const snapshot = createSnapshot({ gameOver: true })
    const ctx = new MockCanvasRenderingContext2D(200, 160) as unknown as CanvasRenderingContext2D

    render(snapshot, ctx)

    const calls = (ctx as unknown as MockCanvasRenderingContext2D).calls
    const fillRects = readFillRects(ctx as unknown as MockCanvasRenderingContext2D)
    const overlayRect = fillRects[5]
    const gameOverText = calls.find((call) => call.method === 'fillText')

    expect(overlayRect.slice(0, 4)).toEqual([0, 0, 200, 160])
    expect(gameOverText).toBeDefined()
    expect(gameOverText?.args[0]).toBe('Game Over')
    expect(gameOverText?.args[1]).toBe(100)
    expect(gameOverText?.args[2]).toBe(80)
  })
})
