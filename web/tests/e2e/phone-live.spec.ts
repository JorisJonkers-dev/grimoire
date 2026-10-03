import AxeBuilder from '@axe-core/playwright'
import { devices, expect, type CDPSession, type Page, test } from '@playwright/test'

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

type Finger = { x: number; y: number; id: number }
const touch = (cdp: CDPSession, type: 'touchStart' | 'touchMove' | 'touchEnd', touchPoints: Finger[]) => cdp.send('Input.dispatchTouchEvent', { type, touchPoints })

/** One finger dragged across the screen in a few steps, as a swipe is. */
async function swipe(cdp: CDPSession, from: { x: number; y: number }, dx: number) {
  await touch(cdp, 'touchStart', [{ ...from, id: 1 }])
  for (let i = 1; i <= 4; i++) await touch(cdp, 'touchMove', [{ x: from.x + (dx * i) / 4, y: from.y, id: 1 }])
  await touch(cdp, 'touchEnd', [])
}

test('on a phone a player swipes between pages over the map, with a sheet on an edge-to-edge bar, and pinches to zoom', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'phone', 'the phone shell is for phones')
  const stamp = String(Date.now())
  const dm = await (await browser.newContext({ ...devices['Desktop Chrome'], extraHTTPHeaders: { 'X-User-Id': `e2e-phone-dm-${stamp}` } })).newPage()
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Phone ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())
  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Aria')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText('Aria (you)')

  await dm.getByTestId('start-session').click()
  await expect(dm.getByTestId('connection')).toHaveText('Live')
  await page.goto(new URL(dm.url()).pathname)
  await dm.getByTestId('token-monster').fill('goblin-warrior')
  await dm.getByTestId('token-kind').selectOption('party')
  await dm.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await dm.getByTestId('token-label').fill('Scout')
  await dm.locator('[data-hex="0,0"]').click()
  await expect(dm.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Scout/)
  await dm.getByTestId('token-kind').selectOption('enemy')
  await dm.getByTestId('token-controller').selectOption('')
  await dm.getByTestId('token-label').fill('Grik')
  await dm.locator('[data-hex="2,0"]').click()
  await expect(dm.locator('[data-hex="2,0"]')).toHaveAttribute('aria-label', /Grik/)
  await dm.getByTestId('choose-combatants').click()
  await dm.getByTestId('begin-combat').click()
  await enter(dm, '5')
  // What needs an answer shows on every page: the player's roll is there on the Map page.
  await enter(page, '20')

  const width = page.viewportSize()?.width ?? 0
  const bar = page.getByTestId('phone-pages')
  const box = await bar.boundingBox()
  expect(box?.x).toBe(0)
  expect(Math.round(box?.width ?? 0)).toBe(width)
  await expect(bar.getByRole('button')).toHaveText(['Map', 'Actions', 'Spells', 'Character', 'Party'])
  await expect(page.getByTestId('page-map')).toHaveAttribute('aria-current', 'page')
  await expect(page.getByTestId('hotbar-Scout')).toBeHidden()
  await expect(page.getByTestId('resources')).toBeVisible()
  await expect(page.getByTestId('chip-action')).toHaveAttribute('aria-label', 'Action: available')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId('page-actions').tap()
  await expect(page.getByTestId('hotbar-Scout')).toBeVisible()
  await expect(page.getByTestId('spell-list')).toBeHidden()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // A swipe across the sheet turns to the next page, and one across the bar turns back.
  const cdp = await page.context().newCDPSession(page)
  const sheet = await page.getByTestId('dock').boundingBox()
  await swipe(cdp, { x: width - 40, y: (sheet?.y ?? 0) + 30 }, -(width - 120))
  await expect(page.getByTestId('page-spells')).toHaveAttribute('aria-current', 'page')
  await expect(page.getByTestId('spell-list')).toBeVisible()
  await expect(page.getByTestId('hotbar-Scout')).toBeHidden()
  await swipe(cdp, { x: 40, y: (box?.y ?? 0) + 20 }, width - 120)
  await expect(page.getByTestId('page-actions')).toHaveAttribute('aria-current', 'page')
  await page.getByTestId('page-spells').tap()
  await page.getByTestId('list-spell-burning-hands').tap()
  await expect(page.getByTestId('page-map')).toHaveAttribute('aria-current', 'page')
  await expect(page.getByTestId('area-aiming')).toBeVisible()
  for (const key of ['character', 'party']) {
    await page.getByTestId(`page-${key}`).tap()
    await expect(page.getByTestId(`page-${key}`)).toHaveAttribute('aria-current', 'page')
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  }
  await expect(page.getByTestId('tokens')).toBeVisible()
  await page.getByTestId('page-map').tap()

  // Two fingers spreading on the map zoom it; the buttons do the same.
  const zoomer = page.getByTestId('zoomer')
  await expect(zoomer).toHaveAttribute('style', /width: 100%/)
  const stage = await page.getByTestId('stage').boundingBox()
  const mid = { x: (stage?.x ?? 0) + (stage?.width ?? 0) / 2, y: (stage?.y ?? 0) + 120 }
  await touch(cdp, 'touchStart', [{ x: mid.x - 30, y: mid.y, id: 1 }, { x: mid.x + 30, y: mid.y, id: 2 }])
  for (const spread of [50, 70, 90]) await touch(cdp, 'touchMove', [{ x: mid.x - spread, y: mid.y, id: 1 }, { x: mid.x + spread, y: mid.y, id: 2 }])
  await touch(cdp, 'touchEnd', [])
  await expect(zoomer).toHaveAttribute('style', /width: 300%/)
  await page.getByTestId('zoom-out').tap()
  await expect(zoomer).toHaveAttribute('style', /width: 250%/)
  await page.getByTestId('zoom-reset').tap()
  await expect(zoomer).toHaveAttribute('style', /width: 100%/)

  // The turn's resources follow the turn.
  await page.getByTestId('page-actions').tap()
  await page.getByTestId('turn-Scout').getByTestId('spend-action').tap()
  await expect(page.getByTestId('chip-action')).toHaveAttribute('aria-label', 'Action: spent')
})
