import { expect, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

async function place(page: Page, hex: string, label: string, kind: string, controller = '') {
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
  await page.getByTestId('token-label').fill(label)
  await page.locator(`[data-hex="${hex}"]`).click()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

test('burning hands catches a creature, its player rolls the save, and the grease in the cone catches fire', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Areas ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-area-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await place(page, '0,0', 'Grik', 'enemy')
  await place(page, '2,0', 'Scout', 'party', 'Aria')

  await page.getByTestId('tool-surface').check()
  await page.getByTestId('surface-kind').selectOption('grease')
  await page.locator('[data-hex="1,0"]').click()
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /grease/)
  await page.getByTestId('tool-tokens').check()

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Grik' }), '20')
  await enter(player.getByTestId('roll-card'), '5')
  await expect(page.getByTestId('rail-Grik')).toHaveAttribute('aria-current', 'step')

  await page.getByTestId('hotbar-Grik').getByTestId('area-spell').selectOption('burning-hands')
  await page.locator('[data-hex="1,0"]').click()
  const preview = page.getByTestId('area-preview')
  await expect(preview).toContainText('Scout')
  await expect(preview.getByTestId('ally-warning')).toHaveCount(0)
  await preview.getByTestId('confirm-area').click()
  await expect(player.getByTestId('roll-card')).toContainText('Dexterity save against Burning Hands')
  await enter(player.getByTestId('roll-card'), '1')
  const damage = page.getByTestId('roll-card').filter({ hasText: 'Burning Hands damage' })
  await damage.getByTestId('roll-rest').click()
  await expect(page.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /fire/)
  await expect(player.locator('[data-hex="2,0"]')).not.toHaveAttribute('aria-label', /10\/10 HP/)
})
