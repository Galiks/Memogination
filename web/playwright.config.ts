import { defineConfig } from '@playwright/test'
import path from 'node:path'
import os from 'node:os'
import fs from 'node:fs'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(__dirname, '..')

const PORT = 18099
const BASE_URL = `http://127.0.0.1:${PORT}`

// Use a fresh temp dir per run so each E2E run starts from a clean data dir.
const runDir = path.join(os.tmpdir(), `memomarium-e2e-${Date.now()}`)
const binPath = path.join(runDir, process.platform === 'win32' ? 'memomarium.exe' : 'memomarium')
const dataDir = path.join(runDir, 'data')
fs.mkdirSync(runDir, { recursive: true })

export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
  },
  webServer: {
    // Build the Go binary (which embeds web/dist) and run it on the E2E port
    // with a throwaway data dir. reuseExistingServer lets local dev reuse a
    // manually started server; CI always starts a fresh one.
    command: `go build -o "${binPath}" ./cmd/memomarium && "${binPath}" --data-dir "${dataDir}" --addr 127.0.0.1:${PORT}`,
    cwd: repoRoot,
    url: `${BASE_URL}/api/v1/health`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
})