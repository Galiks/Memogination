<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import type { MemeDTO } from '@/types/api'
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
import { ApiError, apiClient } from '@/services/apiClient'
import { useGameSessionStore } from '@/stores/gameSession'
import { useConnectionStore } from '@/stores/connection'
import { useMemeCatalog } from '@/composables/useMemeCatalog'
import { useWakeLock } from '@/composables/useWakeLock'
import { sendCommand } from '@/composables/useCommands'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import ConnectionBanner from '@/components/ConnectionBanner.vue'
import PlayerList from '@/components/PlayerList.vue'
import MemeGrid from '@/components/MemeGrid.vue'
import SituationInput from '@/components/SituationInput.vue'
import VoteOptionCard from '@/components/VoteOptionCard.vue'
import RevealPanel from '@/components/RevealPanel.vue'
import Leaderboard from '@/components/Leaderboard.vue'

const route = useRoute()
const roomCode = String(route.params.roomCode ?? '')

const session = useGameSessionStore()
const connection = useConnectionStore()
const { memes: catalogMemes, get: getMeme, load: loadMemes } = useMemeCatalog()
const { request: wakeRequest, release: wakeRelease } = useWakeLock()

const joined = ref(false)
const name = ref('')
const joinError = ref('')
const actionError = ref('')

const situationText = ref('')
const selectedMemeId = ref<string | null>(null)
const submitted = ref(false)
const voted = ref(false)
const votedOptionId = ref<string | null>(null)

const snapshot = computed(() => session.snapshot)
const phase = computed(() => snapshot.value?.phase ?? '')
const roomState = computed(() => snapshot.value?.room.state ?? '')
const actor = computed(() => snapshot.value?.actor)
const game = computed(() => snapshot.value?.game)
const phaseData = computed(() => snapshot.value?.phaseData ?? {})

const prepData = computed(() => phaseData.value as PreparationPhaseData)
const selectionData = computed(() => phaseData.value as RoundSelectionPhaseData)
const votingData = computed(() => phaseData.value as RoundVotingPhaseData)
const resultsData = computed(() => phaseData.value as RoundResultsPhaseData)
const cycleData = computed(() => phaseData.value as CycleResultsPhaseData)
const gameResultsData = computed(() => phaseData.value as GameResultsPhaseData)

const isHost = computed(() => actor.value?.isHost ?? false)
// The active player is identified by matching the actor's player id against the
// game player whose id equals the round's activeGamePlayerId (the snapshot
// exposes the game-player id here, not the player id).
const isActivePlayer = computed(() => {
  const activeGamePlayerId = selectionData.value.activeGamePlayerId
  if (!activeGamePlayerId) return false
  const gp = game.value?.players.find((p) => p.id === activeGamePlayerId)
  return gp ? gp.playerId === actor.value?.playerId : false
})

const prepHandMemes = computed(() =>
  (prepData.value.hand ?? []).map((id) => getMeme(id)).filter((m): m is MemeDTO => !!m),
)
const selectionHandMemes = computed(() =>
  (selectionData.value.hand ?? []).map((id) => getMeme(id)).filter((m): m is MemeDTO => !!m),
)
const voteOptions = computed<VoteOption[]>(() => votingData.value.voteOptions ?? [])
const reveal = computed<RevealData | undefined>(() => resultsData.value.reveal)

const canStart = computed(() => {
  const s = snapshot.value
  if (!s) return false
  return s.players.length >= s.settings.minPlayers
})

const prepReady = computed(() => situationText.value.trim().length > 0 && selectedMemeId.value !== null)
const selectionReady = computed(() => selectedMemeId.value !== null)
const votingReady = computed(() => votedOptionId.value !== null)

const winners = computed(() => {
  const entries = gameResultsData.value.leaderboard ?? []
  if (entries.length === 0) return []
  const max = Math.max(...entries.map((e) => e.score))
  return entries.filter((e) => e.score === max)
})

const activePhases = ['PREPARATION', 'ROUND_SELECTION', 'ROUND_VOTING']

watch(phase, async (p) => {
  if (activePhases.includes(p)) await wakeRequest()
  else await wakeRelease()
  situationText.value = ''
  selectedMemeId.value = null
  submitted.value = false
  voted.value = false
  votedOptionId.value = null
})

