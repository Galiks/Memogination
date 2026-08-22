<script setup lang="ts">
import type { RoomPlayerDTO } from '@/types/api'
import PlayerAvatar from '@/components/PlayerAvatar.vue'
import AppButton from '@/components/AppButton.vue'

defineProps<{
  players: RoomPlayerDTO[]
  hostPlayerId?: string
  currentPlayerId?: string
  showKick?: boolean
}>()

const emit = defineEmits<{ kick: [playerId: string] }>()
</script>

<template>
  <ul class="divide-y divide-slate-100">
    <li
      v-for="player in players"
      :key="player.id"
      class="flex items-center gap-3 py-2"
    >
      <PlayerAvatar :name="player.name" size="sm" />
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <span class="truncate text-sm font-medium text-slate-800">{{ player.name }}</span>
          <span
            v-if="player.id === hostPlayerId"
            class="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-amber-700"
          >
            Host
          </span>
          <span
            v-if="player.id === currentPlayerId"
            class="rounded bg-indigo-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-indigo-700"
          >
            Вы
          </span>
        </div>
        <div class="flex items-center gap-2 text-xs text-slate-400">
          <span
            class="inline-block h-1.5 w-1.5 rounded-full"
            :class="player.connected ? 'bg-emerald-500' : 'bg-slate-300'"
          />
          {{ player.connected ? 'в сети' : 'не в сети' }}
        </div>
      </div>
      <AppButton
        v-if="showKick && player.id !== currentPlayerId"
        variant="danger"
        size="sm"
        @click="emit('kick', player.id)"
      >
        Кикнуть
      </AppButton>
    </li>
  </ul>
</template>