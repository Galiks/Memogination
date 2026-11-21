<script setup lang="ts">
import type { LeaderboardEntry } from '@/types/api'

const props = withDefaults(
  defineProps<{
    entries: LeaderboardEntry[]
    /** Per-game-player score delta to visualize (e.g. the last round's). */
    deltas?: Record<string, number>
  }>(),
  { deltas: () => ({}) },
)

function deltaFor(id: string): number | undefined {
  const d = props.deltas[id]
  return d === undefined ? undefined : d
}
</script>

<template>
  <TransitionGroup
    tag="ol"
    name="leaderboard"
    class="divide-y divide-slate-100"
  >
    <li
      v-for="(entry, index) in entries"
      :key="entry.gamePlayerId"
      class="flex items-center gap-3 py-2"
    >
      <span class="w-6 text-center text-sm font-semibold text-slate-400">{{ index + 1 }}</span>
      <span class="flex-1 truncate text-sm text-slate-800">{{ entry.displayName }}</span>
      <Transition name="delta" mode="out-in">
        <span
          v-if="deltaFor(entry.gamePlayerId) !== undefined"
          :key="`delta-${entry.gamePlayerId}`"
          class="rounded-full px-2 py-0.5 text-xs font-bold tabular-nums"
          :class="
            (deltaFor(entry.gamePlayerId) ?? 0) >= 0 ? 'bg-emerald-100 text-emerald-700' : 'bg-red-100 text-red-700'
          "
        >
          {{ (deltaFor(entry.gamePlayerId) ?? 0) >= 0 ? '+' : '' }}{{ deltaFor(entry.gamePlayerId) }}
        </span>
      </Transition>
      <span class="text-sm font-semibold tabular-nums text-slate-900">{{ entry.score }}</span>
    </li>
  </TransitionGroup>
</template>

<style scoped>
.leaderboard-move {
  transition: transform 0.4s ease;
}

.delta-enter-active,
.delta-leave-active {
  transition:
    opacity 0.3s ease,
    transform 0.3s ease;
}

.delta-enter-from,
.delta-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>