<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'

const props = defineProps<{ value: string; size?: number }>()

const canvasRef = ref<HTMLCanvasElement | null>(null)

async function render(): Promise<void> {
  if (!canvasRef.value || !props.value) return
  try {
    await QRCode.toCanvas(canvasRef.value, props.value, {
      width: props.size ?? 160,
      margin: 1,
    })
  } catch {
    // ignore render errors
  }
}

onMounted(render)
watch(() => props.value, render)
</script>

<template>
  <canvas ref="canvasRef" class="rounded-lg" />
</template>