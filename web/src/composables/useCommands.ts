import { ApiError, apiClient } from '@/services/apiClient'
import { useGameSessionStore } from '@/stores/gameSession'

/**
 * Generates a UUID v4 command id. crypto.randomUUID is only available in
 * secure contexts (HTTPS/localhost), so fall back to getRandomValues and
 * finally to Math.random for LAN HTTP games.
 */
export function newCommandId(): string {
  const c = globalThis.crypto
  if (c && typeof c.randomUUID === 'function') return c.randomUUID()

  const bytes = new Uint8Array(16)
  if (c && typeof c.getRandomValues === 'function') {
    c.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

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
      commandId: newCommandId(),
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