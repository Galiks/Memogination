<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import type {
  CycleResultsPhaseData,
  GameResultsPhaseData,
  PreparationPhaseData,
  RevealData,
  RoundResultsPhaseData,
  RoundSelectionPhaseData,
  RoundVotingPhaseData,
  VoteOption,
} from '@/types/phaseData'
import { useGameSessionStore } from '@/stores/gameSession'
import { useConnectionStore } from '@/stores/connection'
import { useMemeCatalog } from '@/composables/useMemeCatalog'
import { useFullscreen } from '@/composables/useFullscreen'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import ConnectionBanner from '@/components/ConnectionBanner.vue'
import PlayerList from '@/components/PlayerList.vue'
import MemeImage from '@/components/MemeImage.vue'
import RevealPanel from '@/components/RevealPanel.vue'
import Leaderboard from '@/components/Leaderboard.vue'

const route = useRoute()
const session = useGameSessionStore()
const connection = useConnectionStore()
const { memes: catalogMemes, get: getMeme, load: loadMemes } = useMemeCatalog()
const { toggle: toggleFullscreen, isFullscreen } = useFullscreen()

const roomCode = computed(() => String(route.params.roomCode ?? session.currentRoomCode ?? ''))

const snapshot = computed(() => session.snapshot)
const phase = computed(() => snapshot.value?.phase ?? '')
const roomState = computed(() => snapshot.value?.room.state ?? '')
const game = computed(() => snapshot.value?.game)
const phaseData = computed(() => snapshot.value?.phaseData ?? {})

const prepData = computed(() => phaseData.value as PreparationPhaseData)
const selectionData = computed(() => phaseData.value as RoundSelectionPhaseData)
const votingData = computed(() => phaseData.value as RoundVotingPhaseData)
const resultsData = computed(() => phaseData.value as RoundResultsPhaseData)
const cycleData = computed(() => phaseData.value as CycleResultsPhaseData)
const gameResultsData = computed(() => phaseData.value as GameResultsPhaseData)

const voteOptions = computed<VoteOption[]>(() => votingData.value.voteOptions ?? [])
const reveal = computed<RevealData | undefined>(() => resultsData.value.reveal)

const activePlayerName = computed(() => {
  const id = selectionData.value.activeGamePlayerId
  if (!id) return ''
  return game.value?.players.find((p) => p.playerId === id)?.displayName ?? ''
})

const winners = computed(() => {
  const entries = gameResultsData.value.leaderboard ?? []
  if (entries.length === 0) return []
  const max = Math.max(...entries.map((e) => e.score))
  return entries.filter((e) => e.score === max)
})

onMounted(() => {
  if (roomCode.value) {
    connection.connect(roomCode.value, { screen: true })
    void loadMemes()
  }
})
</script>

<template>
  <div class="min-h-screen bg-slate-900 p-6 text-white">
    <div class="mx-auto max-w-5xl">
      <div class="mb-6 flex items-center justify-between">
        <h1 class="text-2xl font-bold">Memomarium</h1>
        <div class="flex items-center gap-3">
          <ConnectionBanner />
          <AppButton variant="secondary" size="sm" @click="toggleFullscreen">
            {{ isFullscreen ? 'Выйти из полноэкранного' : 'Полный экран' }}
          </AppButton>
        </div>
      </div>

      <!-- LOBBY -->
      <AppCard v-if="roomState === 'LOBBY'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-xl font-semibold">Комната {{ snapshot?.room.code }}</h2>
        <PlayerList :players="snapshot?.players ?? []" />
      </AppCard>

      <!-- PREPARATION -->
      <AppCard v-else-if="phase === 'PREPARATION'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Подготовка…</h2>
        <div class="mb-2 flex items-center justify-between text-lg">
          <span>Игроки готовятся</span>
          <span class="tabular-nums">{{ prepData.preparedCount ?? 0 }}/{{ prepData.totalPlayers ?? 0 }}</span>
        </div>
        <div class="h-4 w-full overflow-hidden rounded-full bg-slate-700">
          <div
            class="h-full bg-indigo-500 transition-all"
            :style="{ width: `${((prepData.preparedCount ?? 0) / Math.max(1, prepData.totalPlayers ?? 1)) * 100}%` }"
          />
        </div>
      </AppCard>

      <!-- ROUND_SELECTION -->
      <AppCard v-else-if="phase === 'ROUND_SELECTION'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Выбор мема</h2>
        <p class="mb-6 rounded-lg bg-slate-700 px-4 py-3 text-xl">{{ selectionData.situationText }}</p>
        <p class="text-lg text-slate-300">
          <span class="font-semibold text-white">{{ activePlayerName }}</span> выбирает мем…
        </p>
        <p class="mt-2 text-slate-400">Игроки выбирают мемы…</p>
      </AppCard>

      <!-- ROUND_VOTING -->
      <AppCard v-else-if="phase === 'ROUND_VOTING'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Голосование</h2>
        <p class="mb-6 rounded-lg bg-slate-700 px-4 py-3 text-xl">{{ votingData.situationText }}</p>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          <div
            v-for="option in voteOptions"
            :key="option.id"
            class="relative aspect-square overflow-hidden rounded-xl border border-slate-600"
          >
            <span class="absolute left-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full bg-slate-900 text-base font-bold text-white">
              {{ option.number }}
            </span>
            <MemeImage v-if="getMeme(option.memeId)" :path="getMeme(option.memeId)!.screenPath" variant="screen" />
          </div>
        </div>
      </AppCard>

      <!-- ROUND_RESULTS -->
      <AppCard v-else-if="phase === 'ROUND_RESULTS'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Результаты раунда</h2>
        <RevealPanel v-if="reveal" :reveal="reveal" :memes="catalogMemes" />
      </AppCard>

      <!-- CYCLE_RESULTS -->
      <AppCard v-else-if="phase === 'CYCLE_RESULTS'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Конец цикла {{ cycleData.cycleNumber }}</h2>
        <Leaderboard :entries="cycleData.leaderboard ?? []" />
      </AppCard>

      <!-- GAME_RESULTS -->
      <AppCard v-else-if="phase === 'GAME_RESULTS'" class="bg-slate-800 text-white">
        <h2 class="mb-4 text-2xl font-semibold">Игра окончена!</h2>
        <div class="mb-4 rounded-lg bg-amber-500/20 px-4 py-3 text-center">
          <p class="text-lg text-amber-300">Победители:</p>
          <p class="text-2xl font-bold text-amber-200">{{ winners.map((w) => w.displayName).join(', ') }}</p>
        </div>
        <Leaderboard :entries="gameResultsData.leaderboard ?? []" />
      </AppCard>

      <!-- CLOSED -->
      <AppCard v-else-if="roomState === 'CLOSED'" class="bg-slate-800 text-white">
        <div class="py-10 text-center">
          <p class="text-2xl font-semibold">Комната закрыта</p>
        </div>
      </AppCard>
    </div>
  </div>
</template>