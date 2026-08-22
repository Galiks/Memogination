import { describe, expect, it } from 'vitest'
import { useCountdown } from '@/composables/useCountdown'

describe('useCountdown', () => {
  it('computes remaining time from a deadline', () => {
    const deadline = Date.now() + 65_000
    const { remainingSeconds, formatted } = useCountdown(deadline)
    expect(remainingSeconds.value).toBeGreaterThan(0)
    expect(formatted.value).toMatch(/^\d+:\d{2}$/)
  })

  it('returns zero when no deadline is provided', () => {
    const { remainingMs, isExpired } = useCountdown(null)
    expect(remainingMs.value).toBe(0)
    expect(isExpired.value).toBe(true)
  })
})