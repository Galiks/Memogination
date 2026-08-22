import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { GameSnapshot, MemeDTO } from '@/types/api'
import PlayerView from '@/views/player/PlayerView.vue'

vi.mock('@/services/apiClient', () => ({
  ApiError: class ApiError extends Error {
    status: number
    code: string
    constructor(status: number, contract: { code: string; message: string }) {
      super(contract.message)
      this.status = status
      this.code = contract.code
    }
  },
  apiClient: {
    reconnect: vi.fn(),
    joinRoom: vi.fn(),
    listMemes: vi.fn(),
    listSituations: vi.fn(),
    sendCommand: vi.fn(),
  },
}))

vi.mock('@/services/gameSocket', () => ({
  gameSocket: {
    connect: vi.fn(),
    close: vi.fn(),
    onSnapshot: vi.fn(() => () => {}),
    onStateUpdated: vi.fn(() => () => {}),
    onStatus: vi.fn(() => () => {}),
  },
}))

vi.mock('@/composables/useWakeLock', () => ({
  useWakeLock: () => ({
    supported: false,
    isActive: { value: false },
    request: vi.fn().mockResolvedValue(undefined),
    release: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/composables/useCommands', () => ({
  sendCommand: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/composables/useMemeCatalog', () => {
  const meme = {
    id: 'm1',
    originalPath: 'm1.jpg',
    screenPath: 'm1.screen.jpg',
    thumbnailPath: 'm1.thumb.jpg',
    originalFilename: 'm1.jpg',
    mimeType: 'image/jpeg',
    sha256: 'x',
    enabled: true,
    source: 'upload',
    createdAt: '2026-01-01T00:00:00Z',
  }
  return {
    useMemeCatalog: () => ({
      memes: { value: [meme] },
      loading: { value: false },
      error: { value: '' },
      load: vi.fn().mockResolvedValue(undefined),
      get: vi.fn((id?: string) => (id === 'm1' ? meme : null)),
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { roomCode: 'ABC' } }),
}))

import { apiClient } from '@/services/apiClient'

const meme: MemeDTO = {
  id: 'm1',
  originalPath: 'm1.jpg',
  screenPath: 'm1.screen.jpg',
  thumbnailPath: 'm1.thumb.jpg',
  originalFilename: 'm1.jpg',
  mimeType: 'image/jpeg',
  sha256: 'x',
  enabled: true,
  source: 'upload',
  createdAt: '2026-01-01T00:00:00Z',
}

function baseSnapshot(phase: string, phaseData: Record<string, unknown>): GameSnapshot {
  return {
    revision: 1,
    serverTime: '2026-01-01T00:00:00Z',
    room: { id: 'r1', code: 'ABC', state: 'IN_GAME', revision: 1 },
    game: {
      id: 'g1',
      state: 'IN_GAME',
      currentCycleNumber: 1,
      settings: {
        minPlayers: 2,
        maxPlayers: 8,
        handSize: 6,
        preparationTimeoutSeconds: 60,
        roundSelectionTimeoutSeconds: 60,
        votingTimeoutSeconds: 60,
        infiniteGame: false,
        situationSeparator: '*',
        scoreConfig: {
          allGuessedActivePlayer: 3,
          allGuessedGuesser: 2,
          noneGuessedActivePlayer: -1,
          noneGuessedOtherPlayer: 0,
          partialActiveBase: 1,
          partialActivePerGuesser: 1,
          partialGuesser: 1,
          voteForSubmittedMeme: 0,
        },
      },
      players: [
        { id: 'gp1', playerId: 'p1', displayName: 'Alice', turnOrder: 0, score: 0, participationStatus: 'active', connected: true },
        { id: 'gp2', playerId: 'p2', displayName: 'Bob', turnOrder: 1, score: 0, participationStatus: 'active', connected: true },
      ],
      leaderboard: [],
    },
    players: [
      { id: 'p1', name: 'Alice', role: 'HOST', connected: true, isHost: true },
      { id: 'p2', name: 'Bob', role: 'PLAYER', connected: true, isHost: false },
    ],
    settings: {
      minPlayers: 2,
      maxPlayers: 8,
      handSize: 6,
      preparationTimeoutSeconds: 60,
      roundSelectionTimeoutSeconds: 60,
      votingTimeoutSeconds: 60,
      infiniteGame: false,
      situationSeparator: '*',
      scoreConfig: {
        allGuessedActivePlayer: 3,
        allGuessedGuesser: 2,
        noneGuessedActivePlayer: -1,
        noneGuessedOtherPlayer: 0,
        partialActiveBase: 1,
        partialActivePerGuesser: 1,
        partialGuesser: 1,
        voteForSubmittedMeme: 0,
      },
    },
    phase,
    actor: { playerId: 'p1', name: 'Alice', role: 'player', isHost: false, isAdmin: false },
    phaseData,
  }
}

describe('PlayerView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('disables Ready until a situation and a meme are selected', async () => {
    const snapshot = baseSnapshot('PREPARATION', {
      preparedCount: 0,
      totalPlayers: 2,
      hand: ['m1'],
    })
    vi.mocked(apiClient.reconnect).mockResolvedValue(snapshot)
    vi.mocked(apiClient.listMemes).mockResolvedValue([meme])

    const wrapper = mount(PlayerView, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await flushPromises()

    const ready = wrapper.find('[data-testid="ready-button"]')
    expect(ready.exists()).toBe(true)
    expect(ready.attributes('disabled')).toBeDefined()

    // Select a meme.
    const memeOption = wrapper.find('[data-testid="meme-option"]')
    await memeOption.trigger('click')

    // Still disabled without a situation.
    expect(wrapper.find('[data-testid="ready-button"]').attributes('disabled')).toBeDefined()

    // Enter a situation.
    await wrapper.find('textarea').setValue('A funny situation')

    expect(wrapper.find('[data-testid="ready-button"]').attributes('disabled')).toBeUndefined()
  })

  it('disables the player own vote option (forbiddenOptionId)', async () => {
    const snapshot = baseSnapshot('ROUND_VOTING', {
      situationText: 'Situation',
      voteOptions: [
        { id: 'opt1', number: 1, memeId: 'm1' },
        { id: 'opt2', number: 2, memeId: 'm2' },
        { id: 'opt3', number: 3, memeId: 'm3' },
      ],
      forbiddenOptionId: 'opt2',
    })
    vi.mocked(apiClient.reconnect).mockResolvedValue(snapshot)
    vi.mocked(apiClient.listMemes).mockResolvedValue([meme])

    const wrapper = mount(PlayerView, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await flushPromises()

    const options = wrapper.findAll('[data-testid="vote-option"]')
    expect(options).toHaveLength(3)
    // The forbidden option (index 1) must be disabled.
    expect(options[1].attributes('disabled')).toBeDefined()
    expect(options[0].attributes('disabled')).toBeUndefined()
    expect(options[2].attributes('disabled')).toBeUndefined()
  })
})