<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Check, ChevronLeft, ChevronRight, Maximize2, X } from 'lucide-vue-next'
import type { MemeDTO } from '@/types/api'

const props = withDefaults(
  defineProps<{
    memes: MemeDTO[]
    selectedId?: string | null
    disabled?: boolean
  }>(),
  {
    selectedId: null,
    disabled: false,
  },
)

const emit = defineEmits<{ select: [memeId: string] }>()

const currentIndex = ref(0)
const fullscreenOpen = ref(false)
const overlayEl = ref<HTMLElement | null>(null)
const closeBtnEl = ref<HTMLElement | null>(null)
const fullscreenTriggerEl = ref<HTMLElement | null>(null)

// Plain (non-reactive) swipe state.
const SWIPE_THRESHOLD = 50
let touchStartX = 0
let touchStartY = 0
let gestureSwiped = false

const currentMeme = computed(() => props.memes[currentIndex.value] ?? null)

// Start on the selected meme when present; clamp the index when the list shrinks.
let previousList: MemeDTO[] | null = null
watch(
  () => [props.memes, props.selectedId] as const,
  ([list, id]) => {
    const listReplaced = list !== previousList
    previousList = list
    if (list.length === 0) {
      currentIndex.value = 0
      return
    }
    const idx = id ? list.findIndex((m) => m.id === id) : -1
    if (idx !== -1) {
      currentIndex.value = idx
    } else if (listReplaced) {
      // A different list arrived and the selection is gone: start over.
      currentIndex.value = 0
    } else if (currentIndex.value >= list.length) {
      currentIndex.value = list.length - 1
    }
  },
  { immediate: true },
)

function goNext(): void {
  if (props.memes.length === 0) return
  currentIndex.value = (currentIndex.value + 1) % props.memes.length
}

function goPrev(): void {
  if (props.memes.length === 0) return
  currentIndex.value = (currentIndex.value - 1 + props.memes.length) % props.memes.length
}

function goTo(index: number): void {
  if (index >= 0 && index < props.memes.length) currentIndex.value = index
}

// Only the slide click guards against the synthetic click browsers fire after
// a swipe; other controls stay immediately usable (e.g. closing fullscreen
// right after a swipe).
function onSlideClick(): void {
  if (props.disabled) return
  if (gestureSwiped) {
    gestureSwiped = false
    return
  }
  const meme = currentMeme.value
  if (meme) emit('select', meme.id)
}

function onPrev(): void {
  goPrev()
}

function onNext(): void {
  goNext()
}

function onDot(index: number): void {
  goTo(index)
}

function onOpenFullscreen(): void {
  fullscreenOpen.value = true
  void nextTick(() => closeBtnEl.value?.focus())
}

function onCloseFullscreen(): void {
  fullscreenOpen.value = false
  void nextTick(() => fullscreenTriggerEl.value?.focus())
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'ArrowRight') {
    e.preventDefault()
    goNext()
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault()
    goPrev()
  } else if (e.key === 'Escape') {
    onCloseFullscreen()
  } else if (e.key === 'Tab') {
    trapFocus(e)
  }
}

// Minimal Tab focus trap while the overlay is open: keep focus cycling
// among the overlay's focusable elements (close, prev, next).
function trapFocus(e: KeyboardEvent): void {
  if (!overlayEl.value) return
  const focusable = overlayEl.value.querySelectorAll<HTMLElement>(
    'button:not([disabled]), [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
  )
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement
  if (e.shiftKey) {
    if (active === first || !active || !overlayEl.value.contains(active)) {
      e.preventDefault()
      last.focus()
    }
  } else if (active === last || !active || !overlayEl.value.contains(active)) {
    e.preventDefault()
    first.focus()
  }
}

function onTouchStart(e: TouchEvent): void {
  const t = e.touches[0]
  if (!t) return
  gestureSwiped = false
  touchStartX = t.clientX
  touchStartY = t.clientY
}

function onTouchMove(e: TouchEvent): void {
  const t = e.touches[0]
  if (!t) return
  const dx = Math.abs(t.clientX - touchStartX)
  const dy = Math.abs(t.clientY - touchStartY)
  // Lock horizontal panning: swallow the move once the gesture is horizontal.
  if (dx > dy && dx > 10) e.preventDefault()
}

function onTouchEnd(e: TouchEvent): void {
  const t = e.changedTouches[0]
  if (!t) return
  const dx = t.clientX - touchStartX
  const dy = t.clientY - touchStartY
  if (Math.abs(dx) >= SWIPE_THRESHOLD && Math.abs(dx) > Math.abs(dy)) {
    if (dx < 0) goNext()
    else goPrev()
    // Mark that a swipe happened so the synthetic click the browser fires right
    // after the gesture is suppressed on the slide itself; other controls stay
    // immediately usable. The flag resets on the next touchstart, so a later
    // deliberate tap selects normally.
    gestureSwiped = true
  }
}
</script>

