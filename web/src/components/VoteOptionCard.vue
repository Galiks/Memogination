<script setup lang="ts">
import type { MemeDTO } from '@/types/api'
import MemeImage from '@/components/MemeImage.vue'

defineProps<{
  number: number
  meme?: MemeDTO | null
  disabled?: boolean
  selected?: boolean
}>()

const emit = defineEmits<{ select: [] }>()
</script>

<template>
  <button
    type="button"
    data-testid="vote-option"
    :disabled="disabled"
    class="relative flex flex-col items-center gap-2 rounded-xl border-2 bg-white p-2 transition-colors disabled:cursor-not-allowed disabled:opacity-40"
    :class="
      selected
        ? 'border-indigo-500 ring-2 ring-indigo-300'
        : disabled
          ? 'border-slate-200'
          : 'border-slate-200 hover:border-indigo-300'
    "
    @click="emit('select')"
  >
    <span
      class="absolute left-2 top-2 z-10 flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-xs font-bold text-white"
    >
      {{ number }}
    </span>
    <div class="aspect-square w-full overflow-hidden rounded-lg">
      <MemeImage v-if="meme" :path="meme.thumbnailPath" :alt="`Вариант ${number}`" />
      <div v-else class="flex h-full w-full items-center justify-center bg-slate-100 text-slate-400">
        …
      </div>
    </div>
    <span v-if="disabled" class="text-xs font-medium text-slate-400">Ваш мем</span>
  </button>
</template>