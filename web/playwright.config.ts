import { defineConfig, devices } from '@playwright/test'

const devSubject = 'e2e-player'
const apiOrigin = 'http://localhost:18765'

export default defineConfig({
  testDir: 'tests/e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: { baseURL: 'http://localhost:4173', trace: 'retain-on-failure' },
  projects: [
    { name: 'phone', use: { ...devices['Pixel 7'] } },
    { name: 'tablet', use: { ...devices['iPad (gen 7) landscape'], browserName: 'chromium' } },
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } } },
    { name: 'tv', use: { ...devices['Desktop Chrome'], viewport: { width: 1920, height: 1080 } } },
  ],
  webServer: process.env.E2E_EXTERNAL_SERVERS ? [] : [
    {
      command: 'go run ./cmd/grimoire serve',
      cwd: '../api',
      url: `${apiOrigin}/readyz`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: {
        GRIMOIRE_DATABASE_URL: process.env.GRIMOIRE_DATABASE_URL ?? 'postgres://grimoire:grimoire@localhost:5432/grimoire?sslmode=disable',
        GRIMOIRE_AUTO_MIGRATE: 'true',
        GRIMOIRE_DEV_SUBJECT: devSubject,
        GRIMOIRE_ADDR: ':18765',
      },
    },
    {
      command: 'pnpm build && pnpm preview',
      url: 'http://localhost:4173',
      env: { GRIMOIRE_API_ORIGIN: apiOrigin },
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
    },
  ],
})
