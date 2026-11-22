import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useGameSessionStore } from '@/stores/gameSession'
import { apiClient } from '@/services/apiClient'
import type { GameSnapshot } from '@/types/api'

function snapshot(code: string, revision: number): GameSnapshot {
  return { room: { code, id: code, revision, state: 'LOBBY' }, revision } as GameSnapshot
}

describe('game session snapshot ordering', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('ignores an older same-room snapshot but accepts an equal revision', () => {
    const session = useGameSessionStore()
    session.beginRoom('A1')
    expect(session.setSnapshot(snapshot('A1', 106))).toBe(true)
    expect(session.setSnapshot(snapshot('A1', 105))).toBe(false)
    expect(session.setSnapshot(snapshot('A1', 106))).toBe(true)
    expect(session.snapshot?.revision).toBe(106)
  })

  it('ignores a late resync response after a room switch', async () => {
    const session = useGameSessionStore()
    session.beginRoom('A1')
    let resolveOld!: (value: GameSnapshot) => void
    vi.spyOn(apiClient, 'getState').mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    const pending = session.resync()
    session.beginRoom('B2')
    resolveOld(snapshot('A1', 200))
    await pending
    expect(session.currentRoomCode).toBe('B2')
    expect(session.snapshot).toBeNull()
  })

  it('keeps revision monotonic when update notifications arrive out of order', () => {
    const session = useGameSessionStore()
    session.applyStateUpdated(18)
    session.applyStateUpdated(17)
    expect(session.revision).toBe(18)
  })
})
