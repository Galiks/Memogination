import { expect, test } from '@playwright/test'
import {
  addSituations,
  completePreparation,
  countPlayers,
  createHostRoom,
  ensureMemes,
  joinPlayer,
  startGame,
  waitForHostPhase,
  waitForPhase,
} from './helpers'

// 3 players with the default hand size (5) require 15 enabled memes to start.
const MEME_TARGET = 18
const PLAYER_COUNT = 3

test('disconnect: a player reconnects without being duplicated and can continue', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)
  await ensureMemes(host, MEME_TARGET)
  await addSituations(host, ['Situation A', 'Situation B', 'Situation C'])

  const p1 = await joinPlayer(await browser.newContext(), code, 'Player1')
  const p2 = await joinPlayer(await browser.newContext(), code, 'Player2')
  const p3 = await joinPlayer(await browser.newContext(), code, 'Player3')

  await startGame(host)
  await waitForHostPhase(host, 'PREPARATION')

  // Player 3 disconnects during preparation by navigating away (closes the WS).
  await p3.goto('about:blank')

  // Reconnect: same browser context (same HttpOnly session cookie), reload the
  // room page. PlayerView.onMounted calls /reconnect with the stored session.
  await p3.goto(`/play/${code}`)
  await waitForPhase(p3, 'PREPARATION')

  // The player was NOT duplicated: still exactly PLAYER_COUNT game players.
  expect(await countPlayers(host, code)).toBe(PLAYER_COUNT)

  // The reconnected player can continue: complete preparation and the game
  // advances once everyone is ready.
  await completePreparation(p3, 'Situation for player 3')
  await completePreparation(p1, 'Situation for player 1')
  await completePreparation(p2, 'Situation for player 2')
  await waitForHostPhase(host, 'ROUND_SELECTION')
})