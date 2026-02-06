import type { IncomingMessage } from '../../shared/websocketClient'
import type { GameSnapshot, GridPoint } from './types'

const DEFAULT_GRID_WIDTH = 20
const DEFAULT_GRID_HEIGHT = 20

const asObject = (value: unknown): Record<string, unknown> | null =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null

const toNumber = (value: unknown): number | null => (typeof value === 'number' && Number.isFinite(value) ? value : null)

const toGridValue = (value: unknown, fallback: number) => {
  const parsed = toNumber(value)
  return parsed !== null && Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

const toInteger = (value: unknown, fallback: number) => {
  const parsed = toNumber(value)
  return parsed !== null ? Math.round(parsed) : fallback
}

const parseGridPoint = (value: unknown): GridPoint | null => {
  const point = asObject(value)
  if (!point) return null

  const x = toNumber(point.x ?? point.X)
  const y = toNumber(point.y ?? point.Y)
  if (x === null || y === null) return null

  return { x: Math.round(x), y: Math.round(y) }
}

const parseSnake = (value: unknown): GridPoint[] => {
  if (!Array.isArray(value)) return []
  return value.map((segment) => parseGridPoint(segment)).filter((segment): segment is GridPoint => segment !== null)
}

const parseSnakes = (value: unknown): Record<string, GridPoint[]> => {
  const snakes = asObject(value)
  if (!snakes) return {}

  return Object.entries(snakes).reduce<Record<string, GridPoint[]>>((result, [playerId, snake]) => {
    result[playerId] = parseSnake(snake)
    return result
  }, {})
}

const parseScores = (value: unknown): Record<string, number> => {
  const scores = asObject(value)
  if (!scores) return {}

  return Object.entries(scores).reduce<Record<string, number>>((result, [playerId, score]) => {
    result[playerId] = toInteger(score, 0)
    return result
  }, {})
}

const parsePlayers = (value: unknown): string[] => {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

const ensurePlayerEntries = (snapshot: GameSnapshot, players: string[]) => {
  players.forEach((playerId) => {
    if (!snapshot.snakes[playerId]) {
      snapshot.snakes[playerId] = []
    }
    if (snapshot.scores[playerId] === undefined) {
      snapshot.scores[playerId] = 0
    }
  })
}

const fromRoomSnapshot = (message: IncomingMessage): Partial<GameSnapshot> | null => {
  if (message.type !== 'room_snapshot') return null

  const players = parsePlayers(message.players)
  const snapshot: Partial<GameSnapshot> = {
    tick: toInteger(message.tick, 0),
    width: toGridValue(message.width, DEFAULT_GRID_WIDTH),
    height: toGridValue(message.height, DEFAULT_GRID_HEIGHT),
    food: parseGridPoint(message.food),
    players,
  }

  return snapshot
}

const fromTickUpdate = (message: IncomingMessage): Partial<GameSnapshot> | null => {
  if (message.type !== 'tick_update') return null

  return {
    tick: toInteger(message.tick, 0),
    snakes: parseSnakes(message.snakes),
    food: parseGridPoint(message.food),
    scores: parseScores(message.scores),
    gameOver: Boolean(message.gameOver),
  }
}

const toCompleteSnapshot = (state: Partial<GameSnapshot>): GameSnapshot => ({
  tick: state.tick ?? 0,
  width: toGridValue(state.width, DEFAULT_GRID_WIDTH),
  height: toGridValue(state.height, DEFAULT_GRID_HEIGHT),
  snakes: state.snakes ?? {},
  food: state.food ?? null,
  scores: state.scores ?? {},
  gameOver: state.gameOver ?? false,
  players: state.players,
})

export function applyIncomingMessage(previous: GameSnapshot | null, message: IncomingMessage): GameSnapshot | null {
  if (message.type === 'error') {
    return previous
  }

  const roomPatch = fromRoomSnapshot(message)
  if (roomPatch) {
    const next = toCompleteSnapshot({ ...previous, ...roomPatch })
    if (next.players?.length) {
      ensurePlayerEntries(next, next.players)
    }
    return next
  }

  const tickPatch = fromTickUpdate(message)
  if (tickPatch) {
    const next = toCompleteSnapshot({ ...previous, ...tickPatch })
    if (next.players?.length) {
      ensurePlayerEntries(next, next.players)
    }
    return next
  }

  return previous
}
