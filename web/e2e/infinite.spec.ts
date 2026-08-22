import { expect, test } from '@playwright/test'
import {
  addSituations,
  completePreparation,
  completeRoundSelection,
  completeVoting,
  createHostRoom,
  ensureMemes,
  getLeaderboard,
  joinPlayer,
  setInfiniteGame,
  startGame,
  waitForHostPhase,
} from './helpers'

// 3 players with the default hand size (5) require 15 enabled memes to start.
const MEME_TARGET = 18
const PLAYER_COUNT = 3

test('infinite game: complete cycle 1, start cycle 2, scores preserved', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)
  await ensureMemes(host, MEME_TARGET)
  await addSituations(host, ['Situation A', 'Situation B', 'Situation C'])
  await setInfiniteGame(host, code)

  const players = []
  for (let i = 0; i < PLAYER_COUNT; i++) {
    const ctx = await browser.newContext()
    players.push(await joinPlayer(ctx, code, `Player${i + 1}`))
  }

  await startGame(host)
  await waitForHostPhase(host, 'PREPARATION')

  for (let i = 0; i < players.length; i++) {
    await completePreparation(players[i], `Situation for player ${i + 1}`)
  }
  await waitForHostPhase(host, 'ROUND_SELECTION')

  // Cycle 1: one round per player.
  for (let round = 0; round < PLAYER_COUNT; round++) {
    for (const p of players) await completeRoundSelection(p)
    await waitForHostPhase(host, 'ROUND_VOTING')

    for (const p of players) await completeVoting(p)
    await waitForHostPhase(host, 'ROUND_RESULTS')

    await host.getByRole('button', { name: 'Следующий раунд' }).click()
  }

  // Cycle 1 finished.
  await waitForHostPhase(host, 'CYCLE_RESULTS')
  const scoresBefore = await getLeaderboard(host, code)
  expect(scoresBefore.length).toBe(PLAYER_COUNT)

  // Host starts the next cycle.
  await host.getByRole('button', { name: 'Следующий цикл' }).click()

  // New cycle begins in PREPARATION and scores are preserved.
  await waitForHostPhase(host, 'PREPARATION')
  const scoresAfter = await getLeaderboard(host, code)
  expect(scoresAfter).toEqual(scoresBefore)
})