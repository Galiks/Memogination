import { expect, test } from '@playwright/test'
import { addSituations, createHostRoom, ensureMemes, joinPlayer, startGame, waitForPhase } from './helpers'

// Defaults: MinPlayers=2, HandSize=5. startGame requires n*h = 10 enabled
// memes for 2 players (transitions.go), so we need at least 10; the player's
// preparation hand still has exactly HandSize=5 memes.
const MEME_TARGET = 10
const PLAYER_COUNT = 2
const VIEWPORT = { width: 390, height: 844 }

test('slider: real-touch swipe, fullscreen and tap-to-select on a player hand', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)
  await ensureMemes(host, MEME_TARGET)
  await addSituations(host, ['Situation A'])

  // Player pages use real touch contexts (mobile viewport).
  const players = []
  for (let i = 0; i < PLAYER_COUNT; i++) {
    const ctx = await browser.newContext({ viewport: VIEWPORT, hasTouch: true, isMobile: true })
    players.push(await joinPlayer(ctx, code, `Player${i + 1}`))
  }
  const player = players[0]

  await startGame(host)
  await waitForPhase(player, 'PREPARATION')

  // The preparation hand has HandSize=5 memes; the slider starts on index 0.
  const slide = player.getByTestId('meme-option')
  await expect(slide).toBeVisible()
  await expect(player.getByText('1 / 5')).toBeVisible()

  // Swipe left via a real touchscreen gesture (CDP touch events).
  await slide.scrollIntoViewIfNeeded()
  const box = await slide.boundingBox()
  expect(box).not.toBeNull()
  const startX = box!.x + box!.width / 2
  const y = box!.y + box!.height / 2
  const client = await player.context().newCDPSession(player)
  await client.send('Input.dispatchTouchEvent', {
    type: 'touchStart',
    touchPoints: [{ x: startX, y }],
  })
  for (let step = 1; step <= 5; step++) {
    await client.send('Input.dispatchTouchEvent', {
      type: 'touchMove',
      touchPoints: [{ x: startX - step * 60, y }],
    })
  }
  await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
  await expect(player.getByText('2 / 5')).toBeVisible()

  // Fullscreen overlay opens and closes via touch taps.
  await player.getByTestId('meme-fullscreen').tap()
  await expect(player.getByTestId('meme-fullscreen-overlay')).toBeVisible()
  await player.getByTestId('meme-fullscreen-close').tap()
  await expect(player.getByTestId('meme-fullscreen-overlay')).toBeHidden()

  // Tap-to-select on the slide must work on touch: a deliberate tap right
  // after a swipe is not suppressed (FIX 2 regression guard).
  await slide.tap()
  await expect(slide).toHaveClass(/ring-4/)
})