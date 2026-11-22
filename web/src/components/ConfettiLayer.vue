<script setup lang="ts">
import { computed } from 'vue'
defineProps<{ active: boolean }>()
const pieces = computed(() => Array.from({ length: 36 }, (_, i) => ({
  id: i,
  left: `${(i * 37) % 100}%`,
  delay: `${(i % 12) * 0.08}s`,
  color: ['#a855f7', '#22d3ee', '#f59e0b', '#22c55e'][i % 4],
})))
</script>

<template>
  <div v-if="active" class="pointer-events-none fixed inset-0 z-30 overflow-hidden" aria-hidden="true">
    <span v-for="piece in pieces" :key="piece.id" class="confetti" :style="{ left: piece.left, animationDelay: piece.delay, backgroundColor: piece.color }" />
  </div>
</template>

<style scoped>
.confetti { position: absolute; top: -12px; width: 8px; height: 14px; animation: fall 3.8s ease-in forwards; }
@keyframes fall { to { transform: translateY(110vh) rotate(620deg); opacity: .2; } }
@media (prefers-reduced-motion: reduce) { .confetti { animation: none; display: none; } }
</style>
