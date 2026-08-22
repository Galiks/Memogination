<script setup lang="ts">
import { useCountdown } from '@/composables/useCountdown'

const props = defineProps<{ deadlineMs: number | null | undefined }>()
const { formatted, isExpired } = useCountdown(props.deadlineMs)
</script>

<template>
  <div
    class="inline-flex items-center gap-2 rounded-lg px-3 py-1.5 font-mono text-lg font-semibold tabular-nums"
    :class="isExpired ? 'bg-red-100 text-red-700' : 'bg-slate-100 text-slate-900'"
  >
    <slot name="icon" />
    {{ formatted }}
  </div>
</template>