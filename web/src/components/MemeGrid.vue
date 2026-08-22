<script setup lang="ts">
import type { MemeDTO } from '@/types/api'
import MemeImage from '@/components/MemeImage.vue'

defineProps<{
  memes: MemeDTO[]
  selectedId?: string | null
  disabled?: boolean
}>()

const emit = defineEmits<{ select: [memeId: string] }>()
</script>

<template>
  <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
    <button
      v-for="meme in memes"
      :key="meme.id"
      type="button"
      data-testid="meme-option"
      :disabled="disabled"
      class="relative aspect-square overflow-hidden rounded-lg border-2 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
      :class="
        selectedId === meme.id
          ? 'border-indigo-500 ring-2 ring-indigo-300'
          : 'border-transparent hover:border-slate-300'
      "
      @click="emit('select', meme.id)"
    >
      <MemeImage :path="meme.thumbnailPath" :alt="meme.originalFilename" />
      <span
        v-if="selectedId === meme.id"
        class="absolute right-1 top-1 rounded-full bg-indigo-600 px-1.5 py-0.5 text-[10px] font-bold text-white"
      >
        ✓
      </span>
    </button>
  </div>
</template>