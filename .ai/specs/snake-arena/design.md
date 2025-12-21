# SnakeArena - Design

## Architecture

- `frontend/` (React + TypeScript + Vite)
  - Canvas renderer
  - WebSocket client
  - Lobby UI

- `backend/` (Go)
  - HTTP endpoints (health, lobby)
  - WebSocket endpoint for game rooms
  - In-memory room manager
  - Deterministic tick engine

## Protocol (draft)

All messages are JSON and must include a `type`.

### Client → Server
- `join_room`: `{ type, roomId, playerName }`
- `input`: `{ type, direction, seq }` where `direction ∈ {up,down,left,right}`

### Server → Client
- `room_snapshot`: `{ type, roomId, tick, players, food }`
- `tick_update`: `{ type, roomId, tick, snakes, food, scores }`
- `error`: `{ type, message }`

## Tick Engine

- Fixed tick rate (e.g. 10 ticks/sec).
- Inputs are buffered per player and applied on next tick.
- Movement step is deterministic.

## Testing

- Backend:
  - Unit tests for collision and tick advance.
- Frontend:
  - Smoke test rendering `<App />`.

## Verification

- AWK: `bash .ai/scripts/evaluate.sh --offline`
- Kit tests: `bash .ai/tests/run_all_tests.sh`
- Backend: `go test ./...` (in `backend/`)
- Frontend: `npm run test -- --run` and `npm run build` (in `frontend/`)
