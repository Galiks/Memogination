<script setup lang="ts">
import { computed } from 'vue'
import { useConnectionStore } from '@/stores/connection'

const connection = useConnectionStore()

const label = computed(() => {
  switch (connection.status) {
    case 'connected':
      return 'Connected'
    case 'connecting':
      return 'Connecting…'
    case 'reconnecting':
      return 'Reconnecting…'
    case 'closed':
      return 'Disconnected'
    default:
      return 'Offline'
  }
})

const isConnected = computed(() => connection.status === 'connected')
</script>

<template>
  <div
    v-if="connection.status !== 'idle'"
    class="flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium"
    :class="isConnected ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'"
  >
    <span class="h-2 w-2 rounded-full" :class="isConnected ? 'bg-emerald-500' : 'bg-amber-500'" />
    {{ label }}
  </div>
</template>