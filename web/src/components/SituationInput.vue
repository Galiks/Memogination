<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'

defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  random: []
}>()
</script>

<template>
  <div class="space-y-2">
    <textarea
      :value="modelValue"
      :disabled="disabled"
      rows="3"
      maxlength="500"
      placeholder="Придумайте ситуацию под мем…"
      class="w-full resize-none rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:bg-slate-50"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    />
    <div class="flex items-center justify-between">
      <AppButton variant="ghost" size="sm" :disabled="disabled" @click="emit('random')">
        Не можете придумать? Вставить случайную ситуацию
      </AppButton>
      <span class="text-xs tabular-nums text-slate-400">{{ modelValue.length }}/500</span>
    </div>
  </div>
</template>