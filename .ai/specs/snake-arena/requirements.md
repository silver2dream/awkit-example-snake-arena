# SnakeArena - Requirements

## Goal

Build a minimal **server-authoritative** Snake arena game to validate the AWK workflow end-to-end (Spec → Implement → PR → Merge).

## Scope

### Gameplay
- Grid-based snake movement with a fixed tick rate.
- Food spawning.
- Collision rules:
  - Wall collision ends the game.
  - Snake body collision ends the game.
  - (Optional) vs opponent collision rules (later).

### Multiplayer
- One room supports 2 players.
- WebSocket for realtime input/state.
- Client sends input only (direction changes).
- Server is authoritative and broadcasts snapshots.

### Lobby
- Create room.
- Join room.
- Basic room status.

### Non-goals (for v0)
- Authentication.
- Spectator mode.
- Persistent storage.
- Anti-cheat beyond basic input validation.

## Acceptance Criteria (v0)
- Two browser tabs can join the same room and see the same game state.
- Server tick rate is stable and deterministic.
- CI passes on PRs to the integration branch.
