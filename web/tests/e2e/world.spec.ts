import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const realm = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAeAAAAGQCAIAAAD5lCQDAAAER0lEQVR42u3UMQ0AAAzDsMLZvXv8cY1GD0tGkCO5HQAKRQIAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaAAMGsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKBVADBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMWgUAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaAAMGsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGwKABDBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMGgCDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAIOWAMCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaBUADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEwaACDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAAwawKABMGgAgwbAoAEMGgCDBsCgAQwaAIMGMGgADBoAgwYwaAAMGsCgATBoAINWAcCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwaAIMGMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwbAoAEMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAAMGgCDBjBoAAwawKABMGgADBrAoAEwaACDBsCgATBoAIMGwKABDBoAgwYwaAA6PKO0jbq+yoX5AAAAAElFTkSuQmCC',
  'base64',
)

test('the DM draws a world map, the party travels it, and places ahead stay hidden', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`World ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())

  await page.getByTestId('maps-link').click()
  await page.getByTestId('map-name').fill('Realm')
  await page.getByTestId('map-kind').selectOption('world')
  await page.getByTestId('map-file').setInputFiles({ name: 'realm.png', mimeType: 'image/png', buffer: realm })
  await page.getByRole('button', { name: 'Upload map' }).click()
  await expect(page.getByRole('heading', { name: 'Realm' })).toBeVisible()
  await page.getByRole('link', { name: '← Maps' }).click()
  await expect(page.getByTestId('map-list')).toContainText('world map')
  await page.getByTestId('to-campaign').click()

  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-world-${stamp}` } })
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
  await expect(page.getByTestId('map-choice').locator('option')).toHaveText(['No map (open grid)'])
  await page.getByTestId('scope-world').check()
  await page.getByTestId('world-choice').selectOption({ label: 'Realm' })
  await page.getByTestId('use-world').click()
  await expect(page.getByTestId('party-at')).toHaveText('The party is not on this map yet.')
  for (const [name, hex] of [['Oakford', '0,0'], ['Mill', '5,0'], ['Keep', '-2,5']] as const) {
    await page.getByTestId('world-node-name').fill(name)
    await page.locator(`[data-hex="${hex}"]`).click()
    await expect(page.locator(`[data-node="${name}"]`)).toHaveCount(1)
  }
  await page.getByTestId('world-tool-route').check()
  await page.getByTestId('world-distance').fill('12')
  await page.locator('[data-hex="0,0"]').click()
  await page.locator('[data-hex="5,0"]').click()
  await expect(page.locator('[data-route]')).toHaveCount(1)
  await page.getByTestId('world-tool-party').check()
  await page.locator('[data-hex="0,0"]').click()
  await expect(page.getByTestId('party-at')).toHaveText('The party is at Oakford.')

  await player.goto(new URL(page.url()).pathname)
  await expect(player.getByTestId('connection')).toHaveText('Live')
  await player.getByTestId('scope-world').check()
  await expect(player.getByTestId('party-marker')).toHaveCount(1)
  await expect(player.getByTestId('roads')).toContainText('Mill · 12 mi · 4 h')
  await expect(player.locator('[data-node="Keep"]')).toHaveCount(0)
  await expect(player.locator('[data-hex="5,0"]')).toHaveClass(/cell--unseen/)
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])

  await page.getByTestId('pace').selectOption('fast')
  await page.getByRole('button', { name: 'Travel to Mill' }).click()
  await expect(page.getByTestId('party-at')).toHaveText('The party is at Mill.')
  await expect(player.getByTestId('party-at')).toHaveText('The party is at Mill.')
  await expect(player.getByTestId('legs')).toContainText('Oakford → Mill · 12 mi at a fast pace · 3 h')
  await expect(player.locator('[data-hex="5,0"]')).toHaveClass(/cell--lit/)
  expect(frames.join('\n')).not.toContain('Keep')

  await page.getByTestId('scope-local').check()
  await expect(page.getByTestId('dm-controls')).toBeVisible()
})
