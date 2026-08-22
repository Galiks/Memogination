<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    path: string
    variant?: 'thumbnail' | 'screen' | 'original'
    alt?: string
    rounded?: boolean
  }>(),
  {
    variant: 'thumbnail',
    alt: '',
    rounded: true,
  },
)

const src = computed(() => `/media/${props.path}`)

const sizeClass = computed(() => {
  if (props.variant === 'screen') return 'h-full w-full object-contain'
  if (props.variant === 'original') return 'h-full w-full object-contain'
  return 'h-full w-full object-cover'
})
</script>

<template>
  <img
    :src="src"
    :alt="alt"
    class="bg-slate-100"
    :class="[sizeClass, rounded ? 'rounded-lg' : '']"
  />
</template>