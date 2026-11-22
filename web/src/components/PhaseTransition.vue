<script setup lang="ts">
defineProps<{ phase: string; active: boolean }>()
</script>

<template>
  <Transition name="phase-pop">
    <div v-if="active" class="pointer-events-none fixed inset-x-0 top-6 z-40 flex justify-center" aria-live="polite">
      <div class="rounded-2xl border border-cyan-200/30 bg-slate-950/90 px-8 py-4 text-center text-2xl font-black uppercase tracking-widest text-cyan-100 shadow-2xl">
        {{ phase.replaceAll('_', ' ') }}
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.phase-pop-enter-active, .phase-pop-leave-active { transition: opacity 220ms ease, transform 220ms ease; }
.phase-pop-enter-from, .phase-pop-leave-to { opacity: 0; transform: translateY(-12px) scale(.98); }
@media (prefers-reduced-motion: reduce) { .phase-pop-enter-active, .phase-pop-leave-active { transition-duration: 1ms; } }
</style>
