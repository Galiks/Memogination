<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
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
    class="inline-flex min-h-11 min-w-11 items-center justify-center gap-2 rounded-xl font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 disabled:cursor-not-allowed disabled:opacity-50"
    :class="[
      size === 'sm' && 'px-3 py-2 text-sm',
      size === 'md' && 'px-4 py-2.5 text-sm',
      size === 'lg' && 'min-h-[52px] px-6 py-3 text-base',
      variant === 'primary' && 'bg-violet-700 text-white shadow-md hover:bg-violet-800',
      variant === 'secondary' && 'bg-cyan-100 text-slate-950 hover:bg-cyan-200',
      variant === 'ghost' && 'bg-transparent text-slate-700 hover:bg-violet-50',
      variant === 'danger' && 'bg-rose-700 text-white hover:bg-rose-800',
      variant === 'success' && 'bg-emerald-700 text-white hover:bg-emerald-800',
    ]"
  >
    <slot />
  </button>
</template>
