import type { GameSnapshot } from '@/types/api'

export type SocketStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'closed'

interface SocketMessage {
  type: string
  revision?: number
  data?: unknown
  snapshot?: unknown
}

const INITIAL_RECONNECT_DELAY_MS = 500
const MAX_RECONNECT_DELAY_MS = 10_000
const STABLE_OPEN_MS = 5_000
const JITTER_RATIO = 0.2

// Close code sent by the server when the player's session is revoked (kicked
// or left). Unlike a network drop this must NOT trigger reconnection.
export const SESSION_REVOKED_CLOSE_CODE = 4000

type SnapshotListener = (snapshot: GameSnapshot) => void
type StateUpdatedListener = (revision: number) => void
type StatusListener = (status: SocketStatus) => void
type SessionRevokedListener = () => void

class GameSocket {
  private ws: WebSocket | null = null
  private roomCode: string | null = null
  private screen = false
  private reconnectAttempt = 0
  private reconnectTimer: number | null = null
  private closedByUser = false
  private generation = 0
  private stableOpenTimer: number | null = null

  private snapshotListeners = new Set<SnapshotListener>()
  private stateUpdatedListeners = new Set<StateUpdatedListener>()
  private statusListeners = new Set<StatusListener>()
  private sessionRevokedListeners = new Set<SessionRevokedListener>()
  private probeListeners = new Set<SessionRevokedListener>()

  connect(roomCode: string, options: { screen?: boolean } = {}): void {
    const screen = options.screen ?? false
    if (this.roomCode === roomCode && this.screen === screen && !this.closedByUser &&
      (this.ws !== null || this.reconnectTimer !== null)) return

    this.generation += 1
    this.clearReconnectTimer()
    this.clearStableOpenTimer()
    const previous = this.ws
    this.ws = null
    if (previous) previous.close()
    this.roomCode = roomCode
    this.screen = screen
    this.closedByUser = false
    this.reconnectAttempt = 0
    this.open(this.generation)
  }

  close(): void {
    this.closedByUser = true
    this.generation += 1
    this.clearReconnectTimer()
    this.clearStableOpenTimer()
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.emitStatus('closed')
  }

  onSnapshot(cb: SnapshotListener): () => void {
    this.snapshotListeners.add(cb)
    return () => this.snapshotListeners.delete(cb)
  }

  onStateUpdated(cb: StateUpdatedListener): () => void {
    this.stateUpdatedListeners.add(cb)
    return () => this.stateUpdatedListeners.delete(cb)
  }

  onStatus(cb: StatusListener): () => void {
    this.statusListeners.add(cb)
    return () => this.statusListeners.delete(cb)
  }

  onSessionRevoked(cb: SessionRevokedListener): () => void {
    this.sessionRevokedListeners.add(cb)
    return () => this.sessionRevokedListeners.delete(cb)
  }

  onProbe(cb: SessionRevokedListener): () => void {
    this.probeListeners.add(cb)
    return () => this.probeListeners.delete(cb)
  }

  private open(generation: number): void {
    if (!this.roomCode) return
    if (generation !== this.generation || this.closedByUser) return
    const query = this.screen ? '?screen=1' : ''
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/v1/rooms/${encodeURIComponent(this.roomCode)}/ws${query}`

    this.emitStatus(this.reconnectAttempt === 0 ? 'connecting' : 'reconnecting')
    const ws = new WebSocket(url)
    this.ws = ws

    ws.onopen = () => {
      if (!this.isCurrent(ws, generation)) return
      this.clearStableOpenTimer()
      this.stableOpenTimer = window.setTimeout(() => {
        this.stableOpenTimer = null
        if (this.isCurrent(ws, generation)) this.reconnectAttempt = 0
      }, STABLE_OPEN_MS)
      this.emitStatus('connected')
    }

    ws.onmessage = (event: MessageEvent) => {
      if (!this.isCurrent(ws, generation)) return
      let msg: SocketMessage
      try {
        msg = JSON.parse(event.data as string) as SocketMessage
      } catch {
        return
      }
      if (msg.type === 'SNAPSHOT' && (msg.data || msg.snapshot)) {
        this.snapshotListeners.forEach((cb) => cb((msg.snapshot ?? msg.data) as GameSnapshot))
      } else if (msg.type === 'STATE_UPDATED') {
        this.stateUpdatedListeners.forEach((cb) => cb(msg.revision ?? 0))
      }
    }

    ws.onclose = (event: CloseEvent) => {
      if (!this.isCurrent(ws, generation)) return
      this.clearStableOpenTimer()
      this.ws = null
      if (this.closedByUser) return
      // The server closes the socket with this code when the session is
      // revoked (kick/leave): reconnecting is pointless because every
      // subsequent attempt would be rejected.
      if (event.code === SESSION_REVOKED_CLOSE_CODE) {
        this.emitStatus('closed')
        this.sessionRevokedListeners.forEach((cb) => cb())
        return
      }
      this.scheduleReconnect(generation)
    }

    ws.onerror = () => {
      // onclose will follow and trigger reconnect
    }
  }

  private scheduleReconnect(generation: number): void {
    if (generation !== this.generation || this.closedByUser || this.reconnectTimer !== null) return
    const baseDelay = Math.min(INITIAL_RECONNECT_DELAY_MS * 2 ** this.reconnectAttempt, MAX_RECONNECT_DELAY_MS)
    const jitter = 1 - JITTER_RATIO + Math.random() * JITTER_RATIO * 2
    const delay = Math.min(MAX_RECONNECT_DELAY_MS, Math.round(baseDelay * jitter))
    this.reconnectAttempt += 1
    this.emitStatus('reconnecting')
    // After a few consecutive failed reconnect attempts the connection may
    // never open again because the session was revoked while the socket was
    // down (the server rejects the upgrade, so no close frame is ever
    // delivered). Let subscribers probe the session via REST to surface that.
    if (this.reconnectAttempt >= 3) {
      this.probeListeners.forEach((cb) => cb())
    }
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.open(generation)
    }, delay)
  }

  private isCurrent(ws: WebSocket, generation: number): boolean {
    return generation === this.generation && ws === this.ws && !this.closedByUser
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer === null) return
    window.clearTimeout(this.reconnectTimer)
    this.reconnectTimer = null
  }

  private clearStableOpenTimer(): void {
    if (this.stableOpenTimer === null) return
    window.clearTimeout(this.stableOpenTimer)
    this.stableOpenTimer = null
  }

  private emitStatus(status: SocketStatus): void {
    this.statusListeners.forEach((cb) => cb(status))
  }
}

export const gameSocket = new GameSocket()
