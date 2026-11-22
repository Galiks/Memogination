import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePreferencesStore } from '@/stores/preferences'

describe('preferences persistence', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('restores defaults when stored JSON is corrupt and persists volume/mute changes', () => {
    localStorage.setItem('memomarium.ui.preferences.v1', '{broken')
    const preferences = usePreferencesStore()
    expect(preferences.muted).toBe(false)
    expect(preferences.sfxVolume).toBe(0.8)
    preferences.muted = true
    preferences.sfxVolume = 0.3
    expect(JSON.parse(localStorage.getItem('memomarium.ui.preferences.v1') ?? '{}')).toMatchObject({ muted: true, sfxVolume: 0.3 })
  })

  it('clamps restored volume values to the supported range', () => {
    localStorage.setItem('memomarium.ui.preferences.v1', JSON.stringify({ musicVolume: 4 }))
    const preferences = usePreferencesStore()
    expect(preferences.musicVolume).toBe(1)
  })
})
