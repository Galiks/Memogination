<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { GameSettingsDTO, RoomSummary } from '@/types/api'
import { ApiError, apiClient } from '@/services/apiClient'
import { useGameSessionStore } from '@/stores/gameSession'
import { useConnectionStore } from '@/stores/connection'
import { sendCommand } from '@/composables/useCommands'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import ConnectionBanner from '@/components/ConnectionBanner.vue'
import QrCodeCard from '@/components/QrCodeCard.vue'
import PlayerList from '@/components/PlayerList.vue'
import SettingsForm from '@/components/SettingsForm.vue'
import ContentManager from '@/components/ContentManager.vue'
import Leaderboard from '@/components/Leaderboard.vue'

const HOST_ROOM_KEY = 'memomarium_host_room'

const session = useGameSessionStore()
const connection = useConnectionStore()

const isAdmin = ref<boolean | null>(null)
const roomCode = ref('')
const creating = ref(false)
const createError = ref('')
const actionError = ref('')
const settingsError = ref('')

const rooms = ref<RoomSummary[]>([])
const roomsError = ref('')
const deletingCode = ref('')

const addresses = ref<string[]>([])
const selectedAddress = ref('')

const snapshot = computed(() => session.snapshot)
const phase = computed(() => snapshot.value?.phase ?? '')
const roomState = computed(() => snapshot.value?.room.state ?? '')
const game = computed(() => snapshot.value?.game)

const isLobby = computed(() => roomState.value === 'LOBBY')
const isInGame = computed(() => roomState.value === 'IN_GAME')
const isClosed = computed(() => roomState.value === 'CLOSED')

const qrUrl = computed(() =>
  roomCode.value && selectedAddress.value ? `${selectedAddress.value}/play/${roomCode.value}` : '',
)

const canStart = computed(() => {
  const s = snapshot.value
  if (!s) return false
  return s.players.length >= s.settings.minPlayers
})

const activePhase = computed(() => {
  return ['PREPARATION', 'ROUND_SELECTION', 'ROUND_VOTING'].includes(phase.value)
})

function roomStateLabel(state: string): string {
  switch (state) {
    case 'LOBBY':
      return 'Лобби'
    case 'IN_GAME':
      return 'Игра идёт'
    case 'CLOSED':
      return 'Закрыта'
    default:
      return state
  }
}

async function loadAddresses(): Promise<void> {
  try {
    const res = await apiClient.getNetworkAddresses()
    addresses.value = res.addresses
    if (res.addresses.length > 0) selectedAddress.value = res.addresses[0]
  } catch {
    // ignore
  }
}

function connectRoom(code: string): void {
  connection.connect(code)
  void session.resync()
}

function openRoom(code: string): void {
  if (code === roomCode.value) return
  connection.disconnect()
  session.reset()
  roomCode.value = code
  localStorage.setItem(HOST_ROOM_KEY, code)
  connectRoom(code)
}

async function loadRooms(): Promise<void> {
  try {
    rooms.value = await apiClient.listRooms()
  } catch {
    // ignore: the management list is best-effort
  }
}

async function bootstrap(): Promise<void> {
  try {
    const res = await apiClient.adminBootstrap()
    isAdmin.value = res.isAdmin
    if (res.isAdmin) {
      await loadAddresses()
      await loadRooms()
      const saved = localStorage.getItem(HOST_ROOM_KEY)
      if (saved) {
        roomCode.value = saved
        connectRoom(saved)
      }
    }
  } catch (err) {
    isAdmin.value = false
    if (err instanceof ApiError && err.status !== 403) {
      createError.value = err.message
    }
  }
}

async function createRoom(): Promise<void> {
  creating.value = true
  createError.value = ''
  try {
    const room = await apiClient.createRoom('Host')
    connection.disconnect()
    session.reset()
    roomCode.value = room.code
    localStorage.setItem(HOST_ROOM_KEY, room.code)
    connectRoom(room.code)
    await loadRooms()
  } catch (err) {
    createError.value = err instanceof Error ? err.message : 'Не удалось создать комнату'
  } finally {
    creating.value = false
  }
}

