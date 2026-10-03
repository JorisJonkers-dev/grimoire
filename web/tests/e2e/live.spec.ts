import AxeBuilder from '@axe-core/playwright'
import { type Browser, expect, type Page, test } from '@playwright/test'

async function joinAs(browser: Browser, subject: string, link: string, name: string): Promise<{ page: Page; frames: string[] }> {
  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': subject } })
  const page = await context.newPage()
  const frames: string[] = []
  page.on('websocket', (ws) => {
    ws.on('framereceived', (f) => frames.push(String(f.payload)))
  })
  await page.goto(new URL(link).pathname + new URL(link).hash)
  await page.getByTestId('join-display-name').fill(name)
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText(`${name} (you)`)
  return { page, frames }
}

test('tokens move live on every screen and hidden ones never leave the DM', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Live ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = await page.getByTestId('invite-link').inputValue()

  const one = await joinAs(browser, `e2e-live-a-${stamp}`, link, 'Aria')
  const two = await joinAs(browser, `e2e-live-b-${stamp}`, link, 'Brom')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  for (const p of [one.page, two.page]) {
    await p.goto(sessionUrl)
    await expect(p.getByTestId('connection')).toHaveText('Live')
    await expect(p.getByTestId('dm-controls')).toHaveCount(0)
  }
  const tableContext = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-live-a-${stamp}` }, viewport: { width: 1920, height: 1080 } })
  const table = await tableContext.newPage()
  const tableFrames: string[] = []
  table.on('websocket', (ws) => {
    ws.on('framereceived', (f) => tableFrames.push(String(f.payload)))
  })
  await table.goto(`${sessionUrl}/table`)
  await expect(table.getByTestId('table-display').getByRole('group', { name: 'The table' })).toBeVisible()
  await expect(table.getByRole('banner')).toHaveCount(0)

  await page.getByTestId('token-label').fill('Goblin')
  await page.locator('[data-hex="1,0"]').click()
  await page.getByTestId('token-label').fill('Lurker')
  await page.getByTestId('token-hidden').check()
  await page.locator('[data-hex="-1,0"]').click()
  await expect(page.locator('[data-hex="-1,0"]')).toHaveAttribute('aria-label', /Lurker \(hidden\)/)

  for (const p of [one.page, two.page, table]) {
    await expect(p.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Goblin/)
  }
  await page.locator('[data-hex="1,0"]').click()
  await page.locator('[data-hex="2,0"]').click()
  await expect(page.getByTestId('walk-preview')).toContainText('Walk 5 ft')
  await page.getByTestId('confirm-walk').click()
  for (const p of [one.page, two.page, table]) {
    await expect(p.locator('[data-hex="2,0"]')).toHaveAttribute('aria-label', /Goblin/)
    await expect(p.locator('[data-hex="-1,0"]')).toHaveAttribute('aria-label', 'Hex -1, 0')
  }
  for (const frames of [one.frames, two.frames, tableFrames]) {
    expect(frames.length).toBeGreaterThan(0)
    expect(frames.join('\n')).not.toContain('Lurker')
  }
  expect((await new AxeBuilder({ page: one.page }).analyze()).violations).toEqual([])

  await page.locator('[data-hex="-1,0"]').click()
  await page.getByTestId('toggle-hidden').click()
  for (const p of [one.page, two.page, table]) {
    await expect(p.locator('[data-hex="-1,0"]')).toHaveAttribute('aria-label', /Lurker/)
  }
  await page.getByTestId('end-session').click()
  await expect(table.getByTestId('session-ended')).toBeVisible()
})
