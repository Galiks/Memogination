import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { GameSnapshot, RoomPlayerDTO } from '@/types/api'
import HostView from '@/views/host/HostView.vue'

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
    adminBootstrap: vi.fn(),
    getNetworkAddresses: vi.fn(),
    getState: vi.fn(),
    createRoom: vi.fn(),
    listRooms: vi.fn(),
    deleteRoom: vi.fn(),
    updateSettings: vi.fn(),
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

vi.mock('@/composables/useCommands', () => ({
  sendCommand: vi.fn().mockResolvedValue(undefined),
}))

import { apiClient } from '@/services/apiClient'

function lobbySnapshot(players: RoomPlayerDTO[] = [], minPlayers = 2): GameSnapshot {
  return {
    revision: 1,
    serverTime: '2026-01-01T00:00:00Z',
    room: { id: 'r1', code: 'ABC', state: 'LOBBY', revision: 1 },
    game: null,
    players,
    settings: {
      minPlayers,
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
    phase: 'PREPARATION',
    actor: { playerId: '', name: '', role: 'admin', isHost: false, isAdmin: true },
    phaseData: {},
  }
}

function hostPlayers(count: number): RoomPlayerDTO[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `p${i + 1}`,
    name: `Player${i + 1}`,
    role: i === 0 ? 'HOST' : 'PLAYER',
    connected: true,
    isHost: i === 0,
  }))
}

describe('HostView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.clear()
    vi.mocked(apiClient.listRooms).mockResolvedValue([])
  })

  it('enables the start button when enough players have joined', async () => {
    vi.mocked(apiClient.adminBootstrap).mockResolvedValue({ isAdmin: true })
    vi.mocked(apiClient.getNetworkAddresses).mockResolvedValue({ addresses: [] })
    vi.mocked(apiClient.getState).mockResolvedValue(lobbySnapshot(hostPlayers(2), 2))
    vi.mocked(apiClient.listMemes).mockResolvedValue([])
    vi.mocked(apiClient.listSituations).mockResolvedValue([])
    localStorage.setItem('memomarium_host_room', 'ABC')

    const wrapper = mount(HostView, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await flushPromises()

    const startButton = wrapper
      .findAll('button')
      .find((b) => b.text() === 'Начать игру')
    expect(startButton).toBeDefined()
    expect(startButton!.attributes('disabled')).toBeUndefined()
  })

  it('disables the start button until enough players have joined', async () => {
    vi.mocked(apiClient.adminBootstrap).mockResolvedValue({ isAdmin: true })
    vi.mocked(apiClient.getNetworkAddresses).mockResolvedValue({ addresses: [] })
    vi.mocked(apiClient.getState).mockResolvedValue(lobbySnapshot(hostPlayers(1), 2))
    vi.mocked(apiClient.listMemes).mockResolvedValue([])
    vi.mocked(apiClient.listSituations).mockResolvedValue([])
    localStorage.setItem('memomarium_host_room', 'ABC')

    const wrapper = mount(HostView, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await flushPromises()

    const startButton = wrapper
      .findAll('button')
      .find((b) => b.text() === 'Начать игру')
    expect(startButton).toBeDefined()
    expect(startButton!.attributes('disabled')).toBeDefined()
  })

  it('shows the not-admin message when bootstrap is forbidden', async () => {
    vi.mocked(apiClient.adminBootstrap).mockRejectedValue(
      new (class extends Error {
        status = 403
        code = 'NOT_ALLOWED'
      })('forbidden'),
    )

    const wrapper = mount(HostView, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('Панель администратора доступна только на этом компьютере')
  })
})