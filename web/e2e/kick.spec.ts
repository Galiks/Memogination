import { expect, test } from '@playwright/test'
import { createHostRoom, joinPlayer } from './helpers'

test('kick: the host removes a player and the freed name/seat can be reused', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)

  await joinPlayer(await browser.newContext(), code, 'Player1')
  await joinPlayer(await browser.newContext(), code, 'Player2')

  // Both players are visible in the host's player list.
  await expect(host.getByText('Player1')).toBeVisible()
  await expect(host.getByText('Player2')).toBeVisible()

  // Kick Player2 (confirm the dialog).
  host.on('dialog', (d) => d.accept())
  await host.locator('li', { hasText: 'Player2' }).getByRole('button', { name: 'Кикнуть' }).click()

  // Player2 disappears from the list; Player1 stays.
  await expect(host.getByText('Player2')).toHaveCount(0)
  await expect(host.getByText('Player1')).toBeVisible()

  // The kicked seat and name are free again: a new player can join as "Player2".
  await joinPlayer(await browser.newContext(), code, 'Player2')
  await expect(host.getByText('Player2')).toBeVisible()
})