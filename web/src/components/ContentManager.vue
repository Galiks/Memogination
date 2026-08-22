<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { MemeDTO, SituationDTO } from '@/types/api'
import { apiClient } from '@/services/apiClient'
import { useUiStore } from '@/stores/ui'
import AppButton from '@/components/AppButton.vue'
import MemeImage from '@/components/MemeImage.vue'

const ui = useUiStore()

const memes = ref<MemeDTO[]>([])
const situations = ref<SituationDTO[]>([])
const loading = ref(false)
const error = ref('')
const uploading = ref(false)

const newSituationText = ref('')
const bulkText = ref('')
const bulkDelimiter = ref('*')

const enabledMemes = computed(() => memes.value.filter((m) => m.enabled))
const disabledMemes = computed(() => memes.value.filter((m) => !m.enabled))

const bulkPreview = computed(() => {
  const parts = bulkText.value
    .split(bulkDelimiter.value || '*')
    .map((s) => s.trim())
    .filter(Boolean)
  return parts
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [m, s] = await Promise.all([apiClient.listMemes(), apiClient.listSituations()])
    memes.value = m
    situations.value = s
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Не удалось загрузить контент'
  } finally {
    loading.value = false
  }
}

async function onUpload(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (files.length === 0) return
  uploading.value = true
  error.value = ''
  try {
    for (const file of files) {
      await apiClient.uploadMeme(file)
    }
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка загрузки'
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function toggleMeme(meme: MemeDTO): Promise<void> {
  try {
    await apiClient.updateMeme(meme.id, { enabled: !meme.enabled })
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка обновления'
  }
}

async function deleteMeme(id: string): Promise<void> {
  if (!window.confirm('Удалить мем?')) return
  try {
    await apiClient.deleteMeme(id)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка удаления'
  }
}

async function addSituation(): Promise<void> {
  const text = newSituationText.value.trim()
  if (!text) return
  try {
    await apiClient.addSituation(text)
    newSituationText.value = ''
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка добавления'
  }
}

async function bulkAdd(): Promise<void> {
  if (!bulkText.value.trim()) return
  try {
    await apiClient.bulkAddSituations(bulkText.value, bulkDelimiter.value || '*')
    bulkText.value = ''
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка импорта'
  }
}

async function deleteSituation(id: string): Promise<void> {
  if (!window.confirm('Удалить ситуацию?')) return
  try {
    await apiClient.deleteSituation(id)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Ошибка удаления'
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</div>

    <div class="flex gap-2">
      <AppButton
        variant="secondary"
        size="sm"
        :class="ui.currentTab === 'memes' ? 'ring-2 ring-indigo-300' : ''"
        @click="ui.currentTab = 'memes'"
      >
        Мемы ({{ memes.length }})
      </AppButton>
      <AppButton
        variant="secondary"
        size="sm"
        :class="ui.currentTab === 'situations' ? 'ring-2 ring-indigo-300' : ''"
        @click="ui.currentTab = 'situations'"
      >
        Ситуации ({{ situations.length }})
      </AppButton>
    </div>

    <div v-if="loading" class="py-6 text-center text-sm text-slate-400">Загрузка…</div>

    <template v-else-if="ui.currentTab === 'memes'">
      <div class="flex items-center gap-2">
        <label class="inline-flex cursor-pointer items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700">
          {{ uploading ? 'Загрузка…' : 'Загрузить мемы' }}
          <input type="file" accept="image/*" multiple class="hidden" :disabled="uploading" @change="onUpload" />
        </label>
      </div>

      <div v-if="enabledMemes.length" class="space-y-2">
        <h4 class="text-xs font-semibold uppercase text-slate-400">Включены</h4>
        <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
          <div v-for="meme in enabledMemes" :key="meme.id" class="group relative aspect-square overflow-hidden rounded-lg border border-slate-200">
            <MemeImage :path="meme.thumbnailPath" :alt="meme.originalFilename" />
            <div class="absolute inset-x-0 bottom-0 flex justify-between bg-black/60 p-1 opacity-0 transition-opacity group-hover:opacity-100">
              <button type="button" class="rounded bg-white/20 px-1.5 py-0.5 text-[10px] text-white" @click="toggleMeme(meme)">Выкл</button>
              <button type="button" class="rounded bg-red-500/80 px-1.5 py-0.5 text-[10px] text-white" @click="deleteMeme(meme.id)">Удал</button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="disabledMemes.length" class="space-y-2">
        <h4 class="text-xs font-semibold uppercase text-slate-400">Выключены</h4>
        <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
          <div v-for="meme in disabledMemes" :key="meme.id" class="group relative aspect-square overflow-hidden rounded-lg border border-slate-200 opacity-50">
            <MemeImage :path="meme.thumbnailPath" :alt="meme.originalFilename" />
            <div class="absolute inset-x-0 bottom-0 flex justify-between bg-black/60 p-1 opacity-0 transition-opacity group-hover:opacity-100">
              <button type="button" class="rounded bg-white/20 px-1.5 py-0.5 text-[10px] text-white" @click="toggleMeme(meme)">Вкл</button>
              <button type="button" class="rounded bg-red-500/80 px-1.5 py-0.5 text-[10px] text-white" @click="deleteMeme(meme.id)">Удал</button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="space-y-2">
        <textarea
          v-model="newSituationText"
          rows="2"
          maxlength="500"
          placeholder="Новая ситуация…"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
        />
        <AppButton variant="secondary" size="sm" :disabled="!newSituationText.trim()" @click="addSituation">
          Добавить
        </AppButton>
      </div>

      <div class="space-y-2 rounded-lg border border-slate-200 p-3">
        <h4 class="text-xs font-semibold uppercase text-slate-400">Массовый импорт</h4>
        <textarea
          v-model="bulkText"
          rows="3"
          placeholder="Ситуация 1 * Ситуация 2 * Ситуация 3"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
        />
        <div class="flex items-center gap-2">
          <label class="flex items-center gap-1 text-xs text-slate-600">
            Разделитель
            <input v-model="bulkDelimiter" type="text" maxlength="4" class="w-12 rounded border border-slate-300 px-1 py-0.5 text-sm" />
          </label>
          <span class="text-xs text-slate-400">Найдено: {{ bulkPreview.length }}</span>
        </div>
        <AppButton variant="secondary" size="sm" :disabled="bulkPreview.length === 0" @click="bulkAdd">
          Импортировать
        </AppButton>
      </div>

      <ul class="space-y-1">
        <li v-for="sit in situations" :key="sit.id" class="flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-2">
          <span class="flex-1 text-sm text-slate-800">{{ sit.text }}</span>
          <span v-if="!sit.enabled" class="text-xs text-slate-400">выкл</span>
          <button type="button" class="text-xs text-red-600 hover:underline" @click="deleteSituation(sit.id)">Удалить</button>
        </li>
      </ul>
    </template>
  </div>
</template>