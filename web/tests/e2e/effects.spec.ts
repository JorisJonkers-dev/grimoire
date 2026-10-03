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

test('conditions show everywhere, shape attack previews, and unmodelled effects go to the DM', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Effects ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-fx-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await place(page, '0,0', 'Scout', 'party', 'Aria')
  await place(page, '1,0', 'Grik', 'enemy')

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(player.getByTestId('roll-card'), '20')
  await enter(page.getByTestId('roll-card').filter({ hasText: 'Initiative for Grik' }), '5')
  await expect(player.getByTestId('your-turn')).toBeVisible()

  await page.locator('[data-hex="0,0"]').click()
  const panel = page.getByTestId('effects-panel')
  await panel.getByTestId('effect-name').fill('prone')
  await panel.getByTestId('apply-effect').click()
  await expect(player.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Scout .*· Prone/)
  await player.getByTestId('hotbar-Scout').getByTestId('attack-0').click()
  await player.locator('[data-hex="1,0"]').click()
  await expect(player.getByTestId('attack-preview')).toContainText('Prone: disadvantage')
  await player.getByTestId('cancel-attack').click()

  await panel.getByTestId('effect-name').fill('hold-person')
  await panel.getByTestId('effect-save').selectOption('wisdom')
  await panel.getByTestId('effect-dc').fill('13')
  await panel.getByTestId('apply-effect').click()
  await expect(page.getByTestId('manual')).toContainText('Scout: Resolve hold-person by hand.')
  await expect(player.getByTestId('resolving')).toBeVisible()
  await page.getByTestId('manual').getByRole('button', { name: 'Done' }).click()
  await expect(player.getByTestId('resolving')).toHaveCount(0)
  await panel.getByTestId('end-effect-prone').click()
  await expect(player.locator('[data-hex="0,0"]')).not.toHaveAttribute('aria-label', /Prone/)
})
