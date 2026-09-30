import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

async function enter(page: Page, faces: string[]) {
  const card = page.getByTestId('roll-card')
  for (const [i, face] of faces.entries()) {
    await card.getByTestId(`manual-${String(i)}`).click()
    await card.getByTestId(`pad-${String(i)}`).getByRole('button', { name: face, exact: true }).click()
  }
}

test('a hidden goblin springs its zone on the party, and whoever missed it is surprised', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Ambush ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())

  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-ambush-${stamp}` } })
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
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  await expect(player.getByTestId('connection')).toHaveText('Live')
  const table = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-ambush-${stamp}` } })).newPage()
  await table.goto(`${sessionUrl}/table`)

  await page.getByTestId('token-label').fill('Aria')
  await page.getByTestId('token-kind').selectOption('party')
  await page.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await page.locator('[data-hex="-3,0"]').click()
  await page.getByTestId('token-label').fill('')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.getByTestId('token-controller').selectOption('')
  await page.getByTestId('token-monster').fill('goblin')
  await page.getByTestId('token-hidden').check()
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.locator('[data-hex="3,0"]')).toHaveAttribute('aria-label', /Goblin.*\(hidden\)/)
  await page.getByTestId('tool-zone').check()
  await page.getByTestId('zone-name').fill('Ambush')
  await page.getByTestId('zone-radius').fill('2')
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.getByTestId('zone-Ambush')).toContainText('1 hidden · Armed')

  await player.locator('[data-hex="1,0"]').click()
  await expect(player.getByTestId('walk-preview')).toBeVisible()
  await player.locator('[data-hex="1,0"]').click()
  await expect(page.getByTestId('zone-Ambush')).toContainText('Waiting for Perception · Stealth DC 16')
  await expect(player.getByTestId('roll-card')).toContainText('Perception for Aria')
  await expect(player.locator('[data-hex="3,0"]')).not.toHaveAttribute('aria-label', /Goblin/)
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])
  await enter(player, ['3'])

  for (const p of [page, player, table]) {
    await expect(p.getByTestId('initiative-rail')).toContainText('Rolling initiative')
    await expect(p.getByTestId('rail-Aria').getByTestId('surprised')).toBeVisible()
  }
  await expect(player.locator('[data-hex="3,0"]')).toHaveAttribute('aria-label', /Goblin/)
  await expect(page.getByTestId('zone-Ambush')).toContainText('Sprung')
  await expect(player.getByTestId('roll-card')).toContainText('Initiative for Aria (surprised)')
  await enter(player, ['18', '4'])
  await expect(page.getByTestId('rail-Aria')).toContainText('4')
  const beforeFight = frames.slice(0, frames.findIndex((f) => f.includes('"combat"')))
  expect(beforeFight.join('\n')).not.toContain('Goblin')
  expect(frames.join('\n')).not.toContain('Ambush')
})
