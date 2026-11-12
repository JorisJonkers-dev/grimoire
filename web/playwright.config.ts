import { defineConfig, devices } from '@playwright/test'

const devSubject = 'e2e-player'
const apiOrigin = 'http://localhost:18765'
const webOrigin = 'http://localhost:4173'
// Every test without its own identity shares the dev subject, and so one rate-limit bucket.
const rateLimit = '100000'
// A fake jorisjonkers.dev (api/cmd/fakeoidc) stands in for the real provider, which the suite never
// reaches. The secret is a test-only value; CI starts the same provider with the same settings.
const fakeOidc = { issuer: 'http://127.0.0.1:18766', clientId: 'grimoire', clientSecret: 'e2e-only-fake-oidc-secret' }

export default defineConfig({
  testDir: 'tests/e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  // A flaky test is a bug to fix, not to retry.
  retries: 0,
  expect: { timeout: process.env.CI ? 10_000 : 5_000 },
  // CI shards each write a blob report, which one job merges into a single HTML report.
  reporter: process.env.CI ? [['blob'], ['github'], ['list']] : 'list',
  use: { baseURL: webOrigin, trace: 'retain-on-failure' },
  projects: [
    { name: 'phone', use: { ...devices['Pixel 7'] } },
    { name: 'tablet', use: { ...devices['iPad (gen 7) landscape'], browserName: 'chromium' } },
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } } },
    { name: 'tv', use: { ...devices['Desktop Chrome'], viewport: { width: 1920, height: 1080 } } },
  ],
  webServer: process.env.E2E_EXTERNAL_SERVERS ? [] : [
    {
      command: `go run ./cmd/fakeoidc -issuer ${fakeOidc.issuer} -client-id ${fakeOidc.clientId} -client-secret ${fakeOidc.clientSecret} -redirect-uri ${webOrigin}/oidc/callback`,
      cwd: '../api',
      url: `${fakeOidc.issuer}/.well-known/openid-configuration`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
    },
    {
      command: 'go run ./cmd/grimoire serve',
      cwd: '../api',
      url: `${apiOrigin}/readyz`,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: {
        GRIMOIRE_DATABASE_URL: process.env.GRIMOIRE_DATABASE_URL ?? 'postgres://grimoire:grimoire@localhost:5432/grimoire?sslmode=disable',
        GRIMOIRE_AUTO_MIGRATE: 'true',
        GRIMOIRE_AUTO_IMPORT: 'true',
        GRIMOIRE_DEV_SUBJECT: devSubject,
        GRIMOIRE_ADDR: ':18765',
        GRIMOIRE_RATE_LIMIT_PER_MINUTE: rateLimit,
        GRIMOIRE_ADMIN_SUBJECTS: 'e2e-admin',
        // The OIDC callback is <base>/oidc/callback; the roles claim and role names stay Grimoire's
        // defaults, which are jorisjonkers.dev's.
        GRIMOIRE_BASE_URL: webOrigin,
        GRIMOIRE_OIDC_ISSUER: fakeOidc.issuer,
        GRIMOIRE_OIDC_CLIENT_ID: fakeOidc.clientId,
        GRIMOIRE_OIDC_CLIENT_SECRET: fakeOidc.clientSecret,
        GRIMOIRE_OIDC_NAME: 'jorisjonkers.dev',
      },
    },
    {
      command: 'pnpm build && pnpm preview',
      url: webOrigin,
      env: { GRIMOIRE_API_ORIGIN: apiOrigin },
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
    },
  ],
})
