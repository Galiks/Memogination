import { expect, test } from '@playwright/test'
import {
  addSituations,
  completePreparation,
  completeRoundSelection,
  completeVoting,
  createHostRoom,
  ensureMemes,
  joinPlayer,
  startGame,
  waitForHostPhase,
} from './helpers'

// 4 players with the default hand size (5) require 20 enabled memes to start.
const MEME_TARGET = 24
const PLAYER_COUNT = 4

test('full game: host + 4 players play 4 rounds to GAME_RESULTS', async ({ browser }) => {
  // Host (admin) context.
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)
  await ensureMemes(host, MEME_TARGET)
  await addSituations(host, ['Situation A', 'Situation B', 'Situation C', 'Situation D'])

  // 4 player contexts.
  const players = []
  for (let i = 0; i < PLAYER_COUNT; i++) {
    const ctx = await browser.newContext()
    players.push(await joinPlayer(ctx, code, `Player${i + 1}`))
  }

  // Start the game by clicking the host's "Начать игру" button.
  await startGame(host)
  await waitForHostPhase(host, 'PREPARATION')

  // Each player completes preparation.
  for (let i = 0; i < players.length; i++) {
    await completePreparation(players[i], `Situation for player ${i + 1}`)
  }
  await waitForHostPhase(host, 'ROUND_SELECTION')

  // Play 4 rounds (one per active player).
  for (let round = 0; round < PLAYER_COUNT; round++) {
    for (const p of players) await completeRoundSelection(p)
    await waitForHostPhase(host, 'ROUND_VOTING')

    for (const p of players) await completeVoting(p)
    await waitForHostPhase(host, 'ROUND_RESULTS')

    await host.getByRole('button', { name: 'Следующий раунд' }).click()
  }

  // After the 4th round the game finishes.
  await waitForHostPhase(host, 'GAME_RESULTS')

  // A player view shows the winner announcement and a leaderboard.
  const winnerView = players[0]
  await expect(winnerView.getByText('Игра окончена!')).toBeVisible()
  await expect(winnerView.getByText('Победители:')).toBeVisible()
  await expect(winnerView.getByText(/Player[1-4]/).first()).toBeVisible()
})