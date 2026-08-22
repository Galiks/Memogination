<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ name: string; size?: 'sm' | 'md' | 'lg' }>(),
  { size: 'md' },
)

const initials = computed(() => {
  const trimmed = props.name.trim()
  if (!trimmed) return '?'
  const parts = trimmed.split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
})

const sizeClass = computed(() => {
  if (props.size === 'sm') return 'h-8 w-8 text-xs'
  if (props.size === 'lg') return 'h-14 w-14 text-lg'
  return 'h-10 w-10 text-sm'
})
</script>

<template>
  <div
    class="flex shrink-0 items-center justify-center rounded-full bg-indigo-100 font-semibold text-indigo-700"
    :class="sizeClass"
  >
    {{ initials }}
  </div>
</template>