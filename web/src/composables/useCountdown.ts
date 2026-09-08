import { computed, onBeforeUnmount, onMounted, ref, toValue, type MaybeRefOrGetter } from 'vue'

/**
 * Counts down from a server-provided deadline timestamp (epoch ms).
 * Returns remaining milliseconds and a formatted mm:ss string. The deadline
 * may be reactive (ref/computed/getter) and is re-read on every tick.
 */
export function useCountdown(deadlineMs: MaybeRefOrGetter<number | null | undefined>) {
  const now = ref(Date.now())
  let timer: number | null = null

  function tick(): void {
    now.value = Date.now()
  }

  onMounted(() => {
    tick()
    timer = window.setInterval(tick, 250)
  })

  onBeforeUnmount(() => {
    if (timer !== null) window.clearInterval(timer)
  })

  const remainingMs = computed(() => {
    const deadline = toValue(deadlineMs)
    if (deadline == null) return 0
    return Math.max(0, deadline - now.value)
  })

  const remainingSeconds = computed(() => Math.ceil(remainingMs.value / 1000))

  const formatted = computed(() => {
    const total = remainingSeconds.value
    const minutes = Math.floor(total / 60)
    const seconds = total % 60
    return `${minutes}:${seconds.toString().padStart(2, '0')}`
  })

  const isExpired = computed(() => remainingMs.value <= 0)

  return { remainingMs, remainingSeconds, formatted, isExpired }
}