import AxeBuilder from '@axe-core/playwright'
import { devices, expect, type Page, test } from '@playwright/test'

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

/** On the phone the DM sets a token up on Tools, then taps the map to place it. */
async function place(page: Page, hex: string, label: string, kind: string, controller = '') {
  await page.getByTestId('page-tools').tap()
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
  await page.getByTestId('token-label').fill(label)
  await page.getByTestId('page-map').tap()
  await page.locator(`[data-hex="${hex}"]`).tap()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

test('the DM runs creatures and the Table Display from a phone', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'phone', 'the remote is for the DM\'s phone')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Remote ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ ...devices['Desktop Chrome'], extraHTTPHeaders: { 'X-User-Id': `e2e-remote-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  const tv = await (await browser.newContext({ ...devices['Desktop Chrome'], viewport: { width: 1920, height: 1080 }, extraHTTPHeaders: { 'X-User-Id': `e2e-remote-${stamp}` } })).newPage()
  await tv.goto(`${sessionUrl}/table`)

  await expect(page.getByTestId('phone-pages').getByRole('button')).toHaveText(['Map', 'Creatures', 'Table', 'Tools', 'Party'])
  await place(page, '0,0', 'Scout', 'party', 'Aria')
  await place(page, '1,0', 'Grik', 'enemy')
  await page.getByTestId('page-tools').tap()
  await page.getByTestId('choose-combatants').tap()
  await page.getByTestId('begin-combat').tap()
  // A roll waits on every page; here it is answered from the Map.
  await page.getByTestId('page-map').tap()
  await enter(page, '20')
  await enter(player, '1')

  // Creatures: the console, with Grik in hand and its Suggested Action ready to use.
  await page.getByTestId('page-actions').tap()
  await expect(page.getByTestId('control-Grik')).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByTestId('creature-Grik').getByTestId('creature-suggestion')).toContainText('against Scout')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('creature-Grik').getByTestId('creature-use').tap()
  await expect(page.getByTestId('roll-card')).toContainText('attack against Scout')
  await enter(page, '1')
  await expect(page.getByTestId('roll-card')).toHaveCount(0)

  // Table: the camera, zoom, ping and blackout all reach the TV.
  await page.getByTestId('page-table').tap()
  await expect(page.getByTestId('table-remote')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  // Grik acts, so following the turn centres on its hex; showing the party centres on Scout's.
  const camera = tv.getByTestId('camera-world')
  await expect(camera).toBeVisible()
  const followed = (await camera.getAttribute('style')) ?? ''
  expect(followed).toContain('scale(1)')
  await page.getByTestId('camera-show_party').check()
  await expect(camera).not.toHaveAttribute('style', followed)
  await page.getByTestId('camera-follow_turn').check()
  await expect(camera).toHaveAttribute('style', followed)
  await page.getByTestId('camera-zoom').fill('200')
  await expect(camera).toHaveAttribute('style', /scale\(2\)/)
  await page.getByTestId('remote-ping').tap()
  await expect(page.getByTestId('page-map')).toHaveAttribute('aria-current', 'page')
  await page.locator('[data-hex="0,1"]').tap()
  await expect(tv.getByTestId('ping')).toBeVisible()
  await page.getByTestId('stop-pointing').tap()
  await page.getByTestId('page-table').tap()
  await page.getByTestId('blackout-toggle').tap()
  await expect(tv.getByTestId('blackout')).toBeVisible()
  await expect(page.getByTestId('blackout-toggle')).toHaveText('Lights back on')
  await page.getByTestId('blackout-toggle').tap()
  await expect(tv.getByTestId('blackout')).toHaveCount(0)
})
