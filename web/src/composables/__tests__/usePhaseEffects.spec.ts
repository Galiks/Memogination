import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent, ref, type Ref } from 'vue'
import type { GameSnapshot } from '@/types/api'
import { audioManager } from '@/services/audioManager'
import { usePreferencesStore } from '@/stores/preferences'
import { usePhaseEffects } from '@/composables/usePhaseEffects'

function makeSnapshot(revision: number, phase: string, deadline = ''): GameSnapshot {
  return { revision, phase, phaseDeadlineAt: deadline, room: { code: 'ROOM1' } } as GameSnapshot
}

describe('usePhaseEffects', () => {
  let state: Ref<GameSnapshot | null>
  let showConfetti: Ref<boolean>
  let wrapper: ReturnType<typeof mount>
  let pinia: ReturnType<typeof createPinia>
  const play = vi.spyOn(audioManager, 'play')

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    pinia = createPinia()
    setActivePinia(pinia)
    usePreferencesStore(pinia).muted = false
    usePreferencesStore(pinia).reduceEffects = false
    audioManager.unlocked.value = true
    play.mockClear()
  })
  afterEach(() => {
    wrapper?.unmount()
    vi.useRealTimers()
  })

  function mountEffects(initial: GameSnapshot) {
    const Harness = defineComponent({
      setup() {
        state = ref<GameSnapshot | null>(initial)
        const effects = usePhaseEffects(state)
        showConfetti = effects.showConfetti
        return { ...effects }
      },
      template: '<div />',
    })
    wrapper = mount(Harness, { global: { plugins: [pinia] } })
  }

  it('uses the first snapshot as baseline and cues each actual phase transition once', async () => {
    mountEffects(makeSnapshot(10, 'PREPARATION'))
    expect(play).not.toHaveBeenCalled()
    state.value = makeSnapshot(11, 'ROUND_SELECTION')
    await wrapper.vm.$nextTick()
    expect(play).toHaveBeenCalledTimes(1)
    state.value = makeSnapshot(12, 'ROUND_SELECTION')
    await wrapper.vm.$nextTick()
    expect(play).toHaveBeenCalledTimes(1)
  })

  it('plays timer ticks only when crossing 5, 4 and 3 seconds', async () => {
    mountEffects(makeSnapshot(1, 'ROUND_VOTING', new Date(5500).toISOString()))
    await vi.advanceTimersByTimeAsync(500)
    expect(play).toHaveBeenLastCalledWith('tick')
    await vi.advanceTimersByTimeAsync(1000)
    await vi.advanceTimersByTimeAsync(1000)
    expect(play.mock.calls.filter(([cue]) => cue === 'tick')).toHaveLength(3)
  })

  it('skips winner confetti when reduced effects are enabled', async () => {
    usePreferencesStore(pinia).reduceEffects = true
    mountEffects(makeSnapshot(1, 'CYCLE_RESULTS'))
    state.value = makeSnapshot(2, 'GAME_RESULTS')
    await wrapper.vm.$nextTick()
    expect(showConfetti.value).toBe(false)
  })
})
