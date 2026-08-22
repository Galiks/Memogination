<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    disabled?: boolean
    type?: 'button' | 'submit'
  }>(),
  {
    variant: 'primary',
    size: 'md',
    disabled: false,
    type: 'button',
  },
)
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    class="inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
    :class="[
      size === 'sm' && 'px-3 py-1.5 text-sm',
      size === 'md' && 'px-4 py-2 text-sm',
      size === 'lg' && 'px-6 py-3 text-base',
      variant === 'primary' && 'bg-indigo-600 text-white hover:bg-indigo-700',
      variant === 'secondary' && 'bg-slate-200 text-slate-900 hover:bg-slate-300',
      variant === 'ghost' && 'bg-transparent text-slate-700 hover:bg-slate-100',
      variant === 'danger' && 'bg-red-600 text-white hover:bg-red-700',
    ]"
  >
    <slot />
  </button>
</template>