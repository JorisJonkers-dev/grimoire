import { expect, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('manual-0').click()
  await card.getByTestId('pad-0').getByRole('button', { name: face, exact: true }).click()
}

async function place(page: Page, hex: string, label: string, kind: string, controller = '', hidden = false) {
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
  await page.getByTestId('token-label').fill(label)
  await page.getByTestId('token-hidden').setChecked(hidden)
  await page.locator(`[data-hex="${hex}"]`).click()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

async function join(browser: import('@playwright/test').Browser, link: URL, user: string, name: string) {
  const page = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': user } })).newPage()
  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill(name)
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText(`${name} (you)`)
  return page
}

test('one roster strip for the DM, two players and the Table Display, each seeing what they may', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Roster ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const aria = await join(browser, link, `e2e-roster-a-${stamp}`, 'Aria')
  const bea = await join(browser, link, `e2e-roster-b-${stamp}`, 'Bea')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await aria.goto(sessionUrl)
  await bea.goto(sessionUrl)
  const table = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-roster-a-${stamp}` } })).newPage()
  await table.goto(`${sessionUrl}/table`)

  await place(page, '0,0', 'Scout', 'party', 'Aria')
  await place(page, '0,1', 'Brom', 'party', 'Bea')
  await place(page, '2,0', 'Grik', 'enemy')
  await place(page, '3,0', 'Lurker', 'enemy', '', true)

  await expect(page.getByTestId('roster-strip').getByRole('listitem')).toHaveCount(4)
  await expect(page.getByTestId('health-Grik')).toHaveAttribute('aria-label', /of \d+ hit points/)
  for (const p of [aria, bea, table]) {
    const strip = p.getByTestId('roster-strip')
    await expect(strip.getByRole('listitem')).toHaveCount(3)
    await expect(strip.getByTestId('rail-Lurker')).toHaveCount(0)
    await expect(p.getByTestId('health-Grik')).toHaveAttribute('aria-label', 'unhurt')
    await expect(p.getByTestId('health-Scout')).toHaveAttribute('aria-label', /of \d+ hit points/)
  }

  await page.locator('[data-hex="2,0"]').click()
  const panel = page.getByTestId('effects-panel')
  await panel.getByTestId('effect-name').fill('prone')
  await panel.getByTestId('apply-effect').click()
  await aria.getByTestId('rail-Grik').getByTestId('statuses').click()
  await expect(aria.getByTestId('effects-card')).toContainText('Prone')
  await aria.getByTestId('effects-card-close').click()
  await expect(aria.getByTestId('effects-card')).toHaveCount(0)

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(aria.getByTestId('roll-card'), '20')
  await enter(bea.getByTestId('roll-card'), '2')
  for (const label of ['Grik', 'Lurker']) await enter(page.getByTestId('roll-card').filter({ hasText: `Initiative for ${label}` }), '10')
  for (const p of [page, aria, bea, table]) {
    await expect(p.getByTestId('initiative-rail')).toContainText('Round 1')
    await expect(p.getByTestId('rail-Scout')).toHaveAttribute('aria-current', 'step')
    await expect(p.getByTestId('roster-strip').getByRole('listitem').first()).toHaveAttribute('data-testid', 'rail-Scout')
    await expect(p.getByTestId('roster-strip').getByRole('listitem').last()).toHaveAttribute('data-testid', 'rail-Brom')
  }
  await expect(aria.getByTestId('rail-Lurker')).toHaveCount(0)
  await expect(page.getByTestId('rail-Lurker')).toBeVisible()

  // The strip floats over the map, but the map underneath still takes clicks.
  await aria.locator('[data-hex="0,0"]').click()
  await aria.getByTestId('turn-Scout').getByTestId('end-turn').click()
  for (const p of [page, aria, bea, table]) await expect(p.getByTestId('rail-Scout')).not.toHaveAttribute('aria-current', 'step')
})