<template>
  <div>
    <!-- Empty state -->
    <div
      v-if="memes.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white py-12 text-center text-sm text-slate-500"
    >
      Нет мемов
    </div>

    <template v-else>
      <!-- Counter -->
      <div class="mb-2 flex items-center justify-between">
        <span class="text-sm font-semibold text-slate-700">{{ currentIndex + 1 }} / {{ memes.length }}</span>
        <span class="text-xs text-slate-400">Проведите пальцем влево/вправо</span>
      </div>

      <!-- Slide -->
      <div
        class="relative h-72 touch-pan-y select-none overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm sm:h-96"
        @touchstart="onTouchStart"
        @touchmove="onTouchMove"
        @touchend="onTouchEnd"
      >
        <button
          type="button"
          data-testid="meme-option"
          class="absolute inset-0 h-full w-full cursor-pointer focus-visible:ring-2 focus-visible:ring-indigo-300 focus-visible:outline-none"
          :class="selectedId === currentMeme?.id ? 'ring-4 ring-inset ring-indigo-400' : ''"
          :disabled="disabled"
          @click="onSlideClick"
          @keydown="onKeydown"
        >
          <img
            v-if="currentMeme"
            :src="`/media/${currentMeme.screenPath}`"
            :alt="currentMeme.originalFilename"
            loading="lazy"
            decoding="async"
            class="h-full w-full bg-white object-contain"
          />
          <span
            v-if="selectedId === currentMeme?.id"
            class="absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded-full bg-indigo-600 text-white shadow"
          >
            <Check class="h-4 w-4" />
          </span>
        </button>

        <button
          type="button"
          ref="fullscreenTriggerEl"
          data-testid="meme-fullscreen"
          class="absolute left-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full bg-white/85 text-slate-600 shadow-sm backdrop-blur transition hover:bg-white hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
          aria-label="На весь экран"
          @click.stop="onOpenFullscreen"
        >
          <Maximize2 class="h-4 w-4" />
        </button>

        <button
          type="button"
          data-testid="meme-prev"
          class="absolute left-2 top-1/2 z-10 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/85 text-slate-600 shadow-sm backdrop-blur transition hover:bg-white hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
          aria-label="Предыдущий мем"
          @click.stop="onPrev"
        >
          <ChevronLeft class="h-5 w-5" />
        </button>

        <button
          type="button"
          data-testid="meme-next"
          class="absolute right-2 top-1/2 z-10 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/85 text-slate-600 shadow-sm backdrop-blur transition hover:bg-white hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
          aria-label="Следующий мем"
          @click.stop="onNext"
        >
          <ChevronRight class="h-5 w-5" />
        </button>
      </div>

      <!-- Dots -->
      <div class="mt-3 flex items-center justify-center gap-1.5">
        <button
          v-for="(meme, i) in memes"
          :key="meme.id"
          type="button"
          data-testid="meme-dot"
          class="h-2 rounded-full transition-all"
          :class="i === currentIndex ? 'w-5 bg-indigo-600' : 'w-2 bg-slate-300 hover:bg-slate-400'"
          :aria-label="`Мем ${i + 1}`"
          :aria-current="i === currentIndex ? 'true' : undefined"
          @click="onDot(i)"
        />
      </div>

      <!-- Thumbnail strip -->
      <div class="mt-3 flex gap-2 overflow-x-auto pb-1">
        <button
          v-for="(meme, i) in memes"
          :key="meme.id"
          type="button"
          data-testid="meme-slide-nav"
          class="h-14 w-20 shrink-0 overflow-hidden rounded-lg border-2 bg-white p-0.5 transition"
          :class="
            i === currentIndex
              ? 'border-indigo-500 ring-2 ring-indigo-200'
              : 'border-transparent hover:border-slate-300'
          "
          :aria-label="`Мем ${i + 1}`"
          :aria-current="i === currentIndex ? 'true' : undefined"
          @click="onDot(i)"
        >
          <img
            :src="`/media/${meme.thumbnailPath}`"
            :alt="meme.originalFilename"
            loading="lazy"
            decoding="async"
            class="h-full w-full bg-white object-contain"
          />
        </button>
      </div>
    </template>

    <!-- Fullscreen overlay -->
    <Teleport to="body">
      <div
        v-if="fullscreenOpen"
        ref="overlayEl"
        role="dialog"
        aria-modal="true"
        aria-label="Просмотр мема на весь экран"
        data-testid="meme-fullscreen-overlay"
        class="fixed inset-0 z-50 flex touch-pan-y select-none flex-col overscroll-contain bg-white"
        @keydown="onKeydown"
        @touchstart="onTouchStart"
        @touchmove="onTouchMove"
        @touchend="onTouchEnd"
      >
        <div class="flex items-center justify-between p-4">
          <span class="text-sm font-semibold text-slate-700">{{ currentIndex + 1 }} / {{ memes.length }}</span>
          <button
            type="button"
            ref="closeBtnEl"
            data-testid="meme-fullscreen-close"
            class="flex h-10 w-10 items-center justify-center rounded-full bg-slate-100 text-slate-600 transition hover:bg-slate-200 hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
            aria-label="Закрыть"
            @click.stop="onCloseFullscreen"
          >
            <X class="h-5 w-5" />
          </button>
        </div>

        <div class="relative flex flex-1 items-center justify-center overflow-hidden px-4 pb-6">
          <img
            v-if="currentMeme"
            data-testid="meme-fullscreen-image"
            :src="`/media/${currentMeme.screenPath}`"
            :alt="currentMeme.originalFilename"
            loading="lazy"
            decoding="async"
            class="h-full w-full bg-white object-contain"
          />

          <button
            type="button"
            data-testid="meme-fullscreen-prev"
            class="absolute left-3 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-slate-100 text-slate-700 shadow-sm transition hover:bg-slate-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
            aria-label="Предыдущий мем"
            @click.stop="onPrev"
          >
            <ChevronLeft class="h-6 w-6" />
          </button>

          <button
            type="button"
            data-testid="meme-fullscreen-next"
            class="absolute right-3 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-slate-100 text-slate-700 shadow-sm transition hover:bg-slate-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300"
            aria-label="Следующий мем"
            @click.stop="onNext"
          >
            <ChevronRight class="h-6 w-6" />
          </button>
        </div>
      </div>
    </Teleport>
  </div>
</template>