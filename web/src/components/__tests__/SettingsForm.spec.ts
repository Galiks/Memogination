import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsForm from '@/components/SettingsForm.vue'
import type { GameSettingsDTO } from '@/types/api'

const settings: GameSettingsDTO = {
  minPlayers: 2, maxPlayers: 8, handSize: 3,
  preparationTimeoutSeconds: 0, roundSelectionTimeoutSeconds: 90, votingTimeoutSeconds: 60,
  infiniteGame: false, situationSeparator: '*',
  scoreConfig: { allGuessedActivePlayer: -3, allGuessedGuesser: 0, noneGuessedActivePlayer: -2, noneGuessedOtherPlayer: 0, partialActiveBase: 3, partialActivePerGuesser: 1, partialGuesser: 3, voteForSubmittedMeme: 1 },
}

describe('SettingsForm', () => {
  it('edits and emits a copied scoreConfig without mutating props', async () => {
    const wrapper = mount(SettingsForm, { props: { settings } })
    const scoreInput = wrapper.findAll('input[type="number"]')[6]
    await scoreInput.setValue(42)
    expect(settings.scoreConfig.allGuessedActivePlayer).toBe(-3)
    await wrapper.find('form').trigger('submit')
    const saved = wrapper.emitted('save')?.[0][0] as GameSettingsDTO
    expect(saved.scoreConfig.allGuessedActivePlayer).toBe(42)
    expect(saved.scoreConfig).not.toBe(settings.scoreConfig)
  })
})
