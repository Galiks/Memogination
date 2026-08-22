import { ApiError, apiClient } from '@/services/apiClient'
import { useGameSessionStore } from '@/stores/gameSession'

/**
 * Sends a game command using the current snapshot revision as expectedRevision.
 * On a STATE_CHANGED conflict it resyncs and retries once.
 */
export async function sendCommand(type: string, payload?: Record<string, unknown>): Promise<void> {
  const session = useGameSessionStore()
  const code = session.currentRoomCode
  if (!code || !session.snapshot) return

  const attempt = async (): Promise<void> => {
    const res = await apiClient.sendCommand(code, {
      commandId: crypto.randomUUID(),
      expectedRevision: session.revision,
      type,
      payload,
    })
    session.setSnapshot(res.snapshot)
  }

  try {
    await attempt()
  } catch (err) {
    if (err instanceof ApiError && err.code === 'STATE_CHANGED') {
      await session.resync()
      await attempt()
      return
    }
    throw err
  }
}