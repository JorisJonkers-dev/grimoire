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

test('leaving reach prompts an opportunity attack that declines itself when nobody answers', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Reactions ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('reaction-timeout').fill('3')
  await page.getByRole('button', { name: 'Save settings' }).click()
  await expect(page.getByTestId('settings-saved')).toBeVisible()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-react-${stamp}` } })).newPage()
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

  await player.locator('[data-hex="-2,0"]').click()
  await expect(player.getByTestId('walk-preview')).toContainText('Walk 10 ft')
  // The plan warns of exactly the opportunity attack the walk then draws.
  await expect(player.getByTestId('walk-threats')).toHaveText("Leaving Grik's reach draws an opportunity attack.")
  await expect(player.getByTestId('walk-sight')).toContainText('Grik sees Scout there.')
  await expect(player.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /leaving here draws an opportunity attack/)
  await player.getByTestId('confirm-walk').click()
  const prompt = page.getByTestId('reaction-prompt')
  await expect(prompt).toContainText('Scout leaves Grik\'s reach')
  await expect(prompt.getByTestId('use-reaction')).toBeVisible()
  await expect(player.getByTestId('reaction-prompt')).toContainText("Waiting for Grik's reaction.")
  await expect(player.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Scout/)
  await expect(page.getByTestId('reaction-prompt')).toHaveCount(0, { timeout: 8000 })
  for (const p of [page, player]) await expect(p.locator('[data-hex="-2,0"]')).toHaveAttribute('aria-label', /Scout/)
})
