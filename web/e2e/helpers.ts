import { expect, type BrowserContext, type Page } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

/** Tiny PNG fixtures used for meme uploads (distinct content, so distinct sha256). */
const fixturesDir = path.join(__dirname, 'fixtures')
export const FIXTURES = fs
  .readdirSync(fixturesDir)
  .filter((f) => f.endsWith('.png'))
  .sort()
  .map((f) => path.join(fixturesDir, f))

/** Player-view phase headings used to detect the current phase. */
const PHASE_HEADINGS: Record<string, RegExp> = {
  PREPARATION: /Подготовка/,
  ROUND_SELECTION: /Выбор мема/,
  ROUND_VOTING: /Голосование/,
  ROUND_RESULTS: /Результаты раунда/,
  CYCLE_RESULTS: /Конец цикла/,
  GAME_RESULTS: /Игра окончена!/,
}

// --- Host helpers ---

/** Opens /host, bootstraps admin, creates a room, and returns its code. */
export async function createHostRoom(page: Page): Promise<string> {
  await page.goto('/host')
  await page.getByRole('button', { name: 'Создать комнату' }).click()
  const codeEl = page.locator('.font-mono.text-2xl').first()
  await expect(codeEl).toBeVisible()
  const code = (await codeEl.textContent())!.trim()
  expect(code).toMatch(/^[A-Z0-9]{6}$/)
  return code
}

/**
 * Uploads the fixture memes via the host UI until at least `target` enabled
 * memes exist. Memes are global to the server and deduplicated by content
 * (sha256 UNIQUE), so each distinct fixture is uploaded one at a time and
 * already-present fixtures are skipped.
 */
export async function ensureMemes(page: Page, target: number): Promise<void> {
  const input = page.locator('input[type="file"]')
  for (let attempt = 0; attempt < FIXTURES.length * 2; attempt++) {
    if ((await memeCount(page)) >= target) return
    const fixture = FIXTURES[attempt % FIXTURES.length]
    await input.setInputFiles(fixture)
    await page.waitForTimeout(500)
  }
  throw new Error(`Failed to reach ${target} enabled memes`)
}

async function memeCount(page: Page): Promise<number> {
  return page.evaluate(async () => {
    const res = await fetch('/api/v1/memes')
    const memes = ((await res.json()) as Array<{ enabled: boolean }> | null) ?? []
    return memes.filter((m) => m.enabled).length
  })
}

/** Adds situations via the host UI (ContentManager "Ситуации" tab). */
export async function addSituations(page: Page, texts: string[]): Promise<void> {
  await page.getByRole('button', { name: /Ситуации/ }).click()
  const textarea = page.locator('textarea[placeholder="Новая ситуация…"]')
  for (const text of texts) {
    await textarea.fill(text)
    await page.getByRole('button', { name: 'Добавить' }).click()
    await page.waitForTimeout(300)
  }
}

/**
 * Starts the game from the host context by clicking the "Начать игру" button.
 * The button is enabled once enough players have joined (snapshot.players >=
 * snapshot.settings.minPlayers), so we wait for it to become enabled and then
 * click it.
 */
export async function startGame(host: Page): Promise<void> {
  const startButton = host.getByRole('button', { name: 'Начать игру' })
  await expect(startButton).toBeEnabled()
  await startButton.click()
}

