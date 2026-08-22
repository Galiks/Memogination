import { onBeforeUnmount, ref } from 'vue'

/**
 * Best-effort Screen Wake Lock wrapper. Never throws; silently no-ops when the
 * API is unavailable or the request is rejected.
 */
export function useWakeLock() {
  const supported = typeof navigator !== 'undefined' && 'wakeLock' in navigator
  const isActive = ref(false)
  let sentinel: WakeLockSentinel | null = null

  async function request(): Promise<void> {
    if (!supported) return
    try {
      sentinel = await navigator.wakeLock.request('screen')
      isActive.value = true
      sentinel.addEventListener?.('release', () => {
        isActive.value = false
      })
    } catch {
      // best-effort: ignore failures
    }
  }

  async function release(): Promise<void> {
    if (sentinel) {
      try {
        await sentinel.release()
      } catch {
        // ignore
      }
      sentinel = null
    }
    isActive.value = false
  }

  onBeforeUnmount(() => {
    void release()
  })

  return { supported, isActive, request, release }
}