async function deleteRoom(code: string): Promise<void> {
  if (!window.confirm(`Удалить комнату ${code}? Все данные комнаты будут удалены безвозвратно.`)) return
  deletingCode.value = code
  roomsError.value = ''
  try {
    await apiClient.deleteRoom(code)
    if (code === roomCode.value) {
      connection.disconnect()
      session.reset()
      localStorage.removeItem(HOST_ROOM_KEY)
      roomCode.value = ''
    }
    await loadRooms()
  } catch (err) {
    roomsError.value = err instanceof Error ? err.message : 'Не удалось удалить комнату'
  } finally {
    deletingCode.value = ''
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

async function kickPlayer(playerId: string): Promise<void> {
  if (!window.confirm('Кикнуть игрока?')) return
  await runCommand('KICK_PLAYER', { playerId })
}

async function startNewGame(): Promise<void> {
  actionError.value = ''
  try {
    await sendCommand('START_NEW_GAME')
    await sendCommand('START_GAME')
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : 'Ошибка команды'
  }
}

async function saveSettings(settings: GameSettingsDTO): Promise<void> {
  settingsError.value = ''
  try {
    await apiClient.updateSettings(roomCode.value, settings)
    await session.resync()
  } catch (err) {
    settingsError.value = err instanceof Error ? err.message : 'Не удалось сохранить настройки'
  }
}

onMounted(() => {
  void bootstrap()
})
</script>

<template>
  <div class="min-h-screen bg-slate-50 p-6">
    <div class="mx-auto max-w-4xl">
      <div class="mb-6 flex items-center justify-between">
        <h1 class="text-2xl font-bold text-slate-900">Панель администратора</h1>
        <ConnectionBanner />
      </div>

      <!-- Not admin -->
      <AppCard v-if="isAdmin === false">
        <p class="text-slate-600">Панель администратора доступна только на этом компьютере.</p>
      </AppCard>

      <!-- Loading -->
      <AppCard v-else-if="isAdmin === null">
        <p class="text-slate-500">Проверка доступа…</p>
      </AppCard>

      <!-- Admin -->
      <template v-else>
        <p v-if="actionError" class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          {{ actionError }}
        </p>

        <!-- Room management -->
        <AppCard class="mb-4">
          <div class="mb-3 flex items-center justify-between">
            <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-400">Комнаты</h2>
            <AppButton size="sm" :disabled="creating" @click="createRoom">
              {{ creating ? 'Создание…' : 'Создать комнату' }}
            </AppButton>
          </div>
          <p v-if="createError" class="mb-2 text-sm text-red-600">{{ createError }}</p>
          <p v-if="roomsError" class="mb-2 text-sm text-red-600">{{ roomsError }}</p>
          <ul v-if="rooms.length" class="divide-y divide-slate-100">
            <li v-for="r in rooms" :key="r.id" class="flex items-center gap-3 py-2">
              <span class="font-mono text-sm font-semibold text-slate-800">{{ r.code }}</span>
              <span class="text-xs text-slate-500">
                {{ roomStateLabel(r.state) }} · {{ r.playerCount }} игр.
              </span>
              <div class="flex-1" />
              <AppButton v-if="r.code !== roomCode" variant="secondary" size="sm" @click="openRoom(r.code)">
                Открыть
              </AppButton>
              <AppButton variant="danger" size="sm" :disabled="deletingCode === r.code" @click="deleteRoom(r.code)">
                {{ deletingCode === r.code ? 'Удаление…' : 'Удалить' }}
              </AppButton>
            </li>
          </ul>
          <p v-else class="text-sm text-slate-400">Комнат пока нет</p>
        </AppCard>

        <!-- No room selected yet -->
        <AppCard v-if="!roomCode" class="mb-4">
          <h2 class="mb-3 text-lg font-semibold text-slate-900">Новая игра</h2>
          <p class="text-sm text-slate-500">
            Создайте комнату и поделитесь QR-кодом с игроками, чтобы начать.
          </p>
        </AppCard>

        <!-- Room active -->
        <template v-else>
          <div class="mb-4 grid gap-4 sm:grid-cols-2">
            <AppCard>
              <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">Комната</h2>
              <p class="font-mono text-2xl font-bold text-slate-900">{{ roomCode }}</p>
              <p class="mt-1 text-sm text-slate-500">Фаза: {{ phase || '—' }}</p>
            </AppCard>
            <AppCard>
              <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-slate-400">
                QR для игроков
              </h2>
              <div v-if="addresses.length" class="flex flex-col items-center gap-2">
                <QrCodeCard :value="qrUrl" :size="140" />
                <select
                  v-model="selectedAddress"
                  class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm"
                >
                  <option v-for="addr in addresses" :key="addr" :value="addr">{{ addr }}</option>
                </select>
                <p class="break-all text-center text-xs text-slate-400">{{ qrUrl }}</p>
              </div>
              <p v-else class="text-sm text-slate-400">Сетевые адреса недоступны</p>
            </AppCard>
          </div>

          <!-- Players -->
          <AppCard class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">Игроки</h2>
            <PlayerList
              :players="snapshot?.players ?? []"
              :host-player-id="snapshot?.players.find((p) => p.isHost)?.id"
              show-kick
              @kick="kickPlayer"
            />
          </AppCard>

          <!-- Game controls -->
          <AppCard v-if="isInGame" class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">Управление игрой</h2>
            <div class="flex flex-wrap gap-2">
              <AppButton v-if="phase === 'ROUND_RESULTS'" @click="runCommand('NEXT_ROUND')">
                Следующий раунд
              </AppButton>
              <AppButton v-if="phase === 'CYCLE_RESULTS'" @click="runCommand('START_NEXT_CYCLE')">
                Следующий цикл
              </AppButton>
              <AppButton v-if="activePhase" variant="secondary" @click="runCommand('FORCE_RESOLVE_PHASE')">
                Принудительно завершить фазу
              </AppButton>
            </div>
          </AppCard>

          <!-- Start game (lobby) -->
          <AppCard v-if="isLobby" class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">Игра</h2>
            <AppButton size="lg" :disabled="!canStart" @click="runCommand('START_GAME')">
              Начать игру
            </AppButton>
            <p v-if="!canStart" class="mt-2 text-sm text-slate-400">
              Нужно минимум {{ snapshot?.settings.minPlayers }} игроков
            </p>
          </AppCard>

          <!-- Game over / restart -->
          <AppCard v-if="isClosed" class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">
              Игра завершена
            </h2>
            <p class="mb-3 text-sm text-slate-600">
              Хотите сыграть ещё раз с теми же игроками?
            </p>
            <AppButton size="lg" @click="startNewGame">Начать игру заново</AppButton>
          </AppCard>

          <!-- Settings -->
          <AppCard v-if="isLobby && snapshot?.settings" class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">Настройки</h2>
            <p v-if="settingsError" class="mb-2 text-sm text-red-600">{{ settingsError }}</p>
            <SettingsForm :settings="snapshot.settings" @save="saveSettings" />
          </AppCard>

          <!-- Content -->
          <AppCard class="mb-4">
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">Контент</h2>
            <ContentManager />
          </AppCard>

          <!-- Snapshot state -->
          <AppCard>
            <h2 class="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-400">
              Текущее состояние
            </h2>
            <div class="space-y-2 text-sm text-slate-600">
              <p>Комната: {{ roomState }}</p>
              <p>Фаза: {{ phase }}</p>
              <p>Цикл: {{ game?.currentCycleNumber ?? '—' }}</p>
              <div v-if="game?.leaderboard?.length">
                <p class="mb-1 font-medium text-slate-700">Лидеры:</p>
                <Leaderboard :entries="game.leaderboard" />
              </div>
            </div>
          </AppCard>
        </template>
      </template>
    </div>
  </div>
</template>