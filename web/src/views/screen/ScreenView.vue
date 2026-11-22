<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
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
import CountdownTimer from '@/components/CountdownTimer.vue'
import PlayerList from '@/components/PlayerList.vue'
import MemeImage from '@/components/MemeImage.vue'
import RevealPanel from '@/components/RevealPanel.vue'
import Leaderboard from '@/components/Leaderboard.vue'
import { apiClient } from '@/services/apiClient'
import AudioControls from '@/components/AudioControls.vue'
import AudioUnlockPrompt from '@/components/AudioUnlockPrompt.vue'
import PhaseTransition from '@/components/PhaseTransition.vue'
import ConfettiLayer from '@/components/ConfettiLayer.vue'
import { usePhaseEffects } from '@/composables/usePhaseEffects'

const route = useRoute()
const router = useRouter()
const session = useGameSessionStore()
const connection = useConnectionStore()
const { snapshot: snapshotRef } = storeToRefs(session)
const { phaseTransition, showConfetti } = usePhaseEffects(snapshotRef)
const { memes: catalogMemes, get: getMeme, load: loadMemes } = useMemeCatalog()
const { toggle: toggleFullscreen, isFullscreen } = useFullscreen()

const roomCode = computed(() => String(route.params.roomCode ?? session.currentRoomCode ?? ''))
const roomInput = ref('')
const roomError = ref('')
const roomLoading = ref(false)

async function openScreen(code: string): Promise<void> {
  const normalized = code.trim().toUpperCase()
  if (!normalized) return
  roomLoading.value = true
  roomError.value = ''
  try {
    const snap = await apiClient.getState(normalized, true)
    session.beginRoom(normalized)
    session.setSnapshot(snap, normalized)
    connection.connect(normalized, { screen: true })
    if (route.params.roomCode !== normalized) await router.replace(`/screen/${encodeURIComponent(normalized)}`)
    void loadMemes()
  } catch (error) {
    roomError.value = error instanceof Error ? error.message : 'Не удалось открыть комнату'
  } finally {
    roomLoading.value = false
  }
}

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
  // activeGamePlayerId is a game-player id (snapshot.game.players[].id), not a
  // room player id.
  return game.value?.players.find((p) => p.id === id)?.displayName ?? ''
})

const winners = computed(() => {
  const entries = gameResultsData.value.leaderboard ?? []
  if (entries.length === 0) return []
  const max = Math.max(...entries.map((e) => e.score))
  return entries.filter((e) => e.score === max)
})

// Server-provided phase deadline (RFC3339) for the countdown, in epoch ms.
const phaseDeadlineMs = computed(() => {
  const d = snapshot.value?.phaseDeadlineAt
  if (!d) return null
  const ms = Date.parse(d)
  return Number.isNaN(ms) ? null : ms
})

onMounted(() => {
  if (roomCode.value) {
    if (!snapshot.value || snapshot.value.room.code.toUpperCase() !== roomCode.value.toUpperCase()) {
      void openScreen(roomCode.value)
    } else {
      connection.connect(roomCode.value, { screen: true })
      if (!route.params.roomCode) void router.replace(`/screen/${encodeURIComponent(roomCode.value)}`)
      void loadMemes()
    }
  }
})

watch(() => route.params.roomCode, (code) => {
  if (code) void openScreen(String(code))
})
</script>

