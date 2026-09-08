<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import type { GameSettingsDTO } from '@/types/api'
import AppButton from '@/components/AppButton.vue'

const props = defineProps<{ settings: GameSettingsDTO }>()
const emit = defineEmits<{ save: [settings: GameSettingsDTO] }>()

// Backend validation ranges (room.RoomSettings.Validate + spec §36):
// minPlayers 2..20, maxPlayers 2..20, min<=max, handSize 3..10, timers 0..3600
// (0 = no timer).
const LIMITS = {
  minPlayers: { min: 2, max: 20 },
  maxPlayers: { min: 2, max: 20 },
  handSize: { min: 3, max: 10 },
  timer: { min: 0, max: 3600 },
}

const form = reactive<GameSettingsDTO>({ ...props.settings })
const errors = ref<Record<string, string>>({})

watch(
  () => props.settings,
  (s) => {
    Object.assign(form, s)
  },
)

function inRange(value: number, lo: number, hi: number): boolean {
  return Number.isInteger(value) && value >= lo && value <= hi
}

function validate(): boolean {
  const e: Record<string, string> = {}
  if (!inRange(form.minPlayers, LIMITS.minPlayers.min, LIMITS.minPlayers.max)) {
    e.minPlayers = `От ${LIMITS.minPlayers.min} до ${LIMITS.minPlayers.max} игроков`
  }
  if (!inRange(form.maxPlayers, LIMITS.maxPlayers.min, LIMITS.maxPlayers.max)) {
    e.maxPlayers = `От ${LIMITS.maxPlayers.min} до ${LIMITS.maxPlayers.max} игроков`
  }
  if (form.maxPlayers < form.minPlayers) e.maxPlayers = 'Максимум не меньше минимума'
  if (!inRange(form.handSize, LIMITS.handSize.min, LIMITS.handSize.max)) {
    e.handSize = `От ${LIMITS.handSize.min} до ${LIMITS.handSize.max} мемов в руке`
  }
  for (const key of [
    'preparationTimeoutSeconds',
    'roundSelectionTimeoutSeconds',
    'votingTimeoutSeconds',
  ] as const) {
    if (!inRange(form[key], LIMITS.timer.min, LIMITS.timer.max)) {
      e[key] = '0 — без таймера, максимум 3600 сек'
    }
  }
  if (!form.situationSeparator.trim()) e.situationSeparator = 'Разделитель не может быть пустым'
  errors.value = e
  return Object.keys(e).length === 0
}

function save(): void {
  if (!validate()) return
  emit('save', { ...form })
}

// Restore the Standard scoring preset (spec §39).
function resetScoreConfig(): void {
  form.scoreConfig = {
    allGuessedActivePlayer: -3,
    allGuessedGuesser: 0,
    noneGuessedActivePlayer: -2,
    noneGuessedOtherPlayer: 0,
    partialActiveBase: 3,
    partialActivePerGuesser: 1,
    partialGuesser: 3,
    voteForSubmittedMeme: 1,
  }
}

const timerFields: Array<{
  key: 'preparationTimeoutSeconds' | 'roundSelectionTimeoutSeconds' | 'votingTimeoutSeconds'
  label: string
  hint: string
}> = [
  {
    key: 'preparationTimeoutSeconds',
    label: 'Подготовка',
    hint: 'Сколько секунд игроки придумывают ситуацию и выбирают мем. 0 — ждать всех без ограничения.',
  },
  {
    key: 'roundSelectionTimeoutSeconds',
    label: 'Выбор мема в раунде',
    hint: 'Сколько секунд остальные выбирают мем под ситуацию. 0 — ждать всех без ограничения.',
  },
  {
    key: 'votingTimeoutSeconds',
    label: 'Голосование',
    hint: 'Сколько секунд игроки голосуют. 0 — ждать всех без ограничения.',
  },
]
</script>

