import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { audioManager } from '@/services/audioManager'
import { usePreferencesStore } from '@/stores/preferences'

class MockAudioContext {
  state = 'suspended'
  currentTime = 0
  destination = {}
  async resume() { this.state = 'running' }
  createOscillator() {
    return { type: 'sine', frequency: { value: 0 }, onended: null as (() => void) | null, connect: vi.fn(), start: vi.fn(), stop: vi.fn() }
  }
  createGain() {
    return { gain: { setValueAtTime: vi.fn(), exponentialRampToValueAtTime: vi.fn() }, connect: vi.fn() }
  }
}

describe('audioManager', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    audioManager.unlocked.value = false
    vi.stubGlobal('AudioContext', MockAudioContext)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('unlocks after a user gesture and deduplicates the same event key', async () => {
    expect(await audioManager.unlock()).toBe(true)
    expect(audioManager.play('phase', { dedupeKey: 'room:10:ROUND_SELECTION' })).toBe(true)
    expect(audioManager.play('phase', { dedupeKey: 'room:10:ROUND_SELECTION' })).toBe(false)
  })

  it('treats blocked audio and muted or zero-volume playback as nonfatal', async () => {
    expect(audioManager.play('success')).toBe(false)
    await audioManager.unlock()
    const preferences = usePreferencesStore()
    preferences.muted = true
    expect(audioManager.play('success')).toBe(false)
    preferences.muted = false
    audioManager.setSfxVolume(0)
    expect(audioManager.play('success')).toBe(false)
    expect(preferences.sfxVolume).toBe(0)
  })
})
