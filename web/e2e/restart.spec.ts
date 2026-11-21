import { expect, test, type Page } from '@playwright/test'
import {
  addSituations,
  createHostRoom,
  ensureMemes,
  joinPlayer,
  waitForHostPhase,
} from './helpers'

// 4 players with the default hand size (5) require 20 enabled memes to start.
const MEME_TARGET = 24
const PLAYER_COUNT = 4
let cmdSeq = 0

interface ApiResult {
  ok: boolean
  status: number
  body: unknown
}

interface SnapshotLike {
  phaseData?: Record<string, unknown>
  actor?: { playerId?: string }
  game?: { players?: Array<{ id: string; playerId: string }> }
}

async function callApi(page: Page, code: string, path: string, init: RequestInit): Promise<ApiResult> {
  return page.evaluate(
    async ({ code, path, init }) => {
      const res = await fetch(`/api/v1/rooms/${code}${path}`, init)
      const body = res.status === 204 ? null : await res.json().catch(() => null)
      return { ok: res.ok, status: res.status, body }
    },
    { code, path, init },
  )
}

/** Sends a command using the calling page's cookies (admin or player session). */
async function cmd(page: Page, code: string, type: string, payload: Record<string, unknown> = {}): Promise<void> {
  const r = await callApi(page, code, '/commands', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ commandId: `c-${cmdSeq++}`, expectedRevision: -1, type, payload }),
  })
  if (!r.ok) throw new Error(`command ${type} failed (${r.status}): ${JSON.stringify(r.body)}`)
}

async function getState(page: Page, code: string): Promise<SnapshotLike> {
  const r = await callApi(page, code, '/state', { method: 'GET' })
  if (!r.ok) throw new Error(`state failed: ${JSON.stringify(r.body)}`)
  return r.body as SnapshotLike
}

/** Returns the active game-player id for the current round. */
async function activeGamePlayerId(host: Page, code: string): Promise<string> {
  const snap = await getState(host, code)
  return (snap.phaseData?.activeGamePlayerId as string | undefined) ?? ''
}

/** Returns this player's game-player id from their own snapshot. */
async function gamePlayerIdOf(page: Page, code: string): Promise<string> {
  const snap = await getState(page, code)
  const playerId = snap.actor?.playerId
  return snap.game?.players?.find((p) => p.playerId === playerId)?.id ?? ''
}

test('restart: after the game ends the host can start a fresh game', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const code = await createHostRoom(host)
  await ensureMemes(host, MEME_TARGET)
  await addSituations(host, ['Situation A', 'Situation B', 'Situation C', 'Situation D'])

  const players: Page[] = []
  for (let i = 0; i < PLAYER_COUNT; i++) {
    players.push(await joinPlayer(await browser.newContext(), code, `Player${i + 1}`))
  }

  // Play a full game deterministically over the API.
  await cmd(host, code, 'START_GAME')
  for (let i = 0; i < players.length; i++) {
    const snap = await getState(players[i], code)
    const hand = (snap.phaseData?.hand as string[] | undefined) ?? []
    await cmd(players[i], code, 'SUBMIT_PREPARATION', {
      situationText: `Situation for player ${i + 1}`,
      memeId: hand[0],
    })
  }

  for (let round = 0; round < PLAYER_COUNT; round++) {
    const active = await activeGamePlayerId(host, code)
    const ids = new Map<Page, string>()
    for (const p of players) ids.set(p, await gamePlayerIdOf(p, code))

    // Round selection: non-active players submit a meme.
    for (const p of players) {
      if (ids.get(p) === active) continue
      const snap = await getState(p, code)
      const hand = (snap.phaseData?.hand as string[] | undefined) ?? []
      await cmd(p, code, 'SUBMIT_ROUND_MEME', { memeId: hand[0] })
    }

    // Voting: non-active players vote, never for their own (forbidden) option.
    for (const p of players) {
      if (ids.get(p) === active) continue
      const snap = await getState(p, code)
      const options = (snap.phaseData?.voteOptions as Array<{ id: string }> | undefined) ?? []
      const forbidden = snap.phaseData?.forbiddenOptionId as string | undefined
      const option = options.find((o) => o.id !== forbidden)
      if (!option) throw new Error(`player in round ${round} has no valid vote option`)
      await cmd(p, code, 'SUBMIT_VOTE', { voteOptionId: option.id })
    }

    // Advance to the next round, or finish the game on the last round.
    await cmd(host, code, 'NEXT_ROUND')
  }

  // Game over: the host UI shows the restart button.
  await waitForHostPhase(host, 'GAME_RESULTS')
  const restartButton = host.getByRole('button', { name: 'Начать игру заново' })
  await expect(restartButton).toBeVisible()

  // Clicking it resets the room and immediately starts a brand-new game.
  await restartButton.click()
  await waitForHostPhase(host, 'PREPARATION')
})