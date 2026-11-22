import { AUDIO_CUES, type AudioCue } from '@/config/audioManifest'
import { usePreferencesStore } from '@/stores/preferences'
import { ref } from 'vue'

export type AudioChannel = 'music' | 'sfx'
export interface PlayOptions { channel?: AudioChannel; dedupeKey?: string; restart?: boolean }

class AudioManager {
  private context: AudioContext | null = null
  private seenKeys = new Set<string>()
  private active = new Map<AudioChannel, Map<AudioCue, Set<OscillatorNode>>>()
  readonly unlocked = ref(false)

  async unlock(): Promise<boolean> {
    try {
      const AudioContextClass = window.AudioContext
      if (!AudioContextClass) return false
      this.context ??= new AudioContextClass()
      if (this.context.state === 'suspended') await this.context.resume()
      this.unlocked.value = this.context.state === 'running'
      return this.unlocked.value
    } catch {
      this.unlocked.value = false
      return false
    }
  }

  async preload(): Promise<void> {
    // Cues are generated from Web Audio oscillators, so there are no files to fetch/decode.
  }

  play(cue: AudioCue, options: PlayOptions = {}): boolean {
    const preferences = usePreferencesStore()
    if (preferences.muted || !this.unlocked.value || !this.context) return false
    const channel = options.channel ?? 'sfx'
    const volume = preferences.masterVolume * (channel === 'music' ? preferences.musicVolume : preferences.sfxVolume)
    if (volume <= 0) return false
    const dedupe = options.dedupeKey ? `${channel}:${options.dedupeKey}` : ''
    if (dedupe && this.seenKeys.has(dedupe)) return false
    if (dedupe) {
      this.seenKeys.add(dedupe)
      if (this.seenKeys.size > 256) this.seenKeys.delete(this.seenKeys.values().next().value!)
    }
    if (options.restart) this.stop(cue)
    try {
      const context = this.context
      const cues = this.active.get(channel) ?? new Map<AudioCue, Set<OscillatorNode>>()
      this.active.set(channel, cues)
      const oscillators = cues.get(cue) ?? new Set<OscillatorNode>()
      cues.set(cue, oscillators)
      AUDIO_CUES[cue].forEach((frequency, index) => {
        const oscillator = context.createOscillator()
        const gain = context.createGain()
        const start = context.currentTime + index * 0.09
        oscillator.type = 'sine'
        oscillator.frequency.value = frequency
        gain.gain.setValueAtTime(Math.max(0.0001, volume * 0.11), start)
        gain.gain.exponentialRampToValueAtTime(0.0001, start + (cue === 'tick' ? 0.07 : 0.19))
        oscillator.connect(gain)
        gain.connect(context.destination)
        oscillators.add(oscillator)
        oscillator.onended = () => oscillators.delete(oscillator)
        oscillator.start(start)
        oscillator.stop(start + 0.2)
      })
      return true
    } catch {
      return false
    }
  }

  stop(cue: AudioCue): void {
    for (const cues of this.active.values()) {
      for (const oscillator of cues.get(cue) ?? []) {
        try { oscillator.stop() } catch { /* already stopped */ }
      }
      cues.delete(cue)
    }
  }

  stopChannel(channel: AudioChannel): void {
    for (const oscillatorSet of this.active.get(channel)?.values() ?? []) {
      for (const oscillator of oscillatorSet) {
        try { oscillator.stop() } catch { /* already stopped */ }
      }
    }
    this.active.delete(channel)
  }

  setMasterVolume(value: number): void { usePreferencesStore().masterVolume = this.clamp(value) }
  setMusicVolume(value: number): void { usePreferencesStore().musicVolume = this.clamp(value) }
  setSfxVolume(value: number): void { usePreferencesStore().sfxVolume = this.clamp(value) }
  setMuted(value: boolean): void { usePreferencesStore().muted = value }
  private clamp(value: number): number { return Math.max(0, Math.min(1, value)) }
}

export const audioManager = new AudioManager()
