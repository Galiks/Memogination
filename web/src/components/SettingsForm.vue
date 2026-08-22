<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { GameSettingsDTO } from '@/types/api'
import AppButton from '@/components/AppButton.vue'

const props = defineProps<{ settings: GameSettingsDTO }>()
const emit = defineEmits<{ save: [settings: GameSettingsDTO] }>()

const form = reactive<GameSettingsDTO>({ ...props.settings })
const errors = ref<Record<string, string>>({})

function validate(): boolean {
  const e: Record<string, string> = {}
  if (form.minPlayers < 2) e.minPlayers = 'Минимум 2 игрока'
  if (form.maxPlayers < form.minPlayers) e.maxPlayers = 'Максимум не меньше минимума'
  if (form.handSize < 1) e.handSize = 'Размер руки минимум 1'
  if (form.preparationTimeoutSeconds < 10) e.preparationTimeoutSeconds = 'Минимум 10 секунд'
  if (form.roundSelectionTimeoutSeconds < 10) e.roundSelectionTimeoutSeconds = 'Минимум 10 секунд'
  if (form.votingTimeoutSeconds < 10) e.votingTimeoutSeconds = 'Минимум 10 секунд'
  errors.value = e
  return Object.keys(e).length === 0
}

function save(): void {
  if (!validate()) return
  emit('save', { ...form })
}
</script>

<template>
  <form class="space-y-4" @submit.prevent="save">
    <div class="grid grid-cols-2 gap-3">
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Мин. игроков</span>
        <input v-model.number="form.minPlayers" type="number" min="2" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.minPlayers" class="text-xs text-red-600">{{ errors.minPlayers }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Макс. игроков</span>
        <input v-model.number="form.maxPlayers" type="number" min="2" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.maxPlayers" class="text-xs text-red-600">{{ errors.maxPlayers }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Размер руки</span>
        <input v-model.number="form.handSize" type="number" min="1" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.handSize" class="text-xs text-red-600">{{ errors.handSize }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Подготовка (сек)</span>
        <input v-model.number="form.preparationTimeoutSeconds" type="number" min="10" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.preparationTimeoutSeconds" class="text-xs text-red-600">{{ errors.preparationTimeoutSeconds }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Выбор мема (сек)</span>
        <input v-model.number="form.roundSelectionTimeoutSeconds" type="number" min="10" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.roundSelectionTimeoutSeconds" class="text-xs text-red-600">{{ errors.roundSelectionTimeoutSeconds }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Голосование (сек)</span>
        <input v-model.number="form.votingTimeoutSeconds" type="number" min="10" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        <span v-if="errors.votingTimeoutSeconds" class="text-xs text-red-600">{{ errors.votingTimeoutSeconds }}</span>
      </label>
      <label class="block">
        <span class="mb-1 block text-xs font-medium text-slate-600">Разделитель ситуаций</span>
        <input v-model="form.situationSeparator" type="text" maxlength="4" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
      </label>
      <label class="flex items-center gap-2 pt-5">
        <input v-model="form.infiniteGame" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
        <span class="text-sm text-slate-700">Бесконечная игра</span>
      </label>
    </div>

    <div>
      <span class="mb-2 block text-xs font-medium text-slate-600">Очки</span>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Все угадали (акт.)</span>
          <input v-model.number="form.scoreConfig.allGuessedActivePlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Все угадали (угад.)</span>
          <input v-model.number="form.scoreConfig.allGuessedGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Никто не угадал (акт.)</span>
          <input v-model.number="form.scoreConfig.noneGuessedActivePlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Никто не угадал (др.)</span>
          <input v-model.number="form.scoreConfig.noneGuessedOtherPlayer" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Частично (база)</span>
          <input v-model.number="form.scoreConfig.partialActiveBase" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Частично (за угад.)</span>
          <input v-model.number="form.scoreConfig.partialActivePerGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Частично (угад.)</span>
          <input v-model.number="form.scoreConfig.partialGuesser" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
        <label class="block">
          <span class="mb-1 block text-[11px] text-slate-500">Голос за свой мем</span>
          <input v-model.number="form.scoreConfig.voteForSubmittedMeme" type="number" class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm" />
        </label>
      </div>
    </div>

    <AppButton type="submit" variant="primary">Сохранить</AppButton>
  </form>
</template>