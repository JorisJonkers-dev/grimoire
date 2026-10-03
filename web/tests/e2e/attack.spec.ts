import { expect, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

async function place(page: Page, hex: string, label: string, kind = 'enemy') {
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-label').fill(label)
  await page.locator(`[data-hex="${hex}"]`).click()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(`${label} \\(10/10 HP\\)`))
}

test('the DM attacks through the hotbar and hit points change on every screen', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Attack ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-attack-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await place(page, '0,0', 'Grik')
  await place(page, '1,0', 'Snag')
  await place(page, '0,3', 'Ally', 'party')
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Snag \(unhurt\)/)

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('fights-Ally').uncheck()
  await page.getByTestId('begin-combat').click()
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Grik' }), '20')
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Snag' }), '5')
  await expect(page.getByTestId('rail-Grik')).toHaveAttribute('aria-current', 'step')

  const bar = page.getByTestId('hotbar-Grik')
  await expect(bar.getByTestId('suggestion')).toContainText('Shortbow attack against Ally. Simple: Ally is the nearest enemy, 15 ft away.')
  await bar.getByTestId('tactics').selectOption('off')
  await expect(bar.getByTestId('suggestion')).toHaveCount(0)
  await bar.getByTestId('attack-0').click()
  await page.locator('[data-hex="1,0"]').click()
  const preview = page.getByTestId('attack-preview')
  await expect(preview.getByTestId('hit-chance')).toHaveText('50% to hit')
  await expect(preview.getByTestId('damage-range')).toHaveText('3–8 damage')
  await preview.getByTestId('confirm-attack').click()
  await expect(page.getByTestId('pending-attack')).toContainText('waiting for the attack roll')
  await enter(page.getByTestId('roll-card').filter({ hasText: 'attack against Snag' }), '20')
  await expect(page.getByTestId('pending-attack')).toContainText('(critical): waiting for the damage roll')
  const damage = page.getByTestId('roll-card').filter({ hasText: 'damage to Snag (critical)' })
  await damage.getByTestId('face-0').fill('3')
  await damage.getByTestId('face-0').press('Enter')
  await damage.getByTestId('face-1').fill('4')
  await damage.getByTestId('face-1').press('Enter')

  await expect(page.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Snag \(1\/10 HP\)/)
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Snag \(bloodied\)/)
  await expect(bar.getByTestId('hotbar-blocked')).toHaveText('The action is used this turn.')

  await page.getByTestId('undo-damage').click()
  await expect(page.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Snag \(10\/10 HP\)/)
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Snag \(unhurt\)/)
})