/** Sets infinite-game mode via the admin-authenticated settings API. */
export async function setInfiniteGame(host: Page, code: string): Promise<void> {
  await host.evaluate(async (code) => {
    const res = await fetch(`/api/v1/rooms/${code}/settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ infiniteGame: true }),
    })
    if (!res.ok) throw new Error(`settings failed: ${await res.text()}`)
  }, code)
}

// --- Player helpers ---

/** Opens /play/<code> in a fresh context, joins with `name`, returns the page. */
export async function joinPlayer(context: BrowserContext, code: string, name: string): Promise<Page> {
  const page = await context.newPage()
  await page.goto(`/play/${code}`)
  await page.locator('input[placeholder="Ваше имя"]').fill(name)
  await page.getByRole('button', { name: 'Присоединиться' }).click()
  await expect(page.getByText('Ждём игроков…')).toBeVisible()
  return page
}

/** Waits until the player view shows the given phase heading. */
export async function waitForPhase(page: Page, phase: string): Promise<void> {
  await expect(page.getByRole('heading', { name: PHASE_HEADINGS[phase] })).toBeVisible()
}

/** Waits until the host view shows the given phase in its state card. */
export async function waitForHostPhase(host: Page, phase: string): Promise<void> {
  await expect(host.getByText(`Фаза: ${phase}`).first()).toBeVisible()
}

/**
 * Clicks the first element matching `selector` via the DOM, retrying on re-renders.
 */
async function domClick(page: Page, selector: string): Promise<void> {
  for (let attempt = 0; attempt < 10; attempt++) {
    const clicked = await page.evaluate((sel) => {
      const el = document.querySelector(sel) as HTMLElement | null
      if (el) {
        el.click()
        return true
      }
      return false
    }, selector)
    if (clicked) return
    await page.waitForTimeout(300)
  }
  throw new Error(`Failed to DOM-click ${selector}`)
}

/** Completes the PREPARATION phase for a player (does not wait for the next phase). */
export async function completePreparation(page: Page, situationText: string): Promise<void> {
  await waitForPhase(page, 'PREPARATION')
  // Already prepared (e.g. after a reconnect) — nothing to do.
  if (await page.getByText('Готово!').isVisible().catch(() => false)) return
  await page.locator('textarea[placeholder="Придумайте ситуацию под мем…"]').fill(situationText)
  await domClick(page, '[data-testid="meme-option"]')
  await expect(page.locator('[data-testid="ready-button"]')).toBeEnabled()
  await domClick(page, '[data-testid="ready-button"]')
}

/** Completes ROUND_SELECTION for a player (active player just waits). */
export async function completeRoundSelection(page: Page): Promise<void> {
  await waitForPhase(page, 'ROUND_SELECTION')
  // Wait for either the active-player notice or the meme grid to render.
  await expect(
    page.getByText('Вы — активный игрок').or(page.locator('[data-testid="meme-option"]').first()),
  ).toBeVisible()
  if (await page.getByText('Вы — активный игрок').isVisible().catch(() => false)) return
  // Select a meme via DOM (reliable despite WebSocket-triggered re-renders).
  await domClick(page, '[data-testid="meme-option"]')
  await expect(page.locator('[data-testid="ready-button"]')).toBeEnabled()
  await domClick(page, '[data-testid="ready-button"]')
}

/** Completes ROUND_VOTING for a player, avoiding their own (forbidden) option. */
export async function completeVoting(page: Page): Promise<void> {
  await waitForPhase(page, 'ROUND_VOTING')
  // Wait for either the active-player notice or the vote options to render.
  await expect(
    page.getByText('Идёт голосование').or(page.locator('[data-testid="vote-option"]:not([disabled])').first()),
  ).toBeVisible()
  if (await page.getByText('Идёт голосование').isVisible().catch(() => false)) return
  // Select a non-forbidden vote option via DOM (reliable despite re-renders).
  await domClick(page, '[data-testid="vote-option"]:not([disabled])')
  await expect(page.locator('[data-testid="ready-button"]')).toBeEnabled()
  await domClick(page, '[data-testid="ready-button"]')
}

// --- API helpers (admin-authenticated via the host page) ---

/** Returns the current leaderboard (name -> score) from the host snapshot. */
export async function getLeaderboard(host: Page, code: string): Promise<Array<{ displayName: string; score: number }>> {
  return host.evaluate(async (code) => {
    const res = await fetch(`/api/v1/rooms/${code}/state`)
    const snap = (await res.json()) as {
      game: { leaderboard: Array<{ displayName: string; score: number }> } | null
    }
    return snap.game?.leaderboard ?? []
  }, code)
}

/** Returns the number of game players from the host snapshot. */
export async function countPlayers(host: Page, code: string): Promise<number> {
  return host.evaluate(async (code) => {
    const res = await fetch(`/api/v1/rooms/${code}/state`)
    const snap = (await res.json()) as { game: { players: unknown[] } | null }
    return snap.game?.players.length ?? 0
  }, code)
}