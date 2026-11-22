import { onBeforeUnmount, ref } from 'vue'

/**
 * Best-effort Screen Wake Lock wrapper. Never throws; silently no-ops when the
 * API is unavailable or the request is rejected.
 */
export function useWakeLock() {
  const supported = typeof navigator !== 'undefined' && 'wakeLock' in navigator
  const isActive = ref(false)
  let requested = false
  let sentinel: WakeLockSentinel | null = null
  let requestPending = false
  let requestGeneration = 0

  async function request(): Promise<void> {
    requested = true
    if (!supported || requestPending || document.visibilityState !== 'visible' || isActive.value) return
    requestPending = true
    const generation = requestGeneration
    try {
      const acquired = await navigator.wakeLock.request('screen')
      if (generation !== requestGeneration || !requested) {
        await acquired.release()
        return
      }
      sentinel = acquired
      isActive.value = true
      sentinel.addEventListener?.('release', () => {
        isActive.value = false
        sentinel = null
      })
    } catch {
      // best-effort: ignore failures
    } finally {
      requestPending = false
    }
  }

  async function release(): Promise<void> {
    requested = false
    requestGeneration += 1
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

  const onVisibilityChange = () => {
    if (requested && document.visibilityState === 'visible') void request()
  }
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', onVisibilityChange)

  onBeforeUnmount(() => {
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', onVisibilityChange)
    void release()
  })

  return { supported, isActive, request, release }
}
