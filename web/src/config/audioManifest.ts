export const AUDIO_CUES = {
  phase: [523, 659],
  tick: [880],
  success: [587, 784],
  reveal: [392, 523],
  winner: [523, 659, 784, 1047],
} as const

export type AudioCue = keyof typeof AUDIO_CUES
