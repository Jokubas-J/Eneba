import { defineConfig, devices } from '@playwright/test'
import { existsSync, readFileSync } from 'node:fs'
import { parseEnv } from 'node:util'

// End-to-end test of the whole purchase: a real browser, the real Go server and a real
// PostgreSQL database. Run with `npm run e2e` (docs/setup.md §7).
//
// The database is the Neon *test* branch (TEST_DATABASE_URL), never dev or main: the
// project rule for every database test. The backend's own .env is read only to find
// that URL; it is then passed to the server as its DATABASE_URL.
const backendEnv = existsSync('../backend/.env') ? parseEnv(readFileSync('../backend/.env', 'utf8')) : {}
const testDatabaseURL = process.env.TEST_DATABASE_URL ?? backendEnv.TEST_DATABASE_URL
const devDatabaseURL = process.env.DATABASE_URL ?? backendEnv.DATABASE_URL
if (!testDatabaseURL) {
  throw new Error('TEST_DATABASE_URL is not set (backend/.env); the e2e test only runs against the test branch')
}
if (testDatabaseURL === devDatabaseURL) {
  throw new Error('TEST_DATABASE_URL equals DATABASE_URL; the e2e test must use its own database')
}

// Own ports, so a running development server (8080 / 5173) is never reused by mistake.
const apiPort = 8091
const webPort = 5174

export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.e2e.ts', // not *.spec.ts, which Vitest would also pick up
  // One purchase flow against one shared database: nothing to run in parallel.
  workers: 1,
  timeout: 120_000,
  expect: { timeout: 20_000 },
  reporter: 'list',
  use: {
    baseURL: `http://localhost:${webPort}`,
    ...devices['Desktop Chrome'],
    trace: 'retain-on-failure',
    // A missing element fails within 20 s instead of using up the whole test timeout.
    actionTimeout: 20_000,
  },
  webServer: [
    {
      // Load the real catalog snapshot into the test branch first (seed load -reset
      // empties catalog and orders), then start the API with the simulated provider.
      command: 'go run ./cmd/seed load -reset && go run ./cmd/server',
      cwd: '../backend',
      env: {
        DATABASE_URL: testDatabaseURL,
        PORT: String(apiPort),
        APP_ENV: 'development',
        APP_BASE_URL: `http://localhost:${webPort}`,
        PAYMENT_PROVIDER: 'simulated',
      },
      url: `http://localhost:${apiPort}/api/healthz`,
      timeout: 300_000,
      reuseExistingServer: false,
      stdout: 'pipe',
    },
    {
      command: `npm run dev -- --port ${webPort} --strictPort`,
      env: { API_PROXY_TARGET: `http://localhost:${apiPort}` },
      url: `http://localhost:${webPort}`,
      timeout: 120_000,
      reuseExistingServer: false,
    },
  ],
})
