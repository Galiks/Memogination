import { onBeforeUnmount, onMounted, ref } from 'vue'

export function useReducedMotion() {
  const prefersReducedMotion = ref(false)
  let media: MediaQueryList | null = null
  const update = () => { prefersReducedMotion.value = !!media?.matches }
  onMounted(() => {
    if (typeof window.matchMedia !== 'function') return
    media = window.matchMedia('(prefers-reduced-motion: reduce)')
    update()
    media.addEventListener?.('change', update)
  })
  onBeforeUnmount(() => media?.removeEventListener?.('change', update))
  return { prefersReducedMotion }
}
