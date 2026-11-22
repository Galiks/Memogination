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
  let resyncSequence = 0

  function setSnapshot(snap: GameSnapshot, expectedRoom = currentRoomCode.value): boolean {
    const incomingRoom = snap.room.code.toUpperCase()
    if (expectedRoom && incomingRoom !== expectedRoom.toUpperCase()) return false
    const existingRoom = snapshot.value?.room.code.toUpperCase()
    if (existingRoom === incomingRoom && snap.revision < revision.value) return false
    snapshot.value = snap
    revision.value = Math.max(revision.value, snap.revision)
    return true
  }

  function applyStateUpdated(newRevision: number): void {
    revision.value = Math.max(revision.value, newRevision)
  }

  async function resync(screen = false): Promise<void> {
    const room = currentRoomCode.value
    if (!room) return
    const sequence = ++resyncSequence
    const snap = await apiClient.getState(room, screen)
    if (sequence !== resyncSequence || currentRoomCode.value !== room) return
    setSnapshot(snap, room)
  }

  function reset(): void {
    resyncSequence += 1
    snapshot.value = null
    revision.value = 0
    sessionStatus.value = 'idle'
    currentRoomCode.value = null
  }

  function beginRoom(room: string): void {
    const normalized = room.toUpperCase()
    if (currentRoomCode.value === normalized) return
    if (currentRoomCode.value === null) {
      if (snapshot.value && snapshot.value.room.code.toUpperCase() !== normalized) reset()
      currentRoomCode.value = normalized
      return
    }
    reset()
    currentRoomCode.value = normalized
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
    beginRoom,
  }
})
