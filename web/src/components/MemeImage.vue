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
</script>

<template>
  <img
    :src="src"
    :alt="alt"
    class="h-full w-full bg-white object-contain"
    :class="[rounded ? 'rounded-lg' : '']"
  />
</template>
