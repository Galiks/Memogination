<script setup lang="ts">
import { computed } from 'vue'
import type { MemeDTO } from '@/types/api'
import type { RevealData } from '@/types/phaseData'
import MemeImage from '@/components/MemeImage.vue'
import Leaderboard from '@/components/Leaderboard.vue'

const props = defineProps<{
  reveal: RevealData
  memes: MemeDTO[]
}>()

const memeById = computed(() => {
  const map = new Map<string, MemeDTO>()
  for (const m of props.memes) map.set(m.id, m)
  return map
})

function meme(id?: string): MemeDTO | null {
  if (!id) return null
  return memeById.value.get(id) ?? null
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h3 class="mb-1 text-sm font-semibold uppercase tracking-wide text-slate-400">Ситуация</h3>
      <p class="text-lg font-medium text-slate-900">{{ reveal.situationText }}</p>
    </div>

    <div>
      <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Оригинальный мем
      </h3>
      <div class="mx-auto max-w-xs">
        <div class="aspect-square overflow-hidden rounded-xl border-2 border-amber-400">
          <MemeImage
            v-if="meme(reveal.originalMemeId)"
            :path="meme(reveal.originalMemeId)!.screenPath"
            variant="screen"
          />
        </div>
      </div>
    </div>

    <div>
      <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Голосование
      </h3>
      <ul class="space-y-2">
        <li
          v-for="option in reveal.voteOptions ?? []"
          :key="option.id"
          class="flex items-center gap-3 rounded-lg border border-slate-200 p-2"
          :class="option.isOriginal ? 'border-amber-400 bg-amber-50' : ''"
        >
          <span class="w-6 text-center text-sm font-bold text-slate-500">{{ option.number }}</span>
          <div class="h-12 w-12 shrink-0 overflow-hidden rounded-md">
            <MemeImage v-if="meme(option.memeId)" :path="meme(option.memeId)!.thumbnailPath" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm text-slate-800">{{ option.ownerGamePlayerId }}</p>
            <p class="text-xs text-slate-400">
              {{ option.votes }} голос(ов)
              <span v-if="option.isOriginal" class="font-semibold text-amber-600">· оригинал</span>
            </p>
          </div>
        </li>
      </ul>
    </div>

    <div>
      <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Отправленные мемы
      </h3>
      <ul class="space-y-2">
        <li
          v-for="sub in reveal.submissions ?? []"
          :key="sub.gamePlayerId"
          class="flex items-center gap-3 rounded-lg border border-slate-200 p-2"
        >
          <div class="h-12 w-12 shrink-0 overflow-hidden rounded-md">
            <MemeImage v-if="meme(sub.memeId)" :path="meme(sub.memeId)!.thumbnailPath" />
          </div>
          <span class="text-sm text-slate-800">{{ sub.displayName }}</span>
        </li>
      </ul>
    </div>

    <div>
      <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Изменение очков
      </h3>
      <ul class="space-y-1">
        <li
          v-for="delta in reveal.scoreDeltas ?? []"
          :key="delta.gamePlayerId"
          class="flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2 text-sm"
        >
          <span class="text-slate-800">{{ delta.displayName }}</span>
          <span class="font-semibold tabular-nums" :class="delta.delta >= 0 ? 'text-emerald-600' : 'text-red-600'">
            {{ delta.delta >= 0 ? '+' : '' }}{{ delta.delta }} → {{ delta.newScore }}
          </span>
        </li>
      </ul>
    </div>

    <div>
      <h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Таблица лидеров
      </h3>
      <Leaderboard :entries="reveal.leaderboard ?? []" />
    </div>
  </div>
</template>