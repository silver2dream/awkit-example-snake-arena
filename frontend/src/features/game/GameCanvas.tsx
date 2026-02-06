import { useEffect, useMemo, useRef } from 'react'
import type { GameSnapshot, GridPoint } from './types'

const DEFAULT_GRID_WIDTH = 20
const DEFAULT_GRID_HEIGHT = 20
const CELL_SIZE_PX = 24
const CELL_INSET_PX = 1

const BACKGROUND_COLOR = '#0f172a'
const GRID_LINE_COLOR = 'rgba(148, 163, 184, 0.28)'
const GRID_BORDER_COLOR = '#64748b'
const FOOD_COLOR = '#ef4444'
const GAME_OVER_OVERLAY_COLOR = 'rgba(15, 23, 42, 0.65)'
const GAME_OVER_TEXT_COLOR = '#f8fafc'

const PLAYER_COLORS = ['#22c55e', '#38bdf8', '#f59e0b', '#a78bfa', '#f97316', '#14b8a6']
const PLAYER_HEAD_COLORS = ['#16a34a', '#0284c7', '#d97706', '#8b5cf6', '#ea580c', '#0d9488']

const toGridDimension = (value: number, fallback: number) =>
  Number.isInteger(value) && value > 0 ? value : fallback

const resolvePlayerOrder = (snapshot: GameSnapshot): string[] => {
  const ordered = [...(snapshot.players ?? []), ...Object.keys(snapshot.scores), ...Object.keys(snapshot.snakes)]
  return Array.from(new Set(ordered))
}

const drawCell = (
  ctx: CanvasRenderingContext2D,
  point: GridPoint,
  color: string,
  gridWidth: number,
  gridHeight: number,
  cellWidth: number,
  cellHeight: number,
) => {
  if (point.x < 0 || point.y < 0 || point.x >= gridWidth || point.y >= gridHeight) {
    return
  }

  const insetX = Math.min(CELL_INSET_PX, cellWidth / 3)
  const insetY = Math.min(CELL_INSET_PX, cellHeight / 3)

  ctx.fillStyle = color
  ctx.fillRect(
    point.x * cellWidth + insetX,
    point.y * cellHeight + insetY,
    Math.max(1, cellWidth - insetX * 2),
    Math.max(1, cellHeight - insetY * 2),
  )
}

export function render(snapshot: GameSnapshot, ctx: CanvasRenderingContext2D) {
  const gridWidth = toGridDimension(snapshot.width, DEFAULT_GRID_WIDTH)
  const gridHeight = toGridDimension(snapshot.height, DEFAULT_GRID_HEIGHT)
  const canvasWidth = ctx.canvas.width
  const canvasHeight = ctx.canvas.height
  const cellWidth = canvasWidth / gridWidth
  const cellHeight = canvasHeight / gridHeight

  ctx.clearRect(0, 0, canvasWidth, canvasHeight)
  ctx.fillStyle = BACKGROUND_COLOR
  ctx.fillRect(0, 0, canvasWidth, canvasHeight)

  ctx.beginPath()
  for (let x = 0; x <= gridWidth; x += 1) {
    const px = x * cellWidth
    ctx.moveTo(px, 0)
    ctx.lineTo(px, canvasHeight)
  }

  for (let y = 0; y <= gridHeight; y += 1) {
    const py = y * cellHeight
    ctx.moveTo(0, py)
    ctx.lineTo(canvasWidth, py)
  }

  ctx.strokeStyle = GRID_LINE_COLOR
  ctx.lineWidth = 1
  ctx.stroke()

  ctx.strokeStyle = GRID_BORDER_COLOR
  ctx.lineWidth = 2
  ctx.strokeRect(0, 0, canvasWidth, canvasHeight)

  const playerIds = resolvePlayerOrder(snapshot)
  playerIds.forEach((playerId, playerIndex) => {
    const snake = snapshot.snakes[playerId] ?? []
    const bodyColor = PLAYER_COLORS[playerIndex % PLAYER_COLORS.length]
    const headColor = PLAYER_HEAD_COLORS[playerIndex % PLAYER_HEAD_COLORS.length]

    snake.forEach((segment, segmentIndex) => {
      drawCell(
        ctx,
        segment,
        segmentIndex === 0 ? headColor : bodyColor,
        gridWidth,
        gridHeight,
        cellWidth,
        cellHeight,
      )
    })
  })

  if (snapshot.food) {
    drawCell(ctx, snapshot.food, FOOD_COLOR, gridWidth, gridHeight, cellWidth, cellHeight)
  }

  if (snapshot.gameOver) {
    ctx.fillStyle = GAME_OVER_OVERLAY_COLOR
    ctx.fillRect(0, 0, canvasWidth, canvasHeight)
    ctx.fillStyle = GAME_OVER_TEXT_COLOR
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.font = 'bold 28px system-ui, -apple-system, sans-serif'
    ctx.fillText('Game Over', canvasWidth / 2, canvasHeight / 2)
  }
}

export interface GameCanvasProps {
  snapshot: GameSnapshot | null
}

function GameCanvas({ snapshot }: GameCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const gridWidth = toGridDimension(snapshot?.width ?? DEFAULT_GRID_WIDTH, DEFAULT_GRID_WIDTH)
  const gridHeight = toGridDimension(snapshot?.height ?? DEFAULT_GRID_HEIGHT, DEFAULT_GRID_HEIGHT)

  useEffect(() => {
    if (!snapshot) return

    const canvas = canvasRef.current
    if (!canvas) return

    const context = canvas.getContext('2d')
    if (!context) return

    render(snapshot, context)
  }, [snapshot])

  const scoreEntries = useMemo(() => {
    if (!snapshot) return []

    return resolvePlayerOrder(snapshot)
      .map((playerId) => ({ playerId, score: snapshot.scores[playerId] ?? 0 }))
      .sort((left, right) => right.score - left.score || left.playerId.localeCompare(right.playerId))
  }, [snapshot])

  return (
    <div style={{ display: 'grid', gap: '0.75rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <strong>Live arena</strong>
        <span style={{ color: '#475569' }}>{snapshot ? `Tick ${snapshot.tick}` : 'Waiting for server snapshot…'}</span>
      </div>

      <canvas
        id="game-canvas"
        ref={canvasRef}
        width={gridWidth * CELL_SIZE_PX}
        height={gridHeight * CELL_SIZE_PX}
        style={{
          width: gridWidth * CELL_SIZE_PX,
          height: gridHeight * CELL_SIZE_PX,
          maxWidth: '100%',
          border: '1px solid #cbd5e1',
          borderRadius: 8,
          background: BACKGROUND_COLOR,
        }}
        aria-label="Snake arena game board"
      />

      <div style={{ display: 'grid', gap: '0.35rem' }}>
        <div style={{ fontWeight: 600 }}>Scores</div>
        {scoreEntries.length === 0 ? (
          <div style={{ color: '#64748b' }}>Waiting for players…</div>
        ) : (
          scoreEntries.map((entry) => (
            <div key={entry.playerId} style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>{entry.playerId}</span>
              <strong>{entry.score}</strong>
            </div>
          ))
        )}

        {snapshot?.gameOver ? (
          <div role="status" style={{ color: '#b91c1c', fontWeight: 700 }}>
            Game over
          </div>
        ) : null}
      </div>
    </div>
  )
}

export default GameCanvas
