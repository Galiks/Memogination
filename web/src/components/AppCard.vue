<script setup lang="ts">
withDefaults(defineProps<{ padded?: boolean; theme?: 'default' | 'show' | 'raised' }>(), { padded: true, theme: 'default' })
</script>

<template>
  <div
    class="rounded-2xl border shadow-lg"
    :class="[padded ? 'p-5' : '', theme === 'show' ? 'border-white/10 bg-[var(--mm-surface)] text-white' : theme === 'raised' ? 'border-violet-200 bg-[var(--mm-surface-raised)] text-white' : 'border-violet-100 bg-white text-slate-900']"
  >
    <slot />
  </div>
</template>
