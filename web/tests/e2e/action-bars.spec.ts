import AxeBuilder from '@axe-core/playwright'
import { devices, expect, type Page, test } from '@playwright/test'
import { buildFighter } from './wizard'

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('manual-0').click()
  await card.getByTestId('pad-0').getByRole('button', { name: face, exact: true }).click()
}

test('a player arranges their Character\'s action bars, and finds them the same on a phone', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Bars ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const user = { 'X-User-Id': `e2e-bars-${stamp}` }
  const player = await (await browser.newContext({ extraHTTPHeaders: user })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await player.getByTestId('build-character').click()
  await buildFighter(player, 'Mira', true)
  await player.getByTestId('create-character').click()
  await expect(player.getByRole('heading', { name: 'Mira' })).toBeVisible()

  await page.reload()
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  await page.getByTestId('token-character').selectOption({ label: 'Mira' })
  await page.locator('[data-hex="0,0"]').click()
  await expect(page.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Mira/)
  await page.getByTestId('token-character').selectOption('')
  await page.getByTestId('token-label').fill('Goblin')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.locator('[data-hex="3,0"]')).toHaveAttribute('aria-label', /Goblin/)
  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(page, '5')
  await enter(player, '20')

  // Until arranged the hotbar shows everything; arranging starts from that order, in edit mode.
  const hotbar = player.getByTestId('hotbar-Mira')
  await expect(hotbar.getByTestId('area-spell')).toBeVisible()
  await hotbar.getByTestId('arrange-bars').click()
  const bars = player.getByTestId('action-bars')
  await expect(bars.getByTestId('bar-1').locator('.tile:not(.tile--add)')).toHaveCount(10)
  await expect(bars.getByTestId('edit-bars')).toHaveAttribute('aria-pressed', 'true')
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])

  // Drag Dodge to the front, put Dash away, and move the sword to the second bar by tapping.
  await bars.getByTestId('action-dodge').dragTo(bars.getByTestId('bar-1').locator('.tile').first())
  await expect(bars.getByTestId('bar-1').locator('.tile').first()).toContainText('Dodge')
  await bars.getByTestId('stow-action:dash').click()
  await expect(bars.getByTestId('stowed-action:dash')).toBeVisible()
  await bars.getByTestId('bar-1').locator('.tile', { hasText: 'Longsword' }).click()
  await bars.getByTestId('bar-2').locator('.tile').first().click()
  await expect(bars.getByTestId('bar-2').locator('.tile').first()).toContainText('Longsword')
  await bars.getByTestId('edit-bars').click()
  await expect(bars.getByTestId('bar-drawer')).toHaveCount(0)
  await expect(player.getByTestId('quick-bar')).toBeHidden()

  // The same Character on a phone: the same bars, a quick bar on the map, and a hold to edit.
  const phone = await (await browser.newContext({ ...devices['Pixel 7'], extraHTTPHeaders: user })).newPage()
  await phone.goto(sessionUrl)
  const small = phone.getByTestId('action-bars')
  await expect(small.getByTestId('bar-1').locator('.tile').first()).toContainText('Dodge')
  await expect(small.getByTestId('bar-2').locator('.tile').first()).toContainText('Longsword')
  await expect(small.getByTestId('action-dash')).toHaveCount(0)
  const quick = phone.getByTestId('stage').getByTestId('quick-bar')
  await expect(quick).toBeVisible()
  await expect(quick.getByRole('button')).toHaveCount(3)
  await small.getByTestId('action-dodge').dispatchEvent('pointerdown')
  await expect(small.getByTestId('edit-bars')).toHaveAttribute('aria-pressed', 'true')
  await small.getByTestId('add-to-bar-1').click()
  await small.getByTestId('stowed-action:dash').click()
  await expect(small.getByTestId('bar-1').locator('.tile:not(.tile--add)').last()).toContainText('Dash')
  await small.getByTestId('edit-bars').click()
  expect((await new AxeBuilder({ page: phone }).analyze()).violations).toEqual([])

  // Back on the desktop the key 1 plays the first tile: Dodge, which takes the action.
  await player.reload()
  await expect(player.getByTestId('bar-1').locator('.tile').first()).toContainText('Dodge')
  await expect(player.getByTestId('bar-1').locator('.tile').last()).toContainText('Dash')
  await player.keyboard.press('1')
  await expect(player.getByTestId('hotbar-blocked')).toContainText('The action is used this turn.')
  await expect(player.getByTestId('action-dodge')).toBeDisabled()
  await expect(quick.getByRole('button').first()).toBeDisabled()
})
