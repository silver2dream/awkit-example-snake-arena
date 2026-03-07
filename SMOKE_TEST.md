# Snake Arena Manual Smoke Test

Use this guide to run the backend, open two browser tabs, and verify two-player gameplay for Snake Arena.

## Prerequisites
- Go 1.22+ installed.
- Node.js 18+ and npm installed.
- Two terminals and two browser tabs (use an incognito/private window or a different browser for player 2 so localStorage does not reuse the same player id).
- Ports 8080 (backend) and 5173 (frontend) available.

## Start the backend server
1. In Terminal A, start the backend:
   ```bash
   cd backend
   go test ./...
   go run ./main.go
   ```
   The dev server should stay running and listen on `http://localhost:8080` for `/api` and `/ws/room/:roomId`. If your team uses a different entrypoint than `main.go`, start that binary instead; the lobby UI expects the same `/api` + `/ws` paths on one origin.
2. Keep this terminal open. If the process exits immediately or only prints `Hello, World!`, launch the actual HTTP/WebSocket server used for gameplay before continuing.

## Start the frontend
1. In Terminal B, start the Vite dev server:
   ```bash
   cd frontend
   npm install
   npm run dev -- --host --port 5173
   ```
2. Open `http://localhost:5173` in your browser.

## Create a room (player 1)
1. In Tab A (regular window), open the lobby.
2. Enter a room name and click **Create room**.
3. Expected:
   - A status message like `Created room room-1`.
   - The room appears in the list with players count `1`.
   - Connection indicator moves to `connecting` then `connected`.
4. Copy the room ID for the second tab.

## Join from a second tab (player 2)
1. Open Tab B in a private/incognito window (or different browser).
2. Load `http://localhost:5173`.
3. Paste the room ID into **Room ID** and click **Join room**.
4. Expected:
   - Status message `Joined room <id>`.
   - Both tabs show the same room ID and player count increments to `2`.
   - Both WebSocket connections report `connected`.

## Verify gameplay sync
1. In Tab A, use arrow keys to move; Tab B should mirror the same snake position and direction updates within a tick.
2. Steer a snake into the food:
   - Food disappears and respawns elsewhere.
   - The moving snake grows and the score/length updates in both tabs.
3. Force a collision (wall or self):
   - Game over is broadcast to both tabs.
   - No further `tick_update` events are received until a new room is created.
4. Optional: refresh the rooms list; the current room should persist while players are connected.

## Troubleshooting
- **API calls fail / 404**: ensure the backend server exposes `/api/rooms` and `/api/rooms/{id}/join` on the same origin as the frontend (configure a proxy if ports differ).
- **WebSocket rejected**: check the backend is listening on `/ws/room/{id}` and that the port is open; avoid corporate VPN/firewall blocks.
- **Second tab reuses player 1**: open Tab B in private mode to get a separate `playerId` stored in localStorage.
- **Inputs ignored**: the server rate limits noisy clients; avoid key spamming and watch the backend logs for validation errors.
- **No ticks or stale state**: restart both processes to clear in-memory rooms, then create and join a new room.