<template>
  <div v-if="!roomCode || roomError" class="flex min-h-screen items-center justify-center bg-slate-950 p-4 text-white">
    <form class="w-full max-w-md space-y-4 rounded-2xl border border-white/10 bg-slate-900 p-6" @submit.prevent="openScreen(roomInput)">
      <h1 class="text-2xl font-bold">Открыть экран комнаты</h1>
      <label class="block space-y-2 text-sm text-slate-200">
        <span>Код комнаты</span>
        <input v-model="roomInput" class="min-h-12 w-full rounded-lg border border-slate-600 bg-slate-800 px-3 text-base text-white" autocomplete="off" maxlength="16" />
      </label>
      <p v-if="roomError" role="alert" class="text-sm text-rose-300">{{ roomError }}</p>
      <AppButton type="submit" size="lg" class="w-full" :disabled="roomLoading || !roomInput.trim()">
        {{ roomLoading ? 'Подключение…' : 'Подключить экран' }}
      </AppButton>
    </form>
  </div>
  <div v-else-if="!snapshot" class="flex min-h-screen items-center justify-center bg-slate-950 text-white" role="status">
    <p>Загружаем комнату {{ roomCode }}…</p>
  </div>
  <div v-else class="show-screen min-h-screen bg-slate-950 p-4 text-white sm:p-6">
    <div class="mx-auto max-w-5xl">
      <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
        <h1 class="text-[clamp(2rem,5vw,4rem)] font-extrabold tracking-tight">Memomarium</h1>
        <div class="flex items-center gap-3">
          <AudioUnlockPrompt />
          <CountdownTimer v-if="phaseDeadlineMs !== null" :deadline-ms="phaseDeadlineMs" />
          <ConnectionBanner />
          <AppButton variant="secondary" size="sm" @click="toggleFullscreen">
            {{ isFullscreen ? 'Выйти из полноэкранного' : 'Полный экран' }}
          </AppButton>
        </div>
      </div>
      <div class="mb-5 flex flex-wrap justify-end"><AudioControls /></div>

      <!-- LOBBY -->
      <AppCard v-if="roomState === 'LOBBY'" theme="show">
        <h2 class="mb-4 text-xl font-semibold">Комната {{ snapshot?.room.code }}</h2>
        <PlayerList :players="snapshot?.players ?? []" />
      </AppCard>

      <!-- PREPARATION -->
      <AppCard v-else-if="phase === 'PREPARATION'" theme="show">
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
      <AppCard v-else-if="phase === 'ROUND_SELECTION'" theme="show">
        <h2 class="mb-4 text-2xl font-semibold">Выбор мема</h2>
        <p class="mb-6 rounded-lg bg-slate-700 px-4 py-3 text-xl">{{ selectionData.situationText }}</p>
        <p class="text-lg text-slate-300">
          <span class="font-semibold text-white">{{ activePlayerName }}</span> выбирает мем…
        </p>
        <p class="mt-2 text-slate-400">Игроки выбирают мемы…</p>
      </AppCard>

      <!-- ROUND_VOTING -->
      <AppCard v-else-if="phase === 'ROUND_VOTING'" theme="show">
        <h2 class="mb-4 text-2xl font-semibold">Голосование</h2>
        <p class="mb-6 rounded-lg bg-slate-700 px-4 py-3 text-xl">{{ votingData.situationText }}</p>
        <div class="screen-vote-grid">
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
      <AppCard v-else-if="phase === 'ROUND_RESULTS'" theme="show">
        <h2 class="mb-4 text-2xl font-semibold">Результаты раунда</h2>
        <RevealPanel v-if="reveal" :reveal="reveal" :memes="catalogMemes" />
      </AppCard>

      <!-- CYCLE_RESULTS -->
      <AppCard v-else-if="phase === 'CYCLE_RESULTS'" theme="show">
        <h2 class="mb-4 text-2xl font-semibold">Конец цикла {{ cycleData.cycleNumber }}</h2>
        <Leaderboard :entries="cycleData.leaderboard ?? []" />
      </AppCard>

      <!-- GAME_RESULTS -->
      <AppCard v-else-if="phase === 'GAME_RESULTS'" theme="show">
        <h2 class="mb-4 text-2xl font-semibold">Игра окончена!</h2>
        <div class="mb-4 rounded-lg bg-amber-500/20 px-4 py-3 text-center">
          <p class="text-lg text-amber-300">Победители:</p>
          <p class="text-2xl font-bold text-amber-200">{{ winners.map((w) => w.displayName).join(', ') }}</p>
        </div>
        <Leaderboard :entries="gameResultsData.leaderboard ?? []" />
      </AppCard>

      <!-- CLOSED -->
      <AppCard v-else-if="roomState === 'CLOSED'" theme="show">
        <div class="py-10 text-center">
          <p class="text-2xl font-semibold">Комната закрыта</p>
        </div>
      </AppCard>
    </div>
    <PhaseTransition :phase="phaseTransition" :active="!!phaseTransition" />
    <ConfettiLayer :active="showConfetti" />
  </div>
</template>
