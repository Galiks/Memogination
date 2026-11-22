import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import ScreenView from '@/views/screen/ScreenView.vue'
import { apiClient } from '@/services/apiClient'
import { useGameSessionStore } from '@/stores/gameSession'

const { route, replace } = vi.hoisted(() => ({ route: { params: {} as Record<string, string> }, replace: vi.fn() }))
const reactiveRoute = reactive(route)
const socket = vi.hoisted(() => ({
  connect: vi.fn(), close: vi.fn(),
  onSnapshot: vi.fn(() => () => {}), onStateUpdated: vi.fn(() => () => {}), onStatus: vi.fn(() => () => {}),
  onSessionRevoked: vi.fn(() => () => {}), onProbe: vi.fn(() => () => {}),
}))
vi.mock('vue-router', () => ({ useRoute: () => reactiveRoute, useRouter: () => ({ replace }) }))
vi.mock('@/services/gameSocket', () => ({ gameSocket: socket }))

describe('ScreenView room entry', () => {
  beforeEach(() => {
    reactiveRoute.params = {}
    replace.mockReset()
    setActivePinia(createPinia())
  })
  afterEach(() => vi.restoreAllMocks())

  it('shows a room-code form on /screen when no saved room exists', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(ScreenView, { global: { plugins: [pinia] } })
    expect(wrapper.text()).toContain('Открыть экран комнаты')
    expect(wrapper.find('input').exists()).toBe(true)
    expect(useGameSessionStore(pinia).currentRoomCode).toBeNull()
    wrapper.unmount()
  })

  it('redirects a saved room from /screen to its explicit screen route', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const session = useGameSessionStore(pinia)
    session.beginRoom('SAVE1')
    session.setSnapshot({ revision: 4, room: { code: 'SAVE1', id: 'r1', state: 'LOBBY', revision: 4 }, players: [], game: null, settings: {}, phase: '', phaseData: {} } as never)
    const wrapper = mount(ScreenView, { global: { plugins: [pinia] } })
    await flushPromises()
    expect(replace).toHaveBeenCalledWith('/screen/SAVE1')
    wrapper.unmount()
  })

  it('validates a room using the screen projection and shows a retryable error', async () => {
    const getState = vi.spyOn(apiClient, 'getState').mockRejectedValue(new Error('Комната не найдена'))
    const wrapper = mount(ScreenView, { global: { plugins: [createPinia()] } })
    await wrapper.find('input').setValue('bad123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(getState).toHaveBeenCalledWith('BAD123', true)
    expect(wrapper.text()).toContain('Комната не найдена')
    expect(wrapper.text()).toContain('Подключить экран')
    wrapper.unmount()
  })
})
