import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const STORAGE_KEY = 'memomarium.ui.preferences.v1'
const defaults = { muted: false, masterVolume: 0.8, musicVolume: 0.45, sfxVolume: 0.8, reduceEffects: null as boolean | null }
type Preferences = typeof defaults

function loadPreferences(): Preferences {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Partial<Preferences>
    return {
      muted: typeof parsed.muted === 'boolean' ? parsed.muted : defaults.muted,
      masterVolume: typeof parsed.masterVolume === 'number' ? Math.max(0, Math.min(1, parsed.masterVolume)) : defaults.masterVolume,
      musicVolume: typeof parsed.musicVolume === 'number' ? Math.max(0, Math.min(1, parsed.musicVolume)) : defaults.musicVolume,
      sfxVolume: typeof parsed.sfxVolume === 'number' ? Math.max(0, Math.min(1, parsed.sfxVolume)) : defaults.sfxVolume,
      reduceEffects: typeof parsed.reduceEffects === 'boolean' ? parsed.reduceEffects : null,
    }
  } catch {
    return { ...defaults }
  }
}

export const usePreferencesStore = defineStore('preferences', () => {
  const initial = loadPreferences()
  const muted = ref(initial.muted)
  const masterVolume = ref(initial.masterVolume)
  const musicVolume = ref(initial.musicVolume)
  const sfxVolume = ref(initial.sfxVolume)
  const reduceEffects = ref<boolean | null>(initial.reduceEffects)

  watch([muted, masterVolume, musicVolume, sfxVolume, reduceEffects], () => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ muted: muted.value, masterVolume: masterVolume.value, musicVolume: musicVolume.value, sfxVolume: sfxVolume.value, reduceEffects: reduceEffects.value }))
    } catch { /* storage may be disabled or full */ }
  }, { flush: 'sync' })

  return { muted, masterVolume, musicVolume, sfxVolume, reduceEffects }
})
