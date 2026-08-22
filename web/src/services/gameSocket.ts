import type { GameSnapshot } from '@/types/api'

export type SocketStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'closed'

interface SocketMessage {
  type: string
  revision?: number
  data?: unknown
  snapshot?: unknown
}

const RECONNECT_DELAYS_MS = [1000, 2000, 4000, 8000, 10000, 10000]

type SnapshotListener = (snapshot: GameSnapshot) => void
type StateUpdatedListener = (revision: number) => void
type StatusListener = (status: SocketStatus) => void

class GameSocket {
  private ws: WebSocket | null = null
  private roomCode: string | null = null
  private screen = false
  private reconnectAttempt = 0
  private reconnectTimer: number | null = null
  private closedByUser = false

  private snapshotListeners = new Set<SnapshotListener>()
  private stateUpdatedListeners = new Set<StateUpdatedListener>()
  private statusListeners = new Set<StatusListener>()

  connect(roomCode: string, options: { screen?: boolean } = {}): void {
    this.roomCode = roomCode
    this.screen = options.screen ?? false
    this.closedByUser = false
    this.reconnectAttempt = 0
    this.open()
  }

  close(): void {
    this.closedByUser = true
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
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

  private open(): void {
    if (!this.roomCode) return
    const query = this.screen ? '?screen=1' : ''
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/v1/rooms/${encodeURIComponent(this.roomCode)}/ws${query}`

    this.emitStatus(this.reconnectAttempt === 0 ? 'connecting' : 'reconnecting')
    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      this.reconnectAttempt = 0
      this.emitStatus('connected')
    }

    this.ws.onmessage = (event: MessageEvent) => {
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

    this.ws.onclose = () => {
      this.ws = null
      if (this.closedByUser) return
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      // onclose will follow and trigger reconnect
    }
  }

  private scheduleReconnect(): void {
    const delay = RECONNECT_DELAYS_MS[Math.min(this.reconnectAttempt, RECONNECT_DELAYS_MS.length - 1)]
    this.reconnectAttempt += 1
    this.emitStatus('reconnecting')
    this.reconnectTimer = window.setTimeout(() => this.open(), delay)
  }

  private emitStatus(status: SocketStatus): void {
    this.statusListeners.forEach((cb) => cb(status))
  }
}

export const gameSocket = new GameSocket()