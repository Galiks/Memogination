import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { GameSnapshot } from '@/types/api'
import { apiClient } from '@/services/apiClient'

export type SessionStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting'

export const useGameSessionStore = defineStore('gameSession', () => {
  const snapshot = ref<GameSnapshot | null>(null)
  const revision = ref(0)
  const sessionStatus = ref<SessionStatus>('idle')
  const currentRoomCode = ref<string | null>(null)

  function setSnapshot(snap: GameSnapshot): void {
    snapshot.value = snap
    revision.value = snap.revision
  }

  function applyStateUpdated(newRevision: number): void {
    revision.value = newRevision
  }

  async function resync(): Promise<void> {
    if (!currentRoomCode.value) return
    const snap = await apiClient.getState(currentRoomCode.value)
    setSnapshot(snap)
  }

  function reset(): void {
    snapshot.value = null
    revision.value = 0
    sessionStatus.value = 'idle'
    currentRoomCode.value = null
  }

  return {
    snapshot,
    revision,
    sessionStatus,
    currentRoomCode,
    setSnapshot,
    applyStateUpdated,
    resync,
    reset,
  }
})