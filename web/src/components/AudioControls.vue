<script setup lang="ts">
import { audioManager } from '@/services/audioManager'
import { usePreferencesStore } from '@/stores/preferences'
import { useReducedMotion } from '@/composables/useReducedMotion'
const preferences = usePreferencesStore()
const { prefersReducedMotion } = useReducedMotion()

async function toggleSound(): Promise<void> {
  if (!audioManager.unlocked.value) {
    if (await audioManager.unlock()) audioManager.setMuted(false)
    return
  }
  audioManager.setMuted(!preferences.muted)
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2 rounded-xl border border-white/10 bg-slate-900/90 p-2 text-sm text-white" aria-label="Настройки звука">
    <button class="min-h-11 rounded-lg bg-white/10 px-3" type="button" @click="toggleSound">
      {{ audioManager.unlocked.value ? (preferences.muted ? 'Включить звук' : 'Выключить звук') : 'Разблокировать звук' }}
    </button>
    <label class="flex min-h-11 items-center gap-2 px-2">Общая
      <input v-model.number="preferences.masterVolume" type="range" min="0" max="1" step="0.05" aria-label="Общая громкость" />
    </label>
    <label class="flex min-h-11 items-center gap-2 px-2">Эффекты
      <input v-model.number="preferences.sfxVolume" type="range" min="0" max="1" step="0.05" aria-label="Громкость эффектов" />
    </label>
    <label class="flex min-h-11 items-center gap-2 px-2">Музыка
      <input v-model.number="preferences.musicVolume" type="range" min="0" max="1" step="0.05" aria-label="Громкость музыки" />
    </label>
    <label class="flex min-h-11 items-center gap-2 px-2">
      <input :checked="preferences.reduceEffects ?? prefersReducedMotion" type="checkbox" @change="preferences.reduceEffects = ($event.target as HTMLInputElement).checked" /> Меньше эффектов
    </label>
  </div>
</template>
