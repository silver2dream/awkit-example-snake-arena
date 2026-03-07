import type { IncomingMessage } from '../../shared/websocketClient'
import type { GameSnapshot, GridPoint } from './types'

const DEFAULT_GRID_WIDTH = 20
const DEFAULT_GRID_HEIGHT = 20

const asObject = (value: unknown): Record<string, unknown> | null =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null

const toNumber = (value: unknown): number | null => (typeof value === 'number' && Number.isFinite(value) ? value : null)

const toInteger = (value: unknown, fallback: number) => {
  const parsed = toNumber(value)
  return parsed !== null ? Math.round(parsed) : fallback
}

const toOptionalInteger = (value: unknown): number | null => {
  const parsed = toNumber(value)
  return parsed !== null ? Math.round(parsed) : null
}

const toOptionalGridValue = (value: unknown): number | null => {
  const parsed = toNumber(value)
  return parsed !== null && Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

const toGridValue = (value: unknown, fallback: number) => {
  const parsed = toOptionalGridValue(value)
  return parsed ?? fallback
}

const toOptionalString = (value: unknown): string | null => {
  if (typeof value !== 'string') {
    return null
  }

  const trimmed = value.trim()
  return trimmed.length > 0 ? trimmed : null
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

interface ParsedPlayers {
  order: string[]
  snakes: Record<string, GridPoint[]>
  scores: Record<string, number>
  aliveStates: boolean[]
}

const parsePlayers = (value: unknown): ParsedPlayers => {
  const parsed: ParsedPlayers = {
    order: [],
    snakes: {},
    scores: {},
    aliveStates: [],
  }

  if (!Array.isArray(value)) {
    return parsed
  }

  value.forEach((playerEntry) => {
    if (typeof playerEntry === 'string') {
      parsed.order.push(playerEntry)
      return
    }

    const player = asObject(playerEntry)
    if (!player) {
      return
    }

    const playerKey = toOptionalString(player.name) ?? toOptionalString(player.id)
    if (!playerKey) {
      return
    }

    parsed.order.push(playerKey)
    parsed.snakes[playerKey] = parseSnake(player.body)
    parsed.scores[playerKey] = toInteger(player.score, 0)

    if (typeof player.alive === 'boolean') {
      parsed.aliveStates.push(player.alive)
    }
  })

  parsed.order = Array.from(new Set(parsed.order))
  return parsed
}

const resolveSnapshotPayload = (message: IncomingMessage): Record<string, unknown> | null => {
  if (message.type === 'error') {
    return null
  }

  return asObject(message.snapshot)
}

const fromSnapshotMessage = (message: IncomingMessage): Partial<GameSnapshot> | null => {
  if (message.type !== 'room_snapshot' && message.type !== 'tick_update') {
    return null
  }

  const payload = resolveSnapshotPayload(message)
  if (!payload) {
    return null
  }

  const players = parsePlayers(payload.players)
  const patch: Partial<GameSnapshot> = {
    players: players.order,
    snakes: {
      ...parseSnakes(payload.snakes),
      ...players.snakes,
    },
    scores: {
      ...parseScores(payload.scores),
      ...players.scores,
    },
    food: parseGridPoint(payload.food),
  }

  const tick = toOptionalInteger(payload.tick)
  if (tick !== null) {
    patch.tick = tick
  }

  const width = toOptionalGridValue(payload.width)
  if (width !== null) {
    patch.width = width
  }

  const height = toOptionalGridValue(payload.height)
  if (height !== null) {
    patch.height = height
  }

  if (typeof payload.gameOver === 'boolean') {
    patch.gameOver = payload.gameOver
  } else if (players.aliveStates.length > 0) {
    patch.gameOver = players.aliveStates.every((alive) => !alive)
  }

  return patch
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

export function applyIncomingMessage(previous: GameSnapshot | null, message: IncomingMessage): GameSnapshot | null {
  if (message.type === 'error') {
    return previous
  }

  const patch = fromSnapshotMessage(message)
  if (!patch) {
    return previous
  }

  const next = toCompleteSnapshot({ ...previous, ...patch })
  if (next.players?.length) {
    ensurePlayerEntries(next, next.players)
  }

  return next
}
