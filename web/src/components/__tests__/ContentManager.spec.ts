import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import ContentManager from '@/components/ContentManager.vue'
import { apiClient } from '@/services/apiClient'
import type { SituationDTO } from '@/types/api'

const situation: SituationDTO = { id: 'sit-1', text: 'Before', enabled: true, source: 'manual', createdAt: '2026-01-01T00:00:00Z' }

describe('ContentManager situation controls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.spyOn(apiClient, 'listMemes').mockResolvedValue([])
    vi.spyOn(apiClient, 'listSituations').mockResolvedValue([situation])
  })

  it('patches edits and enabled state; failed updates leave the prior row intact', async () => {
    const update = vi.spyOn(apiClient, 'updateSituation').mockImplementation(async (_id, patch) => ({ ...situation, ...patch }))
    vi.spyOn(window, 'prompt').mockReturnValue('After')
    const wrapper = mount(ContentManager)
    await flushPromises()
    await wrapper.findAll('button')[1].trigger('click') // switch from memes to situations
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Изменить'))!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith('sit-1', { text: 'After' })
    expect(wrapper.text()).toContain('After')
    await wrapper.findAll('button').find((button) => button.text().includes('Выключить'))!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenLastCalledWith('sit-1', { enabled: false })
    expect(wrapper.text()).toContain('Включить')

    update.mockRejectedValueOnce(new Error('network error'))
    await wrapper.findAll('button').find((button) => button.text().includes('Включить'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Включить')
    expect(wrapper.text()).toContain('network error')
  })
})
