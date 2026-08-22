import { onBeforeUnmount, ref } from 'vue'

/**
 * Fullscreen API wrapper. Best-effort; never throws.
 */
export function useFullscreen() {
  const supported = typeof document !== 'undefined' && 'fullscreenEnabled' in document
  const isFullscreen = ref(false)

  function sync(): void {
    isFullscreen.value = Boolean(document.fullscreenElement)
  }

  async function enter(): Promise<void> {
    if (!supported) return
    try {
      await document.documentElement.requestFullscreen()
      sync()
    } catch {
      // ignore
    }
  }

  async function exit(): Promise<void> {
    if (!supported) return
    try {
      if (document.fullscreenElement) await document.exitFullscreen()
      sync()
    } catch {
      // ignore
    }
  }

  async function toggle(): Promise<void> {
    if (document.fullscreenElement) await exit()
    else await enter()
  }

  document.addEventListener('fullscreenchange', sync)
  onBeforeUnmount(() => {
    document.removeEventListener('fullscreenchange', sync)
  })

  return { supported, isFullscreen, enter, exit, toggle }
}