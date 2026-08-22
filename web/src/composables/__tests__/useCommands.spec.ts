import { describe, expect, it } from 'vitest'
import { newCommandId } from '@/composables/useCommands'

const UUID_V4_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

describe('newCommandId', () => {
  it('returns a UUID v4 with crypto.randomUUID available', () => {
    expect(newCommandId()).toMatch(UUID_V4_RE)
  })

  it('falls back when crypto.randomUUID is unavailable', () => {
    const c = globalThis.crypto as (Crypto & { randomUUID?: () => string }) | undefined
    if (!c) {
      expect(newCommandId()).toMatch(UUID_V4_RE)
      return
    }
    const original = c.randomUUID
    Object.defineProperty(c, 'randomUUID', { value: undefined, configurable: true })
    try {
      expect(newCommandId()).toMatch(UUID_V4_RE)
    } finally {
      if (original) {
        Object.defineProperty(c, 'randomUUID', { value: original, configurable: true })
      } else {
        delete (c as { randomUUID?: unknown }).randomUUID
      }
    }
  })
})