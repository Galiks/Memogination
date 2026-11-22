import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { GameSnapshot } from '@/types/api'
import { audioManager } from '@/services/audioManager'
import { usePreferencesStore } from '@/stores/preferences'
import { useReducedMotion } from '@/composables/useReducedMotion'

export function usePhaseEffects(snapshot: Ref<GameSnapshot | null>) {
  const phaseTransition = ref('')
  const showConfetti = ref(false)
  const preferences = usePreferencesStore()
  const { prefersReducedMotion } = useReducedMotion()
  let lastRoom = ''
  let lastPhase = ''
  let lastRevision = -1
  let transitionTimer: ReturnType<typeof setTimeout> | null = null
  let confettiTimer: ReturnType<typeof setTimeout> | null = null
  let countdownTimer: ReturnType<typeof setTimeout> | null = null
  let lastTickKey = ''

  const scheduleCountdown = (snap: GameSnapshot) => {
    if (countdownTimer) clearTimeout(countdownTimer)
    countdownTimer = null
    if (preferences.muted || !audioManager.unlocked.value || (preferences.reduceEffects ?? prefersReducedMotion.value)) return
    const deadline = Date.parse(snap.phaseDeadlineAt ?? '')
    if (!Number.isFinite(deadline)) return
    const remaining = deadline - Date.now()
    if (remaining <= 0) return
    const seconds = Math.ceil(remaining / 1000)
    const delay = remaining - Math.max(0, seconds - 1) * 1000
    countdownTimer = setTimeout(() => {
      const current = snapshot.value
      if (!current || current.room.code !== snap.room.code || current.phase !== snap.phase) return
      const left = Math.ceil((Date.parse(current.phaseDeadlineAt ?? '') - Date.now()) / 1000)
      if (left > 0 && left <= 5) {
        const key = `${snap.room.code}:${snap.phase}:${left}`
        if (lastTickKey !== key) {
          lastTickKey = key
          audioManager.play('tick')
        }
      }
      scheduleCountdown(current)
    }, Math.max(1, delay))
  }

  const stopCountdown = () => { if (countdownTimer) clearTimeout(countdownTimer); countdownTimer = null }
  const stopConfetti = () => { if (confettiTimer) clearTimeout(confettiTimer); confettiTimer = null; showConfetti.value = false }

  const stopPreferenceWatch = watch([() => preferences.muted, () => preferences.reduceEffects, audioManager.unlocked], () => {
    if (preferences.muted || (preferences.reduceEffects ?? prefersReducedMotion.value)) stopCountdown()
    else if (snapshot.value) scheduleCountdown(snapshot.value)
    if ((preferences.reduceEffects ?? prefersReducedMotion.value)) stopConfetti()
  })
  const stopMotionWatch = watch(prefersReducedMotion, (reduced) => { if (reduced) stopConfetti() })
  const stopSnapshotWatch = watch(snapshot, (snap) => {
    if (!snap) return
    const room = snap.room.code
    const roomChanged = room !== lastRoom
    if (roomChanged) { lastRoom = room; lastPhase = ''; lastRevision = -1; lastTickKey = '' }
    if (lastRevision < 0) {
      lastRevision = snap.revision
      lastPhase = snap.phase
      scheduleCountdown(snap)
      return
    }
    if (snap.phase !== lastPhase) {
      lastTickKey = ''
      const key = `${room}:${snap.revision}:${snap.phase}`
      phaseTransition.value = snap.phase
      if (transitionTimer) clearTimeout(transitionTimer)
      transitionTimer = setTimeout(() => { phaseTransition.value = '' }, 1400)
      const reduceEffects = preferences.reduceEffects ?? prefersReducedMotion.value
      if (!preferences.muted && audioManager.unlocked.value && !reduceEffects) {
        audioManager.play(snap.phase === 'GAME_RESULTS' ? 'winner' : snap.phase === 'ROUND_RESULTS' ? 'reveal' : 'phase', {
          channel: snap.phase === 'GAME_RESULTS' ? 'music' : 'sfx', dedupeKey: key,
        })
      }
      if (snap.phase === 'GAME_RESULTS' && !reduceEffects) {
        stopConfetti()
        showConfetti.value = true
        confettiTimer = setTimeout(stopConfetti, 4200)
      }
      lastPhase = snap.phase
    }
    lastRevision = Math.max(lastRevision, snap.revision)
    scheduleCountdown(snap)
  }, { deep: false, immediate: true })

  onBeforeUnmount(() => {
    stopSnapshotWatch(); stopPreferenceWatch(); stopMotionWatch(); stopCountdown(); stopConfetti()
    if (transitionTimer) clearTimeout(transitionTimer)
  })

  return { phaseTransition, showConfetti }
}
