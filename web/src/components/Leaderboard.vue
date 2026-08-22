<script setup lang="ts">
import type { LeaderboardEntry } from '@/types/api'

defineProps<{ entries: LeaderboardEntry[] }>()
</script>

<template>
  <ol class="divide-y divide-slate-100">
    <li
      v-for="(entry, index) in entries"
      :key="entry.gamePlayerId"
      class="flex items-center gap-3 py-2"
    >
      <span class="w-6 text-center text-sm font-semibold text-slate-400">{{ index + 1 }}</span>
      <span class="flex-1 truncate text-sm text-slate-800">{{ entry.displayName }}</span>
      <span class="text-sm font-semibold text-slate-900">{{ entry.score }}</span>
    </li>
  </ol>
</template>