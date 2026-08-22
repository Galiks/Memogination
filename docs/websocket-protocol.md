# Memomarium WebSocket Protocol

The WebSocket endpoint provides live state updates for a room. Commands are
sent over the REST API (`POST /api/v1/rooms/{code}/commands`); the WebSocket is
primarily a push channel for state changes.

## Endpoint

```
GET /api/v1/rooms/{code}/ws
```

Authentication is one of:

- **Player**: a valid `memomarium_session` cookie (set by `/join` or
  `/reconnect`).
- **Admin**: a valid `memomarium_admin` cookie (set by `/admin/bootstrap`).
- **Screen (read-only)**: the `?screen=1` query parameter. No player session is
  required. Screen clients only ever receive the public `ScreenSnapshot` and
  cannot send commands. This is the simplest way to drive a shared TV/projector
  display.

## Message framing

All messages are JSON text frames.

### Server → Client

#### `SNAPSHOT`

Sent immediately after the connection is established. Contains the
viewer-appropriate projection.

```json
{
  "type": "SNAPSHOT",
  "snapshot": { "revision": 3, "phase": "ROUND_VOTING", "...": "..." }
}
```

The projection depends on the viewer:

| Viewer  | Projection      | Notes                                   |
| ------- | --------------- | --------------------------------------- |
| Player  | PlayerSnapshot  | Includes private hand / prepared turn.  |
| Admin   | HostSnapshot    | Admin controls, no hidden answers.      |
| Screen  | ScreenSnapshot  | Public data only, no private hands.     |

#### `STATE_UPDATED`

Broadcast to every client in the room whenever the room state changes (a
command is applied or a phase times out).

```json
{
  "type": "STATE_UPDATED",
  "revision": 4
}
```

Clients should re-fetch the full snapshot via
`GET /api/v1/rooms/{code}/state` (or rely on the next `SNAPSHOT`) after
receiving this message.

### Client → Server

Inbound messages are currently ignored. All game actions are performed through
the REST command endpoint.

## Reconnect & backoff

- If the WebSocket drops, the client should reconnect to the same endpoint.
- On reconnect, the client receives a fresh `SNAPSHOT`, so it can resynchronize
  without a separate state fetch.
- Use exponential backoff with jitter for reconnect attempts, e.g. start at
  500 ms and double up to a 10 s cap, resetting the backoff after a successful
  connection that stays open for a few seconds.
- The `memomarium_session` cookie is required for player reconnects; it is
  preserved across reconnects and is also used by
  `POST /api/v1/rooms/{code}/reconnect` to mark the player connected again.

## Concurrency / revision handling

- Commands carry an `expectedRevision`. If the room has advanced past it, the
  REST endpoint returns `409 STATE_CHANGED` with the current revision. The
  client should re-fetch state and retry.
- `STATE_UPDATED` messages are best-effort notifications; always treat the
  snapshot from `GET /state` (or the next `SNAPSHOT`) as authoritative.