<template>
  <form class="space-y-6" @submit.prevent="save">
    <!-- Players -->
    <section class="space-y-3">
      <h3 class="text-sm font-semibold text-slate-800">Игроки и руки</h3>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <label class="block">
          <span class="mb-1 block text-xs font-medium text-slate-600">Минимум игроков</span>
          <input
            v-model.number="form.minPlayers"
            type="number"
            :min="LIMITS.minPlayers.min"
            :max="LIMITS.minPlayers.max"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
          <span v-if="errors.minPlayers" class="text-xs text-red-600">{{ errors.minPlayers }}</span>
          <span class="mt-0.5 block text-[11px] leading-snug text-slate-400">
            Сколько нужно игроков, чтобы начать игру
          </span>
        </label>
        <label class="block">
          <span class="mb-1 block text-xs font-medium text-slate-600">Максимум игроков</span>
          <input
            v-model.number="form.maxPlayers"
            type="number"
            :min="LIMITS.maxPlayers.min"
            :max="LIMITS.maxPlayers.max"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
          <span v-if="errors.maxPlayers" class="text-xs text-red-600">{{ errors.maxPlayers }}</span>
          <span class="mt-0.5 block text-[11px] leading-snug text-slate-400">
            Сколько игроков поместится в комнату
          </span>
        </label>
        <label class="block">
          <span class="mb-1 block text-xs font-medium text-slate-600">Мемов в руке</span>
          <input
            v-model.number="form.handSize"
            type="number"
            :min="LIMITS.handSize.min"
            :max="LIMITS.handSize.max"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
          <span v-if="errors.handSize" class="text-xs text-red-600">{{ errors.handSize }}</span>
          <span class="mt-0.5 block text-[11px] leading-snug text-slate-400">
            Сколько мемов получает каждый игрок для выбора
          </span>
        </label>
      </div>
    </section>

    <!-- Timers -->
    <section class="space-y-3">
      <h3 class="text-sm font-semibold text-slate-800">Таймеры фаз</h3>
      <p class="text-xs text-slate-500">
        Ограничивают время на каждом шаге. Значение 0 означает «без таймера»: фаза ждёт всех игроков
        или завершится вручную кнопкой «Принудительно завершить фазу».
      </p>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label v-for="f in timerFields" :key="f.key" class="block">
          <span class="mb-1 block text-xs font-medium text-slate-600">{{ f.label }} (сек)</span>
          <input
            v-model.number="form[f.key]"
            type="number"
            :min="LIMITS.timer.min"
            :max="LIMITS.timer.max"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
          <span v-if="errors[f.key]" class="text-xs text-red-600">{{ errors[f.key] }}</span>
          <span class="mt-0.5 block text-[11px] leading-snug text-slate-400">{{ f.hint }}</span>
        </label>
      </div>
    </section>

    <!-- Game mode -->
    <section class="space-y-2">
      <h3 class="text-sm font-semibold text-slate-800">Режим игры</h3>
      <label class="flex items-start gap-2 rounded-lg border border-slate-200 p-3">
        <input v-model="form.infiniteGame" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-slate-300" />
        <span>
          <span class="block text-sm text-slate-700">Бесконечная игра</span>
          <span class="block text-[11px] leading-snug text-slate-400">
            После полного круга (все побывали активным игроком) игра не заканчивается, а начинается
            новый цикл. Очки продолжают копиться.
          </span>
        </span>
      </label>
    </section>

    <!-- Situations -->
    <section class="space-y-2">
      <h3 class="text-sm font-semibold text-slate-800">Ситуации</h3>
      <label class="block max-w-xs">
        <span class="mb-1 block text-xs font-medium text-slate-600">Разделитель для массового импорта</span>
        <input
          v-model="form.situationSeparator"
          type="text"
          maxlength="4"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
        />
        <span v-if="errors.situationSeparator" class="text-xs text-red-600">{{ errors.situationSeparator }}</span>
        <span class="mt-0.5 block text-[11px] leading-snug text-slate-400">
          Строка, которая на отдельной строке разделяет ситуации в массовом импорте (вкладка «Контент»).
        </span>
      </label>
    </section>

    <!-- Scoring -->
    <section class="space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">Очки</h3>
        <AppButton variant="secondary" size="sm" type="button" @click="resetScoreConfig">
          Вернуть стандартные очки
        </AppButton>
      </div>
      <p class="text-xs text-slate-500">
        Настройки начисления очков за раунд. Отрицательные значения штрафуют, положительные —
        награждают.
      </p>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Все угадали · активный игрок</span>
          <input v-model.number="form.scoreConfig.allGuessedActivePlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Все выбрали его мем — слишком легко</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Все угадали · угадавший</span>
          <input v-model.number="form.scoreConfig.allGuessedGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Очки каждому угадавшему</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Никто не угадал · активный</span>
          <input v-model.number="form.scoreConfig.noneGuessedActivePlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Штраф, если мем никто не выбрал</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Никто не угадал · остальные</span>
          <input v-model.number="form.scoreConfig.noneGuessedOtherPlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Очки остальным игрокам</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Частично · база активного</span>
          <input v-model.number="form.scoreConfig.partialActiveBase" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Награда активному за частичное угадывание</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Частично · за каждого угадавшего</span>
          <input v-model.number="form.scoreConfig.partialActivePerGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Дополнительно за каждого угадавшего</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Частично · угадавший</span>
          <input v-model.number="form.scoreConfig.partialGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Очки игроку, угадавшему мем</span>
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] font-medium text-slate-500">Голос за свой мем</span>
          <input v-model.number="form.scoreConfig.voteForSubmittedMeme" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
          <span class="mt-0.5 block text-[10px] leading-snug text-slate-400">Бонус, если голосовали за свой мем</span>
        </label>
      </div>
    </section>

    <AppButton type="submit" variant="primary">Сохранить настройки</AppButton>
  </form>
</template>