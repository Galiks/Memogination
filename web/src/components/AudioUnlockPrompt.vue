<script setup lang="ts">
import { audioManager } from '@/services/audioManager'
</script>

<template>
  <button v-if="!audioManager.unlocked.value" type="button" class="min-h-11 rounded-lg border border-cyan-200/30 bg-slate-950/80 px-4 py-2 text-sm text-cyan-100" @click="audioManager.unlock()">
    Включить звук
  </button>
</template>
