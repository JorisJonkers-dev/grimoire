import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const crypt = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAeAAAAGQCAIAAAD5lCQDAAAER0lEQVR42u3UMQ0AAAzDsMLZvXv8cY1GD0tGkCO5HQAKRQIAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaAAMGsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKBVADBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMWgUAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaAAMGsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMGgCDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAIOWAMCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaBUADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMGgCDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAINWAcCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAA6PKO0jbq+yoX5AAAAAElFTkSuQmCC',
  'base64',
)

test('the party sees only lit hexes of a dark map and never the creatures beyond them', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Fog ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())

  await page.getByTestId('maps-link').click()
  await page.getByTestId('map-name').fill('Crypt')
  await page.getByTestId('map-file').setInputFiles({ name: 'crypt.png', mimeType: 'image/png', buffer: crypt })
  await page.getByRole('button', { name: 'Upload map' }).click()
  await expect(page.getByRole('heading', { name: 'Crypt' })).toBeVisible()
  await page.getByLabel('Ambient light').selectOption('dark')
  await page.getByRole('button', { name: 'Save calibration' }).click()
  await expect(page.getByTestId('calibration-saved')).toBeVisible()
  await page.getByRole('link', { name: '← Maps' }).click()
  await expect(page.getByTestId('map-list')).toContainText('dark')
  await page.getByRole('link', { name: '← Campaign' }).click()

  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-fog-${stamp}` } })
  const player = await context.newPage()
  const frames: string[] = []
  player.on('websocket', (ws) => {
    ws.on('framereceived', (f) => frames.push(String(f.payload)))
  })
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await page.getByTestId('map-choice').selectOption({ label: 'Crypt' })
  await page.getByTestId('use-map').click()
  await expect(page.getByTestId('map-board')).toBeVisible()
  await player.goto(new URL(page.url()).pathname)
  await expect(player.getByTestId('connection')).toHaveText('Live')

  await page.getByTestId('tool-light').check()
  await page.getByLabel('Bright (ft)').fill('5')
  await page.getByLabel('Dim (ft)').fill('10')
  await page.locator('[data-hex="0,0"]').click()
  await expect(page.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /light/)
  await page.getByTestId('tool-tokens').check()
  await page.getByTestId('token-label').fill('Aria')
  await page.getByTestId('token-kind').selectOption('party')
  await page.locator('[data-hex="0,0"]').click()
  await page.getByTestId('token-label').fill('Shade')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.locator('[data-hex="5,0"]').click()
  await page.getByTestId('token-label').fill('Lurker')
  await page.getByTestId('token-hidden').check()
  await page.locator('[data-hex="1,0"]').click()
  await expect(page.locator('[data-hex="5,0"]')).toHaveAttribute('aria-label', /Shade/)
  await expect(page.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Lurker \(hidden\)/)

  await expect(player.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Aria/)
  await expect(player.locator('[data-hex="0,0"]')).toHaveClass(/cell--lit/)
  await expect(player.locator('[data-hex="5,0"]')).toHaveAttribute('aria-label', 'Hex 5, 0: never seen')
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', 'Hex 1, 0')
  const seen = frames.join('\n')
  expect(seen).toContain('Aria')
  expect(seen).not.toContain('Shade')
  expect(seen).not.toContain('Lurker')
  expect(seen).not.toContain('"walls"')
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])

  const image = await player.locator('[data-testid="map-board"] image').getAttribute('href')
  const masked = await player.request.get(String(image), { headers: { 'X-User-Id': `e2e-fog-${stamp}` } })
  expect(masked.headers()['content-type']).toBe('image/png')
  expect(Buffer.compare(await masked.body(), crypt)).not.toBe(0)
})
