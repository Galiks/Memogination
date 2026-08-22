import { defineStore } from 'pinia'
import { ref } from 'vue'
import { gameSocket, type SocketStatus } from '@/services/gameSocket'
import { useGameSessionStore } from '@/stores/gameSession'

export const useConnectionStore = defineStore('connection', () => {
  const status = ref<SocketStatus>('idle')
  const roomCode = ref<string | null>(null)

  function connect(code: string, options: { screen?: boolean } = {}): void {
    roomCode.value = code
    const session = useGameSessionStore()
    session.currentRoomCode = code
    session.sessionStatus = options.screen ? 'connected' : 'connecting'
    gameSocket.connect(code, options)
  }

  function disconnect(): void {
    gameSocket.close()
    roomCode.value = null
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

  gameSocket.onStateUpdated((revision) => {
    const session = useGameSessionStore()
    session.applyStateUpdated(revision)
    void session.resync()
  })

  return { status, roomCode, connect, disconnect }
})