function connectWS(): void {
  connection.connect(roomCode)
}

async function tryReconnect(): Promise<void> {
  try {
    const snap = await apiClient.reconnect(roomCode)
    session.setSnapshot(snap)
    joined.value = true
    connectWS()
    void loadMemes()
  } catch (err) {
    if (err instanceof ApiError && (err.code === 'INVALID_SESSION' || err.code === 'PLAYER_NOT_FOUND')) {
      joined.value = false
    } else {
      joinError.value = err instanceof Error ? err.message : 'Не удалось подключиться'
    }
  }
}

async function join(): Promise<void> {
  const trimmed = name.value.trim()
  if (trimmed.length < 1 || trimmed.length > 32) {
    joinError.value = 'Имя должно быть от 1 до 32 символов'
    return
  }
  joinError.value = ''
  try {
    const snap = await apiClient.joinRoom(roomCode, trimmed)
    session.setSnapshot(snap)
    joined.value = true
    connectWS()
    void loadMemes()
  } catch (err) {
    joinError.value = err instanceof Error ? err.message : 'Не удалось присоединиться'
  }
}

async function randomSituation(): Promise<void> {
  try {
    const situations = await apiClient.listSituations()
    const enabled = situations.filter((s) => s.enabled)
    if (enabled.length === 0) return
    const pick = enabled[Math.floor(Math.random() * enabled.length)]
    situationText.value = pick.text
  } catch {
    // ignore
  }
}

async function runCommand(type: string, payload?: Record<string, unknown>): Promise<void> {
  actionError.value = ''
  try {
    await sendCommand(type, payload)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : 'Ошибка команды'
  }
}

async function submitPreparation(): Promise<void> {
  if (!prepReady.value) return
  await runCommand('SUBMIT_PREPARATION', {
    situationText: situationText.value.trim(),
    memeId: selectedMemeId.value!,
  })
  submitted.value = true
}

async function submitRoundMeme(): Promise<void> {
  if (!selectionReady.value) return
  await runCommand('SUBMIT_ROUND_MEME', { memeId: selectedMemeId.value! })
  submitted.value = true
}

async function submitVote(): Promise<void> {
  if (!votingReady.value) return
  await runCommand('SUBMIT_VOTE', { voteOptionId: votedOptionId.value! })
  voted.value = true
  votedOptionId.value = null
}

async function startGame(): Promise<void> {
  await runCommand('START_GAME')
}

async function nextRound(): Promise<void> {
  await runCommand('NEXT_ROUND')
}

async function startNextCycle(): Promise<void> {
  await runCommand('START_NEXT_CYCLE')
}

async function leaveRoom(): Promise<void> {
  if (!window.confirm('Выйти из комнаты?')) return
  try {
    await sendCommand('LEAVE_ROOM')
  } catch {
    // ignore
  }
  connection.disconnect()
  session.reset()
  joined.value = false
  name.value = ''
  situationText.value = ''
  selectedMemeId.value = null
  submitted.value = false
  voted.value = false
  votedOptionId.value = null
}

onMounted(() => {
  void tryReconnect()
})
</script>

