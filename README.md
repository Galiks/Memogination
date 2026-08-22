# Memomarium

Memomarium is a local multiplayer party game. Players submit memes for a
situation, guess which meme the active player chose, and score points across
rounds and cycles. It runs on a single machine over the local network: one
person hosts the game on their computer and everyone else joins from their
phones.

## Architecture

- **Backend**: a Go modular monolith. The game engine is an event-sourced
  aggregate (`internal/engine`), orchestrated by the coordinator
  (`internal/coordinator`), persisted to SQLite (`internal/repository/sqlite`),
  and exposed over a REST API (`/api/v1`) plus a WebSocket for live updates
  (`/api/v1/rooms/{code}/ws`). The frontend is embedded into the binary via
  `web/dist`.
- **Frontend**: Vue 3 + TypeScript + Vite in `web/`. Views: `/host` (admin
  panel), `/screen` (public display), `/play/:roomCode` (player client).

## Run locally (development)

```sh
go run ./cmd/memomarium --data-dir ./data
```

The server listens on `:8080` by default. Open `http://localhost:8080/host` to
create a room; players join at `http://<your-lan-ip>:8080/play/<code>`.

## Build the Windows executable

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

This installs frontend dependencies, builds the frontend, and cross-compiles
`dist\Memomarium.exe` (Windows amd64). On Linux/macOS use `scripts/build.sh`,
which produces `dist/memomarium` for the current platform.

## Tests

Backend unit and integration tests:

```sh
go test ./...
```

Frontend unit tests (Vitest):

```sh
cd web && npm test
```

End-to-end tests (Playwright). These build and run the server on port 18099
with a throwaway data dir, then drive a full game through the browser:

```sh
cd web
npx playwright install chromium   # first time only
npm run e2e
```

## Tech stack

- Go 1.26, SQLite (modernc.org/sqlite), chi router, coder/websocket
- Vue 3, TypeScript, Vite, Pinia, Tailwind CSS, Playwright, Vitest