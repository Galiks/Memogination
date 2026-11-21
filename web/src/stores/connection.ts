import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ApiError, apiClient } from '@/services/apiClient'
import { gameSocket, type SocketStatus } from '@/services/gameSocket'
import { useGameSessionStore } from '@/stores/gameSession'

// Session-invalidating error codes: after them the player is no longer part of
// the room (kicked, left, or the session expired) and the UI must return to
// the join form.
const SESSION_LOST_CODES = new Set(['INVALID_SESSION', 'PLAYER_NOT_FOUND', 'PLAYER_ALREADY_LEFT'])

export const useConnectionStore = defineStore('connection', () => {
  const status = ref<SocketStatus>('idle')
  const roomCode = ref<string | null>(null)
  // True when this connection is the read-only public screen (uses the screen
  // projection and must resync via ?screen=1).
  const screen = ref(false)
  const sessionLostListeners = new Set<() => void>()

  function onSessionLost(cb: () => void): () => void {
    sessionLostListeners.add(cb)
    return () => sessionLostListeners.delete(cb)
  }

  function notifySessionLost(): void {
    gameSocket.close()
    sessionLostListeners.forEach((cb) => cb())
  }

  function connect(code: string, options: { screen?: boolean } = {}): void {
    roomCode.value = code
    screen.value = options.screen ?? false
    const session = useGameSessionStore()
    session.currentRoomCode = code
    session.sessionStatus = options.screen ? 'connected' : 'connecting'
    gameSocket.connect(code, options)
  }

  function disconnect(): void {
    gameSocket.close()
    roomCode.value = null
    screen.value = false
    status.value = 'idle'
    const session = useGameSessionStore()
    session.sessionStatus = 'idle'
  }

  // Wire socket events into the session store.
  gameSocket.onStatus((s) => {
    status.value = s
    const session = useGameSessionStore()
    if (s === 'connected') session.sessionStatus = 'connected'
    else if (s === 'reconnecting') session.sessionStatus = 'reconnecting'
    else if (s === 'connecting') session.sessionStatus = 'connecting'
  })

  gameSocket.onSnapshot((snap) => {
    useGameSessionStore().setSnapshot(snap)
  })

  // The server closes the socket with the session-revoked code when the player
  // is kicked or leaves; the client must stop reconnecting and return to the
  // join screen.
  gameSocket.onSessionRevoked(() => {
    notifySessionLost()
  })

  // If reconnects keep failing without the socket ever opening, the session may
  // have been revoked while the connection was down (a revoked session makes
  // the server reject the upgrade, so no close frame is delivered). Probe the
  // session over REST so a revocation still surfaces as session-lost.
  gameSocket.onProbe(() => {
    if (screen.value || !roomCode.value) return
    void apiClient
      .reconnect(roomCode.value)
      .then((snap) => useGameSessionStore().setSnapshot(snap))
      .catch((err: unknown) => {
        if (err instanceof ApiError && SESSION_LOST_CODES.has(err.code)) notifySessionLost()
      })
  })

  gameSocket.onStateUpdated((revision) => {
    const session = useGameSessionStore()
    session.applyStateUpdated(revision)
    void session.resync(screen.value).catch((err: unknown) => {
      if (err instanceof ApiError && SESSION_LOST_CODES.has(err.code)) notifySessionLost()
    })
  })

  return { status, roomCode, screen, connect, disconnect, onSessionLost }
})