<template>
  <div class="min-h-screen bg-slate-50 pb-24">
    <!-- Join form -->
    <div v-if="!joined" class="mx-auto flex min-h-screen max-w-md flex-col justify-center p-6">
      <AppCard>
        <h1 class="mb-1 text-xl font-bold text-slate-900">Memomarium</h1>
        <p class="mb-4 text-sm text-slate-500">
          Комната <span class="font-mono font-semibold">{{ roomCode }}</span>
        </p>
        <form class="space-y-3" @submit.prevent="join">
          <input
            v-model="name"
            type="text"
            maxlength="32"
            placeholder="Ваше имя"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200"
          />
          <AppButton type="submit" size="lg" class="w-full">Присоединиться</AppButton>
        </form>
        <p v-if="joinError" class="mt-3 text-sm text-red-600">{{ joinError }}</p>
      </AppCard>
    </div>

    <!-- Game view -->
    <div v-else class="mx-auto max-w-3xl p-4">
      <div class="mb-4 flex items-center justify-between">
        <div>
          <h1 class="text-lg font-bold text-slate-900">Комната {{ roomCode }}</h1>
          <p v-if="actor" class="text-sm text-slate-500">{{ actor.name }}</p>
        </div>
        <ConnectionBanner />
      </div>

      <p v-if="actionError" class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
        {{ actionError }}
      </p>

      <!-- LOBBY -->
      <AppCard v-if="roomState === 'LOBBY'">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-base font-semibold text-slate-900">Игроки</h2>
          <span v-if="isHost" class="rounded bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700">
            Вы HOST
          </span>
        </div>
        <PlayerList :players="snapshot?.players ?? []" :host-player-id="snapshot?.players.find((p) => p.isHost)?.id" :current-player-id="actor?.playerId" />
        <p class="mt-4 text-sm text-slate-500">
          Ждём игроков… ({{ snapshot?.players.length ?? 0 }}/{{ snapshot?.settings.minPlayers ?? '?' }})
        </p>
        <div v-if="isHost" class="mt-4">
          <AppButton size="lg" class="w-full" :disabled="!canStart" @click="startGame">
            Начать игру
          </AppButton>
          <p v-if="!canStart" class="mt-2 text-center text-xs text-slate-400">
            Нужно минимум {{ snapshot?.settings.minPlayers }} игроков
          </p>
        </div>
      </AppCard>

      <!-- PREPARATION -->
      <AppCard v-else-if="phase === 'PREPARATION'">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-base font-semibold text-slate-900">Подготовка</h2>
          <span class="text-sm text-slate-500">
            {{ prepData.preparedCount ?? 0 }}/{{ prepData.totalPlayers ?? 0 }} готовы
          </span>
        </div>

        <template v-if="prepData.preparedTurn">
          <div class="rounded-lg bg-emerald-50 px-4 py-6 text-center">
            <p class="text-lg font-semibold text-emerald-700">Готово!</p>
            <p class="mt-1 text-sm text-emerald-600">Ждём остальных игроков…</p>
          </div>
        </template>
        <template v-else>
          <p class="mb-3 text-sm text-slate-600">Придумайте ситуацию под мем:</p>
          <SituationInput v-model="situationText" @random="randomSituation" />
          <p class="mb-2 mt-4 text-sm font-medium text-slate-700">Выберите мем:</p>
          <MemeGrid :memes="prepHandMemes" :selected-id="selectedMemeId" @select="selectedMemeId = $event" />
        </template>
      </AppCard>

      <!-- ROUND_SELECTION -->
      <AppCard v-else-if="phase === 'ROUND_SELECTION'">
        <h2 class="mb-2 text-base font-semibold text-slate-900">Выбор мема</h2>
        <p class="mb-3 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-700">
          {{ selectionData.situationText }}
        </p>

        <template v-if="isActivePlayer">
          <div class="rounded-lg bg-indigo-50 px-4 py-6 text-center">
            <p class="text-lg font-semibold text-indigo-700">Вы — активный игрок</p>
            <p class="mt-1 text-sm text-indigo-600">Ждём остальных…</p>
          </div>
        </template>
        <template v-else-if="selectionData.submitted">
          <div class="rounded-lg bg-emerald-50 px-4 py-6 text-center">
            <p class="text-lg font-semibold text-emerald-700">Отправлено</p>
            <p class="mt-1 text-sm text-emerald-600">Ждём остальных…</p>
          </div>
        </template>
        <template v-else>
          <p class="mb-2 text-sm font-medium text-slate-700">Выберите мем:</p>
          <MemeGrid :memes="selectionHandMemes" :selected-id="selectedMemeId" @select="selectedMemeId = $event" />
        </template>
      </AppCard>

      <!-- ROUND_VOTING -->
      <AppCard v-else-if="phase === 'ROUND_VOTING'">
        <h2 class="mb-2 text-base font-semibold text-slate-900">Голосование</h2>
        <p class="mb-3 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-700">
          {{ votingData.situationText }}
        </p>

        <template v-if="isActivePlayer">
          <div class="rounded-lg bg-indigo-50 px-4 py-6 text-center">
            <p class="text-lg font-semibold text-indigo-700">Идёт голосование</p>
            <p class="mt-1 text-sm text-indigo-600">Ждём остальных игроков…</p>
          </div>
        </template>
        <template v-else-if="voted">
          <div class="rounded-lg bg-emerald-50 px-4 py-6 text-center">
            <p class="text-lg font-semibold text-emerald-700">Проголосовано</p>
          </div>
        </template>
        <template v-else>
          <p class="mb-2 text-sm font-medium text-slate-700">Выберите мем, который лучше всего подходит:</p>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <VoteOptionCard
              v-for="option in voteOptions"
              :key="option.id"
              :number="option.number"
              :meme="getMeme(option.memeId)"
              :disabled="option.id === votingData.forbiddenOptionId"
              :selected="votedOptionId === option.id"
              @select="votedOptionId = option.id"
            />
          </div>
        </template>
      </AppCard>

      <!-- ROUND_RESULTS -->
      <AppCard v-else-if="phase === 'ROUND_RESULTS'">
        <h2 class="mb-4 text-base font-semibold text-slate-900">Результаты раунда</h2>
        <RevealPanel v-if="reveal" :reveal="reveal" :memes="catalogMemes" />
      </AppCard>

      <!-- CYCLE_RESULTS -->
      <AppCard v-else-if="phase === 'CYCLE_RESULTS'">
        <h2 class="mb-4 text-base font-semibold text-slate-900">
          Конец цикла {{ cycleData.cycleNumber }}
        </h2>
        <Leaderboard :entries="cycleData.leaderboard ?? []" />
      </AppCard>

      <!-- GAME_RESULTS -->
      <AppCard v-else-if="phase === 'GAME_RESULTS'">
        <h2 class="mb-4 text-base font-semibold text-slate-900">Игра окончена!</h2>
        <div class="mb-4 rounded-lg bg-amber-50 px-4 py-3 text-center">
          <p class="text-sm text-amber-700">Победители:</p>
          <p class="text-lg font-bold text-amber-800">{{ winners.map((w) => w.displayName).join(', ') }}</p>
        </div>
        <Leaderboard :entries="gameResultsData.leaderboard ?? []" />
      </AppCard>

      <!-- CLOSED -->
      <AppCard v-else-if="roomState === 'CLOSED'">
        <div class="py-6 text-center">
          <p class="text-lg font-semibold text-slate-700">Комната закрыта</p>
        </div>
      </AppCard>

      <!-- Bottom action bar -->
      <div
        v-if="joined && roomState !== 'CLOSED'"
        class="fixed inset-x-0 bottom-0 border-t border-slate-200 bg-white/95 p-4 backdrop-blur"
      >
        <div class="mx-auto flex max-w-3xl items-center gap-3">
          <AppButton variant="ghost" size="sm" @click="leaveRoom">Выйти</AppButton>
          <div class="flex-1" />
          <AppButton
            v-if="phase === 'PREPARATION' && !prepData.preparedTurn"
            size="lg"
            data-testid="ready-button"
            :disabled="!prepReady"
            @click="submitPreparation"
          >
            Готово
          </AppButton>
          <AppButton
            v-else-if="phase === 'ROUND_SELECTION' && !isActivePlayer && !selectionData.submitted"
            size="lg"
            data-testid="ready-button"
            :disabled="!selectionReady"
            @click="submitRoundMeme"
          >
            Готово
          </AppButton>
          <AppButton
            v-else-if="phase === 'ROUND_VOTING' && !isActivePlayer && !voted && votedOptionId !== null"
            size="lg"
            data-testid="ready-button"
            :disabled="!votingReady"
            @click="submitVote"
          >
            Готово
          </AppButton>
          <AppButton
            v-else-if="phase === 'ROUND_RESULTS' && isHost"
            size="lg"
            @click="nextRound"
          >
            Следующий раунд
          </AppButton>
          <AppButton
            v-else-if="phase === 'CYCLE_RESULTS' && isHost"
            size="lg"
            @click="startNextCycle"
          >
            Следующий цикл
          </AppButton>
        </div>
      </div>
    </div>
  </div>
</template>